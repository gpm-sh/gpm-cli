package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/api"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

// validatePath ensures the path is safe and doesn't escape the destination directory
func validatePathPublish(filePath, destDir string) error {
	// Clean the path to resolve any . or .. elements
	cleanPath := filepath.Clean(filePath)

	// Convert to absolute path
	absPath, err := filepath.Abs(filepath.Join(destDir, cleanPath))
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Ensure the absolute path is within the destination directory
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("failed to resolve destination directory: %w", err)
	}

	if !strings.HasPrefix(absPath, destAbs) {
		return fmt.Errorf("path traversal attempt detected: %s", filePath)
	}

	return nil
}

// validateCommand sanitizes git command arguments
func validateGitCommandPublish(args ...string) error {
	for _, arg := range args {
		// Reject arguments that could be dangerous
		if strings.Contains(arg, ";") || strings.Contains(arg, "&") ||
			strings.Contains(arg, "|") || strings.Contains(arg, "`") ||
			strings.Contains(arg, "$") {
			return fmt.Errorf("potentially dangerous command argument: %s", arg)
		}
	}
	return nil
}

var (
	publishAccess string
	publishTag    string
	publishDryRun bool
)

var publishCmd = &cobra.Command{
	Use:   "publish [package-spec]",
	Short: "Publish a package to GPM registry",
	Long: `Publish a package to the GPM registry.

Publishes a package to the registry so that it can be installed by name.
If no package-spec is provided, publishes the package in the current directory.

Package Specs (NPM Compatible):
  a) Current directory (default)          # gpm publish
  b) Folder containing package.json       # gpm publish ./my-package  
  c) Gzipped tarball (.tgz/.tar.gz)      # gpm publish package.tgz
  d) Git remote URL                       # gpm publish git+https://github.com/user/repo.git

Access Levels:

  public      Visible and downloadable from any domain without authentication
  scoped      Visible only on the current studio domain without authentication
  private     Visible only on the current studio domain and requires authentication

Examples:
  gpm publish                                          # Publish current directory
  gpm publish ./my-package                             # Publish specific folder
  gpm publish package.tgz                              # Publish tarball
  gpm publish git+https://github.com/user/repo.git    # Publish from Git
  gpm publish --access=scoped                         # Publish as scoped
  gpm publish --access=private                        # Publish as private
  gpm publish --tag=beta                              # Publish with dist-tag
  gpm publish --dry-run                               # Simulate publish`,
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
	publishCmd.Flags().StringVar(&publishAccess, "access", "public", "Package access level (public, scoped, private)")
	publishCmd.Flags().StringVar(&publishTag, "tag", "latest", "Dist-tag to publish under")
	publishCmd.Flags().BoolVar(&publishDryRun, "dry-run", false, "Simulate publish without uploading")
}

