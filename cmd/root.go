package cmd

import (
	"github.com/spf13/cobra"
)

func AddCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(registerCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(whoamiCmd)
	rootCmd.AddCommand(publishCmd)
	rootCmd.AddCommand(packCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(distTagCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(versionCmd)
}
