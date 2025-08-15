package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gpm.sh/gpm/gpm-cli/internal/config"
)

func TestLogoutCommand(t *testing.T) {
	// Setup temporary config for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()
	_ = os.Setenv("HOME", tempDir)

	// Initialize config
	config.InitConfig()

	t.Run("logout when logged in", func(t *testing.T) {
		// Set up logged in state
		config.SetToken("test-token")
		config.SetUsername("testuser")

		// Verify initial state
		cfg := config.GetConfig()
		assert.NotEmpty(t, cfg.Token)
		assert.NotEmpty(t, cfg.Username)

		// Logout
		err := logout(nil, []string{})
		assert.NoError(t, err)

		// Verify logout
		cfg = config.GetConfig()
		assert.Empty(t, cfg.Token)
		assert.Empty(t, cfg.Username)
	})

	t.Run("logout when not logged in", func(t *testing.T) {
		// Ensure clean state
		config.SetToken("")
		config.SetUsername("")

		// Try to logout
		err := logout(nil, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not logged in")
	})

	t.Run("logout clears both token and username", func(t *testing.T) {
		// Set up logged in state
		config.SetToken("another-test-token")
		config.SetUsername("anotheruser")

		// Verify setup
		cfg := config.GetConfig()
		assert.Equal(t, "another-test-token", cfg.Token)
		assert.Equal(t, "anotheruser", cfg.Username)

		// Logout
		err := logout(nil, []string{})
		assert.NoError(t, err)

		// Verify both are cleared
		cfg = config.GetConfig()
		assert.Empty(t, cfg.Token)
		assert.Empty(t, cfg.Username)
	})
}

func TestLogoutCmdStructure(t *testing.T) {
	// Test command structure
	assert.NotNil(t, logoutCmd)
	assert.Equal(t, "logout", logoutCmd.Use)
	assert.Equal(t, "Logout from the GPM registry", logoutCmd.Short)
	assert.NotEmpty(t, logoutCmd.Long)
	assert.NotNil(t, logoutCmd.RunE)
	assert.False(t, logoutCmd.HasSubCommands())
}

func TestLogoutPersistence(t *testing.T) {
	// Setup temporary config for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()
	_ = os.Setenv("HOME", tempDir)

	// Initialize config
	config.InitConfig()

	// Set up logged in state
	config.SetToken("persistent-token")
	config.SetUsername("persistentuser")
	_ = config.SaveConfig()

	// Verify initial state
	cfg := config.GetConfig()
	assert.NotEmpty(t, cfg.Token)
	assert.NotEmpty(t, cfg.Username)

	// Logout
	err := logout(nil, []string{})
	assert.NoError(t, err)

	// Reinitialize config (simulating restart)
	config.InitConfig()

	// Verify logout persisted
	cfg = config.GetConfig()
	assert.Empty(t, cfg.Token)
	assert.Empty(t, cfg.Username)
}
