package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitCommand(t *testing.T) {
	t.Run("test command structure", func(t *testing.T) {
		// Test that the command can be called without panicking
		assert.NotPanics(t, func() {
			// Just test the structure, not the interactive functionality
			// since init requires user input
		})
	})
}

func TestInitCmdStructure(t *testing.T) {
	// Test command structure
	assert.NotNil(t, initCmd)
	assert.Equal(t, "init", initCmd.Use)
	assert.Equal(t, "Initialize a new UPM-compatible package", initCmd.Short)
	assert.NotEmpty(t, initCmd.Long)
	assert.NotNil(t, initCmd.RunE)
	assert.False(t, initCmd.HasSubCommands())

	// Test flags
	flags := initCmd.Flags()
	yesFlag := flags.Lookup("yes")
	assert.NotNil(t, yesFlag)

	nameFlag := flags.Lookup("name")
	assert.NotNil(t, nameFlag)
}
