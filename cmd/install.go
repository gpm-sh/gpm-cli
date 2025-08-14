package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

// validatePath ensures the path is safe and doesn't escape the destination directory
func validatePath(filePath, destDir string) error {
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

// validateGitCommand sanitizes git command arguments
func validateGitCommand(args ...string) error {
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
	installGlobal  bool
	installVersion string
	installSave    bool
	installSaveDev bool
)

var installCmd = &cobra.Command{
	Use:   "install [package[@version]...]",
	Short: "Install packages",
	Long: `Install packages from the registry.

Examples:
  gpm install                              # Install from package.json
  gpm install package-name                 # Install package
  gpm install package-name@1.0.0           # Install specific version
  gpm install package-name@^1.0.0          # Install with version range
  gpm install pkg1 pkg2 pkg3               # Install multiple packages
  gpm install --save package-name          # Save to dependencies
  gpm install --save-dev test-utils        # Save to devDependencies
  
Advanced:
  gpm install git+https://github.com/user/repo.git  # Install from Git
  gpm install file:../local-package                 # Install from local directory`,
	RunE: install,
}

func init() {
	installCmd.Flags().BoolVarP(&installGlobal, "global", "g", false, "Install package globally")
	installCmd.Flags().StringVar(&installVersion, "version", "", "Specific version to install")
	installCmd.Flags().BoolVar(&installSave, "save", false, "Save to package.json dependencies")
	installCmd.Flags().BoolVar(&installSaveDev, "save-dev", false, "Save to package.json devDependencies")
}

func install(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return installFromPackageJSON()
	}

	fmt.Println(styling.Header("📦  Package Installation"))
	fmt.Println(styling.Separator())

	if installGlobal {
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Global package installation not yet supported"),
			styling.Hint("Use local installation instead"))
	}

	// Install multiple packages
	for _, specStr := range args {
		spec := parsePackageSpec(specStr)

		if installVersion != "" && len(args) == 1 && spec.Source == "registry" {
			spec.Version = installVersion
		}

		if err := installPackageBySpec(spec); err != nil {
			return err
		}
	}

	return nil
}

type PackageSpec struct {
	Name     string
	Version  string
	Source   string // "registry", "git", "file"
	URL      string
	Branch   string
	FilePath string
}

func parsePackageSpec(spec string) PackageSpec {
	// Handle Git URLs
	if strings.HasPrefix(spec, "git+") {
		return parseGitSpec(spec)
	}

	// Handle file URLs
	if strings.HasPrefix(spec, "file:") {
		return parseFileSpec(spec)
	}

	// Handle npm scoped packages (@scope/package@version)
	if strings.HasPrefix(spec, "@") {
		return parseNpmScopedSpec(spec)
	}

	// Handle UPM packages and regular packages with version (package@version)
	if strings.Contains(spec, "@") {
		parts := strings.Split(spec, "@")
		return PackageSpec{
			Name:    parts[0],
			Version: parts[1],
			Source:  "registry",
		}
	}

	return PackageSpec{
		Name:    spec,
		Version: "latest",
		Source:  "registry",
	}
}

func parseNpmScopedSpec(spec string) PackageSpec {
	// Handle npm scoped packages: @scope/package or @scope/package@version
	// Examples: @mycompany/package, @mycompany/package@1.0.0

	// Remove leading @
	withoutAt := strings.TrimPrefix(spec, "@")

	// Split by @ to separate name and version
	parts := strings.Split(withoutAt, "@")
	scopedName := "@" + parts[0] // Restore the @ for the scoped name

	version := "latest"
	if len(parts) > 1 {
		version = parts[1]
	}

	return PackageSpec{
		Name:    scopedName,
		Version: version,
		Source:  "registry",
	}
}

func parseGitSpec(spec string) PackageSpec {
	// Remove git+ prefix
	gitURL := strings.TrimPrefix(spec, "git+")

	// Handle branch/tag after #
	branch := "main"
	if strings.Contains(gitURL, "#") {
		parts := strings.Split(gitURL, "#")
		gitURL = parts[0]
		branch = parts[1]
	}

	// Extract package name from URL
	name := extractPackageNameFromGit(gitURL)

	return PackageSpec{
		Name:   name,
		Source: "git",
		URL:    gitURL,
		Branch: branch,
	}
}

