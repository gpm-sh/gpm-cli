package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/api"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/styling"
	"gpm.sh/gpm/gpm-cli/internal/validation"
)

var updateCmd = &cobra.Command{
	Use:   "update [package...]",
	Short: "Update packages to their latest versions",
	Long: `Update installed packages to their latest versions.

If no package names are specified, all packages in the dependencies
will be updated to their latest versions.

Examples:
  gpm update                    # Update all packages
  gpm update com.company.pkg    # Update specific package
  gpm update pkg1 pkg2          # Update multiple packages`,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().Bool("save", true, "Save updated versions to package.json")
	updateCmd.Flags().Bool("global", false, "Update global packages")
	updateCmd.Flags().Bool("dry-run", false, "Show what would be updated without making changes")
	updateCmd.Flags().String("registry", "", "Use specific registry")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	save, _ := cmd.Flags().GetBool("save")
	global, _ := cmd.Flags().GetBool("global")

	if global {
		return fmt.Errorf("%s", styling.Error("global package updates not yet implemented"))
	}

	fmt.Println(styling.Header("📦 Updating Packages"))

	packageJSON, err := readPackageJSONUpdate(".")
	if err != nil {
		return fmt.Errorf("%s", styling.Error("failed to read package.json: "+err.Error()))
	}

	dependencies := packageJSON.Dependencies
	if dependencies == nil {
		dependencies = make(map[string]string)
	}

	var packagesToUpdate []string
	if len(args) > 0 {
		packagesToUpdate = args
	} else {
		for pkg := range dependencies {
			packagesToUpdate = append(packagesToUpdate, pkg)
		}
	}

	if len(packagesToUpdate) == 0 {
		fmt.Println(styling.Info("No packages to update"))
		return nil
	}

	client := api.NewClient(config.GetConfig().Registry, config.GetToken())
	updates := make(map[string]string)

	for _, pkgName := range packagesToUpdate {
		if err := validation.ValidatePackageName(pkgName); err != nil {
			fmt.Printf("%s Invalid package name: %s\n", styling.Warning("⚠"), pkgName)
			continue
		}

		currentVersion, exists := dependencies[pkgName]
		if !exists && len(args) > 0 {
			fmt.Printf("%s Package not found in dependencies: %s\n", styling.Warning("⚠"), pkgName)
			continue
		}

		latestInfo, err := client.GetPackageInfo(pkgName, "latest")
		if err != nil {
			fmt.Printf("%s Failed to get info for %s: %v\n", styling.Error("✗"), pkgName, err)
			continue
		}

		latestVersion := latestInfo.Version
		if currentVersion == latestVersion {
			fmt.Printf("%s %s@%s (already up to date)\n", styling.Success("✓"), pkgName, currentVersion)
			continue
		}

		if dryRun {
			fmt.Printf("%s %s@%s → %s (dry run)\n", styling.Info("→"), pkgName, currentVersion, latestVersion)
		} else {
			fmt.Printf("%s %s@%s → %s\n", styling.Success("↗"), pkgName, currentVersion, latestVersion)
			updates[pkgName] = latestVersion
		}
	}

	if dryRun || len(updates) == 0 {
		return nil
	}

	if save {
		for pkgName, version := range updates {
			dependencies[pkgName] = version
		}

		err = writePackageJSONUpdate(packageJSON)
		if err != nil {
			return fmt.Errorf("%s", styling.Error("failed to update package.json: "+err.Error()))
		}

		fmt.Println(styling.Success("✅ package.json updated"))
	}

	fmt.Printf("\n%s Updated %d package(s)\n", styling.Success("🎉"), len(updates))
	return nil
}

type PackageJSONUpdate struct {
	Name         string            `json:"name,omitempty"`
	Version      string            `json:"version,omitempty"`
	Description  string            `json:"description,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

func readPackageJSONUpdate(dir string) (*PackageJSONUpdate, error) {
	cleanDir := filepath.Clean(dir)
	if cleanDir == "." {
		cleanDir, _ = os.Getwd()
	}
	packagePath := filepath.Join(cleanDir, "package.json")
	cleanPackagePath := filepath.Clean(packagePath)

	if !strings.HasPrefix(cleanPackagePath, cleanDir) {
		return nil, fmt.Errorf("invalid path: potential directory traversal")
	}

	data, err := os.ReadFile(cleanPackagePath)
	if err != nil {
		return nil, err
	}

	var pkg PackageJSONUpdate
	err = json.Unmarshal(data, &pkg)
	if err != nil {
		return nil, err
	}

	return &pkg, nil
}

func writePackageJSONUpdate(pkg *PackageJSONUpdate) error {
	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile("package.json", data, 0600)
}
