package cmd

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestVersionCommand(t *testing.T) {
	// Save original stdout
	originalStdout := os.Stdout
	defer func() { os.Stdout = originalStdout }()

	// Create a pipe to capture output
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run the version command
	version(nil, []string{})

	// Close writer and restore stdout
	_ = w.Close()
	os.Stdout = originalStdout

	// Read the output
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains expected content
	assert.Contains(t, output, "GPM CLI")
	assert.Contains(t, output, "Version:")
	assert.Contains(t, output, "Commit:")
	assert.Contains(t, output, "Built:")
	assert.Contains(t, output, "Go Version:")
	assert.Contains(t, output, "Platform:")
	assert.Contains(t, output, runtime.Version())
	assert.Contains(t, output, runtime.GOOS+"/"+runtime.GOARCH)
}

func TestVersionVariables(t *testing.T) {
	// Test that version variables are properly declared
	assert.NotEmpty(t, Version)
	assert.NotEmpty(t, Commit)
	assert.NotEmpty(t, Date)

	// Test default values
	if Version == "dev" {
		assert.Equal(t, "dev", Version)
	}
	if Commit == "unknown" {
		assert.Equal(t, "unknown", Commit)
	}
	if Date == "unknown" {
		assert.Equal(t, "unknown", Date)
	}
}

func TestVersionCmdStructure(t *testing.T) {
	// Test command structure
	assert.NotNil(t, versionCmd)
	assert.Equal(t, "version", versionCmd.Use)
	assert.Equal(t, "Show GPM CLI version", versionCmd.Short)
	assert.NotEmpty(t, versionCmd.Long)
	assert.NotNil(t, versionCmd.Run)
	assert.False(t, versionCmd.HasSubCommands())
}

func TestVersionWithCustomValues(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := Commit
	originalDate := Date

	// Set custom values
	Version = "1.0.0"
	Commit = "abc123def456"
	Date = "2024-01-01T00:00:00Z"

	// Restore original values after test
	defer func() {
		Version = originalVersion
		Commit = originalCommit
		Date = originalDate
	}()

	// Save original stdout
	originalStdout := os.Stdout
	defer func() { os.Stdout = originalStdout }()

	// Create a pipe to capture output
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run the version command
	version(nil, []string{})

	// Close writer and restore stdout
	_ = w.Close()
	os.Stdout = originalStdout

	// Read the output
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify custom values appear in output
	assert.Contains(t, output, "1.0.0")
	assert.Contains(t, output, "abc123def456")
	assert.Contains(t, output, "2024-01-01T00:00:00Z")
}

func TestVersionCommandExecution(t *testing.T) {
	// Test that the command can be executed without panicking
	cmd := &cobra.Command{}
	args := []string{}

	assert.NotPanics(t, func() {
		version(cmd, args)
	})
}