func parseFileSpec(spec string) PackageSpec {
	// Remove file: prefix
	filePath := strings.TrimPrefix(spec, "file:")

	// Extract package name from path
	name := filepath.Base(filePath)

	return PackageSpec{
		Name:     name,
		Source:   "file",
		FilePath: filePath,
	}
}

func extractPackageNameFromGit(gitURL string) string {
	// Extract repo name from URL
	parts := strings.Split(gitURL, "/")
	if len(parts) > 0 {
		repoName := parts[len(parts)-1]
		// Remove .git suffix if present
		return strings.TrimSuffix(repoName, ".git")
	}
	return "unknown-package"
}

func installFromPackageJSON() error {
	packageJSONPath := "package.json"
	if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
		return fmt.Errorf("%s\n\n%s",
			styling.Error("No package.json found in current directory"),
			styling.Hint("Run 'npm init' or create a package.json file first, or specify a package name to install"))
	}

	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return fmt.Errorf("invalid package.json: %w", err)
	}

	fmt.Println(styling.Info("Installing dependencies from package.json..."))

	// Install production dependencies
	for name, version := range pkg.Dependencies {
		if err := downloadAndInstallPackage(name, version, false); err != nil {
			fmt.Printf("%s %s@%s\n", styling.Error("✗ Failed to install"), name, version)
			return err
		}
		fmt.Printf("%s %s@%s\n", styling.Success("✓ Installed"), name, version)
	}

	// Install dev dependencies
	for name, version := range pkg.DevDependencies {
		if err := downloadAndInstallPackage(name, version, true); err != nil {
			fmt.Printf("%s %s@%s (dev)\n", styling.Error("✗ Failed to install"), name, version)
			return err
		}
		fmt.Printf("%s %s@%s (dev)\n", styling.Success("✓ Installed"), name, version)
	}

	return nil
}

func installPackageBySpec(spec PackageSpec) error {
	switch spec.Source {
	case "registry":
		return installFromRegistry(spec)
	case "git":
		return installFromGit(spec)
	case "file":
		return installFromFile(spec)
	default:
		return fmt.Errorf("unsupported package source: %s", spec.Source)
	}
}

func installFromRegistry(spec PackageSpec) error {
	fmt.Printf("%s %s@%s\n",
		styling.Label("Installing:"),
		styling.Package(spec.Name),
		styling.Version(spec.Version))

	if err := downloadAndInstallPackage(spec.Name, spec.Version, installSaveDev); err != nil {
		return err
	}

	if installSave || installSaveDev {
		if err := updatePackageJSON(spec.Name, spec.Version, installSaveDev); err != nil {
			fmt.Printf("%s\n", styling.Warning("Package installed but failed to update package.json: "+err.Error()))
		}
	}

	fmt.Println(styling.Separator())
	fmt.Printf("%s %s@%s\n",
		styling.Success("✓ Successfully installed"),
		styling.Package(spec.Name),
		styling.Version(spec.Version))
	fmt.Println(styling.Separator())

	return nil
}

func installFromGit(spec PackageSpec) error {
	fmt.Printf("%s %s from %s#%s\n", styling.Label("Installing:"), styling.Package(spec.Name), styling.URL(spec.URL), styling.Version(spec.Branch))

	if err := cloneAndInstallGitPackage(spec); err != nil {
		return err
	}

	if installSave || installSaveDev {
		gitSpec := fmt.Sprintf("git+%s#%s", spec.URL, spec.Branch)
		if err := updatePackageJSON(spec.Name, gitSpec, installSaveDev); err != nil {
			fmt.Printf("%s\n", styling.Warning("Package installed but failed to update package.json: "+err.Error()))
		}
	}

	fmt.Println(styling.Separator())
	fmt.Printf("%s %s from Git\n", styling.Success("✓ Successfully installed"), styling.Package(spec.Name))
	fmt.Println(styling.Separator())

	return nil
}

