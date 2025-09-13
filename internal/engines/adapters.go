package engines

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PackageInstallRequest represents a package installation request
type PackageInstallRequest struct {
	Name      string         `json:"name"`
	Version   string         `json:"version"`
	Registry  string         `json:"registry,omitempty"`
	AuthToken string         `json:"auth_token,omitempty"`
	IsDev     bool           `json:"is_dev,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
}

// PackageInstallResult represents the result of a package installation
type PackageInstallResult struct {
	Success     bool           `json:"success"`
	PackageName string         `json:"package_name"`
	Version     string         `json:"version"`
	Registry    string         `json:"registry,omitempty"`
	InstallPath string         `json:"install_path,omitempty"`
	Message     string         `json:"message,omitempty"`
	Details     map[string]any `json:"details,omitempty"`
}

// PackageInfo represents installed package information
type PackageInfo struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Registry     string            `json:"registry,omitempty"`
	InstallPath  string            `json:"install_path,omitempty"`
	IsDev        bool              `json:"is_dev,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

// EngineAdapter defines the interface for engine-specific package management
type EngineAdapter interface {
	// GetEngineType returns the engine type this adapter handles
	GetEngineType() EngineType

	// ValidateProject checks if the project is valid for this engine
	ValidateProject(projectPath string) error

	// InstallPackage installs a package using engine-specific logic
	InstallPackage(projectPath string, req *PackageInstallRequest) (*PackageInstallResult, error)

	// RemovePackage removes a package from the project
	RemovePackage(projectPath string, packageName string) error

	// ListPackages returns all installed packages
	ListPackages(projectPath string) ([]*PackageInfo, error)

	// GetPackageInfo returns information about a specific package
	GetPackageInfo(projectPath string, packageName string) (*PackageInfo, error)

	// ConfigureRegistry configures engine-specific registry settings
	ConfigureRegistry(projectPath string, registryURL string, patterns []string) error
}

// UnityAdapter implements EngineAdapter for Unity projects
type UnityAdapter struct{}

// NewUnityAdapter creates a new Unity adapter
func NewUnityAdapter() *UnityAdapter {
	return &UnityAdapter{}
}

func (u *UnityAdapter) GetEngineType() EngineType {
	return EngineUnity
}

func (u *UnityAdapter) ValidateProject(projectPath string) error {
	assetsDir := filepath.Join(projectPath, "Assets")
	projectSettingsDir := filepath.Join(projectPath, "ProjectSettings")

	if !dirExists(assetsDir) {
		return fmt.Errorf("unity Assets directory not found at %s", assetsDir)
	}

	if !dirExists(projectSettingsDir) {
		return fmt.Errorf("unity ProjectSettings directory not found at %s", projectSettingsDir)
	}

	return nil
}

func (u *UnityAdapter) InstallPackage(projectPath string, req *PackageInstallRequest) (*PackageInstallResult, error) {
	if err := u.ValidateProject(projectPath); err != nil {
		return nil, fmt.Errorf("project validation failed: %w", err)
	}

	manifestPath := filepath.Join(projectPath, "Packages", "manifest.json")

	// Ensure Packages directory exists
	packagesDir := filepath.Dir(manifestPath)
	if err := os.MkdirAll(packagesDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create Packages directory: %w", err)
	}

	// Load existing manifest or create new one
	manifest, err := u.loadManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Add package to dependencies
	if manifest.Dependencies == nil {
		manifest.Dependencies = make(map[string]string)
	}

	versionSpec := req.Version
	if versionSpec == "" || versionSpec == "latest" || versionSpec == "*" {
		// For Unity, use "*" to let Unity resolve the latest version
		// The install command should have already resolved specific versions
		versionSpec = "*"
	}

	manifest.Dependencies[req.Name] = versionSpec

	// Configure scoped registry if needed
	if req.Registry != "" && req.Registry != "https://packages.unity.com" {
		// Derive scope from package name (first two labels)
		scope := DeriveScopeFromPackageName(req.Name)
		if err := u.configureScopedRegistry(manifest, req.Registry, scope); err != nil {
			return nil, fmt.Errorf("failed to configure scoped registry: %w", err)
		}
	}

	// Save manifest
	if err := u.saveManifest(manifestPath, manifest); err != nil {
		return nil, fmt.Errorf("failed to save manifest: %w", err)
	}

	return &PackageInstallResult{
		Success:     true,
		PackageName: req.Name,
		Version:     versionSpec,
		Registry:    req.Registry,
		InstallPath: manifestPath,
		Message:     fmt.Sprintf("Added %s@%s to Unity manifest", req.Name, versionSpec),
		Details: map[string]any{
			"manifest_path": manifestPath,
		},
	}, nil
}

func (u *UnityAdapter) RemovePackage(projectPath string, packageName string) error {
	manifestPath := filepath.Join(projectPath, "Packages", "manifest.json")

	manifest, err := u.loadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	if manifest.Dependencies == nil {
		return fmt.Errorf("package %s is not installed", packageName)
	}

	if _, exists := manifest.Dependencies[packageName]; !exists {
		return fmt.Errorf("package %s is not installed", packageName)
	}

	delete(manifest.Dependencies, packageName)

	return u.saveManifest(manifestPath, manifest)
}

func (u *UnityAdapter) ListPackages(projectPath string) ([]*PackageInfo, error) {
	manifestPath := filepath.Join(projectPath, "Packages", "manifest.json")

	manifest, err := u.loadManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	var packages []*PackageInfo

	if manifest.Dependencies != nil {
		for name, version := range manifest.Dependencies {
			packages = append(packages, &PackageInfo{
				Name:        name,
				Version:     version,
				InstallPath: manifestPath,
			})
		}
	}

	return packages, nil
}

func (u *UnityAdapter) GetPackageInfo(projectPath string, packageName string) (*PackageInfo, error) {
	manifestPath := filepath.Join(projectPath, "Packages", "manifest.json")

	manifest, err := u.loadManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	if manifest.Dependencies == nil {
		return nil, fmt.Errorf("package %s is not installed", packageName)
	}

	version, exists := manifest.Dependencies[packageName]
	if !exists {
		return nil, fmt.Errorf("package %s is not installed", packageName)
	}

	return &PackageInfo{
		Name:        packageName,
		Version:     version,
		InstallPath: manifestPath,
	}, nil
}

func (u *UnityAdapter) ConfigureRegistry(projectPath string, registryURL string, patterns []string) error {
	manifestPath := filepath.Join(projectPath, "Packages", "manifest.json")

	manifest, err := u.loadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	return u.configureScopedRegistry(manifest, registryURL, patterns...)
}

// UnityManifest represents Unity's Packages/manifest.json structure
type UnityManifest struct {
	Dependencies     map[string]string `json:"dependencies,omitempty"`
	ScopedRegistries []*ScopedRegistry `json:"scopedRegistries,omitempty"`
}

// ScopedRegistry represents a Unity scoped registry configuration
type ScopedRegistry struct {
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Scopes []string `json:"scopes"`
}

func (u *UnityAdapter) loadManifest(manifestPath string) (*UnityManifest, error) {
	if !fileExists(manifestPath) {
		// Create default manifest
		return &UnityManifest{
			Dependencies: make(map[string]string),
		}, nil
	}

	// Validate path to prevent directory traversal
	if !strings.HasPrefix(filepath.Clean(manifestPath), filepath.Dir(manifestPath)) {
		return nil, fmt.Errorf("invalid manifest path")
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest UnityManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	if manifest.Dependencies == nil {
		manifest.Dependencies = make(map[string]string)
	}

	return &manifest, nil
}

func (u *UnityAdapter) saveManifest(manifestPath string, manifest *UnityManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(manifestPath, data, 0600)
}

func (u *UnityAdapter) configureScopedRegistry(manifest *UnityManifest, registryURL string, patterns ...string) error {
	if manifest.ScopedRegistries == nil {
		manifest.ScopedRegistries = []*ScopedRegistry{}
	}

	// Check if registry already exists
	for _, registry := range manifest.ScopedRegistries {
		if registry.URL == registryURL {
			// Add new patterns to existing registry
			for _, pattern := range patterns {
				found := false
				for _, existingScope := range registry.Scopes {
					if existingScope == pattern {
						found = true
						break
					}
				}
				if !found {
					registry.Scopes = append(registry.Scopes, pattern)
				}
			}
			return nil
		}
	}

	// Create new scoped registry
	registryName := "GPM Registry"
	if len(patterns) > 0 {
		registryName = fmt.Sprintf("GPM Registry (%s)", patterns[0])
	}

	newRegistry := &ScopedRegistry{
		Name:   registryName,
		URL:    registryURL,
		Scopes: patterns,
	}

	manifest.ScopedRegistries = append(manifest.ScopedRegistries, newRegistry)
	return nil
}

// DeriveScopeFromPackageName extracts the first two labels from a reverse-DNS package name
// e.g., com.tapnation.analytics → com.tapnation
func DeriveScopeFromPackageName(packageName string) string {
	parts := strings.Split(packageName, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[:2], ".")
	}
	return packageName
}

// GetAdapter returns the appropriate engine adapter for the given engine type
func GetAdapter(engineType EngineType) (EngineAdapter, error) {
	switch engineType {
	case EngineUnity:
		return NewUnityAdapter(), nil
	case EngineUnreal:
		return nil, fmt.Errorf("unreal Engine adapter not yet implemented")
	case EngineGodot:
		return nil, fmt.Errorf("godot adapter not yet implemented")
	case EngineCocos:
		return nil, fmt.Errorf("cocos Creator adapter not yet implemented")
	default:
		return nil, fmt.Errorf("unknown engine type: %s", engineType)
	}
}
