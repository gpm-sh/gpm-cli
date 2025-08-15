package cmd

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/api"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/filtering"
	"gpm.sh/gpm/gpm-cli/internal/packaging"
	"gpm.sh/gpm/gpm-cli/internal/styling"
	"gpm.sh/gpm/gpm-cli/internal/validation"
)

var (
	publishAccess   string
	publishTag      string
	publishDryRun   bool
	publishRegistry string
	publishScope    string
)

var publishCmd = &cobra.Command{
	Use:   "publish [package-spec]",
	Short: "Publish a package to GPM registry",
	Long: `Publish a package to the GPM registry.

Publishes a package to the registry so that it can be installed by name.
If no package-spec is provided, publishes the package in the current directory.

Package Specs:
  a) Current directory (default)          # gpm publish
  b) Folder containing package.json       # gpm publish ./my-package  
  c) Gzipped tarball (.tgz/.tar.gz)      # gpm publish package.tgz

Access Levels:

  public      Visible and downloadable from any domain without authentication
  scoped      Visible only on the current studio domain without authentication
  private     Visible only on the current studio domain and requires authentication

Examples:
  gpm publish                             # Publish current directory
  gpm publish ./my-package                # Publish specific folder
  gpm publish package.tgz                 # Publish tarball
  gpm publish --access=scoped             # Publish as scoped
  gpm publish --access=private            # Publish as private
  gpm publish --tag=beta                  # Publish with dist-tag
  gpm publish --scope=@myscope            # Publish with specific scope
  gpm publish --registry=https://npmjs.org # Publish to specific registry
  gpm publish --dry-run                   # Simulate publish`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var packageSpec string
		if len(args) == 0 {
			packageSpec = "."
		} else {
			packageSpec = args[0]
		}
		return publish(packageSpec)
	},
}

func init() {
	publishCmd.Flags().StringVar(&publishAccess, "access", "", "Package access level (public, scoped, private) - auto-detected if not specified")
	publishCmd.Flags().StringVar(&publishTag, "tag", "latest", "Dist-tag to publish under")
	publishCmd.Flags().BoolVar(&publishDryRun, "dry-run", false, "Simulate publish without uploading")
	publishCmd.Flags().StringVar(&publishRegistry, "registry", "", "Registry URL to publish to (overrides config)")
	publishCmd.Flags().StringVar(&publishScope, "scope", "", "Scope for scoped packages (e.g., @myscope)")
}

type PublishInfo struct {
	PackageInfo   *validation.PackageJSON
	TarballPath   string
	FileSize      int64
	Sha1          string
	Sha512        string
	Integrity     string
	FilteredFiles []string
}

