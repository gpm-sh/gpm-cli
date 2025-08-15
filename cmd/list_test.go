package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListCommand(t *testing.T) {
	// Setup temporary directory for testing
	tempDir := t.TempDir()

	t.Run("list in empty directory", func(t *testing.T) {
		oldWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldWd) }()

		_ = os.Chdir(tempDir)

		err := list(nil, []string{})
		assert.NoError(t, err)
	})

	t.Run("list with package.json", func(t *testing.T) {
		oldWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldWd) }()

		// Create a test package.json
		packageJSON := `{
			"name": "test-project",
			"version": "1.0.0",
			"dependencies": {
				"test-dep": "1.0.0"
			}
		}`

		packageDir := filepath.Join(tempDir, "test-project")
		_ = os.MkdirAll(packageDir, 0755)
		_ = os.WriteFile(filepath.Join(packageDir, "package.json"), []byte(packageJSON), 0644)

		_ = os.Chdir(packageDir)

		err := list(nil, []string{})
		assert.NoError(t, err)
	})

	t.Run("list global packages", func(t *testing.T) {
		listGlobal = true
		defer func() { listGlobal = false }()

		err := list(nil, []string{})
		assert.NoError(t, err)
	})
}

func TestListCmdStructure(t *testing.T) {
	// Test command structure
	assert.NotNil(t, listCmd)
	assert.Equal(t, "list", listCmd.Use)
	assert.Equal(t, "List installed packages", listCmd.Short)
	assert.NotEmpty(t, listCmd.Long)
	assert.NotNil(t, listCmd.RunE)
	assert.False(t, listCmd.HasSubCommands())

	// Test flags
	flags := listCmd.Flags()
	globalFlag := flags.Lookup("global")
	assert.NotNil(t, globalFlag)
}
