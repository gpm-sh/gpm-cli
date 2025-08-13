package validation

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gpm.sh/gpm/gpm-cli/internal/errors"
)

type PackageJSON struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description,omitempty"`
	Author       string            `json:"author,omitempty"`
	License      string            `json:"license,omitempty"`
	Repository   string            `json:"repository,omitempty"`
	Homepage     string            `json:"homepage,omitempty"`
	Keywords     []string          `json:"keywords,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Files        []string          `json:"files,omitempty"`
	Main         string            `json:"main,omitempty"`
	Unity        string            `json:"unity,omitempty"`
	DisplayName  string            `json:"displayName,omitempty"`
	Category     string            `json:"category,omitempty"`
}

type ValidationResult struct {
	Valid    bool
	Package  *PackageJSON
	Errors   []error
	Warnings []string
}

func ValidatePackage(path string) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []error{},
		Warnings: []string{},
	}

	// Security: Clean the path and validate it's safe
	cleanPath := filepath.Clean(path)
	packagePath := filepath.Join(cleanPath, "package.json")
	cleanPackagePath := filepath.Clean(packagePath)

	// Security: Ensure the resulting path is still within the expected directory
	if !strings.HasPrefix(cleanPackagePath, cleanPath) {
		return nil, fmt.Errorf("invalid path: potential directory traversal")
	}

	data, err := os.ReadFile(cleanPackagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse package.json: %w", err)
	}

	result.Package = &pkg

	if err := validateRequiredFields(&pkg); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err)
	}

	if err := validatePackageName(pkg.Name); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err)
	}

	if err := validateVersion(pkg.Version); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err)
	}

	if err := validateUnityField(&pkg); err != nil {
		result.Warnings = append(result.Warnings, err.Error())
	}

	return result, nil
}

func validateRequiredFields(pkg *PackageJSON) error {
	if pkg.Name == "" {
		return errors.ErrPackageJSONInvalid("name")
	}
	if pkg.Version == "" {
		return errors.ErrPackageJSONInvalid("version")
	}
	if pkg.Description == "" {
		return errors.ErrPackageJSONInvalid("description")
	}
	return nil
}

func validatePackageName(name string) error {
	return errors.ValidatePackageName(name)
}

func validateVersion(version string) error {
	if version == "" {
		return errors.ErrVersionInvalid(version)
	}
	return nil
}

func validateUnityField(pkg *PackageJSON) error {
	if pkg.Unity == "" && pkg.DisplayName == "" {
		return fmt.Errorf("unity package should include 'unity' or 'displayName' field for better UPM compatibility")
	}
	return nil
}

func CreateTarball(path string) (string, error) {
	cmd := exec.Command("npm", "pack", "--json")
	cmd.Dir = path

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("npm pack failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("npm pack failed: %w", err)
	}

	var packResult []map[string]interface{}
	if err := json.Unmarshal(output, &packResult); err != nil {
		return "", fmt.Errorf("failed to parse npm pack output: %w", err)
	}

	if len(packResult) == 0 {
		return "", fmt.Errorf("npm pack produced no output")
	}

	filename, ok := packResult[0]["filename"].(string)
	if !ok {
		return "", fmt.Errorf("invalid npm pack output format")
	}

	tarballPath := filepath.Join(path, filename)
	if _, err := os.Stat(tarballPath); err != nil {
		return "", fmt.Errorf("tarball file not found: %w", err)
	}

	return tarballPath, nil
}

func ValidateTarball(tarballPath string) error {
	if !strings.HasSuffix(tarballPath, ".tgz") {
		return fmt.Errorf("tarball must have .tgz extension")
	}

	info, err := os.Stat(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to stat tarball: %w", err)
	}

	if info.Size() == 0 {
		return errors.ErrTarballInvalid()
	}

	return nil
}

func CleanupTarball(tarballPath string) error {
	return os.Remove(tarballPath)
}
