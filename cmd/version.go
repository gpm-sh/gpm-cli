package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

var (
	Version = "v0.1.0-alpha.9"
	Commit  = "unknown"
	Date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show GPM CLI version",
	Long:  `Display version information for the GPM CLI.`,
	Run:   version,
}

func version(cmd *cobra.Command, args []string) {
	fmt.Println(styling.Header("🚀  GPM CLI"))
	fmt.Println(styling.Separator())

	fmt.Printf("%s %s\n", styling.Label("Version:"), styling.Version(Version))
	fmt.Printf("%s %s\n", styling.Label("Commit:"), styling.Hash(Commit))
	fmt.Printf("%s %s\n", styling.Label("Built:"), styling.Value(Date))

	fmt.Printf("%s %s\n", styling.Label("Go Version:"), styling.Value(runtime.Version()))
	fmt.Printf("%s %s\n", styling.Label("Platform:"), styling.Value(runtime.GOOS+"/"+runtime.GOARCH))

	fmt.Println(styling.Separator())
}
