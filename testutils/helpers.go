package testutils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type MockServer struct {
	Server   *httptest.Server
	Handlers map[string]http.HandlerFunc
}

func NewMockServer() *MockServer {
	ms := &MockServer{
		Handlers: make(map[string]http.HandlerFunc),
	}

	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		if handler, exists := ms.Handlers[key]; exists {
			handler(w, r)
		} else {
			http.NotFound(w, r)
		}
	}))

	return ms
}

func (ms *MockServer) AddHandler(method, path string, handler http.HandlerFunc) {
	ms.Handlers[method+" "+path] = handler
}

func (ms *MockServer) Close() {
	ms.Server.Close()
}

// CreateTempPackage creates a temporary package structure for testing
func CreateTempPackage(t *testing.T, name, version string) string {
	tmpDir := t.TempDir()

	packageJSON := map[string]interface{}{
		"name":        name,
		"version":     version,
		"displayName": "Test Package",
		"description": "A test package",
		"unity":       "2022.3",
		"license":     "MIT",
	}

	data, err := json.MarshalIndent(packageJSON, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "package.json"), data, 0644))

	runtimeDir := filepath.Join(tmpDir, "Runtime", "Scripts")
	require.NoError(t, os.MkdirAll(runtimeDir, 0755))

	script := `using UnityEngine;

namespace TestPackage
{
    public class TestScript : MonoBehaviour
    {
        void Start()
        {
            Debug.Log("Test package loaded");
        }
    }
}`
	require.NoError(t, os.WriteFile(filepath.Join(runtimeDir, "TestScript.cs"), []byte(script), 0644))

	return tmpDir
}

// CreateTempConfig creates a temporary GPM config file
func CreateTempConfig(t *testing.T, registry, token, username string) string {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".gpmrc")

	config := map[string]string{
		"registry": registry,
		"token":    token,
		"username": username,
	}

	content := ""
	for key, value := range config {
		if value != "" {
			content += key + ": \"" + value + "\"\n"
		}
	}

	require.NoError(t, os.WriteFile(configFile, []byte(content), 0644))
	return tmpDir
}

func AssertJSONResponse(t *testing.T, expected, actual interface{}) {
	expectedJSON, err := json.Marshal(expected)
	require.NoError(t, err)

	actualJSON, err := json.Marshal(actual)
	require.NoError(t, err)

	require.JSONEq(t, string(expectedJSON), string(actualJSON))
}

func WithTempDir(t *testing.T, fn func(string)) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	fn(tmpDir)
}
