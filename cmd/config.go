package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage GPM configuration",
	Long:  `View and modify GPM configuration settings`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return showConfig()
	},
}

var (
	configSetCmd = &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a configuration value",
		Long:  `Set a configuration key to a specific value`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setConfig(args[0], args[1])
		},
	}

	configGetCmd = &cobra.Command{
		Use:   "get [key]",
		Short: "Get a configuration value",
		Long:  `Get the value of a configuration key`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return getConfig(args[0])
		},
	}
)

func init() {
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
}

func showConfig() error {
	cfg := config.GetConfig()

	fmt.Println(styling.Header("GPM Configuration"))
	fmt.Println(styling.Separator())
	fmt.Printf("%s %s\n", styling.Label("Registry:"), styling.URL(cfg.Registry))
	fmt.Printf("%s %s\n", styling.Label("Username:"), styling.Value(cfg.Username))

	if cfg.Token != "" {
		fmt.Printf("%s %s\n", styling.Label("Token:"), styling.Muted(cfg.Token[:20]+"..."))
	} else {
		fmt.Printf("%s %s\n", styling.Label("Token:"), styling.Warning("Not set"))
	}

	return nil
}

func setConfig(key, value string) error {
	switch key {
	case "registry":
		config.SetRegistry(value)
		fmt.Printf("%s %s\n", styling.Success("Registry set to:"), styling.Value(value))
	case "token":
		config.SetToken(value)
		fmt.Printf("%s\n", styling.Success("Token updated successfully"))
	case "username":
		config.SetUsername(value)
		fmt.Printf("%s %s\n", styling.Success("Username set to:"), styling.Value(value))
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}

	return config.SaveConfig()
}

func getConfig(key string) error {
	cfg := config.GetConfig()

	switch key {
	case "registry":
		fmt.Printf("%s\n", styling.Value(cfg.Registry))
	case "token":
		if cfg.Token != "" {
			fmt.Printf("%s\n", styling.Value(cfg.Token))
		} else {
			fmt.Printf("%s\n", styling.Warning("Not set"))
		}
	case "username":
		fmt.Printf("%s\n", styling.Value(cfg.Username))
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}

	return nil
}