func installFromFile(spec PackageSpec) error {
	fmt.Printf("%s %s from %s\n", styling.Label("Installing:"), styling.Package(spec.Name), styling.Value(spec.FilePath))

	if err := copyLocalPackage(spec); err != nil {
		return err
	}

	if installSave || installSaveDev {
		fileSpec := fmt.Sprintf("file:%s", spec.FilePath)
		if err := updatePackageJSON(spec.Name, fileSpec, installSaveDev); err != nil {
			fmt.Printf("%s\n", styling.Warning("Package installed but failed to update package.json: "+err.Error()))
		}
	}

	fmt.Println(styling.Separator())
	fmt.Printf("%s %s from local file\n", styling.Success("✓ Successfully installed"), styling.Package(spec.Name))
	fmt.Println(styling.Separator())

	return nil
}

func downloadAndInstallPackage(packageName, version string, isDev bool) error {
	cfg := config.GetConfig()

	// Create Packages directory if it doesn't exist
	packagesDir := "Packages"
	if err := os.MkdirAll(packagesDir, 0750); err != nil {
		return fmt.Errorf("failed to create Packages directory: %w", err)
	}

	// Download package metadata to get tarball URL
	baseURL, err := url.Parse(cfg.Registry)
	if err != nil {
		return fmt.Errorf("invalid registry URL: %w", err)
	}
	packageURL := baseURL.JoinPath(packageName).String()
	// #nosec G107 - URL is validated using url.Parse and JoinPath above
	resp, err := http.Get(packageURL)
	if err != nil {
		return fmt.Errorf("failed to fetch package metadata: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 {
		return fmt.Errorf("package not found: %s", packageName)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("registry error (HTTP %d) for package: %s", resp.StatusCode, packageName)
	}

	var packageInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&packageInfo); err != nil {
		return fmt.Errorf("failed to parse package metadata: %w", err)
	}

	// Get the version to install
	actualVersion, tarballURL, err := getVersionInfo(packageInfo, version)
	if err != nil {
		return err
	}

	// Download and extract the package
	packageDir := filepath.Join(packagesDir, packageName)
	if err := downloadAndExtractPackage(tarballURL, packageDir); err != nil {
		return fmt.Errorf("failed to download package: %w", err)
	}

	// Create or update Unity manifest.json
	if err := updateUnityManifest(packageName, actualVersion, isDev); err != nil {
		fmt.Printf("%s\n", styling.Warning("Package installed but failed to update manifest.json: "+err.Error()))
	}

	return nil
}

func getVersionInfo(packageInfo map[string]interface{}, requestedVersion string) (string, string, error) {
	versions, ok := packageInfo["versions"].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("no versions available for package")
	}

	// Handle "latest" version
	var actualVersion string
	if requestedVersion == "latest" {
		distTags, ok := packageInfo["dist-tags"].(map[string]interface{})
		if !ok {
			return "", "", fmt.Errorf("no dist-tags available")
		}
		latest, ok := distTags["latest"].(string)
		if !ok {
			return "", "", fmt.Errorf("no latest version found")
		}
		actualVersion = latest
	} else if isVersionRange(requestedVersion) {
		// Handle version ranges (^1.0.0, ~1.2.0, >=1.0.0, etc.)
		matchedVersion, err := findMatchingVersion(versions, requestedVersion)
		if err != nil {
			return "", "", err
		}
		actualVersion = matchedVersion
	} else {
		actualVersion = requestedVersion
	}

	// Get version info
	versionInfo, ok := versions[actualVersion].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("version %s not found", actualVersion)
	}

	// Get tarball URL
	dist, ok := versionInfo["dist"].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("no distribution info for version %s", actualVersion)
	}

	tarballURL, ok := dist["tarball"].(string)
	if !ok {
		return "", "", fmt.Errorf("no tarball URL for version %s", actualVersion)
	}

	return actualVersion, tarballURL, nil
}

func isVersionRange(version string) bool {
	// Check if version contains range operators
	return strings.HasPrefix(version, "^") ||
		strings.HasPrefix(version, "~") ||
		strings.HasPrefix(version, ">=") ||
		strings.HasPrefix(version, "<=") ||
		strings.HasPrefix(version, ">") ||
		strings.HasPrefix(version, "<") ||
		strings.Contains(version, " - ")
}

