package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUninstallCommand(t *testing.T) {
	// Setup temporary directory for testing
	tempDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	t.Run("uninstall without package name", func(t *testing.T) {
		// This should be handled by cobra's Args validation
		// Skip direct function testing since it requires args
		assert.NotNil(t, uninstallCmd)
	})

	t.Run("uninstall with package name but no package.json", func(t *testing.T) {
		emptyDir := filepath.Join(tempDir, "empty")
		_ = os.MkdirAll(emptyDir, 0755)
		_ = os.Chdir(emptyDir)

		err := uninstall(nil, []string{"test-package"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not installed")
	})

	t.Run("uninstall with valid package.json", func(t *testing.T) {
		projectDir := filepath.Join(tempDir, "project")
		_ = os.MkdirAll(projectDir, 0755)

		// Create a test package.json with dependencies
		packageJSON := `{
			"name": "test-project",
			"version": "1.0.0",
			"dependencies": {
				"test-package": "1.0.0",
				"other-package": "2.0.0"
			}
		}`

		_ = os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(packageJSON), 0644)
		_ = os.Chdir(projectDir)

		err := uninstall(nil, []string{"test-package"})
		assert.Error(t, err) // Will fail because package isn't actually installed
	})

	t.Run("uninstall global package", func(t *testing.T) {
		uninstallGlobal = true
		defer func() { uninstallGlobal = false }()

		err := uninstall(nil, []string{"test-package"})
		assert.Error(t, err) // Will fail - global uninstall not supported
		assert.Contains(t, err.Error(), "not yet supported")
	})

	t.Run("uninstall package not in dependencies", func(t *testing.T) {
		projectDir := filepath.Join(tempDir, "project2")
		_ = os.MkdirAll(projectDir, 0755)

		// Create a test package.json without the package
		packageJSON := `{
			"name": "test-project",
			"version": "1.0.0",
			"dependencies": {
				"other-package": "2.0.0"
			}
		}`

		_ = os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(packageJSON), 0644)
		_ = os.Chdir(projectDir)

		err := uninstall(nil, []string{"nonexistent-package"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not installed")
	})
}

func TestUninstallCmdStructure(t *testing.T) {
	// Test command structure
	assert.NotNil(t, uninstallCmd)
	assert.Equal(t, "uninstall <package>", uninstallCmd.Use)
	assert.Equal(t, "Uninstall a package", uninstallCmd.Short)
	assert.NotEmpty(t, uninstallCmd.Long)
	assert.NotNil(t, uninstallCmd.RunE)
	assert.False(t, uninstallCmd.HasSubCommands())

	// Test flags
	flags := uninstallCmd.Flags()
	globalFlag := flags.Lookup("global")
	assert.NotNil(t, globalFlag)

	saveFlag := flags.Lookup("save")
	assert.NotNil(t, saveFlag)
}
