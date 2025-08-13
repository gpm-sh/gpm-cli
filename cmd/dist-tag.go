package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

var distTagCmd = &cobra.Command{
	Use:   "dist-tag",
	Short: "Manage distribution tags",
	Long:  `Add, remove, and list distribution tags for packages.`,
}

var distTagAddCmd = &cobra.Command{
	Use:   "add <package> <tag> <version>",
	Short: "Add a distribution tag",
	Args:  cobra.ExactArgs(3),
	RunE:  distTagAdd,
}

var distTagRemoveCmd = &cobra.Command{
	Use:   "remove <package> <tag>",
	Short: "Remove a distribution tag",
	Args:  cobra.ExactArgs(2),
	RunE:  distTagRemove,
}

var distTagListCmd = &cobra.Command{
	Use:   "list <package>",
	Short: "List distribution tags",
	Args:  cobra.ExactArgs(1),
	RunE:  distTagList,
}

func init() {
	distTagCmd.AddCommand(distTagAddCmd)
	distTagCmd.AddCommand(distTagRemoveCmd)
	distTagCmd.AddCommand(distTagListCmd)
}

func distTagAdd(cmd *cobra.Command, args []string) error {
	packageName := args[0]
	tag := args[1]
	version := args[2]

	cfg := config.GetConfig()
	if cfg.Token == "" {
		return fmt.Errorf("%s", styling.Error("not logged in. Run 'gpm login' first"))
	}

	fmt.Println(styling.Header("🏷️  Adding Distribution Tag"))
	fmt.Println(styling.Separator())
	fmt.Printf("%s %s\n", styling.Label("Package:"), styling.Package(packageName))
	fmt.Printf("%s %s\n", styling.Label("Tag:"), styling.Value(tag))
	fmt.Printf("%s %s\n", styling.Label("Version:"), styling.Version(version))
	fmt.Println(styling.Separator())
	fmt.Println(styling.Warning("⚠️  API endpoint not yet implemented"))
	fmt.Println(styling.Muted("This feature is coming soon..."))

	return nil
}

func distTagRemove(cmd *cobra.Command, args []string) error {
	packageName := args[0]
	tag := args[1]

	cfg := config.GetConfig()
	if cfg.Token == "" {
		return fmt.Errorf("%s", styling.Error("not logged in. Run 'gpm login' first"))
	}

	fmt.Println(styling.Header("🗑️  Removing Distribution Tag"))
	fmt.Println(styling.Separator())
	fmt.Printf("%s %s\n", styling.Label("Package:"), styling.Package(packageName))
	fmt.Printf("%s %s\n", styling.Label("Tag:"), styling.Value(tag))
	fmt.Println(styling.Separator())
	fmt.Println(styling.Warning("⚠️  API endpoint not yet implemented"))
	fmt.Println(styling.Muted("This feature is coming soon..."))

	return nil
}

func distTagList(cmd *cobra.Command, args []string) error {
	packageName := args[0]

	fmt.Println(styling.Header("📋 Distribution Tags"))
	fmt.Println(styling.Separator())
	fmt.Printf("%s %s\n", styling.Label("Package:"), styling.Package(packageName))
	fmt.Println(styling.Separator())
	fmt.Println(styling.Warning("⚠️  API endpoint not yet implemented"))
	fmt.Println(styling.Muted("This feature is coming soon..."))

	return nil
}
