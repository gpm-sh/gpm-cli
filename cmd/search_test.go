package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gpm.sh/gpm/gpm-cli/internal/config"
)

func TestSearchCommand(t *testing.T) {
	// Setup temporary config for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()
	_ = os.Setenv("HOME", tempDir)

	// Initialize config
	config.InitConfig()

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/-/v1/search" {
			mockResponse := map[string]interface{}{
				"objects": []map[string]interface{}{
					{
						"package": map[string]interface{}{
							"name":        "test-package",
							"version":     "1.0.0",
							"description": "A test package for search",
							"keywords":    []string{"test", "unity"},
							"author": map[string]interface{}{
								"name":  "Test Author",
								"email": "test@example.com",
							},
							"license":  "MIT",
							"homepage": "https://example.com",
						},
						"score": map[string]interface{}{
							"final": 0.8,
						},
					},
				},
				"total": 1,
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

	t.Run("successful search", func(t *testing.T) {
		// Reset flags
		searchLimit = 10
		searchDetail = false

		err := search(nil, []string{"unity"})
		assert.NoError(t, err)
	})

	t.Run("successful search with details", func(t *testing.T) {
		// Reset flags
		searchLimit = 10
		searchDetail = true

		err := search(nil, []string{"unity"})
		assert.NoError(t, err)
	})

	t.Run("search with custom limit", func(t *testing.T) {
		// Reset flags
		searchLimit = 5
		searchDetail = false

		err := search(nil, []string{"unity"})
		assert.NoError(t, err)
	})
}

func TestSearchCommandError(t *testing.T) {
	// Setup temporary config for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()
	_ = os.Setenv("HOME", tempDir)

	// Initialize config
	config.InitConfig()

	t.Run("invalid registry URL", func(t *testing.T) {
		config.SetRegistry("invalid-url")

		err := search(nil, []string{"unity"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid registry URL")
	})

	t.Run("server error", func(t *testing.T) {
		// Create mock server that returns error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		config.SetRegistry(server.URL)

		err := search(nil, []string{"unity"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Search failed")
	})

	t.Run("no results", func(t *testing.T) {
		// Create mock server with empty results
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mockResponse := map[string]interface{}{
				"objects": []map[string]interface{}{},
				"total":   0,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockResponse)
		}))
		defer server.Close()

		config.SetRegistry(server.URL)

		err := search(nil, []string{"nonexistent"})
		assert.NoError(t, err)
	})
}

func TestSearchCmdStructure(t *testing.T) {
	// Test command structure
	assert.NotNil(t, searchCmd)
	assert.Equal(t, "search <term>", searchCmd.Use)
	assert.Equal(t, "Search for packages", searchCmd.Short)
	assert.NotEmpty(t, searchCmd.Long)
	assert.NotNil(t, searchCmd.RunE)
	assert.False(t, searchCmd.HasSubCommands())

	// Test flags
	flags := searchCmd.Flags()
	assert.True(t, flags.HasFlags())

	limitFlag := flags.Lookup("limit")
	assert.NotNil(t, limitFlag)
	assert.Equal(t, "10", limitFlag.DefValue)

	detailFlag := flags.Lookup("detail")
	assert.NotNil(t, detailFlag)
	assert.Equal(t, "false", detailFlag.DefValue)
}

func TestSearchFlags(t *testing.T) {
	// Setup temporary config for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()
	_ = os.Setenv("HOME", tempDir)

	// Initialize config
	config.InitConfig()

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check query parameters
		query := r.URL.Query()

		mockResponse := map[string]interface{}{
			"objects": []map[string]interface{}{
				{
					"package": map[string]interface{}{
						"name":        "test-package",
						"version":     "1.0.0",
						"description": "A test package",
					},
					"score": map[string]interface{}{
						"final": 0.5,
					},
				},
			},
			"total": 1,
		}

		// Verify size parameter when limit is set
		if query.Get("size") != "" {
			require.Equal(t, "5", query.Get("size"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	config.SetRegistry(server.URL)

	t.Run("search with limit flag", func(t *testing.T) {
		searchLimit = 5
		searchDetail = false

		err := search(nil, []string{"test"})
		assert.NoError(t, err)
	})
}

func TestMinFunction(t *testing.T) {
	assert.Equal(t, 5, min(5, 10))
	assert.Equal(t, 5, min(10, 5))
	assert.Equal(t, 5, min(5, 5))
	assert.Equal(t, 0, min(0, 5))
	assert.Equal(t, -1, min(-1, 5))
}
