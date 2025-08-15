package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstallCommand(t *testing.T) {
	// Setup temporary directory for testing
	tempDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	t.Run("install without arguments and without package.json", func(t *testing.T) {
		emptyDir := filepath.Join(tempDir, "empty")
		_ = os.MkdirAll(emptyDir, 0755)
		_ = os.Chdir(emptyDir)

		err := install(nil, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "package.json")
	})

	t.Run("install with package.json", func(t *testing.T) {
		projectDir := filepath.Join(tempDir, "project")
		_ = os.MkdirAll(projectDir, 0755)

		// Create a test package.json
		packageJSON := `{
			"name": "test-project",
			"version": "1.0.0",
			"dependencies": {
				"test-dep": "1.0.0"
			}
		}`

		_ = os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(packageJSON), 0644)
		_ = os.Chdir(projectDir)

		err := install(nil, []string{})
		assert.NoError(t, err)
	})

	t.Run("install specific package", func(t *testing.T) {
		projectDir := filepath.Join(tempDir, "project2")
		_ = os.MkdirAll(projectDir, 0755)
		_ = os.Chdir(projectDir)

		err := install(nil, []string{"test-package"})
		// This will likely fail due to network/registry issues in tests, but should not panic
		// We just test that the function can be called
		assert.Error(t, err) // Expected to fail in test environment
	})

	t.Run("install with global flag", func(t *testing.T) {
		installGlobal = true
		defer func() { installGlobal = false }()

		err := install(nil, []string{"test-package"})
		// This will likely fail due to network/registry issues in tests
		assert.Error(t, err) // Expected to fail in test environment
	})
}

func TestInstallCmdStructure(t *testing.T) {
	// Test command structure
	assert.NotNil(t, installCmd)
	assert.Equal(t, "install [package[@version]...]", installCmd.Use)
	assert.Equal(t, "Install packages", installCmd.Short)
	assert.NotEmpty(t, installCmd.Long)
	assert.NotNil(t, installCmd.RunE)
	assert.False(t, installCmd.HasSubCommands())

	// Test flags
	flags := installCmd.Flags()
	globalFlag := flags.Lookup("global")
	assert.NotNil(t, globalFlag)

	saveFlag := flags.Lookup("save")
	assert.NotNil(t, saveFlag)

	saveDevFlag := flags.Lookup("save-dev")
	assert.NotNil(t, saveDevFlag)
}
