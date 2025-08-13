package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/cmd"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

var (
	Verbose    = false
	Debug      = false
	Quiet      = false
	JSONOutput = false
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "gpm",
		Short: "GPM.sh - Game Package Manager CLI",
		Long: `GPM.sh CLI - A game-dev package registry with npm-compatible workflows
but explicit, studio-aware rules for Unity and other game engines.

Features:
- Reverse-DNS package naming (UPM-compatible)
- Studio scoping by subdomain
- Explicit visibility controls
- Plan-based publishing permissions`,
		Version: cmd.Version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			setupLogging()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Add global flags
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&Debug, "debug", false, "Enable debug output")
	rootCmd.PersistentFlags().BoolVarP(&Quiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&JSONOutput, "json", false, "Output in JSON format")

	config.InitConfig()

	cmd.AddCommands(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		if !Quiet {
			if JSONOutput {
				fmt.Fprintf(os.Stderr, `{"error":{"message":"%s"}}`+"\n", err.Error())
			} else {
				fmt.Fprintf(os.Stderr, "%s\n", styling.Error(fmt.Sprintf("Error: %v", err)))
			}
		}
		os.Exit(1)
	}
}

func setupLogging() {
	if Quiet {
		log.SetOutput(io.Discard)
	} else {
		log.SetOutput(os.Stderr)
	}
}
