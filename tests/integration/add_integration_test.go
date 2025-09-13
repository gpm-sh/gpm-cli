package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/cmd"
	"gpm.sh/gpm/gpm-cli/internal/config"
)

func TestAddCommand_Integration(t *testing.T) {
	// Setup mock registry with realistic npm-style responses
	registry := NewRegistryMock()
	defer registry.Close()

	// Add test packages
	registry.AddPackage(CreateTestPackage("com.unity.analytics", "2.1.0", "public"))
	registry.AddPackage(CreateUnityPackage("com.company.toolkit", "1.5.0", "2020.3"))

	// Package without latest dist-tag
	noLatestPkg := CreateTestPackage("com.test.no-latest", "1.0.0", "public")
	noLatestPkg.DistTags = map[string]string{"beta": "1.0.0"} // No latest tag
	registry.AddPackage(noLatestPkg)

	// Package with no dist-tags
	noDistTagsPkg := CreateTestPackage("com.test.no-dist-tags", "1.0.0", "public")
	noDistTagsPkg.DistTags = nil
	registry.AddPackage(noDistTagsPkg)

	// Private package requiring auth
	privatePkg := CreateTestPackage("com.private.package", "1.0.0", "private")
	registry.AddPackage(privatePkg)

	// Add test user
	registry.AddUser(&User{
		Username: "testuser",
		Email:    "test@example.com",
		Token:    "test-token-123",
	})

	tests := []struct {
		name            string
		args            []string
		setupProject    func(string) error
		setupConfig     func()
		wantExitCode    int
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:         "successful add with latest version",
			args:         []string{"add", "com.unity.analytics", "--json"},
			setupProject: setupUnityProject,
			setupConfig: func() {
				config.SetConfigForTesting(&config.Config{
					Registry: registry.URL(),
				})
			},
			wantExitCode: 0,
			wantContains: []string{
				`"success": true`,
				`"package": "com.unity.analytics"`,
				`"version": "2.1.0"`,
				`"engine": "unity"`,
			},
		},
		{
			name:         "successful add with specific version",
			args:         []string{"add", "com.company.toolkit@1.5.0", "--json"},
			setupProject: setupUnityProject,
			setupConfig: func() {
				config.SetConfigForTesting(&config.Config{
					Registry: registry.URL(),
				})
			},
			wantExitCode: 0,
			wantContains: []string{
				`"success": true`,
				`"package": "com.company.toolkit"`,
				`"version": "1.5.0"`,
				`"changed": true`,
			},
		},
		{
			name:         "package not found error",
			args:         []string{"add", "com.nonexistent.package", "--json"},
			setupProject: setupUnityProject,
			setupConfig: func() {
				config.SetConfigForTesting(&config.Config{
					Registry: registry.URL(),
				})
			},
			wantExitCode: 1,
			wantContains: []string{
				`"success": false`,
				`failed to check package existence`,
				`HTTP 404`,
			},
		},
		{
			name:         "version not available error",
			args:         []string{"add", "com.unity.analytics@99.99.99", "--json"},
			setupProject: setupUnityProject,
			setupConfig: func() {
				config.SetConfigForTesting(&config.Config{
					Registry: registry.URL(),
				})
			},
			wantExitCode: 1,
			wantContains: []string{
				`"success": false`,
				`version '99.99.99' not available`,
			},
		},
		{
			name:         "no latest dist-tag error",
			args:         []string{"add", "com.test.no-latest", "--json"},
			setupProject: setupUnityProject,
			setupConfig: func() {
				config.SetConfigForTesting(&config.Config{
					Registry: registry.URL(),
				})
			},
			wantExitCode: 1,
			wantContains: []string{
				`"success": false`,
				`no 'latest' dist-tag`,
			},
		},
		{
			name:         "no dist-tags at all error",
			args:         []string{"add", "com.test.no-dist-tags", "--json"},
			setupProject: setupUnityProject,
			setupConfig: func() {
				config.SetConfigForTesting(&config.Config{
					Registry: registry.URL(),
				})
			},
			wantExitCode: 1,
			wantContains: []string{
				`"success": false`,
				`no dist-tags`,
			},
		},
		{
			name:         "invalid package name error",
			args:         []string{"add", "Invalid.Package.Name", "--json"},
			setupProject: setupUnityProject,
			setupConfig: func() {
				config.SetConfigForTesting(&config.Config{
					Registry: registry.URL(),
				})
			},
			wantExitCode: 1,
			wantContains: []string{
				`"success": false`,
				`invalid package name`,
				`reverse-DNS names must use lowercase`,
			},
		},
		{
			name:         "no engine detected error",
			args:         []string{"add", "com.unity.analytics", "--json"},
			setupProject: func(path string) error { return nil }, // No Unity structure
			setupConfig: func() {
				config.SetConfigForTesting(&config.Config{
					Registry: registry.URL(),
				})
			},
			wantExitCode: 1,
			wantContains: []string{
				`"success": false`,
				`no supported engine detected`,
			},
		},
		{
			name:         "no registry configured error",
			args:         []string{"add", "com.unity.analytics", "--json"},
			setupProject: setupUnityProject,
			setupConfig: func() {
				config.SetConfigForTesting(&config.Config{
					Registry: "", // No registry configured
				})
			},
			wantExitCode: 1,
			wantContains: []string{
				`"success": false`,
				`no registry configured`,
			},
		},
		{
			name:         "human readable output",
			args:         []string{"add", "com.unity.analytics"},
			setupProject: setupUnityProject,
			setupConfig: func() {
				config.SetConfigForTesting(&config.Config{
					Registry: registry.URL(),
				})
			},
			wantExitCode: 0,
			wantContains: []string{
				"📦 Package Added Successfully",
				"Engine:",
				"unity",
				"Package:",
				"com.unity.analytics@2.1.0",
				"Added com.unity.analytics@2.1.0 to Unity manifest",
			},
		},
		{
			name: "idempotent behavior - already installed",
			args: []string{"add", "com.unity.analytics@2.1.0"},
			setupProject: func(path string) error {
				if err := setupUnityProject(path); err != nil {
					return err
				}
				// Pre-install the package
				manifestPath := filepath.Join(path, "Packages", "manifest.json")
				manifest := `{
  "dependencies": {
    "com.unity.analytics": "2.1.0"
  }
}`
				return os.WriteFile(manifestPath, []byte(manifest), 0644)
			},
			setupConfig: func() {
				config.SetConfigForTesting(&config.Config{
					Registry: registry.URL(),
				})
			},
			wantExitCode: 0,
			wantContains: []string{
				"Package com.unity.analytics@2.1.0 is already installed",
			},
		},
		{
			name:         "registry override flag",
			args:         []string{"add", "com.unity.analytics", "--registry", registry.URL(), "--json"},
			setupProject: setupUnityProject,
			setupConfig: func() {
				config.SetConfigForTesting(&config.Config{
					Registry: "https://different-registry.test",
				})
			},
			wantExitCode: 0,
			wantContains: []string{
				`"success": true`,
				`"registry": "` + registry.URL() + `"`,
			},
		},
		{
			name:         "explicit engine flag",
			args:         []string{"add", "com.unity.analytics", "--engine", "unity", "--json"},
			setupProject: func(path string) error { return nil }, // No Unity structure
			setupConfig: func() {
				config.SetConfigForTesting(&config.Config{
					Registry: registry.URL(),
				})
			},
			wantExitCode: 1, // Should still fail project validation
			wantContains: []string{
				`"success": false`,
				`"engine": "unity"`,
				`project validation failed`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup temporary project directory
			tmpDir := t.TempDir()

			// Setup project structure
			if err := tt.setupProject(tmpDir); err != nil {
				t.Fatalf("failed to setup project: %v", err)
			}

			// Setup config
			oldConfig := config.GetConfig()
			defer func() {
				config.SetConfigForTesting(oldConfig)
			}()
			tt.setupConfig()

			// Change to project directory
			oldWd, _ := os.Getwd()
			defer func() { _ = os.Chdir(oldWd) }()
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("failed to change directory: %v", err)
			}

			// Execute command
			output, exitCode := executeCommand(tt.args...)

			// Verify exit code
			if exitCode != tt.wantExitCode {
				t.Errorf("wrong exit code: got %d, want %d", exitCode, tt.wantExitCode)
				t.Logf("Output: %s", output)
			}

			// Verify output contains expected strings
			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output doesn't contain %q\nOutput: %s", want, output)
				}
			}

			// Verify output doesn't contain unwanted strings
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(output, notWant) {
					t.Errorf("output contains unwanted %q\nOutput: %s", notWant, output)
				}
			}
		})
	}
}

