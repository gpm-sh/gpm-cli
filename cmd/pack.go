package cmd

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/filtering"
	"gpm.sh/gpm/gpm-cli/internal/packaging"
	"gpm.sh/gpm/gpm-cli/internal/styling"
	"gpm.sh/gpm/gpm-cli/internal/validation"
)

var (
	packDryRun      bool
	packJSON        bool
	packDestination string
	packScope       string
)

var packCmd = &cobra.Command{
	Use:   "pack [package-spec...]",
	Short: "Create a package tarball",
	Long: `Create a tarball (.tgz) from packages for publishing, matching npm pack behavior.

Package Specs:
  Current directory (default)     # gpm pack
  Package folders                 # gpm pack ./my-package ./another-package  
  Existing tarballs              # gpm pack package.tgz another.tgz

Examples:
  gpm pack                       # Pack current directory
  gpm pack ./my-package          # Pack specific folder
  gpm pack package.tgz           # Repack existing tarball
  gpm pack --dry-run             # Show what would be packed
  gpm pack --json                # Output in JSON format
  gpm pack --pack-destination /tmp  # Output to specific directory
  gpm pack --scope=@myscope      # Pack with specific scope`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return packPackages(cmd, args)
	},
}

func init() {
	packCmd.Flags().BoolVar(&packDryRun, "dry-run", false, "Simulate pack without creating tarball")
	packCmd.Flags().BoolVar(&packJSON, "json", false, "Output results in JSON format")
	packCmd.Flags().StringVar(&packDestination, "pack-destination", "", "Specify output directory (default: current directory)")
	packCmd.Flags().StringVar(&packScope, "scope", "", "Scope for scoped packages (e.g., @myscope)")
}

type PackResult struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Filename     string   `json:"filename"`
	Files        []string `json:"files"`
	FileCount    int      `json:"fileCount"`
	UnpackedSize int64    `json:"unpackedSize"`
	PackedSize   int64    `json:"packedSize"`
	Sha1         string   `json:"sha1"`
	Sha512       string   `json:"sha512"`
	Integrity    string   `json:"integrity"`
}

type PackOutput struct {
	Results []PackResult `json:"results,omitempty"`
	Success bool         `json:"success"`
	Error   string       `json:"error,omitempty"`
}

