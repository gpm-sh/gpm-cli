package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gpm.sh/gpm/gpm-cli/internal/engines"
)

func TestParseAddPackageSpec(t *testing.T) {
	tests := []struct {
		input       string
		wantName    string
		wantVersion string
		wantError   bool
	}{
		{"com.unity.analytics", "com.unity.analytics", "", false},
		{"com.unity.analytics@2.1.0", "com.unity.analytics", "2.1.0", false},
		{"com.company.sdk@latest", "com.company.sdk", "latest", false},
		{"", "", "", true},
		{"package@@version", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, version, err := parseAddPackageSpec(tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error for input %q, got none", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
				return
			}

			if name != tt.wantName {
				t.Errorf("wrong name: got %q, want %q", name, tt.wantName)
			}

			if version != tt.wantVersion {
				t.Errorf("wrong version: got %q, want %q", version, tt.wantVersion)
			}
		})
	}
}

func TestDetectOrValidateEngine(t *testing.T) {
	tests := []struct {
		name         string
		engineFlag   string
		setupProject func(string) error
		wantEngine   engines.EngineType
		wantError    bool
	}{
		{
			name:         "explicit unity engine",
			engineFlag:   "unity",
			setupProject: func(string) error { return nil },
			wantEngine:   engines.EngineUnity,
			wantError:    false,
		},
		{
			name:         "unsupported engine flag",
			engineFlag:   "invalid",
			setupProject: func(string) error { return nil },
			wantEngine:   engines.EngineUnknown,
			wantError:    true,
		},
		{
			name:       "auto-detect unity project",
			engineFlag: "auto",
			setupProject: func(path string) error {
				// Create Unity project structure
				if err := os.MkdirAll(filepath.Join(path, "Assets"), 0755); err != nil {
					return err
				}
				return os.MkdirAll(filepath.Join(path, "ProjectSettings"), 0755)
			},
			wantEngine: engines.EngineUnity,
			wantError:  false,
		},
		{
			name:         "auto-detect no engine",
			engineFlag:   "auto",
			setupProject: func(string) error { return nil },
			wantEngine:   engines.EngineUnknown,
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if err := tt.setupProject(tmpDir); err != nil {
				t.Fatalf("failed to setup test project: %v", err)
			}

			engine, err := detectOrValidateEngine(tmpDir, tt.engineFlag)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if engine != tt.wantEngine {
				t.Errorf("wrong engine: got %v, want %v", engine, tt.wantEngine)
			}
		})
	}
}

func TestUnityProjectBackupAndRestore(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "project")

	// Create Unity project structure
	manifestDir := filepath.Join(projectPath, "Packages")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatalf("failed to create manifest directory: %v", err)
	}

	// Create initial manifest
	initialManifest := `{
  "dependencies": {
    "com.unity.test": "1.0.0"
  }
}`
	manifestPath := filepath.Join(manifestDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(initialManifest), 0644); err != nil {
		t.Fatalf("failed to write initial manifest: %v", err)
	}

	// Test backup
	backupPath, err := createProjectBackup(projectPath, engines.EngineUnity)
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	// Verify backup exists
	backupManifestPath := filepath.Join(backupPath, "manifest.json")
	if _, err := os.Stat(backupManifestPath); os.IsNotExist(err) {
		t.Errorf("backup manifest not created at %s", backupManifestPath)
	}

	// Verify backup content
	backupData, err := os.ReadFile(backupManifestPath)
	if err != nil {
		t.Fatalf("failed to read backup manifest: %v", err)
	}

	if string(backupData) != initialManifest {
		t.Errorf("backup content mismatch:\ngot:  %s\nwant: %s", string(backupData), initialManifest)
	}

	// Modify original manifest
	modifiedManifest := `{
  "dependencies": {
    "com.unity.test": "1.0.0",
    "com.new.package": "2.0.0"
  }
}`
	if err := os.WriteFile(manifestPath, []byte(modifiedManifest), 0644); err != nil {
		t.Fatalf("failed to modify manifest: %v", err)
	}

	// Test restore
	if err := restoreFromBackup(backupPath, projectPath, engines.EngineUnity); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	// Verify restoration
	restoredData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read restored manifest: %v", err)
	}

	if string(restoredData) != initialManifest {
		t.Errorf("restore content mismatch:\ngot:  %s\nwant: %s", string(restoredData), initialManifest)
	}
}

func TestAddCommandJSONOutput(t *testing.T) {
	// Test JSON marshaling
	output := &AddOutput{
		Success:  true,
		Engine:   "unity",
		Project:  "/test/project",
		Package:  "com.test.package",
		Version:  "1.0.0",
		Registry: "https://registry.gpm.sh",
		Changed:  true,
		Message:  "Package added successfully",
		Details:  map[string]any{"manifest_path": "/test/project/Packages/manifest.json"},
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		t.Fatalf("JSON marshaling failed: %v", err)
	}

	// Verify JSON structure
	var parsed AddOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshaling failed: %v", err)
	}

	if parsed.Success != output.Success {
		t.Errorf("Success mismatch: got %v, want %v", parsed.Success, output.Success)
	}

	if parsed.Package != output.Package {
		t.Errorf("Package mismatch: got %q, want %q", parsed.Package, output.Package)
	}
}

func TestUnityManifestNoOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "project")

	// Create Unity project structure
	manifestDir := filepath.Join(projectPath, "Packages")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatalf("failed to create manifest directory: %v", err)
	}

	// Create Assets directory required for Unity project validation
	assetsDir := filepath.Join(projectPath, "Assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("failed to create Assets directory: %v", err)
	}

	// Create ProjectSettings directory required for Unity project validation
	projectSettingsDir := filepath.Join(projectPath, "ProjectSettings")
	if err := os.MkdirAll(projectSettingsDir, 0755); err != nil {
		t.Fatalf("failed to create ProjectSettings directory: %v", err)
	}

	// Create Unity adapter
	adapter := engines.NewUnityAdapter()

	// First installation
	req1 := &engines.PackageInstallRequest{
		Name:     "com.test.package",
		Version:  "1.0.0",
		Registry: "https://test.gpm.sh",
	}

	result1, err := adapter.InstallPackage(projectPath, req1)
	if err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	if !result1.Success {
		t.Errorf("first install not successful: %s", result1.Message)
	}

	// Check existing package info
	existingInfo, err := adapter.GetPackageInfo(projectPath, "com.test.package")
	if err != nil {
		t.Fatalf("failed to get existing package info: %v", err)
	}

	if existingInfo.Version != "1.0.0" {
		t.Errorf("wrong existing version: got %q, want %q", existingInfo.Version, "1.0.0")
	}

	// Second installation with same version (should be no-op)
	result2, err := adapter.InstallPackage(projectPath, req1)
	if err != nil {
		t.Fatalf("second install failed: %v", err)
	}

	if !result2.Success {
		t.Errorf("second install not successful: %s", result2.Message)
	}

	// Installation with different version should update
	req3 := &engines.PackageInstallRequest{
		Name:     "com.test.package",
		Version:  "2.0.0",
		Registry: "https://test.gpm.sh",
	}

	result3, err := adapter.InstallPackage(projectPath, req3)
	if err != nil {
		t.Fatalf("third install failed: %v", err)
	}

	if !result3.Success {
		t.Errorf("third install not successful: %s", result3.Message)
	}

	// Verify updated version
	updatedInfo, err := adapter.GetPackageInfo(projectPath, "com.test.package")
	if err != nil {
		t.Fatalf("failed to get updated package info: %v", err)
	}

	if updatedInfo.Version != "2.0.0" {
		t.Errorf("wrong updated version: got %q, want %q", updatedInfo.Version, "2.0.0")
	}
}

func TestScopeDerivation(t *testing.T) {
	tests := []struct {
		packageName   string
		expectedScope string
	}{
		{"com.unity.analytics", "com.unity"},
		{"com.tapnation.sdk", "com.tapnation"},
		{"org.example.package", "org.example"},
		{"single", "single"},
		{"a.b.c.d.e", "a.b"},
	}

	for _, tt := range tests {
		t.Run(tt.packageName, func(t *testing.T) {
			scope := engines.DeriveScopeFromPackageName(tt.packageName)
			if scope != tt.expectedScope {
				t.Errorf("wrong scope: got %q, want %q", scope, tt.expectedScope)
			}
		})
	}
}

func TestAddCommandIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Unity project
	assetsDir := filepath.Join(tmpDir, "Assets")
	projectSettingsDir := filepath.Join(tmpDir, "ProjectSettings")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("failed to create Assets directory: %v", err)
	}
	if err := os.MkdirAll(projectSettingsDir, 0755); err != nil {
		t.Fatalf("failed to create ProjectSettings directory: %v", err)
	}

	// Test package specification parsing in integration
	packageSpec := "com.test.package@1.0.0"
	packageName, version, err := parseAddPackageSpec(packageSpec)
	if err != nil {
		t.Fatalf("failed to parse package spec: %v", err)
	}

	if packageName != "com.test.package" {
		t.Errorf("wrong package name: got %q, want %q", packageName, "com.test.package")
	}

	if version != "1.0.0" {
		t.Errorf("wrong version: got %q, want %q", version, "1.0.0")
	}

	// Test engine detection
	engineType, err := detectOrValidateEngine(tmpDir, "auto")
	if err != nil {
		t.Fatalf("engine detection failed: %v", err)
	}

	if engineType != engines.EngineUnity {
		t.Errorf("wrong engine detected: got %v, want %v", engineType, engines.EngineUnity)
	}

	// Test adapter
	adapter, err := engines.GetAdapter(engineType)
	if err != nil {
		t.Fatalf("failed to get adapter: %v", err)
	}

	if err := adapter.ValidateProject(tmpDir); err != nil {
		t.Fatalf("project validation failed: %v", err)
	}
}
