package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/api"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

var (
	installGlobal  bool
	installVersion string
	installSave    bool
	installSaveDev bool
)

var installCmd = &cobra.Command{
	Use:   "install [package[@version]]",
	Short: "Install a package",
	Long: `Install a package from the GPM registry.

Examples:
  gpm install com.unity.ugui
  gpm install com.unity.ugui@1.0.0
  gpm install --save com.company.package
  gpm install --save-dev com.company.test-utils`,
	Args: cobra.MaximumNArgs(1),
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

	packageSpec := args[0]
	packageName, version := parsePackageSpec(packageSpec)

	if installVersion != "" {
		version = installVersion
	}

	fmt.Println(styling.Header("📦  Package Installation"))
	fmt.Println(styling.Separator())

	if installGlobal {
		return installGlobalPackage(packageName, version)
	}

	return installLocalPackage(packageName, version)
}

func parsePackageSpec(spec string) (name, version string) {
	if strings.Contains(spec, "@") {
		parts := strings.Split(spec, "@")
		name = parts[0]
		if len(parts) > 1 {
			version = parts[1]
		}
	} else {
		name = spec
		version = "latest"
	}
	return
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

func installGlobalPackage(packageName, version string) error {
	return fmt.Errorf("%s\n\n%s",
		styling.Error("Global package installation not yet supported"),
		styling.Hint("Use local installation instead: 'gpm install "+packageName+"'"))
}

func installLocalPackage(packageName, version string) error {
	fmt.Printf("%s %s@%s\n", styling.Label("Installing:"), styling.Package(packageName), styling.Version(version))

	if err := downloadAndInstallPackage(packageName, version, installSaveDev); err != nil {
		return err
	}

	if installSave || installSaveDev {
		if err := updatePackageJSON(packageName, version, installSaveDev); err != nil {
			fmt.Printf("%s\n", styling.Warning("Package installed but failed to update package.json: "+err.Error()))
		}
	}

	fmt.Println(styling.Separator())
	fmt.Printf("%s %s@%s\n", styling.Success("✓ Successfully installed"), styling.Package(packageName), styling.Version(version))
	fmt.Println(styling.Separator())

	return nil
}

func downloadAndInstallPackage(packageName, version string, isDev bool) error {
	cfg := config.GetConfig()
	_ = api.NewClient(cfg.Registry, cfg.Token)

	// Create Packages directory if it doesn't exist
	packagesDir := "Packages"
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		return fmt.Errorf("failed to create Packages directory: %w", err)
	}

	// Download package metadata
	url := fmt.Sprintf("%s/%s", cfg.Registry, packageName)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch package metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("package not found: %s", packageName)
	}

	// For now, create a manifest file indicating the package is "installed"
	// In a real implementation, this would download and extract the tarball
	manifestPath := filepath.Join(packagesDir, packageName+".json")
	manifest := map[string]interface{}{
		"name":    packageName,
		"version": version,
		"isDev":   isDev,
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	return os.WriteFile(manifestPath, manifestData, 0644)
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

	return os.WriteFile(packageJSONPath, updatedData, 0644)
}