func packPackages(cmd *cobra.Command, args []string) error {
	var packageSpecs []string
	if len(args) == 0 {
		packageSpecs = []string{"."}
	} else {
		packageSpecs = args
	}

	var results []PackResult
	var allErrors []string

	for _, spec := range packageSpecs {
		result, err := packSinglePackage(spec)
		if err != nil {
			allErrors = append(allErrors, fmt.Sprintf("%s: %v", spec, err))
			continue
		}
		results = append(results, *result)
	}

	if packJSON {
		output := PackOutput{
			Results: results,
			Success: len(allErrors) == 0,
		}
		if len(allErrors) > 0 {
			output.Error = strings.Join(allErrors, "; ")
		}

		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON output: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	for _, result := range results {
		printPackResult(result)
	}

	if len(allErrors) > 0 {
		for _, err := range allErrors {
			fmt.Printf("%s %s\n", styling.Error("✗"), err)
		}
		return fmt.Errorf("failed to pack %d package(s)", len(allErrors))
	}

	return nil
}

func packSinglePackage(packageSpec string) (*PackResult, error) {
	specType := packaging.DetectPackageSpecType(packageSpec)

	var sourceDir string
	var cleanup func()

	switch specType {
	case "tarball":
		return repackTarball(packageSpec)
	case "folder":
		sourceDir = packageSpec
	case "folder_no_package_json":
		return nil, fmt.Errorf("no package.json found in %s", packageSpec)
	default:
		return nil, fmt.Errorf("invalid package spec: %s", packageSpec)
	}

	// Handle scope configuration for pack
	if packScope != "" {
		if !strings.HasPrefix(packScope, "@") {
			return nil, fmt.Errorf("scope must start with @ (e.g., @myscope)")
		}
	}

	validationResult, err := validation.ValidatePackage(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	if !validationResult.Valid {
		for _, validationErr := range validationResult.Errors {
			if !packJSON {
				fmt.Printf("%s %v\n", styling.Warning("⚠"), validationErr)
			}
		}
		return nil, fmt.Errorf("package validation failed")
	}

	if !packJSON && len(validationResult.Warnings) > 0 {
		for _, warning := range validationResult.Warnings {
			fmt.Printf("%s %s\n", styling.Warning("⚠"), warning)
		}
	}

	filterEngine, err := filtering.NewFileFilterEngine(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create file filter: %w", err)
	}

	filterResult, err := filterEngine.FilterFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to filter files: %w", err)
	}

	if packDryRun {
		return createDryRunResult(validationResult.Package, filterResult)
	}

	return createPackage(sourceDir, validationResult.Package, filterResult, cleanup)
}

func createDryRunResult(pkg *validation.PackageJSON, filterResult *filtering.FilterResult) (*PackResult, error) {
	result := &PackResult{
		Name:         pkg.Name,
		Version:      pkg.Version,
		Filename:     fmt.Sprintf("%s-%s.tgz", pkg.Name, pkg.Version),
		FileCount:    filterResult.FileCount,
		UnpackedSize: filterResult.TotalSize,
	}

	for _, file := range filterResult.Files {
		if !file.IsDir {
			result.Files = append(result.Files, file.RelativePath)
		}
	}

	if !packJSON {
		fmt.Println(styling.Header("🧪 Dry Run - Would Pack"))
		fmt.Println(styling.Separator())
		fmt.Printf("%s %s@%s\n", styling.Label("Package:"), styling.Package(pkg.Name), styling.Version(pkg.Version))
		fmt.Printf("%s %s\n", styling.Label("Output:"), styling.File(result.Filename))
		fmt.Printf("%s %s\n", styling.Label("Files:"), styling.Value(fmt.Sprintf("%d", result.FileCount)))
		fmt.Printf("%s %s\n", styling.Label("Unpacked Size:"), styling.Size(fmt.Sprintf("%.1f kB", float64(result.UnpackedSize)/1024)))
		fmt.Println(styling.Separator())

		if len(result.Files) > 0 {
			fmt.Println(styling.Info("📋 Files to be included:"))
			for _, file := range result.Files {
				fmt.Printf("  %s\n", file)
			}
		}
	}

	return result, nil
}

func createPackage(sourceDir string, pkg *validation.PackageJSON, filterResult *filtering.FilterResult, cleanup func()) (*PackResult, error) {
	if cleanup != nil {
		defer cleanup()
	}

	outputDir := packDestination
	if outputDir == "" {
		outputDir = "."
	}

	if !packaging.IsValidPackageNameForFilename(pkg.Name) || !packaging.IsValidVersionForFilename(pkg.Version) {
		return nil, fmt.Errorf("invalid package name or version for filename")
	}

	outputFile := fmt.Sprintf("%s-%s.tgz", pkg.Name, pkg.Version)
	outputPath := filepath.Join(outputDir, outputFile)
	cleanOutputPath := filepath.Clean(outputPath)

	file, err := os.Create(cleanOutputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() { _ = file.Close() }()

	gw := gzip.NewWriter(file)
	defer func() { _ = gw.Close() }()

	tw := tar.NewWriter(gw)
	defer func() { _ = tw.Close() }()

	sha1Hash := sha1.New()
	sha512Hash := sha512.New()

	var filePaths []string

	for _, filteredFile := range filterResult.Files {
		if filteredFile.IsDir {
			continue
		}

		filePaths = append(filePaths, filteredFile.RelativePath)

		relativePath := strings.ReplaceAll(filteredFile.RelativePath, "\\", "/")

		info, err := os.Stat(filteredFile.AbsolutePath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat file %s: %w", filteredFile.RelativePath, err)
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return nil, fmt.Errorf("failed to create tar header: %w", err)
		}

		header.Name = fmt.Sprintf("package/%s", relativePath)
		if err := tw.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("failed to write tar header: %w", err)
		}

		fileData, err := os.ReadFile(filteredFile.AbsolutePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filteredFile.RelativePath, err)
		}

		if _, err := tw.Write(fileData); err != nil {
			return nil, fmt.Errorf("failed to write file data: %w", err)
		}

		sha1Hash.Write(fileData)
		sha512Hash.Write(fileData)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	sha1Bytes := sha1Hash.Sum(nil)
	sha512Bytes := sha512Hash.Sum(nil)
	integrity := fmt.Sprintf("sha512-%s", base64.StdEncoding.EncodeToString(sha512Bytes))

	result := &PackResult{
		Name:         pkg.Name,
		Version:      pkg.Version,
		Filename:     outputFile,
		Files:        filePaths,
		FileCount:    filterResult.FileCount,
		UnpackedSize: filterResult.TotalSize,
		PackedSize:   fileInfo.Size(),
		Sha1:         hex.EncodeToString(sha1Bytes),
		Sha512:       hex.EncodeToString(sha512Bytes),
		Integrity:    integrity,
	}

	if !packJSON {
		fmt.Println(outputFile)
	}

	return result, nil
}

func printPackResult(result PackResult) {
	fmt.Println(styling.Header("📦  GPM Package Created Successfully"))
	fmt.Println(styling.Separator())
	fmt.Printf("%s %s@%s\n", styling.Label("Package:"), styling.Package(result.Name), styling.Version(result.Version))
	fmt.Printf("%s %s\n", styling.Label("Output:"), styling.File(result.Filename))
	fmt.Printf("%s %s (compressed) / %s (unpacked)\n", styling.Label("Size:"),
		styling.Size(fmt.Sprintf("%.1f kB", float64(result.PackedSize)/1024)),
		styling.Size(fmt.Sprintf("%.1f kB", float64(result.UnpackedSize)/1024)))
	fmt.Printf("%s %s\n", styling.Label("Files:"), styling.Value(fmt.Sprintf("%d", result.FileCount)))
	if len(result.Sha1) >= 20 {
		fmt.Printf("%s %s\n", styling.Label("SHA1:"), styling.Hash(result.Sha1[:20]))
	} else {
		fmt.Printf("%s %s\n", styling.Label("SHA1:"), styling.Hash(result.Sha1))
	}
	fmt.Printf("%s %s\n", styling.Label("Integrity:"), styling.Hash(result.Integrity))
	fmt.Println(styling.Separator())
	fmt.Printf("Ready to publish with: %s\n", styling.Command(fmt.Sprintf("gpm publish %s", result.Filename)))
}

func repackTarball(tarballPath string) (*PackResult, error) {
	packageInfo, err := packaging.ExtractPackageInfo(tarballPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract package info: %w", err)
	}

	info, err := os.Stat(tarballPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat tarball: %w", err)
	}

	result := &PackResult{
		Name:       packageInfo.Name,
		Version:    packageInfo.Version,
		Filename:   filepath.Base(tarballPath),
		PackedSize: info.Size(),
	}

	if !packJSON {
		fmt.Printf("%s\n", result.Filename)
	}

	return result, nil
}
