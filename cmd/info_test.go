package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gpm.sh/gpm/gpm-cli/internal/config"
)

func TestInfoCommand(t *testing.T) {
	// Setup temporary config for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()
	_ = os.Setenv("HOME", tempDir)

	// Initialize config
	config.InitConfig()

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test-package" {
			mockResponse := map[string]interface{}{
				"name":        "test-package",
				"description": "A test package",
				"version":     "1.0.0",
				"author": map[string]interface{}{
					"name":  "Test Author",
					"email": "test@example.com",
				},
				"license":    "MIT",
				"homepage":   "https://example.com",
				"repository": "https://github.com/test/test-package",
				"keywords":   []string{"test", "unity"},
				"versions": map[string]interface{}{
					"1.0.0": map[string]interface{}{
						"version": "1.0.0",
						"dist": map[string]interface{}{
							"tarball": "https://example.com/test-package-1.0.0.tgz",
						},
					},
				},
				"dist-tags": map[string]interface{}{
					"latest": "1.0.0",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockResponse)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Set test registry
	config.SetRegistry(server.URL)

	t.Run("successful info", func(t *testing.T) {
		err := info(nil, []string{"test-package"})
		assert.NoError(t, err)
	})

	t.Run("package not found", func(t *testing.T) {
		err := info(nil, []string{"nonexistent-package"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestInfoCmdStructure(t *testing.T) {
	// Test command structure
	assert.NotNil(t, infoCmd)
	assert.Equal(t, "info <package>", infoCmd.Use)
	assert.Equal(t, "Show package information", infoCmd.Short)
	assert.NotEmpty(t, infoCmd.Long)
	assert.NotNil(t, infoCmd.RunE)
	assert.False(t, infoCmd.HasSubCommands())
}
