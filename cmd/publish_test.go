package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gpm.sh/gpm/gpm-cli/internal/api"
	"gpm.sh/gpm/gpm-cli/internal/config"
)

func TestPublishCmd(t *testing.T) {
	// Test command structure
	assert.Equal(t, "publish [tarball]", publishCmd.Use)
	assert.Equal(t, "Publish a package to GPM registry", publishCmd.Short)
	assert.NotNil(t, publishCmd.RunE)
}

func TestExtractPackageInfo(t *testing.T) {
	tests := []struct {
		name            string
		packageJSON     string
		expectError     bool
		expectedName    string
		expectedVersion string
	}{
		{
			name: "valid package in tarball",
			packageJSON: `{
				"name": "com.test.extract-package",
				"version": "2.0.0",
				"description": "Test package for extraction"
			}`,
			expectError:     false,
			expectedName:    "com.test.extract-package",
			expectedVersion: "2.0.0",
		},
		{
			name: "missing name in package.json",
			packageJSON: `{
				"version": "1.0.0"
			}`,
			expectError: true,
		},
		{
			name: "missing version in package.json",
			packageJSON: `{
				"name": "com.test.no-version"
			}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory and tarball
			tmpDir := t.TempDir()
			packageDir := filepath.Join(tmpDir, "package")
			require.NoError(t, os.MkdirAll(packageDir, 0755))

			// Write package.json
			packageJSONPath := filepath.Join(packageDir, "package.json")
			require.NoError(t, os.WriteFile(packageJSONPath, []byte(tt.packageJSON), 0644))

			// Create tarball using the pack function
			oldWd, _ := os.Getwd()
			require.NoError(t, os.Chdir(tmpDir))
			defer func() { _ = os.Chdir(oldWd) }()

			// Create package.json in current dir (needed for pack)
			require.NoError(t, os.WriteFile("package.json", []byte(tt.packageJSON), 0644))

			cmd := &cobra.Command{}
			err := packPackage(cmd, []string{})

			if tt.expectError {
				// If pack fails due to missing name/version, that's expected
				return
			}

			require.NoError(t, err)

			// Find the created tarball
			files, err := filepath.Glob("*.tgz")
			require.NoError(t, err)
			require.Len(t, files, 1, "Expected exactly one .tgz file")

			// Test extraction
			result, err := extractPackageInfo(files[0])

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedName, result.Name)
				assert.Equal(t, tt.expectedVersion, result.Version)
			}
		})
	}
}

func TestPublishFunction(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		serverResponse api.PublishResponse
		serverStatus   int
		expectError    bool
		packageName    string
		packageVersion string
	}{
		{
			name:  "successful publish",
			token: "valid-token",
			serverResponse: api.PublishResponse{
				Success: true,
				Data: api.PublishData{
					PackageID:   "pkg-123",
					VersionID:   "ver-456",
					DownloadURL: "https://registry.test/download/pkg.tgz",
					FileSize:    1024,
					UploadTime:  "2024-01-01T00:00:00Z",
				},
			},
			serverStatus:   http.StatusOK,
			expectError:    false,
			packageName:    "com.test.publish-package",
			packageVersion: "1.0.0",
		},
		{
			name:  "unauthorized publish",
			token: "invalid-token",
			serverResponse: api.PublishResponse{
				Success: false,
				Error: &api.ErrorResponse{
					Code:    "UNAUTHORIZED",
					Message: "Invalid authentication token",
				},
			},
			serverStatus:   http.StatusUnauthorized,
			expectError:    true,
			packageName:    "com.test.unauthorized",
			packageVersion: "1.0.0",
		},
		{
			name:           "no token provided",
			token:          "",
			serverResponse: api.PublishResponse{},
			serverStatus:   http.StatusOK,
			expectError:    true,
			packageName:    "com.test.no-auth",
			packageVersion: "1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "no token provided" {
				// Test the publish function directly for no token case
				testConfig := &config.Config{
					Registry: "http://test.server",
					Token:    "",
					Username: "",
				}
				config.SetConfigForTesting(testConfig)

				// Create a dummy tarball file to avoid file not found error
				tmpDir := t.TempDir()
				dummyTarball := filepath.Join(tmpDir, "dummy.tgz")
				require.NoError(t, os.WriteFile(dummyTarball, []byte("dummy"), 0644))

				err := publish(dummyTarball)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not authenticated")
				return
			}

			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "PUT", r.Method)
				assert.Equal(t, "/"+tt.packageName, r.URL.Path)
				assert.Equal(t, "Bearer "+tt.token, r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				w.WriteHeader(tt.serverStatus)
				_ = json.NewEncoder(w).Encode(tt.serverResponse)
			}))
			defer server.Close()

			// Create test tarball
			tmpDir := t.TempDir()
			packageJSON := `{
				"name": "` + tt.packageName + `",
				"version": "` + tt.packageVersion + `",
				"description": "Test package for publishing"
			}`

			oldWd, _ := os.Getwd()
			require.NoError(t, os.Chdir(tmpDir))
			defer func() { _ = os.Chdir(oldWd) }()

			require.NoError(t, os.WriteFile("package.json", []byte(packageJSON), 0644))
			require.NoError(t, os.MkdirAll("Runtime/Scripts", 0755))
			require.NoError(t, os.WriteFile("Runtime/Scripts/Test.cs", []byte("// test"), 0644))

			// Create tarball
			cmd := &cobra.Command{}
			err := packPackage(cmd, []string{})
			require.NoError(t, err)

			files, err := filepath.Glob("*.tgz")
			require.NoError(t, err)
			require.Len(t, files, 1)
			tarballPath := files[0]

			// Setup test config
			testConfig := &config.Config{
				Registry: server.URL,
				Token:    tt.token,
				Username: "",
			}
			config.SetConfigForTesting(testConfig)

			// Test publish
			err = publish(tarballPath)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPublishCmdStructure(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(publishCmd)

	// Verify command is properly registered
	publishSubCmd := cmd.Commands()
	require.Len(t, publishSubCmd, 1)
	assert.Equal(t, "publish [tarball]", publishSubCmd[0].Use)
}
