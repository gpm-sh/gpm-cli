package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

var (
	listGlobal     bool
	listProduction bool
	listDev        bool
	listDepth      int
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed packages",
	Long: `List packages installed in the current project or globally.

Examples:
  gpm list                    # List all installed packages
  gpm list --production       # List only production dependencies
  gpm list --dev              # List only development dependencies
  gpm list --global           # List globally installed packages`,
	RunE: list,
}

func init() {
	listCmd.Flags().BoolVarP(&listGlobal, "global", "g", false, "List globally installed packages")
	listCmd.Flags().BoolVar(&listProduction, "production", false, "List only production dependencies")
	listCmd.Flags().BoolVar(&listDev, "dev", false, "List only development dependencies")
	listCmd.Flags().IntVar(&listDepth, "depth", 1, "Maximum depth of dependency tree to show")
}

func list(cmd *cobra.Command, args []string) error {
	fmt.Println(styling.Header("📋  Installed Packages"))
	fmt.Println(styling.Separator())

	if listGlobal {
		return listGlobalPackages()
	}

	return listLocalPackages()
}

func listLocalPackages() error {
	packagesDir := "Packages"

	// Check if Packages directory exists
	if _, err := os.Stat(packagesDir); os.IsNotExist(err) {
		fmt.Printf("%s\n\n%s\n",
			styling.Warning("No packages directory found"),
			styling.Hint("Run 'gpm install <package>' to install packages"))
		return nil
	}

	// Read package.json to get declared dependencies
	var declaredDeps map[string]string
	var declaredDevDeps map[string]string

	if data, err := os.ReadFile("package.json"); err == nil {
		var pkg struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			declaredDeps = pkg.Dependencies
			declaredDevDeps = pkg.DevDependencies
		}
	}

	// List installed packages
	var prodPackages, devPackages []string

	err := filepath.WalkDir(packagesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		// Validate path is within the expected directory structure
		cleanPath := filepath.Clean(path)
		if !strings.HasPrefix(cleanPath, packagesDir) {
			return nil
		}

		data, err := os.ReadFile(cleanPath)
		if err != nil {
			return nil
		}

		var manifest struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			IsDev   bool   `json:"isDev"`
		}

		if json.Unmarshal(data, &manifest) != nil {
			return nil
		}

		packageInfo := fmt.Sprintf("%s@%s", manifest.Name, manifest.Version)

		if manifest.IsDev {
			devPackages = append(devPackages, packageInfo)
		} else {
			prodPackages = append(prodPackages, packageInfo)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to read packages directory: %w", err)
	}

	// Display results
	if !listDev && len(prodPackages) > 0 {
		fmt.Println(styling.SubHeader("Production Dependencies:"))
		for _, pkg := range prodPackages {
			parts := strings.Split(pkg, "@")
			name, version := parts[0], parts[1]

			status := ""
			if declaredVersion, exists := declaredDeps[name]; exists {
				if declaredVersion == version {
					status = styling.Success(" ✓")
				} else {
					status = styling.Warning(fmt.Sprintf(" (declared: %s)", declaredVersion))
				}
			} else {
				status = styling.Muted(" (not in package.json)")
			}

			fmt.Printf("  %s@%s%s\n", styling.Package(name), styling.Version(version), status)
		}
		fmt.Println()
	}

	if !listProduction && len(devPackages) > 0 {
		fmt.Println(styling.SubHeader("Development Dependencies:"))
		for _, pkg := range devPackages {
			parts := strings.Split(pkg, "@")
			name, version := parts[0], parts[1]

			status := ""
			if declaredVersion, exists := declaredDevDeps[name]; exists {
				if declaredVersion == version {
					status = styling.Success(" ✓")
				} else {
					status = styling.Warning(fmt.Sprintf(" (declared: %s)", declaredVersion))
				}
			} else {
				status = styling.Muted(" (not in package.json)")
			}

			fmt.Printf("  %s@%s%s\n", styling.Package(name), styling.Version(version), status)
		}
		fmt.Println()
	}

	if len(prodPackages) == 0 && len(devPackages) == 0 {
		fmt.Printf("%s\n\n%s\n",
			styling.Info("No packages installed"),
			styling.Hint("Run 'gpm install <package>' to install packages"))
	}

	fmt.Println(styling.Separator())
	return nil
}

func listGlobalPackages() error {
	fmt.Printf("%s\n\n%s\n",
		styling.Warning("Global package listing not yet supported"),
		styling.Hint("Use local package listing instead: 'gpm list'"))
	return nil
}