func findMatchingVersion(versions map[string]interface{}, versionRange string) (string, error) {
	var availableVersions []string
	for version := range versions {
		availableVersions = append(availableVersions, version)
	}

	// Handle caret range (^1.0.0)
	if strings.HasPrefix(versionRange, "^") {
		baseVersion := strings.TrimPrefix(versionRange, "^")
		return findCaretMatch(availableVersions, baseVersion)
	}

	// Handle tilde range (~1.2.0)
	if strings.HasPrefix(versionRange, "~") {
		baseVersion := strings.TrimPrefix(versionRange, "~")
		return findTildeMatch(availableVersions, baseVersion)
	}

	// Handle >= range
	if strings.HasPrefix(versionRange, ">=") {
		baseVersion := strings.TrimPrefix(versionRange, ">=")
		return findGreaterOrEqualMatch(availableVersions, baseVersion)
	}

	// For now, fallback to exact match
	for version := range versions {
		if version == versionRange {
			return version, nil
		}
	}

	return "", fmt.Errorf("no version matching %s found", versionRange)
}

func findCaretMatch(versions []string, baseVersion string) (string, error) {
	// ^1.2.3 := >=1.2.3 <2.0.0 (reasonably close to 1.2.3)
	baseParts := parseVersion(baseVersion)
	if len(baseParts) < 3 {
		return "", fmt.Errorf("invalid base version: %s", baseVersion)
	}

	var bestMatch string
	var bestParts []int

	for _, version := range versions {
		parts := parseVersion(version)
		if len(parts) < 3 {
			continue
		}

		// Must have same major version
		if parts[0] != baseParts[0] {
			continue
		}

		// Must be >= base version
		if compareVersions(parts, baseParts) < 0 {
			continue
		}

		// Track best match
		if bestMatch == "" || compareVersions(parts, bestParts) > 0 {
			bestMatch = version
			bestParts = parts
		}
	}

	if bestMatch == "" {
		return "", fmt.Errorf("no caret match for %s", baseVersion)
	}

	return bestMatch, nil
}

func findTildeMatch(versions []string, baseVersion string) (string, error) {
	// ~1.2.3 := >=1.2.3 <1.3.0 (reasonably close to 1.2.3)
	baseParts := parseVersion(baseVersion)
	if len(baseParts) < 3 {
		return "", fmt.Errorf("invalid base version: %s", baseVersion)
	}

	var bestMatch string
	var bestParts []int

	for _, version := range versions {
		parts := parseVersion(version)
		if len(parts) < 3 {
			continue
		}

		// Must have same major and minor version
		if parts[0] != baseParts[0] || parts[1] != baseParts[1] {
			continue
		}

		// Must be >= base version
		if compareVersions(parts, baseParts) < 0 {
			continue
		}

		// Track best match
		if bestMatch == "" || compareVersions(parts, bestParts) > 0 {
			bestMatch = version
			bestParts = parts
		}
	}

	if bestMatch == "" {
		return "", fmt.Errorf("no tilde match for %s", baseVersion)
	}

	return bestMatch, nil
}

func findGreaterOrEqualMatch(versions []string, baseVersion string) (string, error) {
	baseParts := parseVersion(baseVersion)
	if len(baseParts) < 3 {
		return "", fmt.Errorf("invalid base version: %s", baseVersion)
	}

	var bestMatch string
	var bestParts []int

	for _, version := range versions {
		parts := parseVersion(version)
		if len(parts) < 3 {
			continue
		}

		// Must be >= base version
		if compareVersions(parts, baseParts) < 0 {
			continue
		}

		// Track best match (highest version)
		if bestMatch == "" || compareVersions(parts, bestParts) > 0 {
			bestMatch = version
			bestParts = parts
		}
	}

	if bestMatch == "" {
		return "", fmt.Errorf("no version >= %s found", baseVersion)
	}

	return bestMatch, nil
}

