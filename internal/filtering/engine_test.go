package filtering

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileFilterEngine(t *testing.T) {
	// Create a temporary test directory
	tempDir, err := os.MkdirTemp("", "gpm-filtering-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create package.json with files field
	packageJSON := `{
		"name": "test-package",
		"version": "1.0.0",
		"description": "Test package",
		"files": ["src/", "dist/", "*.js"]
	}`
	err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	// Create .gpmignore
	gpmignore := `# GPM ignore test
node_modules/
.git/
*.log
test/
coverage/`
	err = os.WriteFile(filepath.Join(tempDir, ".gpmignore"), []byte(gpmignore), 0644)
	if err != nil {
		t.Fatalf("Failed to write .gpmignore: %v", err)
	}

	// Create test files
	testFiles := []string{
		"src/main.js",
		"dist/bundle.js",
		"index.js",
		"README.md",
		"test/test.js",
		"node_modules/lodash/index.js",
		".git/config",
		"coverage/report.html",
	}

	for _, file := range testFiles {
		dir := filepath.Dir(filepath.Join(tempDir, file))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(tempDir, file), []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", file, err)
		}
	}

	// Test file filtering
	engine, err := NewFileFilterEngine(tempDir)
	if err != nil {
		t.Fatalf("Failed to create file filter engine: %v", err)
	}

	result, err := engine.FilterFiles()
	if err != nil {
		t.Fatalf("Failed to filter files: %v", err)
	}

	// Verify that files field is respected (should only include specified files)
	expectedFiles := []string{"src/main.js", "dist/bundle.js", "index.js"}
	for _, expected := range expectedFiles {
		found := false
		for _, file := range result.Files {
			if file.RelativePath == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected file %s not found in filtered results", expected)
		}
	}

	// Verify that ignored files are excluded
	excludedFiles := []string{"test/test.js", "node_modules/lodash/index.js", ".git/config", "coverage/report.html"}
	for _, excluded := range excludedFiles {
		for _, file := range result.Files {
			if file.RelativePath == excluded {
				t.Errorf("Excluded file %s found in filtered results", excluded)
			}
		}
	}

	t.Logf("File filtering test passed. Included %d files, total size: %d bytes",
		result.FileCount, result.TotalSize)
}

func TestGpmignorePriority(t *testing.T) {
	// Create a temporary test directory
	tempDir, err := os.MkdirTemp("", "gpm-priority-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create package.json without files field to test ignore files
	packageJSON := `{
		"name": "test-package",
		"version": "1.0.0",
		"description": "Test package"
	}`
	err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	// Create .gpmignore that excludes src/
	gpmignore := `src/
test/`
	err = os.WriteFile(filepath.Join(tempDir, ".gpmignore"), []byte(gpmignore), 0644)
	if err != nil {
		t.Fatalf("Failed to write .gpmignore: %v", err)
	}

	// Create .npmignore that excludes dist/ (should be ignored)
	npmignore := `dist/
coverage/`
	err = os.WriteFile(filepath.Join(tempDir, ".npmignore"), []byte(npmignore), 0644)
	if err != nil {
		t.Fatalf("Failed to write .npmignore: %v", err)
	}

	// Create test files
	testFiles := []string{
		"src/main.js",
		"dist/bundle.js",
		"index.js",
		"test/test.js",
	}

	for _, file := range testFiles {
		dir := filepath.Dir(filepath.Join(tempDir, file))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(tempDir, file), []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", file, err)
		}
	}

	// Test file filtering
	engine, err := NewFileFilterEngine(tempDir)
	if err != nil {
		t.Fatalf("Failed to create file filter engine: %v", err)
	}

	// Debug: Check if files field is detected
	t.Logf("Has files field: %v", engine.HasFilesField())

	result, err := engine.FilterFiles()
	if err != nil {
		t.Fatalf("Failed to filter files: %v", err)
	}

	// Check that files are filtered correctly by .gpmignore
	// Since there's no files field, ignore files should be used

	var includedFiles []string

	for _, file := range result.Files {
		if !file.IsDir {
			includedFiles = append(includedFiles, file.RelativePath)
		}
	}

	// dist/bundle.js should be included (not in .gpmignore)
	// index.js should be included (not in .gpmignore)
	// package.json should be included (builtin include)
	// src/main.js should be excluded (by .gpmignore src/)
	// test/test.js should be excluded (by .gpmignore test/)
	// .gpmignore should be excluded (builtin exclude)
	// .npmignore should be excluded (builtin exclude)

	shouldBeIncluded := []string{"dist/bundle.js", "index.js", "package.json"}
	shouldBeExcluded := []string{"src/main.js", "test/test.js", ".gpmignore", ".npmignore"}

	for _, expected := range shouldBeIncluded {
		found := false
		for _, included := range includedFiles {
			if included == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected file %s to be included but was not found. Included files: %v", expected, includedFiles)
		}
	}

	for _, excluded := range shouldBeExcluded {
		for _, included := range includedFiles {
			if included == excluded {
				t.Errorf("Expected file %s to be excluded but was included. Included files: %v", excluded, includedFiles)
			}
		}
	}

	t.Logf("GPM ignore priority test passed. .gpmignore takes precedence over .npmignore")
}
