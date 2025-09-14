package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gpm.sh/gpm/gpm-cli/internal/config"
)

func TestConfigCommand(t *testing.T) {
	// Setup temporary config for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		_ = os.Setenv("HOME", originalHome)
		config.ResetConfigForTesting()
	}()
	_ = os.Setenv("HOME", tempDir)

	// Initialize config
	config.InitConfig()

	t.Run("show config", func(t *testing.T) {
		err := showConfig()
		assert.NoError(t, err)
	})

	t.Run("set registry", func(t *testing.T) {
		testRegistry := "https://test-registry.example.com"
		err := setConfig("registry", testRegistry)
		assert.NoError(t, err)

		cfg := config.GetConfig()
		assert.Equal(t, testRegistry, cfg.Registry)
	})

	t.Run("get registry", func(t *testing.T) {
		err := getConfig("registry")
		assert.NoError(t, err)
	})

	t.Run("set username", func(t *testing.T) {
		testUsername := "testuser"
		err := setConfig("username", testUsername)
		assert.NoError(t, err)

		cfg := config.GetConfig()
		assert.Equal(t, testUsername, cfg.Username)
	})

	t.Run("get username", func(t *testing.T) {
		err := getConfig("username")
		assert.NoError(t, err)
	})

	t.Run("set token", func(t *testing.T) {
		testToken := "test-token-12345"
		err := setConfig("token", testToken)
		assert.NoError(t, err)

		cfg := config.GetConfig()
		assert.Equal(t, testToken, cfg.Token)
	})

	t.Run("get token", func(t *testing.T) {
		err := getConfig("token")
		assert.NoError(t, err)
	})

	t.Run("set invalid key", func(t *testing.T) {
		err := setConfig("invalid", "value")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown configuration key")
	})

	t.Run("get invalid key", func(t *testing.T) {
		err := getConfig("invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown configuration key")
	})

	t.Run("get empty token", func(t *testing.T) {
		// Reset token
		config.SetToken("")
		err := getConfig("token")
		assert.NoError(t, err)
	})
}

func TestConfigCmdIntegration(t *testing.T) {
	// Setup temporary config for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		_ = os.Setenv("HOME", originalHome)
		config.ResetConfigForTesting()
	}()
	_ = os.Setenv("HOME", tempDir)

	// Test subcommands exist
	assert.NotNil(t, configCmd)
	assert.Equal(t, "config", configCmd.Use)
	assert.True(t, configCmd.HasSubCommands())

	// Test subcommand structure
	commands := configCmd.Commands()
	var setCmd, getCmd *cobra.Command

	for _, cmd := range commands {
		if strings.HasPrefix(cmd.Use, "set") {
			setCmd = cmd
		} else if strings.HasPrefix(cmd.Use, "get") {
			getCmd = cmd
		}
	}

	assert.NotNil(t, setCmd)
	assert.NotNil(t, getCmd)
	assert.Equal(t, "set [key] [value]", setCmd.Use)
	assert.Equal(t, "get [key]", getCmd.Use)
}

func TestConfigPersistence(t *testing.T) {
	// Setup temporary config for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		_ = os.Setenv("HOME", originalHome)
		config.ResetConfigForTesting()
	}()
	_ = os.Setenv("HOME", tempDir)

	// Test that config values can be set and retrieved
	config.InitConfig()

	// Set some values
	testRegistry := "https://registry.gpm.sh"
	testUsername := "persistentuser"

	err := setConfig("registry", testRegistry)
	require.NoError(t, err)

	err = setConfig("username", testUsername)
	require.NoError(t, err)

	// Verify values were set
	cfg := config.GetConfig()
	assert.Equal(t, testRegistry, cfg.Registry)
	assert.Equal(t, testUsername, cfg.Username)
}
