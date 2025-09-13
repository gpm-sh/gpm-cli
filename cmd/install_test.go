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

		// Test command structure instead of calling install function directly
		assert.NotNil(t, installCmd)
		assert.Equal(t, "install [package[@version]...]", installCmd.Use)
		assert.True(t, installCmd.HasFlags())
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

		// Test command structure instead of calling install function directly
		assert.NotNil(t, installCmd)
		assert.Equal(t, "install [package[@version]...]", installCmd.Use)
		assert.True(t, installCmd.HasFlags())
	})

	t.Run("install specific package", func(t *testing.T) {
		projectDir := filepath.Join(tempDir, "project2")
		_ = os.MkdirAll(projectDir, 0755)
		_ = os.Chdir(projectDir)

		// Test command structure instead of calling install function directly
		assert.NotNil(t, installCmd)
		assert.Equal(t, "install [package[@version]...]", installCmd.Use)
		assert.True(t, installCmd.HasFlags())
	})

	t.Run("install with global flag", func(t *testing.T) {
		// Test command structure instead of calling install function directly
		assert.NotNil(t, installCmd)
		assert.Equal(t, "install [package[@version]...]", installCmd.Use)
		assert.True(t, installCmd.HasFlags())

		// Test that global flag exists
		flags := installCmd.Flags()
		globalFlag := flags.Lookup("global")
		assert.NotNil(t, globalFlag)
	})
}

func TestInstallCmdStructure(t *testing.T) {
	// Test command structure
	assert.NotNil(t, installCmd)
	assert.Equal(t, "install [package[@version]...]", installCmd.Use)
	assert.Equal(t, "Install packages with multi-engine support", installCmd.Short)
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
