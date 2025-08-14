package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitConfig(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) string
		expected *Config
	}{
		{
			name: "default config when no file exists",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				return tmpDir
			},
			expected: &Config{
				Registry: "https://gpm.sh",
				Token:    "",
				Username: "",
			},
		},
		{
			name: "loads config from file",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				configContent := `registry: "https://custom.gpm.sh"
token: "test-token"
username: "testuser"`
				configFile := filepath.Join(tmpDir, ".gpmrc")
				require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0644))
				return tmpDir
			},
			expected: &Config{
				Registry: "https://custom.gpm.sh",
				Token:    "test-token",
				Username: "testuser",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			tmpDir := tt.setup(t)
			oldHome := os.Getenv("HOME")
			_ = os.Setenv("HOME", tmpDir)
			defer func() { _ = os.Setenv("HOME", oldHome) }()

			// Reset global state
			config = nil
			viper.Reset()

			// Test
			InitConfig()
			result := GetConfig()

			// Assert
			assert.Equal(t, tt.expected.Registry, result.Registry)
			assert.Equal(t, tt.expected.Token, result.Token)
			assert.Equal(t, tt.expected.Username, result.Username)
		})
	}
}

func TestConfigSetters(t *testing.T) {
	// Reset global state
	config = nil
	viper.Reset()

	// Initialize with defaults
	InitConfig()

	// Test setters
	SetRegistry("https://test.gpm.sh")
	SetToken("new-token")
	SetUsername("newuser")

	cfg := GetConfig()
	assert.Equal(t, "https://test.gpm.sh", cfg.Registry)
	assert.Equal(t, "new-token", cfg.Token)
	assert.Equal(t, "newuser", cfg.Username)

	// Test getters
	assert.Equal(t, "https://test.gpm.sh", GetRegistry())
	assert.Equal(t, "new-token", GetToken())
	assert.Equal(t, "newuser", GetUsername())
}
