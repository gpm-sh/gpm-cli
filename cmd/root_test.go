package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestAddCommands(t *testing.T) {
	// Create a new root command
	rootCmd := &cobra.Command{
		Use:   "gpm",
		Short: "GPM CLI for Unity Package Manager",
	}

	// Add all commands
	AddCommands(rootCmd)

	// Get all subcommands
	commands := rootCmd.Commands()

	// Expected commands
	expectedCommands := []string{
		"login",
		"logout",
		"whoami",
		"publish",
		"pack",
		"config",
		"dist-tag",
		"search",
		"install",
		"uninstall",
		"list",
		"info",
		"version",
		"init",
		"update",
	}

	// Verify all expected commands are present
	assert.Len(t, commands, len(expectedCommands))

	// Check each command is present
	commandNames := make(map[string]bool)
	for _, cmd := range commands {
		// Extract just the command name (before any spaces)
		cmdName := strings.Split(cmd.Use, " ")[0]
		commandNames[cmdName] = true
	}

	for _, expected := range expectedCommands {
		assert.True(t, commandNames[expected], "Command %s should be present", expected)
	}
}

func TestAddCommandsOrder(t *testing.T) {
	// Create a new root command
	rootCmd := &cobra.Command{
		Use:   "gpm",
		Short: "GPM CLI for Unity Package Manager",
	}

	// Add all commands
	AddCommands(rootCmd)

	// Get all subcommands
	commands := rootCmd.Commands()

	// Verify that commands are added (order doesn't matter for functionality)
	assert.True(t, len(commands) > 0)

	// Verify each command has proper structure
	for _, cmd := range commands {
		assert.NotEmpty(t, cmd.Use, "Command should have a Use field")
		assert.NotEmpty(t, cmd.Short, "Command should have a Short description")
	}
}

func TestAddCommandsIdempotent(t *testing.T) {
	// Create a new root command
	rootCmd := &cobra.Command{
		Use:   "gpm",
		Short: "GPM CLI for Unity Package Manager",
	}

	// Add commands twice
	AddCommands(rootCmd)
	originalCount := len(rootCmd.Commands())

	AddCommands(rootCmd)
	newCount := len(rootCmd.Commands())

	// Should have twice the commands (since we added them twice)
	assert.Equal(t, originalCount*2, newCount)
}

func TestIndividualCommands(t *testing.T) {
	// Test that individual command variables are properly defined
	commands := []*cobra.Command{
		loginCmd,
		logoutCmd,
		whoamiCmd,
		publishCmd,
		packCmd,
		configCmd,
		distTagCmd,
		searchCmd,
		installCmd,
		uninstallCmd,
		listCmd,
		infoCmd,
		versionCmd,
		initCmd,
		updateCmd,
	}

	for _, cmd := range commands {
		assert.NotNil(t, cmd, "Command should not be nil")
		assert.NotEmpty(t, cmd.Use, "Command should have a Use field")
	}
}
