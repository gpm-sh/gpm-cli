package validation

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"gpm.sh/gpm/gpm-cli/internal/errors"
)

type AccessLevel string

const (
	AccessPublic  AccessLevel = "public"
	AccessScoped  AccessLevel = "scoped"
	AccessPrivate AccessLevel = "private"
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

// Legacy ValidationResult for backward compatibility
type ValidationResult struct {
	Valid    bool
	Package  *PackageJSON
	Errors   []error
	Warnings []string
}

// Modern ValidationResult with additional features
type PackageValidationResult struct {
	Valid             bool
	Package           *PackageJSON
	Errors            []error
	Warnings          []string
	RecommendedAccess AccessLevel
	NpmCompatible     bool
	RequiredFields    []string
}

var (
	npmNameRegex         = regexp.MustCompile(`^(@[a-z0-9-~][a-z0-9-._~]*\/)?[a-z0-9-~][a-z0-9-._~]*$`)
	scopedNameRegex      = regexp.MustCompile(`^@([a-z0-9-~][a-z0-9-._~]*)\/([a-z0-9-~][a-z0-9-._~]*)$`)
	semanticVersionRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*|[0-9a-zA-Z-]*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*|[0-9a-zA-Z-]*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
)

var reservedNames = []string{
	"node_modules", "favicon.ico", "..", ".", "npm", "gpm", "package", "packages",
	"admin", "administrator", "root", "www", "ftp", "mail", "email", "api",
	"test", "tests", "testing", "spec", "specs", "bin", "binary", "binaries",
	"lib", "libs", "libraries", "src", "source", "sources", "doc", "docs",
	"documentation", "example", "examples", "sample", "samples", "demo", "demos",
}

func ValidatePackage(path string) (*PackageValidationResult, error) {
	result := &PackageValidationResult{
		Valid:          true,
		Errors:         []error{},
		Warnings:       []string{},
		NpmCompatible:  true,
		RequiredFields: []string{"name", "version", "description"},
	}

	// Load package.json
	data, err := os.ReadFile(filepath.Join(path, "package.json")) // #nosec G304 - Path is validated and safe
	if err != nil {
		return nil, fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse package.json: %w", err)
	}

	result.Package = &pkg

	// Modern validation - more lenient than GPM's strict validation
	if pkg.Name == "" {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Errorf("package name is required"))
	}

	if pkg.Version == "" {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Errorf("package version is required"))
	}

	if pkg.Description == "" {
		result.Warnings = append(result.Warnings, "package description is recommended")
	}

	if err := validateNpmCompatibleName(pkg.Name); err != nil {
		result.Valid = false
		result.NpmCompatible = false
		result.Errors = append(result.Errors, err)
	}

	if err := validateSemanticVersion(pkg.Version); err != nil {
		result.Valid = false
		result.NpmCompatible = false
		result.Errors = append(result.Errors, err)
	}

	result.RecommendedAccess = determineRecommendedAccess(pkg.Name)

	validateOptionalFields(result)
	validateUnitySpecificFields(result)
	validateNpmCompatibility(result)

	return result, nil
}

// Validation functions
func validateNpmCompatibleName(name string) error {
	if name == "" {
		return errors.ErrPackageJSONInvalid("name")
	}

	if len(name) > 214 {
		return fmt.Errorf("package name cannot be longer than 214 characters")
	}

	if !npmNameRegex.MatchString(name) {
		return fmt.Errorf("package name %q is not a valid npm package name", name)
	}

	nameLower := strings.ToLower(name)
	if name != nameLower {
		return fmt.Errorf("package name cannot contain uppercase letters")
	}

	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return fmt.Errorf("package name cannot start with . or _")
	}

	for _, reserved := range reservedNames {
		if strings.EqualFold(name, reserved) || strings.EqualFold(name, "@"+reserved) {
			return fmt.Errorf("package name %q is reserved", name)
		}
	}

	for _, r := range name {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return fmt.Errorf("package name cannot contain whitespace or control characters")
		}
	}

	return nil
}

func validateSemanticVersion(version string) error {
	if version == "" {
		return errors.ErrVersionInvalid(version)
	}

	if !semanticVersionRegex.MatchString(version) {
		return fmt.Errorf("version %q is not a valid semantic version", version)
	}

	return nil
}

func determineRecommendedAccess(name string) AccessLevel {
	if scopedNameRegex.MatchString(name) {
		return AccessScoped
	}
	return AccessPublic
}

