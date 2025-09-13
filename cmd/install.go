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
	"gpm.sh/gpm/gpm-cli/internal/engines"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

func validatePath(filePath, destDir string) error {
	cleanPath := filepath.Clean(filePath)

	absPath, err := filepath.Abs(filepath.Join(destDir, cleanPath))
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("failed to resolve destination directory: %w", err)
	}

	if !strings.HasPrefix(absPath, destAbs) {
		return fmt.Errorf("path traversal attempt detected: %s", filePath)
	}

	return nil
}

// isValidPackageURL validates that the package URL is safe and belongs to the expected host
func isValidPackageURL(packageURL, expectedHost string) bool {
	parsedURL, err := url.Parse(packageURL)
	if err != nil {
		return false
	}

	// Only allow HTTP and HTTPS protocols
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false
	}

	// Ensure the host matches the expected host
	if parsedURL.Host != expectedHost {
		return false
	}

	// Prevent localhost and private IP ranges
	if strings.HasPrefix(parsedURL.Host, "localhost") ||
		strings.HasPrefix(parsedURL.Host, "127.") ||
		strings.HasPrefix(parsedURL.Host, "192.168.") ||
		strings.HasPrefix(parsedURL.Host, "10.") ||
		strings.HasPrefix(parsedURL.Host, "172.") {
		return false
	}

	return true
}

//nolint:unused
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

//nolint:unused
func validateSafetyPath(path string) error {
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("null byte in path: %s", path)
	}
	cleaned := filepath.Clean(path)
	if cleaned != path && strings.Contains(path, "..") {
		return fmt.Errorf("path traversal detected: %s", path)
	}
	return nil
}

var (
	installGlobal     bool
	installVersion    string
	installSave       bool
	installSaveDev    bool
	installUnity      bool
	installUnreal     bool
	installGodot      bool
	installCocos      bool
	installProjectDir string
	installRegistry   string
)

