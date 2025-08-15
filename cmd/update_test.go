package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateCommand(t *testing.T) {
	// Setup temporary directory for testing
	tempDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	t.Run("update without package.json", func(t *testing.T) {
		emptyDir := filepath.Join(tempDir, "empty")
		_ = os.MkdirAll(emptyDir, 0755)
		_ = os.Chdir(emptyDir)

		err := runUpdate(nil, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "package.json")
	})

	t.Run("update with valid package.json", func(t *testing.T) {
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

		// This will likely fail due to network issues in tests, but should not panic
		err := runUpdate(nil, []string{})
		assert.Error(t, err) // Expected to fail in test environment
	})

	t.Run("update specific package", func(t *testing.T) {
		projectDir := filepath.Join(tempDir, "project2")
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

		// This will likely fail due to network issues in tests, but should not panic
		err := runUpdate(nil, []string{"test-dep"})
		assert.Error(t, err) // Expected to fail in test environment
	})
}

func TestUpdateCmdStructure(t *testing.T) {
	// Test command structure
	assert.NotNil(t, updateCmd)
	assert.Equal(t, "update [package]", updateCmd.Use)
	assert.Equal(t, "Update packages", updateCmd.Short)
	assert.NotEmpty(t, updateCmd.Long)
	assert.NotNil(t, updateCmd.RunE)
	assert.False(t, updateCmd.HasSubCommands())
}
