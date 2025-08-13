package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

var (
	uninstallSave    bool
	uninstallSaveDev bool
	uninstallGlobal  bool
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <package>",
	Short: "Uninstall a package",
	Long: `Remove a package from the current project.

Examples:
  gpm uninstall com.unity.ugui
  gpm uninstall com.company.package --save
  gpm uninstall com.company.test-utils --save-dev`,
	Args: cobra.ExactArgs(1),
	RunE: uninstall,
}

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallSave, "save", false, "Remove from package.json dependencies")
	uninstallCmd.Flags().BoolVar(&uninstallSaveDev, "save-dev", false, "Remove from package.json devDependencies")
	uninstallCmd.Flags().BoolVarP(&uninstallGlobal, "global", "g", false, "Uninstall global package")
}

func uninstall(cmd *cobra.Command, args []string) error {
	packageName := args[0]

	fmt.Println(styling.Header("🗑️   Package Removal"))
	fmt.Println(styling.Separator())

	if uninstallGlobal {
		return uninstallGlobalPackage(packageName)
	}

	return uninstallLocalPackage(packageName)
}

func uninstallLocalPackage(packageName string) error {
	fmt.Printf("%s %s\n", styling.Label("Removing:"), styling.Package(packageName))

	packagesDir := "Packages"

	// Security: Validate package name format
	if !isValidPackageName(packageName) {
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Invalid package name: "+packageName),
			styling.Hint("Package names should follow reverse-DNS convention"))
	}

	manifestPath := filepath.Join(packagesDir, packageName+".json")

	// Security: Ensure the path is still within the packages directory
	cleanPath := filepath.Clean(manifestPath)
	if !strings.HasPrefix(cleanPath, packagesDir) {
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Invalid package path"),
			styling.Hint("Package path must be within the Packages directory"))
	}

	// Check if package is installed
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Package not installed: "+packageName),
			styling.Hint("Use 'gpm list' to see installed packages"))
	}

	// Read package manifest to get version info
	var packageVersion string
	if data, err := os.ReadFile(cleanPath); err == nil {
		var manifest struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(data, &manifest) == nil {
			packageVersion = manifest.Version
		}
	}

	// Remove the manifest file
	if err := os.Remove(manifestPath); err != nil {
		return fmt.Errorf("failed to remove package manifest: %w", err)
	}

	// Update package.json if requested
	if uninstallSave || uninstallSaveDev {
		if err := removeFromPackageJSON(packageName, uninstallSaveDev); err != nil {
			fmt.Printf("%s\n", styling.Warning("Package removed but failed to update package.json: "+err.Error()))
		}
	}

	fmt.Println()
	fmt.Printf("%s %s", styling.Success("✓ Successfully removed"), styling.Package(packageName))
	if packageVersion != "" {
		fmt.Printf("@%s", styling.Version(packageVersion))
	}
	fmt.Println()
	fmt.Println(styling.Separator())

	return nil
}

func uninstallGlobalPackage(packageName string) error {
	return fmt.Errorf("%s\n\n%s",
		styling.Error("Global package removal not yet supported"),
		styling.Hint("Use local package removal instead: 'gpm uninstall "+packageName+"'"))
}

func removeFromPackageJSON(packageName string, fromDevDeps bool) error {
	packageJSONPath := "package.json"

	// Check if package.json exists
	if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
		return fmt.Errorf("package.json not found")
	}

	// Read package.json
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return fmt.Errorf("invalid package.json: %w", err)
	}

	// Remove from dependencies or devDependencies
	depKey := "dependencies"
	if fromDevDeps {
		depKey = "devDependencies"
	}

	if deps, exists := pkg[depKey].(map[string]interface{}); exists {
		delete(deps, packageName)

		// Remove empty dependency sections
		if len(deps) == 0 {
			delete(pkg, depKey)
		}
	}

	// Write back to file
	updatedData, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal package.json: %w", err)
	}

	return os.WriteFile(packageJSONPath, updatedData, 0600)
}

// isValidPackageName checks if a package name follows valid naming conventions
func isValidPackageName(name string) bool {
	// Basic validation for reverse-DNS format or npm-compatible names
	// Allow letters, numbers, dots, hyphens, and underscores
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '.' || char == '-' || char == '_') {
			return false
		}
	}

	// Must not be empty and must not start/end with special characters
	if len(name) == 0 || name[0] == '.' || name[0] == '-' ||
		name[len(name)-1] == '.' || name[len(name)-1] == '-' {
		return false
	}

	return true
}
