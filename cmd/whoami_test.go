package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gpm.sh/gpm/gpm-cli/internal/api"
	"gpm.sh/gpm/gpm-cli/internal/config"
)

func TestWhoamiCmd(t *testing.T) {
	// Test command structure
	assert.Equal(t, "whoami", whoamiCmd.Use)
	assert.Equal(t, "Show current user information", whoamiCmd.Short)
	assert.NotNil(t, whoamiCmd.RunE)
}

func TestWhoamiFunction(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		serverResponse api.WhoamiResponse
		serverStatus   int
		expectError    bool
		expectedUser   string
	}{
		{
			name:  "successful whoami with studio",
			token: "valid-token",
			serverResponse: api.WhoamiResponse{
				Username: "testuser",
				Studio:   "test-studio",
			},
			serverStatus: http.StatusOK,
			expectError:  false,
			expectedUser: "testuser",
		},
		{
			name:  "successful whoami without studio",
			token: "valid-token",
			serverResponse: api.WhoamiResponse{
				Username: "globaluser",
				Studio:   "",
			},
			serverStatus: http.StatusOK,
			expectError:  false,
			expectedUser: "globaluser",
		},
		{
			name:           "no token provided",
			token:          "",
			serverResponse: api.WhoamiResponse{},
			serverStatus:   http.StatusOK,
			expectError:    true,
			expectedUser:   "",
		},
		{
			name:           "invalid token",
			token:          "invalid-token",
			serverResponse: api.WhoamiResponse{},
			serverStatus:   http.StatusUnauthorized,
			expectError:    true,
			expectedUser:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "no token provided" {
				// Test the whoami function directly for no token case
				testConfig := &config.Config{
					Registry: "http://test.server",
					Token:    "",
					Username: "",
				}
				config.SetConfigForTesting(testConfig)

				err := whoami()
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not authenticated")
				return
			}

			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/-/whoami", r.URL.Path)
				assert.Equal(t, "Bearer "+tt.token, r.Header.Get("Authorization"))

				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			// Setup test config
			testConfig := &config.Config{
				Registry: server.URL,
				Token:    tt.token,
				Username: "",
			}
			config.SetConfigForTesting(testConfig)

			// Test the API client
			client := api.NewClient(server.URL, tt.token)
			result, err := client.Whoami()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedUser, result.Username)

				// Test studio information if present
				assert.Equal(t, tt.serverResponse.Studio, result.Studio)
			}
		})
	}
}

func TestWhoamiCmdStructure(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(whoamiCmd)

	// Verify command is properly registered
	whoamiSubCmd := cmd.Commands()
	require.Len(t, whoamiSubCmd, 1)
	assert.Equal(t, "whoami", whoamiSubCmd[0].Use)
}
