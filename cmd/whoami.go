package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/api"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current user information",
	Long:  `Display information about the currently authenticated user`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return whoami()
	},
}

func whoami() error {
	cfg := config.GetConfig()
	if cfg.Token == "" {
		return fmt.Errorf("not authenticated. Please run 'gpm login' first")
	}

	client := api.NewClient(cfg.Registry, cfg.Token)

	fmt.Println(styling.Info("Fetching user information..."))

	resp, err := client.Whoami()
	if err != nil {
		return fmt.Errorf("failed to get user info: %v", err)
	}

	fmt.Println(styling.Header("User Information"))
	fmt.Println(styling.Separator())
	fmt.Printf("%s %s\n", styling.Label("Username:"), styling.Value(resp.Username))

	fmt.Println(styling.Separator())
	return nil
}
