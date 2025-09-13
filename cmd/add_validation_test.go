package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gpm.sh/gpm/gpm-cli/internal/api"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/engines"
	"gpm.sh/gpm/gpm-cli/internal/validation"
)

// MockRegistry provides test responses for registry queries
type MockRegistry struct {
	server   *httptest.Server
	packages map[string]*api.PackageMetadata
}

func NewMockRegistry() *MockRegistry {
	mr := &MockRegistry{
		packages: make(map[string]*api.PackageMetadata),
	}

	mr.server = httptest.NewServer(http.HandlerFunc(mr.handler))
	return mr
}

func (mr *MockRegistry) Close() {
	mr.server.Close()
}

func (mr *MockRegistry) URL() string {
	return mr.server.URL
}

func (mr *MockRegistry) AddPackage(name string, metadata *api.PackageMetadata) {
	mr.packages[name] = metadata
}

func (mr *MockRegistry) handler(w http.ResponseWriter, r *http.Request) {
	// Handle package metadata requests
	if strings.HasPrefix(r.URL.Path, "/") && !strings.HasPrefix(r.URL.Path, "/-/") {
		packageName := strings.TrimPrefix(r.URL.Path, "/")

		if metadata, exists := mr.packages[packageName]; exists {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Simple JSON encoding for test
			_, _ = fmt.Fprintf(w, `{
				"name": "%s",
				"description": "%s",
				"dist-tags": %s,
				"versions": %s
			}`, metadata.Name, metadata.Description, marshalDistTags(metadata.DistTags), marshalVersions(metadata.Versions))
			return
		}

		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, `{"error":"Not found"}`)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func marshalDistTags(distTags map[string]string) string {
	if distTags == nil {
		return "null"
	}

	parts := []string{}
	for k, v := range distTags {
		parts = append(parts, fmt.Sprintf(`"%s":"%s"`, k, v))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func marshalVersions(versions map[string]*api.PackageVersion) string {
	if versions == nil {
		return "null"
	}

	parts := []string{}
	for k, v := range versions {
		parts = append(parts, fmt.Sprintf(`"%s":{"name":"%s","version":"%s"}`, k, v.Name, v.Version))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func TestAddCommandValidation(t *testing.T) {
	// Setup mock registry
	mockRegistry := NewMockRegistry()
	defer mockRegistry.Close()

	// Add test packages
	mockRegistry.AddPackage("com.test.valid", &api.PackageMetadata{
		Name:        "com.test.valid",
		Description: "A valid test package",
		DistTags:    map[string]string{"latest": "1.0.0"},
		Versions: map[string]*api.PackageVersion{
			"1.0.0": {Name: "com.test.valid", Version: "1.0.0"},
			"0.9.0": {Name: "com.test.valid", Version: "0.9.0"},
		},
	})

	mockRegistry.AddPackage("com.test.no-latest", &api.PackageMetadata{
		Name:        "com.test.no-latest",
		Description: "Package without latest tag",
		DistTags:    map[string]string{}, // No latest tag
		Versions: map[string]*api.PackageVersion{
			"1.0.0": {Name: "com.test.no-latest", Version: "1.0.0"},
		},
	})

	mockRegistry.AddPackage("com.test.no-dist-tags", &api.PackageMetadata{
		Name:        "com.test.no-dist-tags",
		Description: "Package without dist-tags",
		DistTags:    nil, // No dist-tags at all
		Versions: map[string]*api.PackageVersion{
			"1.0.0": {Name: "com.test.no-dist-tags", Version: "1.0.0"},
		},
	})

	tests := []struct {
		name          string
		packageSpec   string
		registryURL   string
		setupProject  func(string) error
		wantError     bool
		errorContains string
	}{
		{
			name:        "non-existent package",
			packageSpec: "com.test.nonexistent@1.0.0",
			registryURL: mockRegistry.URL(),
			setupProject: func(path string) error {
				return setupUnityProject(path)
			},
			wantError:     true,
			errorContains: "not found",
		},
		{
			name:        "valid package with latest",
			packageSpec: "com.test.valid",
			registryURL: mockRegistry.URL(),
			setupProject: func(path string) error {
				return setupUnityProject(path)
			},
			wantError: false,
		},
		{
			name:        "valid package with specific version",
			packageSpec: "com.test.valid@0.9.0",
			registryURL: mockRegistry.URL(),
			setupProject: func(path string) error {
				return setupUnityProject(path)
			},
			wantError: false,
		},
		{
			name:        "package with missing version",
			packageSpec: "com.test.valid@2.0.0",
			registryURL: mockRegistry.URL(),
			setupProject: func(path string) error {
				return setupUnityProject(path)
			},
			wantError:     true,
			errorContains: "version '2.0.0' not available",
		},
		{
			name:        "package with no latest dist-tag",
			packageSpec: "com.test.no-latest",
			registryURL: mockRegistry.URL(),
			setupProject: func(path string) error {
				return setupUnityProject(path)
			},
			wantError:     true,
			errorContains: "no 'latest' dist-tag",
		},
		{
			name:        "package with no dist-tags",
			packageSpec: "com.test.no-dist-tags",
			registryURL: mockRegistry.URL(),
			setupProject: func(path string) error {
				return setupUnityProject(path)
			},
			wantError:     true,
			errorContains: "no dist-tags",
		},
		{
			name:        "invalid package name",
			packageSpec: "com.Invalid.Name@1.0.0",
			registryURL: mockRegistry.URL(),
			setupProject: func(path string) error {
				return setupUnityProject(path)
			},
			wantError:     true,
			errorContains: "invalid package name",
		},
		{
			name:        "no engine detected",
			packageSpec: "com.test.valid@1.0.0",
			registryURL: mockRegistry.URL(),
			setupProject: func(path string) error {
				// Don't create Unity structure
				return nil
			},
			wantError:     true,
			errorContains: "no supported engine detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Setup project
			if err := tt.setupProject(tmpDir); err != nil {
				t.Fatalf("failed to setup project: %v", err)
			}

			// Setup config for test
			oldConfig := config.GetConfig()
			defer func() {
				config.SetConfigForTesting(oldConfig)
			}()

			config.SetConfigForTesting(&config.Config{
				Registry: tt.registryURL,
			})

			// Parse package spec
			packageName, version, err := parseAddPackageSpec(tt.packageSpec)
			if err != nil && !tt.wantError {
				t.Fatalf("unexpected error parsing package spec: %v", err)
			}

			// Run add validation
			output := &AddOutput{Details: make(map[string]any)}
			output.Package = packageName
			output.Version = version

			// Test only the validation parts
			err = validateAddRequest(tmpDir, packageName, version, tt.registryURL, output)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got none")
					return
				}
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errorContains)) {
					t.Errorf("error doesn't contain expected text:\ngot:  %s\nwant: %s", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// validateAddRequest simulates the validation part of executeAdd
func validateAddRequest(projectPath, packageName, version, registryURL string, output *AddOutput) error {
	// Detect engine
	engineType, err := detectOrValidateEngine(projectPath, "auto")
	if err != nil {
		return err
	}
	output.Engine = string(engineType)

	// Get engine adapter
	adapter, err := engines.GetAdapter(engineType)
	if err != nil {
		return fmt.Errorf("engine adapter not available: %w", err)
	}

	// Validate project
	if err := adapter.ValidateProject(projectPath); err != nil {
		return fmt.Errorf("project validation failed: %w", err)
	}

	output.Registry = registryURL

	// Validate package name first
	if err := validation.ValidatePackageName(packageName); err != nil {
		return fmt.Errorf("invalid package name: %w", err)
	}

	// Query registry for package metadata - fail fast if package doesn't exist
	client := api.NewClient(registryURL, "")

	// Check if package exists in registry
	packageExists, err := client.CheckPackageExists(packageName)
	if err != nil {
		return fmt.Errorf("failed to check package existence: %w", err)
	}
	if !packageExists {
		return fmt.Errorf("package '%s' not found in registry", packageName)
	}

	// Resolve and validate version
	resolvedVersion, err := client.ResolvePackageVersion(packageName, version)
	if err != nil {
		return err
	}
	output.Version = resolvedVersion

	return nil
}

func setupUnityProject(path string) error {
	if err := os.MkdirAll(filepath.Join(path, "Assets"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(path, "ProjectSettings"), 0755); err != nil {
		return err
	}
	return nil
}

func TestAddIdempotentBehavior(t *testing.T) {
	// Setup mock registry
	mockRegistry := NewMockRegistry()
	defer mockRegistry.Close()

	mockRegistry.AddPackage("com.test.package", &api.PackageMetadata{
		Name:        "com.test.package",
		Description: "Test package for idempotent behavior",
		DistTags:    map[string]string{"latest": "1.0.0"},
		Versions: map[string]*api.PackageVersion{
			"1.0.0": {Name: "com.test.package", Version: "1.0.0"},
		},
	})

	tmpDir := t.TempDir()
	if err := setupUnityProject(tmpDir); err != nil {
		t.Fatalf("failed to setup Unity project: %v", err)
	}

	// Setup config
	oldConfig := config.GetConfig()
	defer func() {
		config.SetConfigForTesting(oldConfig)
	}()

	config.SetConfigForTesting(&config.Config{
		Registry: mockRegistry.URL(),
	})

	// Create Unity adapter
	adapter := engines.NewUnityAdapter()

	// First installation
	req := &engines.PackageInstallRequest{
		Name:     "com.test.package",
		Version:  "1.0.0",
		Registry: mockRegistry.URL(),
	}

	_, err := adapter.InstallPackage(tmpDir, req)
	if err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	// Test idempotent behavior
	output := &AddOutput{Details: make(map[string]any)}
	err = validateAddRequest(tmpDir, "com.test.package", "1.0.0", mockRegistry.URL(), output)
	if err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	// Check if already installed (this would be done in the full executeAdd)
	existingInfo, _ := adapter.GetPackageInfo(tmpDir, "com.test.package")
	if existingInfo == nil {
		t.Errorf("package should be installed")
	} else if existingInfo.Version != "1.0.0" {
		t.Errorf("wrong version: got %s, want 1.0.0", existingInfo.Version)
	}
}

func TestAddRollbackOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	if err := setupUnityProject(tmpDir); err != nil {
		t.Fatalf("failed to setup Unity project: %v", err)
	}

	// Create initial manifest
	manifestDir := filepath.Join(tmpDir, "Packages")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatalf("failed to create manifest directory: %v", err)
	}

	initialManifest := `{
  "dependencies": {
    "com.unity.core": "1.0.0"
  }
}`
	manifestPath := filepath.Join(manifestDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(initialManifest), 0644); err != nil {
		t.Fatalf("failed to write initial manifest: %v", err)
	}

	// Create backup
	backupPath, err := createProjectBackup(tmpDir, engines.EngineUnity)
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	// Simulate failure by corrupting manifest
	corruptedManifest := `{invalid json`
	if err := os.WriteFile(manifestPath, []byte(corruptedManifest), 0644); err != nil {
		t.Fatalf("failed to corrupt manifest: %v", err)
	}

	// Test restore
	if err := restoreFromBackup(backupPath, tmpDir, engines.EngineUnity); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	// Verify restoration
	restoredData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read restored manifest: %v", err)
	}

	if strings.TrimSpace(string(restoredData)) != strings.TrimSpace(initialManifest) {
		t.Errorf("restore failed:\ngot:  %s\nwant: %s", string(restoredData), initialManifest)
	}
}