func parseVersion(version string) []int {
	parts := strings.Split(version, ".")
	var nums []int
	for _, part := range parts {
		// Remove any non-numeric suffixes (like -alpha, -beta)
		re := regexp.MustCompile(`^(\d+)`)
		matches := re.FindStringSubmatch(part)
		if len(matches) > 1 {
			if num, err := strconv.Atoi(matches[1]); err == nil {
				nums = append(nums, num)
			}
		}
	}
	return nums
}

func compareVersions(a, b []int) int {
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	for i := 0; i < maxLen; i++ {
		aVal := 0
		bVal := 0
		if i < len(a) {
			aVal = a[i]
		}
		if i < len(b) {
			bVal = b[i]
		}

		if aVal < bVal {
			return -1
		} else if aVal > bVal {
			return 1
		}
	}
	return 0
}

func cloneAndInstallGitPackage(spec PackageSpec) error {
	packagesDir := "Packages"
	packageDir := filepath.Join(packagesDir, spec.Name)

	// Create Packages directory if it doesn't exist
	if err := os.MkdirAll(packagesDir, 0750); err != nil {
		return fmt.Errorf("failed to create Packages directory: %w", err)
	}

	// Remove existing package directory
	if err := os.RemoveAll(packageDir); err != nil {
		return fmt.Errorf("failed to remove existing package: %w", err)
	}

	// Validate git command arguments for security
	if err := validateGitCommand("clone", "--branch", spec.Branch, "--depth", "1", spec.URL, packageDir); err != nil {
		return fmt.Errorf("invalid git command arguments: %w", err)
	}

	// Clone the repository
	cmd := exec.Command("git", "clone", "--branch", spec.Branch, "--depth", "1", spec.URL, packageDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Remove .git directory to clean up
	gitDir := filepath.Join(packageDir, ".git")
	if err := os.RemoveAll(gitDir); err != nil {
		// Non-fatal error, just warn
		fmt.Printf("%s\n", styling.Warning("Could not remove .git directory: "+err.Error()))
	}

	// Get package name from package.json if available
	packageJSONPath := filepath.Join(packageDir, "package.json")
	if data, err := os.ReadFile(packageJSONPath); err == nil {
		var pkg map[string]interface{}
		if err := json.Unmarshal(data, &pkg); err == nil {
			if name, ok := pkg["name"].(string); ok && name != "" {
				spec.Name = name
			}
		}
	}

	// Update Unity manifest
	if err := updateUnityManifest(spec.Name, fmt.Sprintf("git+%s#%s", spec.URL, spec.Branch), false); err != nil {
		fmt.Printf("%s\n", styling.Warning("Package installed but failed to update manifest.json: "+err.Error()))
	}

	return nil
}

func copyLocalPackage(spec PackageSpec) error {
	packagesDir := "Packages"
	packageDir := filepath.Join(packagesDir, spec.Name)

	// Create Packages directory if it doesn't exist
	if err := os.MkdirAll(packagesDir, 0750); err != nil {
		return fmt.Errorf("failed to create Packages directory: %w", err)
	}

	// Convert relative path to absolute
	sourcePath, err := filepath.Abs(spec.FilePath)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Check if source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("source path does not exist: %s", sourcePath)
	}

	// Remove existing package directory
	if err := os.RemoveAll(packageDir); err != nil {
		return fmt.Errorf("failed to remove existing package: %w", err)
	}

	// Copy the package
	if err := copyDir(sourcePath, packageDir); err != nil {
		return fmt.Errorf("failed to copy package: %w", err)
	}

	// Get package name from package.json if available
	packageJSONPath := filepath.Join(packageDir, "package.json")
	if data, err := os.ReadFile(packageJSONPath); err == nil {
		var pkg map[string]interface{}
		if err := json.Unmarshal(data, &pkg); err == nil {
			if name, ok := pkg["name"].(string); ok && name != "" {
				spec.Name = name
			}
		}
	}

	// Update Unity manifest
	if err := updateUnityManifest(spec.Name, fmt.Sprintf("file:%s", spec.FilePath), false); err != nil {
		fmt.Printf("%s\n", styling.Warning("Package installed but failed to update manifest.json: "+err.Error()))
	}

	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = srcFile.Close() }()

		// Create destination directory if needed
		if err := os.MkdirAll(filepath.Dir(dstPath), 0750); err != nil {
			return err
		}

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer func() { _ = dstFile.Close() }()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

