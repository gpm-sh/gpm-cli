package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from the GPM registry",
	Long:  `Clear your authentication token from the local configuration.`,
	RunE:  logout,
}

func logout(cmd *cobra.Command, args []string) error {
	cfg := config.GetConfig()

	if cfg.Token == "" {
		return fmt.Errorf("%s", styling.Error("not logged in"))
	}

	config.SetToken("")
	config.SetUsername("")

	if err := config.SaveConfig(); err != nil {
		return fmt.Errorf("%s", styling.Error("failed to save config: "+err.Error()))
	}

	fmt.Println(styling.Separator())
	fmt.Println(styling.Success("✓ Successfully logged out"))
	fmt.Printf("%s %s\n", styling.Label("Status:"), styling.Value("Token removed from ~/.gpmrc"))
	fmt.Println(styling.Separator())

	return nil
}
