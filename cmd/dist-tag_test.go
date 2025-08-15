package cmd

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"gpm.sh/gpm/gpm-cli/internal/config"
)

func TestDistTagCommand(t *testing.T) {
	// Setup temporary config for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()
	_ = os.Setenv("HOME", tempDir)

	// Initialize config
	config.InitConfig()

	t.Run("add tag when logged in", func(t *testing.T) {
		// Set up logged in state
		config.SetToken("test-token")
		config.SetUsername("testuser")

		err := distTagAdd(nil, []string{"test-package", "beta", "1.0.0"})
		assert.NoError(t, err)
	})

	t.Run("add tag when not logged in", func(t *testing.T) {
		// Ensure not logged in
		config.SetToken("")

		err := distTagAdd(nil, []string{"test-package", "beta", "1.0.0"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not logged in")
	})

	t.Run("remove tag when logged in", func(t *testing.T) {
		// Set up logged in state
		config.SetToken("test-token")
		config.SetUsername("testuser")

		err := distTagRemove(nil, []string{"test-package", "beta"})
		assert.NoError(t, err)
	})

	t.Run("remove tag when not logged in", func(t *testing.T) {
		// Ensure not logged in
		config.SetToken("")

		err := distTagRemove(nil, []string{"test-package", "beta"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not logged in")
	})

	t.Run("list tags", func(t *testing.T) {
		// List doesn't require authentication
		err := distTagList(nil, []string{"test-package"})
		assert.NoError(t, err)
	})
}

func TestDistTagCmdStructure(t *testing.T) {
	// Test main command structure
	assert.NotNil(t, distTagCmd)
	assert.Equal(t, "dist-tag", distTagCmd.Use)
	assert.Equal(t, "Manage distribution tags", distTagCmd.Short)
	assert.NotEmpty(t, distTagCmd.Long)
	assert.True(t, distTagCmd.HasSubCommands())

	// Test subcommands
	commands := distTagCmd.Commands()
	assert.Len(t, commands, 3)

	// Find each subcommand
	var addCmd, removeCmd, listCmd *cobra.Command
	for _, cmd := range commands {
		switch cmd.Use {
		case "add <package> <tag> <version>":
			addCmd = cmd
		case "remove <package> <tag>":
			removeCmd = cmd
		case "list <package>":
			listCmd = cmd
		}
	}

	// Verify add command
	assert.NotNil(t, addCmd)
	assert.Equal(t, "Add a distribution tag", addCmd.Short)
	assert.NotNil(t, addCmd.RunE)

	// Verify remove command
	assert.NotNil(t, removeCmd)
	assert.Equal(t, "Remove a distribution tag", removeCmd.Short)
	assert.NotNil(t, removeCmd.RunE)

	// Verify list command
	assert.NotNil(t, listCmd)
	assert.Equal(t, "List distribution tags", listCmd.Short)
	assert.NotNil(t, listCmd.RunE)
}

func TestDistTagArguments(t *testing.T) {
	// Setup temporary config for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()
	_ = os.Setenv("HOME", tempDir)

	// Initialize config
	config.InitConfig()
	config.SetToken("test-token")

	t.Run("add with valid arguments", func(t *testing.T) {
		err := distTagAdd(nil, []string{"com.test.package", "latest", "1.0.0"})
		assert.NoError(t, err)
	})

	t.Run("remove with valid arguments", func(t *testing.T) {
		err := distTagRemove(nil, []string{"com.test.package", "latest"})
		assert.NoError(t, err)
	})

	t.Run("list with valid arguments", func(t *testing.T) {
		err := distTagList(nil, []string{"com.test.package"})
		assert.NoError(t, err)
	})
}

func TestDistTagAuth(t *testing.T) {
	// Setup temporary config for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()
	_ = os.Setenv("HOME", tempDir)

	// Initialize config
	config.InitConfig()

	t.Run("add requires authentication", func(t *testing.T) {
		config.SetToken("")
		err := distTagAdd(nil, []string{"package", "tag", "1.0.0"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not logged in")
	})

	t.Run("remove requires authentication", func(t *testing.T) {
		config.SetToken("")
		err := distTagRemove(nil, []string{"package", "tag"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not logged in")
	})

	t.Run("list does not require authentication", func(t *testing.T) {
		config.SetToken("")
		err := distTagList(nil, []string{"package"})
		assert.NoError(t, err)
	})
}