func publish(packageSpec string) error {
	cfg := config.GetConfig()
	if cfg.Token == "" {
		return fmt.Errorf("not authenticated. Run 'gpm login'")
	}

	registry := cfg.Registry
	if publishRegistry != "" {
		registry = publishRegistry
	}

	// Validate registry URL format
	if registry != "" {
		if !strings.HasPrefix(registry, "http://") && !strings.HasPrefix(registry, "https://") {
			return fmt.Errorf("invalid registry URL: %s (must start with http:// or https://)", registry)
		}
	}

	publishInfo, cleanup, err := prepareEnhancedPackageForPublish(packageSpec)
	if err != nil {
		return err
	}
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	if err := validateDistTag(publishTag); err != nil {
		return fmt.Errorf("invalid dist-tag: %w", err)
	}

	// Handle scope configuration
	packageName := publishInfo.PackageInfo.Name
	if publishScope != "" {
		if !strings.HasPrefix(publishScope, "@") {
			return fmt.Errorf("scope must start with @ (e.g., @myscope)")
		}
		if !strings.Contains(packageName, "/") {
			packageName = publishScope + "/" + strings.TrimPrefix(packageName, "@")
		} else if !strings.HasPrefix(packageName, publishScope) {
			return fmt.Errorf("package name %s doesn't match specified scope %s", packageName, publishScope)
		}
	}

	actualAccess := publishAccess
	if actualAccess == "" {
		actualAccess = string(determineRecommendedAccess(packageName))
	}

	if err := validateAccessLevel(actualAccess, packageName); err != nil {
		return fmt.Errorf("access level validation failed: %w", err)
	}

	client := api.NewClient(registry, cfg.Token)

	if err := performPrePublishChecks(client, publishInfo.PackageInfo, actualAccess); err != nil {
		return fmt.Errorf("pre-publish validation failed: %w", err)
	}

	headerText := "📤 Publishing Package"
	if publishDryRun {
		headerText = "🧪 Dry Run - Simulating Publish"
	}

	fmt.Println(styling.Header(headerText))
	fmt.Println(styling.Separator())
	fmt.Printf("%s %s\n", styling.Label("Package:"), styling.Package(packageName))
	fmt.Printf("%s %s\n", styling.Label("Version:"), styling.Version(publishInfo.PackageInfo.Version))
	fmt.Printf("%s %s\n", styling.Label("Access Level:"), styling.Value(getAccessDescription(actualAccess)))
	fmt.Printf("%s %s\n", styling.Label("Tag:"), styling.Value(publishTag))
	fmt.Printf("%s %s\n", styling.Label("Registry:"), styling.URL(registry))
	if publishScope != "" {
		fmt.Printf("%s %s\n", styling.Label("Scope:"), styling.Value(publishScope))
	}
	fmt.Printf("%s %s\n", styling.Label("File:"), styling.File(publishInfo.TarballPath))
	fmt.Printf("%s %d bytes (%.1f kB)\n", styling.Label("Size:"), publishInfo.FileSize, float64(publishInfo.FileSize)/1024)
	fmt.Printf("%s %s files\n", styling.Label("Files:"), styling.Value(fmt.Sprintf("%d", len(publishInfo.FilteredFiles))))
	fmt.Printf("%s %s\n", styling.Label("SHA1:"), styling.Hash(publishInfo.Sha1[:20]))
	fmt.Printf("%s %s\n", styling.Label("Integrity:"), styling.Hash(publishInfo.Integrity))
	if publishDryRun {
		fmt.Printf("%s %s\n", styling.Label("Mode:"), styling.Warning("DRY RUN"))
	}
	fmt.Println(styling.Separator())

	if publishDryRun {
		fmt.Println(styling.Success("✓ Dry run completed successfully!"))
		fmt.Println(styling.Info("📋 What would be published:"))
		fmt.Printf("  %s %s@%s\n", styling.Label("•"), styling.Package(packageName), styling.Version(publishInfo.PackageInfo.Version))
		fmt.Printf("  %s %s\n", styling.Label("•"), styling.Value(fmt.Sprintf("Tagged as '%s'", publishTag)))
		fmt.Printf("  %s %s\n", styling.Label("•"), styling.Value(fmt.Sprintf("Access level: %s", getAccessDescription(actualAccess))))
		fmt.Printf("  %s %s\n", styling.Label("•"), styling.Value(fmt.Sprintf("Registry: %s", registry)))
		if publishScope != "" {
			fmt.Printf("  %s %s\n", styling.Label("•"), styling.Value(fmt.Sprintf("Scope: %s", publishScope)))
		}
		fmt.Printf("  %s %d files\n", styling.Label("•"), len(publishInfo.FilteredFiles))

		if len(publishInfo.FilteredFiles) > 0 && len(publishInfo.FilteredFiles) <= 20 {
			fmt.Println(styling.Info("📁 Files to be published:"))
			for _, file := range publishInfo.FilteredFiles {
				fmt.Printf("    %s\n", file)
			}
		} else if len(publishInfo.FilteredFiles) > 20 {
			fmt.Printf("    %s (showing first 20 of %d files)\n", styling.Info("📁 Files to be published"), len(publishInfo.FilteredFiles))
			for i, file := range publishInfo.FilteredFiles[:20] {
				fmt.Printf("    %s\n", file)
				if i == 19 {
					fmt.Printf("    ... and %d more\n", len(publishInfo.FilteredFiles)-20)
				}
			}
		}

		fmt.Println(styling.Hint("Use 'gpm publish' without --dry-run to actually publish"))
		return nil
	}

	req := &api.PublishRequest{
		Name:    packageName,
		Version: publishInfo.PackageInfo.Version,
		Access:  actualAccess,
		Tag:     publishTag,
	}

	resp, err := client.Publish(req, publishInfo.TarballPath)
	if err != nil {
		return fmt.Errorf("publish failed: %v", err)
	}

	if resp.Success {
		fmt.Println(styling.Success("✓ Package published successfully!"))
		fmt.Printf("%s %s\n", styling.Label("Package ID:"), styling.Value(resp.Data.PackageID))
		fmt.Printf("%s %s\n", styling.Label("Version ID:"), styling.Value(resp.Data.VersionID))
		fmt.Printf("%s %s\n", styling.Label("Download URL:"), styling.URL(resp.Data.DownloadURL))
		fmt.Printf("%s %s\n", styling.Label("File Size:"), styling.Size(fmt.Sprintf("%d bytes", resp.Data.FileSize)))
		fmt.Printf("%s %s\n", styling.Label("Upload Time:"), styling.Value(resp.Data.UploadTime))
		fmt.Printf("%s %s\n", styling.Label("Integrity:"), styling.Hash(publishInfo.Integrity))
	} else {
		if resp.Error != nil {
			return fmt.Errorf("publish failed: %s - %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("publish failed with unknown error")
	}

	return nil
}

func prepareEnhancedPackageForPublish(packageSpec string) (*PublishInfo, func(), error) {
	specType := packaging.DetectPackageSpecType(packageSpec)

	switch specType {
	case "tarball":
		return prepareExistingTarball(packageSpec)
	case "folder":
		return prepareFolderWithFiltering(packageSpec)
	case "folder_no_package_json":
		if packageSpec == "." {
			return nil, nil, fmt.Errorf("no package.json found in the current directory. Run this inside a package, or pass a tarball/folder")
		} else {
			return nil, nil, fmt.Errorf("no package.json found in %s. Run 'gpm publish .' to publish the current folder, or pass a tarball", packageSpec)
		}
	default:
		if packageSpec == "." {
			return nil, nil, fmt.Errorf("no package.json found in the current directory. Run this inside a package, or pass a tarball/folder")
		}
		return nil, nil, fmt.Errorf("ENOENT: no such file or directory, open '%s'. Run 'gpm publish .' to publish the current folder, or pass a tarball", packageSpec)
	}
}

func prepareExistingTarball(tarballPath string) (*PublishInfo, func(), error) {
	if _, err := os.Stat(tarballPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("ENOENT: no such file or directory, open '%s'", tarballPath)
	}

	packageInfo, err := packaging.ExtractPackageInfo(tarballPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to extract package info: %w", err)
	}

	info, err := os.Stat(tarballPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat tarball: %w", err)
	}

	sha1Hash, sha512Hash, err := calculateTarballHashes(tarballPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to calculate checksums: %w", err)
	}

	integrity := fmt.Sprintf("sha512-%s", base64.StdEncoding.EncodeToString(sha512Hash))

	publishInfo := &PublishInfo{
		PackageInfo: &validation.PackageJSON{
			Name:    packageInfo.Name,
			Version: packageInfo.Version,
		},
		TarballPath: tarballPath,
		FileSize:    info.Size(),
		Sha1:        hex.EncodeToString(sha1Hash),
		Sha512:      hex.EncodeToString(sha512Hash),
		Integrity:   integrity,
	}

	return publishInfo, nil, nil
}

func prepareFolderWithFiltering(folderPath string) (*PublishInfo, func(), error) {
	validationResult, err := validation.ValidatePackage(folderPath)
	if err != nil {
		return nil, nil, fmt.Errorf("validation failed: %w", err)
	}

	if !validationResult.Valid {
		for _, validationErr := range validationResult.Errors {
			fmt.Printf("%s %v\n", styling.Warning("⚠"), validationErr)
		}
		return nil, nil, fmt.Errorf("package validation failed")
	}

	filterEngine, err := filtering.NewFileFilterEngine(folderPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create file filter: %w", err)
	}

	filterResult, err := filterEngine.FilterFiles()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to filter files: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "gpm-publish-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	tarballName := fmt.Sprintf("%s-%s.tgz", validationResult.Package.Name, validationResult.Package.Version)
	tarballPath := filepath.Join(tempDir, tarballName)

	sha1Hash, sha512Hash, filteredFiles, err := createFilteredTarball(folderPath, tarballPath, filterResult)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to create tarball: %w", err)
	}

	info, err := os.Stat(tarballPath)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to stat created tarball: %w", err)
	}

	integrity := fmt.Sprintf("sha512-%s", base64.StdEncoding.EncodeToString(sha512Hash))

	publishInfo := &PublishInfo{
		PackageInfo:   validationResult.Package,
		TarballPath:   tarballPath,
		FileSize:      info.Size(),
		Sha1:          hex.EncodeToString(sha1Hash),
		Sha512:        hex.EncodeToString(sha512Hash),
		Integrity:     integrity,
		FilteredFiles: filteredFiles,
	}

	return publishInfo, cleanup, nil
}