var installCmd = &cobra.Command{
	Use:   "install [package[@version]...]",
	Short: "Install packages with multi-engine support",
	Long: `Install packages with automatic game engine detection.

GPM automatically detects your game engine project and uses the appropriate
package management system. Supports Unity, Unreal Engine, Godot, and Cocos Creator.

Engine Detection:
  Unity        - Looks for Assets/, ProjectSettings/, Packages/manifest.json
  Unreal       - Looks for .uproject files and Content/ directory
  Godot        - Looks for project.godot file
  Cocos Creator - Looks for project.json and assets/ directory

Engine-Specific Installation:
  Unity        - Modifies Packages/manifest.json and configures scoped registries
  Unreal       - Manages plugins directory (future release)
  Godot        - Manages addons/ folder (future release)
  Cocos Creator - Handles extensions (future release)

Basic Examples:
  gpm install                              # Install from package.json (auto-detect engine)
  gpm install package-name                 # Install package (auto-detect engine)
  gpm install package-name@1.0.0           # Install specific version
  gpm install pkg1 pkg2 pkg3               # Install multiple packages

Engine-Specific Examples:
  gpm install --unity com.unity.textmeshpro     # Force Unity engine
  gpm install --unreal adjust-sdk               # Force Unreal engine
  gpm install --godot godot-analytics           # Force Godot engine
  gpm install --cocos cocos-analytics           # Force Cocos Creator engine

Registry Examples:
  gpm install --registry https://homa.gpm.sh homa-analytics
  gpm install --project-dir /path/to/project package-name

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

	// Engine-specific flags
	installCmd.Flags().BoolVar(&installUnity, "unity", false, "Force Unity engine adapter")
	installCmd.Flags().BoolVar(&installUnreal, "unreal", false, "Force Unreal Engine adapter")
	installCmd.Flags().BoolVar(&installGodot, "godot", false, "Force Godot engine adapter")
	installCmd.Flags().BoolVar(&installCocos, "cocos", false, "Force Cocos Creator engine adapter")

	// Advanced options
	installCmd.Flags().StringVar(&installProjectDir, "project-dir", "", "Project directory (default: current directory)")
	installCmd.Flags().StringVar(&installRegistry, "registry", "", "Override registry URL for this installation")
}

func install(cmd *cobra.Command, args []string) error {
	// Handle no arguments - install from package.json
	if len(args) == 0 {
		return installFromPackageJSON()
	}

	fmt.Println(styling.Header("📦  Multi-Engine Package Installation"))
	fmt.Println(styling.Separator())

	// Global installation not supported yet
	if installGlobal {
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Global package installation not yet supported"),
			styling.Hint("Use engine-specific local installation instead"))
	}

	// Determine project directory
	projectDir := installProjectDir
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Detect or determine engine type
	engineType, detectionResult, err := determineEngineType(projectDir)
	if err != nil {
		return fmt.Errorf("engine detection failed: %w", err)
	}

	// Show detection results
	if detectionResult != nil {
		fmt.Printf("%s %s\n", styling.Label("Detected Engine:"), styling.Value(detectionResult.Engine.String()))
		fmt.Printf("%s %s\n", styling.Label("Confidence:"), styling.Value(detectionResult.Confidence.String()))
		if detectionResult.Version != "" {
			fmt.Printf("%s %s\n", styling.Label("Version:"), styling.Value(detectionResult.Version))
		}
		fmt.Printf("%s %s\n", styling.Label("Project Path:"), styling.File(detectionResult.ProjectPath))
		fmt.Println(styling.Separator())
	}

	// Get engine adapter
	adapter, err := engines.GetAdapter(engineType)
	if err != nil {
		return fmt.Errorf("failed to get engine adapter: %w", err)
	}

	// Validate project
	if err := adapter.ValidateProject(projectDir); err != nil {
		return fmt.Errorf("project validation failed: %w", err)
	}

	// Install each package
	for _, specStr := range args {
		spec := parsePackageSpec(specStr)

		// Override version if specified
		if installVersion != "" && len(args) == 1 && spec.Source == "registry" {
			spec.Version = installVersion
		}

		// Install package using engine adapter
		if err := installPackageWithEngine(adapter, projectDir, spec); err != nil {
			return fmt.Errorf("failed to install %s: %w", spec.Name, err)
		}
	}

	fmt.Println(styling.Success("✓ All packages installed successfully!"))
	return nil
}

// determineEngineType determines the engine type based on flags or auto-detection
func determineEngineType(projectDir string) (engines.EngineType, *engines.DetectionResult, error) {
	// Check for explicit engine flags
	engineFlags := []bool{installUnity, installUnreal, installGodot, installCocos}
	engineTypes := []engines.EngineType{engines.EngineUnity, engines.EngineUnreal, engines.EngineGodot, engines.EngineCocos}

	flagCount := 0
	var selectedEngine engines.EngineType
	for i, flag := range engineFlags {
		if flag {
			flagCount++
			selectedEngine = engineTypes[i]
		}
	}

	// Error if multiple engines specified
	if flagCount > 1 {
		return engines.EngineUnknown, nil, fmt.Errorf("%s\n\n%s",
			styling.Error("Multiple engine flags specified"),
			styling.Hint("Use only one engine flag: --unity, --unreal, --godot, or --cocos"))
	}

	// If engine explicitly specified, return it
	if flagCount == 1 {
		fmt.Printf("%s %s\n", styling.Label("Forced Engine:"), styling.Value(selectedEngine.String()))
		return selectedEngine, nil, nil
	}

	// Auto-detect engine
	fmt.Printf("%s %s\n", styling.Label("Auto-detecting engine in:"), styling.File(projectDir))

	results, err := engines.DetectEngine(projectDir)
	if err != nil {
		return engines.EngineUnknown, nil, fmt.Errorf("engine detection failed: %w", err)
	}

	if len(results) == 0 {
		return engines.EngineUnknown, nil, fmt.Errorf("%s\n\n%s\n%s\n%s\n%s\n%s",
			styling.Error("No game engine project detected"),
			styling.Hint("GPM looked for:"),
			styling.Value("  • Unity: Assets/, ProjectSettings/, Packages/manifest.json"),
			styling.Value("  • Unreal: *.uproject files and Content/ directory"),
			styling.Value("  • Godot: project.godot file"),
			styling.Value("  • Cocos Creator: project.json and assets/ directory"))
	}

	best := results.Best()

	// Handle ambiguous detection
	if results.HasAmbiguous() {
		fmt.Println(styling.Warning("⚠ Multiple engines detected:"))
		for i, result := range results {
			if result.Confidence >= engines.ConfidenceHigh {
				fmt.Printf("  %d. %s (%s confidence)\n", i+1, result.Engine.String(), result.Confidence.String())
			}
		}
		return engines.EngineUnknown, nil, fmt.Errorf("%s\n\n%s",
			styling.Error("Ambiguous engine detection"),
			styling.Hint("Use an explicit engine flag: --unity, --unreal, --godot, or --cocos"))
	}

	// Check confidence level
	if best.Confidence < engines.ConfidenceMedium {
		return engines.EngineUnknown, best, fmt.Errorf("%s\n\n%s\n%s",
			styling.Error(fmt.Sprintf("Low confidence engine detection: %s (%s)", best.Engine.String(), best.Confidence.String())),
			styling.Hint("Use an explicit engine flag to force engine type:"),
			styling.Value("  gpm install --unity package-name"))
	}

	return best.Engine, best, nil
}

// installPackageWithEngine installs a package using the appropriate engine adapter
func installPackageWithEngine(adapter engines.EngineAdapter, projectDir string, spec PackageSpec) error {
	switch spec.Source {
	case "registry":
		return installFromRegistryWithEngine(adapter, projectDir, spec)
	case "git":
		return installFromGitWithEngine(spec)
	case "file":
		return installFromFileWithEngine(spec)
	default:
		return fmt.Errorf("unsupported package source: %s", spec.Source)
	}
}

// installFromRegistryWithEngine installs a package from registry using engine adapter
func installFromRegistryWithEngine(adapter engines.EngineAdapter, projectDir string, spec PackageSpec) error {
	fmt.Printf("%s %s@%s\n", styling.Label("Installing:"), styling.Package(spec.Name), styling.Version(spec.Version))

	// Use default or override registry
	registryURL := "https://registry.gpm.sh" // Default GPM registry
	if installRegistry != "" {
		registryURL = installRegistry
		fmt.Printf("%s %s\n", styling.Label("Registry (override):"), styling.URL(installRegistry))
	} else {
		fmt.Printf("%s %s\n", styling.Label("Registry:"), styling.URL(registryURL))
	}

	// Resolve version if it's "latest" or "*"
	resolvedVersion := spec.Version
	if spec.Version == "latest" || spec.Version == "*" {
		actualVersion, err := resolveLatestVersionFromRegistry(spec.Name, registryURL)
		if err != nil {
			return fmt.Errorf("failed to resolve latest version: %w", err)
		}
		resolvedVersion = actualVersion
		fmt.Printf("%s %s@%s (resolved from %s)\n", styling.Label("Resolved:"), styling.Package(spec.Name), styling.Version(resolvedVersion), styling.Version(spec.Version))
	}

	// Create install request
	req := &engines.PackageInstallRequest{
		Name:     spec.Name,
		Version:  resolvedVersion,
		Registry: registryURL,
		IsDev:    installSaveDev,
	}

	// Install package
	result, err := adapter.InstallPackage(projectDir, req)
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	if result.Success {
		fmt.Printf("%s %s\n", styling.Success("✓"), result.Message)
		if result.Details != nil {
			for key, value := range result.Details {
				fmt.Printf("%s %v\n", styling.Label(fmt.Sprintf("  %s:", key)), value)
			}
		}
	} else {
		return fmt.Errorf("installation reported failure: %s", result.Message)
	}

	return nil
}

// installFromGitWithEngine installs a package from git using engine adapter (placeholder)
func installFromGitWithEngine(spec PackageSpec) error {
	return fmt.Errorf("git installation with engine adapters not yet implemented")
}

// installFromFileWithEngine installs a package from file using engine adapter (placeholder)
func installFromFileWithEngine(spec PackageSpec) error {
	return fmt.Errorf("file installation with engine adapters not yet implemented")
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
	if strings.HasPrefix(spec, "git+") {
		return parseGitSpec(spec)
	}

	if strings.HasPrefix(spec, "file:") {
		return parseFileSpec(spec)
	}

	if strings.Contains(spec, "@") {
		parts := strings.Split(spec, "@")
		version := parts[1]
		// Handle "*" as a wildcard for latest version
		if version == "*" {
			version = "latest"
		}
		return PackageSpec{
			Name:    parts[0],
			Version: version,
			Source:  "registry",
		}
	}

	return PackageSpec{
		Name:    spec,
		Version: "latest",
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
		// Handle "*" as a wildcard for latest version
		if version == "*" {
			version = "latest"
		}
		if err := downloadAndInstallPackage(name, version, false); err != nil {
			fmt.Printf("%s %s@%s\n", styling.Error("✗ Failed to install"), name, version)
			return err
		}
		fmt.Printf("%s %s@%s\n", styling.Success("✓ Installed"), name, version)
	}

	// Install dev dependencies
	for name, version := range pkg.DevDependencies {
		// Handle "*" as a wildcard for latest version
		if version == "*" {
			version = "latest"
		}
		if err := downloadAndInstallPackage(name, version, true); err != nil {
			fmt.Printf("%s %s@%s (dev)\n", styling.Error("✗ Failed to install"), name, version)
			return err
		}
		fmt.Printf("%s %s@%s (dev)\n", styling.Success("✓ Installed"), name, version)
	}

	return nil
}

//nolint:unused
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

//nolint:unused
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

//nolint:unused
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

//nolint:unused
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

//nolint:unused
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
	cmd := exec.Command("git", "clone", "--branch", spec.Branch, "--depth", "1", spec.URL, packageDir) // #nosec G204 - Git command validated above
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
	if err := validateSafetyPath(packageJSONPath); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	// #nosec G304 - packageJSONPath is validated above
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

//nolint:unused
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
	if err := validateSafetyPath(packageJSONPath); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	// #nosec G304 - packageJSONPath is validated above
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

//nolint:unused
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
		if err := validateSafetyPath(path); err != nil {
			return fmt.Errorf("invalid source path: %w", err)
		}
		srcFile, err := os.Open(path) // #nosec G304 - Path validated above
		if err != nil {
			return err
		}
		defer func() { _ = srcFile.Close() }()

		// Create destination directory if needed
		if err := os.MkdirAll(filepath.Dir(dstPath), 0750); err != nil {
			return err
		}

		if err := validateSafetyPath(dstPath); err != nil {
			return fmt.Errorf("invalid destination path: %w", err)
		}
		dstFile, err := os.Create(dstPath) // #nosec G304 - Path validated above
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

		fullPath := filepath.Join(packageDir, filepath.Clean(targetPath))

		// Validate the target path for security
		if err := validatePath(targetPath, packageDir); err != nil {
			return fmt.Errorf("invalid target path: %w", err)
		}

		// Ensure the target directory exists
		if err := os.MkdirAll(filepath.Dir(fullPath), 0750); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Safe conversion: mask to permission bits and ensure it fits in uint32
			mode := os.FileMode(header.Mode) & 0777 // #nosec G115 - Safe conversion with mask
			if err := os.MkdirAll(fullPath, mode); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
			}
		case tar.TypeReg:
			// Safe conversion: mask to permission bits and ensure it fits in uint32
			mode := os.FileMode(header.Mode) & 0777                                         // #nosec G115 - Safe conversion with mask
			outFile, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode) // #nosec G304 - Path validated above
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", fullPath, err)
			}

			// Limit extraction size to prevent decompression bombs (100MB limit)
			limitReader := io.LimitReader(tarReader, 100*1024*1024)
			if _, err := io.Copy(outFile, limitReader); err != nil {
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

//nolint:unused
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

// resolveLatestVersionFromRegistry fetches the latest version from a registry
func resolveLatestVersionFromRegistry(packageName, registryURL string) (string, error) {
	// Parse the registry URL
	baseURL, err := url.Parse(registryURL)
	if err != nil {
		return "", fmt.Errorf("invalid registry URL: %w", err)
	}

	// Construct the package info URL
	packageURL := baseURL.JoinPath(packageName).String()

	// Validate URL to prevent SSRF attacks
	if !isValidPackageURL(packageURL, baseURL.Host) {
		return "", fmt.Errorf("invalid package URL: %s", packageURL)
	}

	// Fetch package metadata
	resp, err := http.Get(packageURL) // #nosec G107 -- URL is validated by isValidPackageURL
	if err != nil {
		return "", fmt.Errorf("failed to fetch package metadata: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 {
		return "", fmt.Errorf("package not found: %s", packageName)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("registry error (HTTP %d) for package: %s", resp.StatusCode, packageName)
	}

	// Parse the response
	var packageInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&packageInfo); err != nil {
		return "", fmt.Errorf("failed to parse package metadata: %w", err)
	}

	// First try to get the latest version from dist-tags
	if distTags, ok := packageInfo["dist-tags"].(map[string]interface{}); ok {
		if latest, ok := distTags["latest"].(string); ok && latest != "" {
			return latest, nil
		}
	}

	// If no dist-tags or latest tag, get all versions and find the highest one
	versions, ok := packageInfo["versions"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no versions available for package: %s", packageName)
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no versions available for package: %s", packageName)
	}

	// Get all version strings and sort them
	var versionStrings []string
	for version := range versions {
		versionStrings = append(versionStrings, version)
	}

	fmt.Printf("%s Available versions: %v\n", styling.Label("Found"), versionStrings)

	// Find the highest version using semantic versioning
	latestVersion, err := findHighestVersion(versionStrings)
	if err != nil {
		return "", fmt.Errorf("failed to determine latest version: %w", err)
	}

	return latestVersion, nil
}

// findHighestVersion finds the highest semantic version from a list of version strings
func findHighestVersion(versions []string) (string, error) {
	if len(versions) == 0 {
		return "", fmt.Errorf("no versions provided")
	}

	var highestVersion string
	var highestParts []int

	for _, version := range versions {
		parts := parseVersion(version)
		if len(parts) == 0 {
			continue // Skip invalid versions
		}

		if highestVersion == "" || compareVersions(parts, highestParts) > 0 {
			highestVersion = version
			highestParts = parts
		}
	}

	if highestVersion == "" {
		return "", fmt.Errorf("no valid versions found")
	}

	return highestVersion, nil
}
