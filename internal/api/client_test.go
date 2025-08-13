package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client := NewClient("https://test.gpm.sh", "test-token")

	assert.Equal(t, "https://test.gpm.sh", client.baseURL)
	assert.Equal(t, "test-token", client.token)
	assert.NotNil(t, client.httpClient)
}

func TestClient_Login(t *testing.T) {
	tests := []struct {
		name           string
		loginReq       *LoginRequest
		serverResponse LoginResponse
		serverStatus   int
		expectError    bool
	}{
		{
			name: "successful login",
			loginReq: &LoginRequest{
				Name:     "testuser",
				Password: "testpass",
				Email:    "test@example.com",
				Type:     "studio",
			},
			serverResponse: LoginResponse{
				Token: "auth-token-123",
				User: struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					Email    string `json:"email"`
				}{
					ID:       "user-123",
					Username: "testuser",
					Email:    "test@example.com",
				},
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name: "invalid credentials",
			loginReq: &LoginRequest{
				Name:     "testuser",
				Password: "wrongpass",
				Email:    "test@example.com",
				Type:     "studio",
			},
			serverResponse: LoginResponse{}, // Empty response for error case
			serverStatus:   http.StatusUnauthorized,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/-/v1/login", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Verify request body
				var loginReq LoginRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&loginReq))
				assert.Equal(t, tt.loginReq.Name, loginReq.Name)
				assert.Equal(t, tt.loginReq.Password, loginReq.Password)

				// Send response
				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			// Test
			client := NewClient(server.URL, "")
			result, err := client.Login(tt.loginReq)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.serverResponse.Token, result.Token)
				assert.Equal(t, tt.serverResponse.User.Username, result.User.Username)
			}
		})
	}
}

func TestClient_makeRequest(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		token        string
		expectAuth   bool
		serverStatus int
		expectError  bool
	}{
		{
			name:         "GET request with auth",
			method:       "GET",
			path:         "/test",
			token:        "test-token",
			expectAuth:   true,
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:         "POST request without auth",
			method:       "POST",
			path:         "/public",
			token:        "",
			expectAuth:   false,
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:         "server error",
			method:       "GET",
			path:         "/error",
			token:        "test-token",
			expectAuth:   true,
			serverStatus: http.StatusInternalServerError,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.method, r.Method)
				assert.Equal(t, tt.path, r.URL.Path)

				if tt.expectAuth {
					authHeader := r.Header.Get("Authorization")
					assert.Equal(t, "Bearer "+tt.token, authHeader)
				}

				w.WriteHeader(tt.serverStatus)
				_, _ = w.Write([]byte(`{"message": "test response"}`))
			}))
			defer server.Close()

			// Test
			client := NewClient(server.URL, tt.token)
			resp, err := client.makeRequest(tt.method, tt.path, nil, nil)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.serverStatus, resp.StatusCode)
				resp.Body.Close()
			}
		})
	}
}
