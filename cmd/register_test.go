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

func TestRegisterCmd(t *testing.T) {
	// Test command structure
	assert.Equal(t, "register", registerCmd.Use)
	assert.Equal(t, "Register a new user account", registerCmd.Short)
	assert.NotNil(t, registerCmd.RunE)
}

func TestRegisterFunction(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse api.RegisterResponse
		serverStatus   int
		expectError    bool
		username       string
	}{
		{
			name: "successful registration",
			serverResponse: api.RegisterResponse{
				Success: true,
				Message: "User registered successfully",
				User: struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					Email    string `json:"email"`
				}{
					ID:       "user-123",
					Username: "newuser",
					Email:    "newuser@example.com",
				},
			},
			serverStatus: http.StatusCreated,
			expectError:  false,
			username:     "newuser",
		},
		{
			name: "user already exists",
			serverResponse: api.RegisterResponse{
				Success: false,
				Message: "Username already exists",
			},
			serverStatus: http.StatusConflict,
			expectError:  true,
			username:     "existinguser",
		},
		{
			name: "invalid input",
			serverResponse: api.RegisterResponse{
				Success: false,
				Message: "Invalid input data",
			},
			serverStatus: http.StatusBadRequest,
			expectError:  true,
			username:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/-/v1/register", r.URL.Path)

				var registerReq api.RegisterRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&registerReq))

				if tt.username != "" {
					assert.Equal(t, tt.username, registerReq.Name)
				}

				w.WriteHeader(tt.serverStatus)
				json.NewEncoder(w).Encode(tt.serverResponse)
			}))
			defer server.Close()

			// Setup test config
			testConfig := &config.Config{
				Registry: server.URL,
				Token:    "",
				Username: "",
			}
			config.SetConfigForTesting(testConfig)

			// Test the API client directly (register function requires stdin input)
			client := api.NewClient(server.URL, "")
			registerReq := &api.RegisterRequest{
				Name:     tt.username,
				Password: "testpass123",
				Type:     "studio",
			}

			result, err := client.Register(registerReq)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.serverResponse.Success, result.Success)
				assert.Equal(t, tt.serverResponse.Message, result.Message)
				if result.Success {
					assert.Equal(t, tt.username, result.User.Username)
				}
			}
		})
	}
}

func TestRegisterCmdStructure(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(registerCmd)

	// Verify command is properly registered
	registerSubCmd := cmd.Commands()
	require.Len(t, registerSubCmd, 1)
	assert.Equal(t, "register", registerSubCmd[0].Use)
}