func publish(packageSpec string) error {
	// Validate access level
	validAccess := publishAccess == "public" || publishAccess == "scoped" || publishAccess == "private"
	if !validAccess {
		return fmt.Errorf("invalid access level '%s'. Must be one of: public, scoped, private", publishAccess)
	}

	cfg := config.GetConfig()
	if cfg.Token == "" {
		return fmt.Errorf("not authenticated. Run 'gpm login'")
	}

	// Determine package spec type and prepare tarball
	tarballPath, cleanup, err := preparePackageForPublish(packageSpec)
	if err != nil {
		return err // Error messages are already user-friendly from preparePackageForPublish
	}
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	packageInfo, err := extractPackageInfo(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to read package information: %w", err)
	}

	accessDescription := getAccessDescription(publishAccess)

	client := api.NewClient(cfg.Registry, cfg.Token)

	headerText := "📤 Publishing Package"
	if publishDryRun {
		headerText = "🧪 Dry Run - Simulating Publish"
	}

	fmt.Println(styling.Header(headerText))
	fmt.Println(styling.Separator())
	fmt.Printf("%s %s\n", styling.Label("Package:"), styling.Package(packageInfo.Name))
	fmt.Printf("%s %s\n", styling.Label("Version:"), styling.Version(packageInfo.Version))
	fmt.Printf("%s %s\n", styling.Label("Access Level:"), styling.Value(accessDescription))
	fmt.Printf("%s %s\n", styling.Label("Tag:"), styling.Value(publishTag))
	fmt.Printf("%s %s\n", styling.Label("File:"), styling.File(tarballPath))
	if publishDryRun {
		fmt.Printf("%s %s\n", styling.Label("Mode:"), styling.Warning("DRY RUN"))
	}
	fmt.Println(styling.Separator())

	if publishDryRun {
		fmt.Println(styling.Success("✓ Dry run completed successfully!"))
		fmt.Println(styling.Info("📋 What would be published:"))
		fmt.Printf("  %s %s@%s\n", styling.Label("•"), styling.Package(packageInfo.Name), styling.Version(packageInfo.Version))
		fmt.Printf("  %s %s\n", styling.Label("•"), styling.Value(fmt.Sprintf("Tagged as '%s'", publishTag)))
		fmt.Printf("  %s %s\n", styling.Label("•"), styling.Value(fmt.Sprintf("Access level: %s", accessDescription)))
		fmt.Println(styling.Hint("Use 'gpm publish' without --dry-run to actually publish"))
		return nil
	}

	req := &api.PublishRequest{
		Name:    packageInfo.Name,
		Version: packageInfo.Version,
		Access:  publishAccess,
		Tag:     publishTag,
	}

	resp, err := client.Publish(req, tarballPath)
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
	} else {
		if resp.Error != nil {
			return fmt.Errorf("publish failed: %s - %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("publish failed with unknown error")
	}

	return nil
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

func extractPackageInfo(tarballPath string) (*PackageInfo, error) {
	// Security: Validate the tarball path
	cleanPath := filepath.Clean(tarballPath)
	if !strings.HasSuffix(cleanPath, ".tgz") && !strings.HasSuffix(cleanPath, ".tar.gz") {
		return nil, fmt.Errorf("invalid file type: only .tgz and .tar.gz files are allowed")
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if header.Name == "package/package.json" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}

			var packageInfo PackageInfo
			if err := json.Unmarshal(data, &packageInfo); err != nil {
				return nil, err
			}

			return &packageInfo, nil
		}
	}

	return nil, fmt.Errorf("package.json not found in tarball")
}

func preparePackageForPublish(packageSpec string) (tarballPath string, cleanup func(), err error) {
	specType := detectPackageSpecType(packageSpec)

	switch specType {
	case "tarball":
		// Validate tarball exists
		if _, err := os.Stat(packageSpec); os.IsNotExist(err) {
			return "", nil, fmt.Errorf("ENOENT: no such file or directory, open '%s'", packageSpec)
		}
		return packageSpec, nil, nil

	case "folder":
		// Pack folder into temporary tarball
		return packFolderToTarball(packageSpec)

	case "folder_no_package_json":
		// Handle missing package.json case with npm-like error
		if packageSpec == "." {
			return "", nil, fmt.Errorf("no package.json found in the current directory. Run this inside a package, or pass a tarball/folder/git URL")
		} else {
			return "", nil, fmt.Errorf("no package.json found in %s. Run 'gpm publish .' to publish the current folder, or pass a tarball/git URL", packageSpec)
		}

	case "git":
		// Clone git repo and pack into temporary tarball
		return packGitRepoToTarball(packageSpec)

	default:
		if packageSpec == "." {
			return "", nil, fmt.Errorf("no package.json found in the current directory. Run this inside a package, or pass a tarball/folder/git URL")
		}
		return "", nil, fmt.Errorf("ENOENT: no such file or directory, open '%s'. Run 'gpm publish .' to publish the current folder, or pass a tarball/git URL", packageSpec)
	}
}

func detectPackageSpecType(packageSpec string) string {
	// Check if it's a Git URL
	if isGitURL(packageSpec) {
		return "git"
	}

	// Check if it's a tarball file
	if strings.HasSuffix(packageSpec, ".tgz") || strings.HasSuffix(packageSpec, ".tar.gz") {
		return "tarball"
	}

	// Check if it's a folder with package.json
	if stat, err := os.Stat(packageSpec); err == nil && stat.IsDir() {
		packageJSONPath := filepath.Join(packageSpec, "package.json")
		if _, err := os.Stat(packageJSONPath); err == nil {
			return "folder"
		}
		// Directory exists but no package.json
		return "folder_no_package_json"
	}

	return "unknown"
}

func isGitURL(spec string) bool {
	gitPatterns := []string{
		`^git\+https?://`,
		`^git\+ssh://`,
		`^git://`,
		`^https://.*\.git($|#)`,
		`^ssh://.*\.git($|#)`,
	}

	for _, pattern := range gitPatterns {
		if matched, _ := regexp.MatchString(pattern, spec); matched {
			return true
		}
	}

	return false
}

