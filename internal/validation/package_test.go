package validation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackageValidation(t *testing.T) {
	// Create a temporary test directory
	tempDir, err := os.MkdirTemp("", "gpm-validation-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Test valid package.json
	validPackageJSON := `{
		"name": "test-package",
		"version": "1.0.0",
		"description": "Test package",
		"author": "Test Author",
		"license": "MIT",
		"repository": "https://github.com/test/test-package",
		"keywords": ["test", "example"],
		"homepage": "https://test-package.example.com"
	}`
	err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(validPackageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	result, err := ValidatePackage(tempDir)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected valid package, got validation errors: %v", result.Errors)
	}

	if result.RecommendedAccess != AccessPublic {
		t.Errorf("Expected public access for unscoped package, got %s", result.RecommendedAccess)
	}

	// Test scoped package
	scopedPackageJSON := `{
		"name": "@test-scope/test-package",
		"version": "1.0.0",
		"description": "Test scoped package"
	}`
	err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(scopedPackageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write scoped package.json: %v", err)
	}

	result, err = ValidatePackage(tempDir)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if result.RecommendedAccess != AccessScoped {
		t.Errorf("Expected scoped access for scoped package, got %s", result.RecommendedAccess)
	}

	t.Log("Package validation test passed")
}

func TestNpmCompatibility(t *testing.T) {
	// Test npm-compatible package names
	validNames := []string{
		"my-package",
		"my_package",
		"my.package",
		"@scope/package",
		"@scope/sub-package",
	}

	invalidNames := []string{
		"MyPackage",  // uppercase
		"my package", // space
		".hidden",    // starts with dot
		"_private",   // starts with underscore
		"",           // empty
	}

	for _, name := range validNames {
		if !IsNpmCompatible(&PackageJSON{Name: name, Version: "1.0.0"}) {
			t.Errorf("Expected valid npm package name: %s", name)
		}
	}

	for _, name := range invalidNames {
		if IsNpmCompatible(&PackageJSON{Name: name, Version: "1.0.0"}) {
			t.Errorf("Expected invalid npm package name: %s", name)
		}
	}

	// Test semantic version validation
	validVersions := []string{
		"1.0.0",
		"1.0.0-alpha",
		"1.0.0-beta.1",
		"1.0.0+20130313144700",
	}

	invalidVersions := []string{
		"1.0",
		"1.0.0.0",
		"v1.0.0",
		"1.0.0-alpha.01",
	}

	for _, version := range validVersions {
		if !IsNpmCompatible(&PackageJSON{Name: "my-package", Version: version}) {
			t.Errorf("Expected valid semantic version: %s", version)
		}
	}

	for _, version := range invalidVersions {
		if IsNpmCompatible(&PackageJSON{Name: "my-package", Version: version}) {
			t.Errorf("Expected invalid semantic version: %s", version)
		}
	}

	t.Log("NPM compatibility test passed")
}

func TestAccessLevelValidation(t *testing.T) {
	// Test access level validation
	testCases := []struct {
		access      string
		packageName string
		shouldPass  bool
	}{
		{"public", "my-package", true},
		{"public", "@scope/package", false}, // scoped packages can't be public
		{"scoped", "my-package", false},     // unscoped packages can't be scoped
		{"scoped", "@scope/package", true},
		{"private", "my-package", true},
		{"private", "@scope/package", true},
		{"invalid", "my-package", false},
	}

	for _, tc := range testCases {
		err := ValidateAccessLevel(tc.access, tc.packageName)
		if tc.shouldPass && err != nil {
			t.Errorf("Expected access level %s to pass for package %s, got error: %v",
				tc.access, tc.packageName, err)
		}
		if !tc.shouldPass && err == nil {
			t.Errorf("Expected access level %s to fail for package %s, but it passed",
				tc.access, tc.packageName)
		}
	}

	// Test dist-tag validation
	validTags := []string{"latest", "beta", "alpha", "1.0.0", "v1.0.0"}
	invalidTags := []string{"", ".beta", "-alpha", "tag with spaces"}

	for _, tag := range validTags {
		if err := ValidateDistTag(tag); err != nil {
			t.Errorf("Expected valid dist-tag %s, got error: %v", tag, err)
		}
	}

	for _, tag := range invalidTags {
		if err := ValidateDistTag(tag); err == nil {
			t.Errorf("Expected invalid dist-tag %s to fail validation", tag)
		}
	}

	t.Log("Access level validation test passed")
}
