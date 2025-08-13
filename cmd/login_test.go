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

func TestLoginCmd(t *testing.T) {
	assert.Equal(t, "login", loginCmd.Use)
	assert.Equal(t, "Login to GPM registry", loginCmd.Short)
	assert.NotNil(t, loginCmd.RunE)
}

func TestLoginFunction(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse api.LoginResponse
		serverStatus   int
		expectError    bool
		expectedToken  string
	}{
		{
			name: "successful login",
			serverResponse: api.LoginResponse{
				OK:    true,
				ID:    "org.couchdb.user:testuser",
				Rev:   "1",
				Token: "test-token-123",
			},
			serverStatus:  http.StatusOK,
			expectError:   false,
			expectedToken: "test-token-123",
		},
		{
			name:           "invalid credentials",
			serverResponse: api.LoginResponse{},
			serverStatus:   http.StatusUnauthorized,
			expectError:    true,
			expectedToken:  "",
		},
		{
			name:           "server error",
			serverResponse: api.LoginResponse{},
			serverStatus:   http.StatusInternalServerError,
			expectError:    true,
			expectedToken:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/-/v1/login", r.URL.Path)

				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			// Setup test config
			testConfig := &config.Config{
				Registry: server.URL,
				Token:    "",
				Username: "",
			}
			config.SetConfigForTesting(testConfig)

			// Test the API client directly (login function requires stdin input)
			client := api.NewClient(server.URL, "")
			loginReq := &api.LoginRequest{
				Name:     "testuser",
				Password: "testpass",
			}

			result, err := client.Login(loginReq)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedToken, result.Token)
				assert.Equal(t, tt.serverResponse.OK, result.OK)
				assert.Equal(t, tt.serverResponse.ID, result.ID)
			}
		})
	}
}

// Helper function to test login command structure without user input
func TestLoginCmdStructure(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(loginCmd)

	// Verify command is properly registered
	loginSubCmd := cmd.Commands()
	require.Len(t, loginSubCmd, 1)
	assert.Equal(t, "login", loginSubCmd[0].Use)
}