func packFolderToTarball(folderPath string) (string, func(), error) {
	// Create temporary tarball
	tempDir, err := os.MkdirTemp("", "gpm-publish-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempDir) // Best effort cleanup
	}

	// Read package.json to get name and version
	packageJSONPath := filepath.Join(folderPath, "package.json")
	if err := validatePathPublish(packageJSONPath, folderPath); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("invalid path: %w", err)
	}
	data, err := os.ReadFile(packageJSONPath) // #nosec G304 - Path validated above
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal(data, &pkg); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to parse package.json: %w", err)
	}

	name, ok := pkg["name"].(string)
	if !ok || name == "" {
		cleanup()
		return "", nil, fmt.Errorf("missing required field 'name' in package.json")
	}

	version, ok := pkg["version"].(string)
	if !ok || version == "" {
		cleanup()
		return "", nil, fmt.Errorf("missing required field 'version' in package.json")
	}

	tarballName := fmt.Sprintf("%s-%s.tgz", name, version)
	tarballPath := filepath.Join(tempDir, tarballName)

	// Create tarball using gpm pack logic
	if err := createTarballFromFolder(folderPath, tarballPath); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to create tarball: %w", err)
	}

	return tarballPath, cleanup, nil
}

func packGitRepoToTarball(gitURL string) (string, func(), error) {
	// Create temporary directory for cloning
	tempDir, err := os.MkdirTemp("", "gpm-git-publish-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempDir) // Best effort cleanup
	}

	cloneDir := filepath.Join(tempDir, "repo")

	// Parse Git URL to extract branch/tag if specified
	gitURL, branch := parseGitURL(gitURL)

	// Clone repository with input validation
	var cmd *exec.Cmd
	if branch != "" {
		if err := validateGitCommandPublish("clone", "--branch", branch, "--depth", "1", gitURL, cloneDir); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("invalid git command arguments: %w", err)
		}
		cmd = exec.Command("git", "clone", "--branch", branch, "--depth", "1", gitURL, cloneDir) // #nosec G204 - Git command validated above
	} else {
		if err := validateGitCommandPublish("clone", "--depth", "1", gitURL, cloneDir); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("invalid git command arguments: %w", err)
		}
		cmd = exec.Command("git", "clone", "--depth", "1", gitURL, cloneDir) // #nosec G204 - Git command validated above
	}

	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Remove .git directory
	gitDir := filepath.Join(cloneDir, ".git")
	_ = os.RemoveAll(gitDir) // Best effort cleanup

	// Now pack the cloned folder
	tarballPath, _, err := packFolderToTarball(cloneDir)
	if err != nil {
		cleanup()
		return "", nil, err
	}

	return tarballPath, cleanup, nil
}

func parseGitURL(gitURL string) (cleanURL, branch string) {
	// Handle fragment (branch/tag) in Git URL: git+https://github.com/user/repo.git#branch
	if idx := strings.Index(gitURL, "#"); idx != -1 {
		branch = gitURL[idx+1:]
		gitURL = gitURL[:idx]
	}

	// Remove git+ prefix
	gitURL = strings.TrimPrefix(gitURL, "git+")

	return gitURL, branch
}

func createTarballFromFolder(srcDir, tarballPath string) error {
	// Validate tarball path
	if err := validatePathPublish(tarballPath, "."); err != nil {
		return fmt.Errorf("invalid tarball path: %w", err)
	}
	// Create tarball file
	file, err := os.Create(tarballPath) // #nosec G304 - Path validated above
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// Create gzip writer
	gzWriter := gzip.NewWriter(file)
	defer func() { _ = gzWriter.Close() }()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer func() { _ = tarWriter.Close() }()

	// Walk directory and add files to tar
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from source directory
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Add "package/" prefix (npm standard)
		tarPath := filepath.Join("package", relPath)

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = tarPath

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// Write file content if it's a regular file
		if info.Mode().IsRegular() {
			// Validate path to prevent access to files outside the intended directory
			if err := validatePathPublish(relPath, srcDir); err != nil {
				return fmt.Errorf("security validation failed: %w", err)
			}

			srcFile, err := os.Open(path) // #nosec G304 - Path validated by filepath.Walk
			if err != nil {
				return err
			}
			defer func() {
				_ = srcFile.Close() // Handle error from defer
			}()

			_, err = io.Copy(tarWriter, srcFile)
			return err
		}

		return nil
	})
}