func createFilteredTarball(srcDir, tarballPath string, filterResult *filtering.FilterResult) ([]byte, []byte, []string, error) {
	file, err := os.Create(tarballPath)
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { _ = file.Close() }()

	gzWriter := gzip.NewWriter(file)
	defer func() { _ = gzWriter.Close() }()

	tarWriter := tar.NewWriter(gzWriter)
	defer func() { _ = tarWriter.Close() }()

	sha1Hash := sha1.New()
	sha512Hash := sha512.New()
	var filteredFiles []string

	for _, filteredFile := range filterResult.Files {
		if filteredFile.IsDir {
			continue
		}

		filteredFiles = append(filteredFiles, filteredFile.RelativePath)
		relativePath := strings.ReplaceAll(filteredFile.RelativePath, "\\", "/")

		info, err := os.Stat(filteredFile.AbsolutePath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to stat file %s: %w", filteredFile.RelativePath, err)
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to create tar header: %w", err)
		}

		header.Name = fmt.Sprintf("package/%s", relativePath)
		if err := tarWriter.WriteHeader(header); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to write tar header: %w", err)
		}

		fileData, err := os.ReadFile(filteredFile.AbsolutePath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to read file %s: %w", filteredFile.RelativePath, err)
		}

		if _, err := tarWriter.Write(fileData); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to write file data: %w", err)
		}

		sha1Hash.Write(fileData)
		sha512Hash.Write(fileData)
	}

	return sha1Hash.Sum(nil), sha512Hash.Sum(nil), filteredFiles, nil
}

func calculateTarballHashes(tarballPath string) ([]byte, []byte, error) {
	file, err := os.Open(tarballPath)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = file.Close() }()

	sha1Hash := sha1.New()
	sha512Hash := sha512.New()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		if header.Typeflag == tar.TypeReg {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, nil, err
			}
			sha1Hash.Write(data)
			sha512Hash.Write(data)
		}
	}

	return sha1Hash.Sum(nil), sha512Hash.Sum(nil), nil
}

func performPrePublishChecks(client *api.Client, pkg *validation.PackageJSON, access string) error {
	return nil
}

func validateDistTag(tag string) error {
	return validation.ValidateDistTag(tag)
}

func validateAccessLevel(access, packageName string) error {
	return validation.ValidateAccessLevel(access, packageName)
}

func determineRecommendedAccess(name string) validation.AccessLevel {
	result, _ := validation.ValidatePackage(".")
	if result != nil {
		return result.RecommendedAccess
	}
	return validation.AccessPublic
}

func getAccessDescription(access string) string {
	switch access {
	case "public":
		return "Public"
	case "scoped":
		return "Scoped"
	case "private":
		return "Private"
	default:
		return "Public"
	}
}
