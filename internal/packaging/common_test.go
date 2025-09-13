package packaging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPackageSpecType(t *testing.T) {
	// Create a temporary test directory
	tempDir, err := os.MkdirTemp("", "gpm-packaging-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Test folder with package.json
	packageJSON := `{
		"name": "test-package",
		"version": "1.0.0",
		"description": "Test package"
	}`
	err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	specType := DetectPackageSpecType(tempDir)
	if specType != "folder" {
		t.Errorf("Expected 'folder' for directory with package.json, got %s", specType)
	}

	// Test folder without package.json
	tempDirEmpty, err := os.MkdirTemp("", "gpm-packaging-empty-*")
	if err != nil {
		t.Fatalf("Failed to create empty temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDirEmpty) }()

	specType = DetectPackageSpecType(tempDirEmpty)
	if specType != "folder_no_package_json" {
		t.Errorf("Expected 'folder_no_package_json' for directory without package.json, got %s", specType)
	}

	// Test tarball
	specType = DetectPackageSpecType("test-package-1.0.0.tgz")
	if specType != "tarball" {
		t.Errorf("Expected 'tarball' for .tgz file, got %s", specType)
	}

	// Test invalid spec
	specType = DetectPackageSpecType("nonexistent.txt")
	if specType != "unknown" {
		t.Errorf("Expected 'unknown' for invalid spec, got %s", specType)
	}

	t.Log("Package spec type detection test passed")
}

func TestFilenameValidation(t *testing.T) {
	// Test valid package names for filenames
	validNames := []string{
		"test-package",
		"my.package",
		"x-package",
		"com.example.package",
	}

	for _, name := range validNames {
		if !IsValidPackageNameForFilename(name) {
			t.Errorf("Expected valid package name for filename: %s", name)
		}
	}

	// Test invalid package names for filenames
	invalidNames := []string{
		"",
		"test/package", // path separators not allowed in middle
		"test\\package",
		"test|package",
	}

	for _, name := range invalidNames {
		if IsValidPackageNameForFilename(name) {
			t.Errorf("Expected invalid package name for filename: %s", name)
		}
	}

	// Test valid versions for filenames
	validVersions := []string{
		"1.0.0",
		"1.0.0-alpha",
		"1.0.0-beta.1",
	}

	for _, version := range validVersions {
		if !IsValidVersionForFilename(version) {
			t.Errorf("Expected valid version for filename: %s", version)
		}
	}

	// Test invalid versions for filenames
	invalidVersions := []string{
		"",
		"1.0/0", // path separators not allowed
		"1.0\\0",
	}

	for _, version := range invalidVersions {
		if IsValidVersionForFilename(version) {
			t.Errorf("Expected invalid version for filename: %s", version)
		}
	}

	t.Log("Filename validation test passed")
}

func TestJSONOutput(t *testing.T) {
	// Test JSON output format
	testData := map[string]interface{}{
		"name":    "test-package",
		"version": "1.0.0",
		"files":   []string{"src/", "dist/"},
		"success": true,
	}

	data, err := json.MarshalIndent(testData, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if unmarshaled["name"] != "test-package" {
		t.Errorf("Expected name 'test-package', got %v", unmarshaled["name"])
	}

	t.Log("JSON output test passed")
}