func downloadAndExtractPackage(tarballURL, packageDir string) error {
	// Download tarball
	// #nosec G107 - tarballURL comes from trusted registry response
	resp, err := http.Get(tarballURL)
	if err != nil {
		return fmt.Errorf("failed to download tarball: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to download tarball (HTTP %d)", resp.StatusCode)
	}

	// Create gzip reader
	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = gzReader.Close() }()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Remove existing package directory
	if err := os.RemoveAll(packageDir); err != nil {
		return fmt.Errorf("failed to remove existing package: %w", err)
	}

	// Extract tarball
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Remove "package/" prefix from path (npm/UPM standard)
		targetPath := strings.TrimPrefix(header.Name, "package/")

		if targetPath == "" {
			continue
		}

		// Validate path to prevent directory traversal
		if err := validatePath(targetPath, packageDir); err != nil {
			return fmt.Errorf("security validation failed: %w", err)
		}

		fullPath := filepath.Join(packageDir, filepath.Clean(targetPath))

		// Ensure the target directory exists
		if err := os.MkdirAll(filepath.Dir(fullPath), 0750); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Ensure mode is within valid range for os.FileMode (uint32)
			mode := header.Mode & 0777 // Mask to file permission bits only
			if err := os.MkdirAll(fullPath, os.FileMode(mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
			}
		case tar.TypeReg:
			// Ensure mode is within valid range for os.FileMode (uint32)
			mode := header.Mode & 0777 // Mask to file permission bits only
			outFile, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", fullPath, err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				_ = outFile.Close() // Best effort cleanup
				return fmt.Errorf("failed to extract file %s: %w", fullPath, err)
			}
			if err := outFile.Close(); err != nil {
				return fmt.Errorf("failed to close file %s: %w", fullPath, err)
			}
		}
	}

	return nil
}

func updateUnityManifest(packageName, version string, isDev bool) error {
	manifestPath := "Packages/manifest.json"

	var manifest map[string]interface{}

	// Read existing manifest or create new one
	if data, err := os.ReadFile(manifestPath); err == nil {
		if err := json.Unmarshal(data, &manifest); err != nil {
			return fmt.Errorf("invalid manifest.json: %w", err)
		}
	} else {
		manifest = map[string]interface{}{
			"dependencies": make(map[string]interface{}),
		}
		// Ensure Packages directory exists
		if err := os.MkdirAll("Packages", 0750); err != nil {
			return fmt.Errorf("failed to create Packages directory: %w", err)
		}
	}

	// Add dependency
	deps, ok := manifest["dependencies"].(map[string]interface{})
	if !ok {
		deps = make(map[string]interface{})
		manifest["dependencies"] = deps
	}

	// For local packages, use file: protocol
	deps[packageName] = "file:./" + packageName

	// Write updated manifest
	updatedData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	return os.WriteFile(manifestPath, updatedData, 0600)
}

func updatePackageJSON(packageName, version string, isDev bool) error {
	packageJSONPath := "package.json"
	var pkg map[string]interface{}

	if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
		// Create minimal package.json
		pkg = map[string]interface{}{
			"name":    "my-project",
			"version": "1.0.0",
		}
	} else {
		data, err := os.ReadFile(packageJSONPath)
		if err != nil {
			return fmt.Errorf("failed to read package.json: %w", err)
		}

		if err := json.Unmarshal(data, &pkg); err != nil {
			return fmt.Errorf("invalid package.json: %w", err)
		}
	}

	// Add to dependencies or devDependencies
	depKey := "dependencies"
	if isDev {
		depKey = "devDependencies"
	}

	if pkg[depKey] == nil {
		pkg[depKey] = make(map[string]interface{})
	}

	deps := pkg[depKey].(map[string]interface{})
	deps[packageName] = version

	// Write back to file
	updatedData, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal package.json: %w", err)
	}

	return os.WriteFile(packageJSONPath, updatedData, 0600)
}