func validateOptionalFields(result *PackageValidationResult) {
	pkg := result.Package

	if pkg.License == "" {
		result.Warnings = append(result.Warnings, "package.json should include 'license' field")
	}

	if pkg.Author == "" {
		result.Warnings = append(result.Warnings, "package.json should include 'author' field")
	}

	if pkg.Repository == "" {
		result.Warnings = append(result.Warnings, "package.json should include 'repository' field")
	}

	if len(pkg.Keywords) == 0 {
		result.Warnings = append(result.Warnings, "package.json should include 'keywords' for better discoverability")
	}

	if pkg.Homepage == "" && pkg.Repository != "" {
		result.Warnings = append(result.Warnings, "consider adding 'homepage' field")
	}
}

func validateUnitySpecificFields(result *PackageValidationResult) {
	pkg := result.Package

	if pkg.Unity != "" {
		if !strings.HasPrefix(pkg.Unity, "20") {
			result.Warnings = append(result.Warnings, "unity version should be in format '2020.3' or higher")
		}
	}

	if pkg.DisplayName != "" && len(pkg.DisplayName) > 50 {
		result.Warnings = append(result.Warnings, "displayName should be 50 characters or less for better UPM display")
	}

	if pkg.Category != "" {
		validCategories := []string{
			"Tools", "Editor", "Runtime", "Rendering", "Networking", "Audio",
			"Animation", "Physics", "Input", "AI", "UI", "Utilities",
		}
		isValid := false
		for _, cat := range validCategories {
			if strings.EqualFold(pkg.Category, cat) {
				isValid = true
				break
			}
		}
		if !isValid {
			result.Warnings = append(result.Warnings, fmt.Sprintf("category '%s' is not a standard Unity category", pkg.Category))
		}
	}
}

func validateNpmCompatibility(result *PackageValidationResult) {
	pkg := result.Package

	if pkg.Main != "" {
		if !strings.HasSuffix(pkg.Main, ".js") && !strings.HasSuffix(pkg.Main, ".mjs") {
			result.Warnings = append(result.Warnings, "main field should point to a JavaScript file for npm compatibility")
		}
	}

	if len(pkg.Files) > 0 {
		hasValidEntry := false
		for _, file := range pkg.Files {
			if file != "" && !strings.Contains(file, "..") {
				hasValidEntry = true
				break
			}
		}
		if !hasValidEntry {
			result.Warnings = append(result.Warnings, "files field should contain valid file patterns")
		}
	}

	if pkg.Dependencies != nil {
		for dep, version := range pkg.Dependencies {
			if err := validateNpmCompatibleName(dep); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("dependency name '%s' is not npm compatible", dep))
			}
			if version == "" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("dependency '%s' has empty version", dep))
			}
		}
	}
}

// Public API functions
func ValidateAccessLevel(access string, packageName string) error {
	switch AccessLevel(access) {
	case AccessPublic:
		if scopedNameRegex.MatchString(packageName) {
			return fmt.Errorf("scoped packages cannot use 'public' access level, use 'scoped' or 'private'")
		}
		return nil
	case AccessScoped:
		if !scopedNameRegex.MatchString(packageName) {
			return fmt.Errorf("only scoped packages can use 'scoped' access level")
		}
		return nil
	case AccessPrivate:
		return nil
	default:
		return fmt.Errorf("invalid access level '%s'. Must be one of: public, scoped, private", access)
	}
}

func ValidateDistTag(tag string) error {
	if tag == "" {
		return fmt.Errorf("dist-tag cannot be empty")
	}

	if len(tag) > 64 {
		return fmt.Errorf("dist-tag cannot be longer than 64 characters")
	}

	tagRegex := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	if !tagRegex.MatchString(tag) {
		return fmt.Errorf("dist-tag can only contain letters, numbers, dots, hyphens, and underscores")
	}

	if strings.HasPrefix(tag, ".") || strings.HasPrefix(tag, "-") {
		return fmt.Errorf("dist-tag cannot start with . or -")
	}

	return nil
}

func IsNpmCompatible(pkg *PackageJSON) bool {
	if err := validateNpmCompatibleName(pkg.Name); err != nil {
		return false
	}

	if err := validateSemanticVersion(pkg.Version); err != nil {
		return false
	}

	return true
}

// Tarball utilities
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
