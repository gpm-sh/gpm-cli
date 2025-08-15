package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackCommand(t *testing.T) {
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
			err := packPackages(cmd, []string{})

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

func TestPackSinglePackage(t *testing.T) {
	tests := []struct {
		name        string
		packageSpec string
		setupFiles  map[string]string
		expectError bool
	}{
		{
			name:        "pack current directory",
			packageSpec: ".",
			setupFiles: map[string]string{
				"package.json": `{
					"name": "test-package",
					"version": "1.0.0",
					"description": "Test package"
				}`,
				"src/main.js": "console.log('Hello World');",
			},
			expectError: false,
		},
		{
			name:        "pack specific folder",
			packageSpec: "./test-package",
			setupFiles: map[string]string{
				"test-package/package.json": `{
					"name": "test-package",
					"version": "1.0.0",
					"description": "Test package"
				}`,
				"test-package/src/main.js": "console.log('Hello World');",
			},
			expectError: false,
		},
		{
			name:        "pack non-existent folder",
			packageSpec: "./non-existent",
			setupFiles:  map[string]string{},
			expectError: true,
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
			result, err := packSinglePackage(tt.packageSpec)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, "test-package", result.Name)
				assert.Equal(t, "1.0.0", result.Version)
			}
		})
	}
}

func TestPackWithFlags(t *testing.T) {
	// Setup temporary directory
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(oldWd) }()

	// Create package.json
	packageJSON := `{
		"name": "test-package",
		"version": "1.0.0",
		"description": "Test package"
	}`
	require.NoError(t, os.WriteFile("package.json", []byte(packageJSON), 0644))

	// Test dry-run mode
	packDryRun = true
	defer func() { packDryRun = false }()

	cmd := &cobra.Command{}
	err := packPackages(cmd, []string{})
	assert.NoError(t, err)

	// Verify no tarball was created in dry-run mode
	files, err := filepath.Glob("*.tgz")
	require.NoError(t, err)
	assert.Len(t, files, 0, "Expected no .tgz file in dry-run mode")
}

func TestPackMultiplePackages(t *testing.T) {
	// Setup temporary directory
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(oldWd) }()

	// Create multiple packages
	packages := []string{"package1", "package2"}
	for _, pkg := range packages {
		pkgDir := filepath.Join(tmpDir, pkg)
		require.NoError(t, os.MkdirAll(pkgDir, 0755))

		packageJSON := fmt.Sprintf(`{
			"name": "%s",
			"version": "1.0.0",
			"description": "Test package %s"
		}`, pkg, pkg)
		require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(packageJSON), 0644))
	}

	// Test packing multiple packages
	cmd := &cobra.Command{}
	err := packPackages(cmd, packages)
	assert.NoError(t, err)

	// Verify tarballs were created
	for _, pkg := range packages {
		files, err := filepath.Glob(filepath.Join(pkg, "*.tgz"))
		require.NoError(t, err)
		assert.Len(t, files, 1, "Expected exactly one .tgz file for package %s", pkg)
	}
}