func TestAddCommand_AuthenticationFlow(t *testing.T) {
	// Setup mock registry
	registry := NewRegistryMock()
	defer registry.Close()

	// Add private package requiring authentication
	privatePkg := CreateTestPackage("com.private.package", "1.0.0", "private")
	registry.AddPackage(privatePkg)

	// Add test user
	testUser := &User{
		Username: "testuser",
		Email:    "test@example.com",
		Token:    "test-token-123",
	}
	registry.AddUser(testUser)

	t.Run("private package without auth", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := setupUnityProject(tmpDir); err != nil {
			t.Fatalf("failed to setup project: %v", err)
		}

		config.SetConfigForTesting(&config.Config{
			Registry: registry.URL(),
		})

		oldWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldWd) }()
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		output, exitCode := executeCommand("add", "com.private.package", "--json")

		if exitCode != 1 {
			t.Errorf("expected exit code 1, got %d", exitCode)
		}

		if !strings.Contains(output, "Authentication required") {
			t.Errorf("expected authentication error, got: %s", output)
		}
	})
}

func TestAddCommand_ManifestValidation(t *testing.T) {
	// Setup mock registry
	registry := NewRegistryMock()
	defer registry.Close()

	registry.AddPackage(CreateTestPackage("com.test.package", "1.0.0", "public"))

	t.Run("successful manifest update", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := setupUnityProject(tmpDir); err != nil {
			t.Fatalf("failed to setup project: %v", err)
		}

		config.SetConfigForTesting(&config.Config{
			Registry: registry.URL(),
		})

		oldWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldWd) }()
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		output, exitCode := executeCommand("add", "com.test.package", "--json")

		if exitCode != 0 {
			t.Fatalf("command failed: %s", output)
		}

		// Verify manifest was created and updated
		manifestPath := filepath.Join(tmpDir, "Packages", "manifest.json")
		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("failed to read manifest: %v", err)
		}

		var manifest map[string]interface{}
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			t.Fatalf("failed to parse manifest: %v", err)
		}

		dependencies, ok := manifest["dependencies"].(map[string]interface{})
		if !ok {
			t.Fatal("manifest missing dependencies")
		}

		version, exists := dependencies["com.test.package"].(string)
		if !exists {
			t.Fatal("package not found in dependencies")
		}

		if version != "1.0.0" {
			t.Errorf("wrong version in manifest: got %s, want 1.0.0", version)
		}

		// Verify scoped registry was added
		scopedRegistries, ok := manifest["scopedRegistries"].([]interface{})
		if !ok || len(scopedRegistries) == 0 {
			t.Fatal("scoped registry not added to manifest")
		}

		registry := scopedRegistries[0].(map[string]interface{})
		scopes := registry["scopes"].([]interface{})
		if len(scopes) == 0 || scopes[0].(string) != "com.test" {
			t.Errorf("wrong scope in scoped registry: got %v, want com.test", scopes)
		}
	})
}

// setupUnityProject creates a basic Unity project structure
func setupUnityProject(path string) error {
	if err := os.MkdirAll(filepath.Join(path, "Assets"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(path, "ProjectSettings"), 0755); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(path, "Packages"), 0755)
}

// executeCommand runs a gpm command and returns output and exit code
func executeCommand(args ...string) (string, int) {
	// Create a completely fresh root command instance for each test to avoid state contamination
	rootCmd := &cobra.Command{
		Use:   "gpm",
		Short: "GPM.sh - Game Package Manager CLI",
		Long: `GPM.sh CLI - A game-dev package registry with npm-compatible workflows
but explicit, studio-aware rules for Unity and other game engines.`,
		Version:       cmd.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Re-create all commands with fresh instances to ensure no global state
	cmd.AddCommands(rootCmd)

	// Reset flags to their default values by re-parsing
	_ = rootCmd.ParseFlags([]string{})

	// Create buffers to capture output
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)

	// Set the arguments
	rootCmd.SetArgs(args)

	// Track exit code
	var exitCode int

	// Execute the command
	if err := rootCmd.Execute(); err != nil {
		exitCode = 1
	}

	// Combine stdout and stderr for output
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	return output, exitCode
}
