package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadPackageJSON(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expected    *PackageInfo
		expectError bool
	}{
		{
			name: "valid package.json",
			content: `{
				"name": "com.test.package",
				"version": "1.0.0",
				"description": "Test package"
			}`,
			expected: &PackageInfo{
				Name:    "com.test.package",
				Version: "1.0.0",
			},
			expectError: false,
		},
		{
			name: "missing name",
			content: `{
				"version": "1.0.0"
			}`,
			expected:    nil,
			expectError: true,
		},
		{
			name: "missing version",
			content: `{
				"name": "com.test.package"
			}`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid json",
			content:     `{invalid json`,
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			packageFile := filepath.Join(tmpDir, "package.json")
			require.NoError(t, os.WriteFile(packageFile, []byte(tt.content), 0644))

			// Test
			result, err := readPackageJSON(packageFile)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.Name, result.Name)
				assert.Equal(t, tt.expected.Version, result.Version)
			}
		})
	}
}

func TestPackPackage(t *testing.T) {
	tests := []struct {
		name        string
		setupFiles  map[string]string
		expectError bool
		expectFile  bool
	}{
		{
			name: "successful pack",
			setupFiles: map[string]string{
				"package.json": `{
					"name": "com.test.package",
					"version": "1.0.0",
					"description": "Test package"
				}`,
				"Runtime/Scripts/TestScript.cs": `using UnityEngine;
public class TestScript : MonoBehaviour {
    void Start() {
        Debug.Log("Hello World");
    }
}`,
			},
			expectError: false,
			expectFile:  true,
		},
		{
			name: "missing package.json",
			setupFiles: map[string]string{
				"Runtime/Scripts/TestScript.cs": "test content",
			},
			expectError: true,
			expectFile:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup temporary directory
			tmpDir := t.TempDir()
			oldWd, _ := os.Getwd()
			require.NoError(t, os.Chdir(tmpDir))
			defer func() { _ = os.Chdir(oldWd) }()

			// Create test files
			for path, content := range tt.setupFiles {
				dir := filepath.Dir(path)
				if dir != "." {
					require.NoError(t, os.MkdirAll(dir, 0755))
				}
				require.NoError(t, os.WriteFile(path, []byte(content), 0644))
			}

			// Test
			cmd := &cobra.Command{}
			err := packPackage(cmd, []string{})

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if tt.expectFile {
					// Check if tarball was created
					files, err := filepath.Glob("*.tgz")
					require.NoError(t, err)
					assert.Len(t, files, 1, "Expected exactly one .tgz file")
				}
			}
		})
	}
}
