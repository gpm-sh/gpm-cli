package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/api"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/engines"
	"gpm.sh/gpm/gpm-cli/internal/styling"
	"gpm.sh/gpm/gpm-cli/internal/validation"
)

var (
	addProject  string
	addEngine   string
	addRegistry string
	addJSON     bool
)

var addCmd = &cobra.Command{
	Use:   "add <package[@version]>",
	Short: "Add a package to a game project",
	Long: `Add a package to a game project with Unity as first priority.

This command adds a package to your game project, automatically detecting the engine
(Unity takes priority) and updating the project's package manifest safely.

Examples:
  gpm add com.unity.analytics          # Add latest version
  gpm add com.unity.analytics@2.1.0    # Add specific version
  gpm add com.company.sdk --engine unity  # Force Unity engine
  gpm add com.package.name --project ./my-project  # Specify project path
  gpm add com.package.name --registry https://custom.gpm.sh  # Override registry`,
	Args: cobra.ExactArgs(1),
	RunE: runAddCommand,
}

type AddOutput struct {
	Success    bool           `json:"success"`
	Engine     string         `json:"engine"`
	Project    string         `json:"project"`
	Package    string         `json:"package"`
	Version    string         `json:"version"`
	Registry   string         `json:"registry"`
	Changed    bool           `json:"changed"`
	BackupPath string         `json:"backup_path,omitempty"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details,omitempty"`
	Error      string         `json:"error,omitempty"`
}

func init() {
	addCmd.Flags().StringVar(&addProject, "project", "", "Project path (default: current directory)")
	addCmd.Flags().StringVar(&addEngine, "engine", "auto", "Engine type: unity, godot, unreal, auto")
	addCmd.Flags().StringVar(&addRegistry, "registry", "", "Override registry URL")
	addCmd.Flags().BoolVar(&addJSON, "json", false, "Output results in JSON format")
}

func runAddCommand(cmd *cobra.Command, args []string) error {
	packageSpec := args[0]

	output := &AddOutput{
		Success: false,
		Details: make(map[string]any),
	}

	// Check if JSON flag was set for this specific command execution
	useJSON, _ := cmd.Flags().GetBool("json")

	// Get flag values before resetting global variables
	projectFlag, _ := cmd.Flags().GetString("project")
	engineFlag, _ := cmd.Flags().GetString("engine")
	registryFlag, _ := cmd.Flags().GetString("registry")

	// Reset global variables after getting flag values to avoid contamination
	addProject = ""
	addEngine = "auto"
	addRegistry = ""
	addJSON = false

	if err := executeAddWithFlags(packageSpec, output, projectFlag, engineFlag, registryFlag); err != nil {
		output.Error = err.Error()
		if useJSON {
			_ = printAddJSON(cmd, output)
			return err // Return error to set proper exit code
		}
		return err
	}

	output.Success = true
	if useJSON {
		return printAddJSON(cmd, output)
	}

	return printAddHuman(cmd, output)
}

func executeAddWithFlags(packageSpec string, output *AddOutput, projectFlag, engineFlag, registryFlag string) error {
	// Parse package specification
	packageName, version, err := parseAddPackageSpec(packageSpec)
	if err != nil {
		return fmt.Errorf("invalid package specification: %w", err)
	}

	output.Package = packageName
	output.Version = version

	// Determine project path
	projectPath := projectFlag
	if projectPath == "" {
		var err error
		projectPath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	projectPath, err = filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to resolve project path: %w", err)
	}
	output.Project = projectPath

	// Detect or validate engine
	engineType, err := detectOrValidateEngine(projectPath, engineFlag)
	if err != nil {
		return err
	}
	output.Engine = string(engineType)

	// Get engine adapter
	adapter, err := engines.GetAdapter(engineType)
	if err != nil {
		return fmt.Errorf("engine adapter not available: %w", err)
	}

	// Validate project for the detected engine
	if err := adapter.ValidateProject(projectPath); err != nil {
		return fmt.Errorf("project validation failed: %w", err)
	}

	// Determine registry
	registryURL := registryFlag
	if registryURL == "" {
		if registryURL, err = getConfiguredRegistry(); err != nil {
			return fmt.Errorf("no registry configured. Please run 'gpm config set registry <url>' or use --registry flag")
		}
		if registryURL == "" {
			return fmt.Errorf("no registry configured. Please run 'gpm config set registry <url>' or use --registry flag")
		}
	}
	output.Registry = registryURL

	// Validate package name first (before any network calls)
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
		return err // Error messages are already descriptive
	}
	version = resolvedVersion
	output.Version = version

	// Check if package is already installed with same version
	existingInfo, _ := adapter.GetPackageInfo(projectPath, packageName)
	if existingInfo != nil && existingInfo.Version == version {
		output.Changed = false
		output.Message = fmt.Sprintf("Package %s@%s is already installed", packageName, version)
		return nil
	}

	// Create backup before making changes
	backupPath, err := createProjectBackup(projectPath, engineType)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	output.BackupPath = backupPath

	// Install package
	installReq := &engines.PackageInstallRequest{
		Name:     packageName,
		Version:  version,
		Registry: registryURL,
	}

	result, err := adapter.InstallPackage(projectPath, installReq)
	if err != nil {
		// Attempt to restore from backup
		if restoreErr := restoreFromBackup(backupPath, projectPath, engineType); restoreErr != nil {
			return fmt.Errorf("package installation failed and backup restore failed: install error: %w, restore error: %v", err, restoreErr)
		}
		return fmt.Errorf("package installation failed (restored from backup): %w", err)
	}

	if !result.Success {
		return fmt.Errorf("package installation was not successful: %s", result.Message)
	}

	output.Changed = true
	output.Message = result.Message
	if result.Details != nil {
		for k, v := range result.Details {
			output.Details[k] = v
		}
	}

	return nil
}

func detectOrValidateEngine(projectPath, engineFlag string) (engines.EngineType, error) {
	if engineFlag != "auto" {
		// Validate specified engine
		switch engineFlag {
		case "unity":
			return engines.EngineUnity, nil
		case "godot":
			return engines.EngineGodot, nil
		case "unreal":
			return engines.EngineUnreal, nil
		default:
			return engines.EngineUnknown, fmt.Errorf("unsupported engine: %s", engineFlag)
		}
	}

	// Auto-detect engine with Unity priority
	results, err := engines.DetectEngine(projectPath)
	if err != nil {
		return engines.EngineUnknown, fmt.Errorf("engine detection failed: %w", err)
	}

	// Unity takes priority - look for Unity first
	for _, result := range results {
		if result.Engine == engines.EngineUnity && result.Confidence >= engines.ConfidenceMedium {
			return engines.EngineUnity, nil
		}
	}

	// Fall back to best detection result
	best := results.Best()
	if best.Confidence < engines.ConfidenceMedium {
		return engines.EngineUnknown, fmt.Errorf("no supported engine detected. Please specify --engine unity or run from inside a Unity project directory (with Assets/ and ProjectSettings/ folders)")
	}

	// Check if the detected engine is supported by add command
	switch best.Engine {
	case engines.EngineUnity:
		return engines.EngineUnity, nil
	case engines.EngineUnreal, engines.EngineGodot:
		return engines.EngineUnknown, fmt.Errorf("detected %s project, but %s engine support is not yet implemented for add command", best.Engine.String(), best.Engine.String())
	default:
		return engines.EngineUnknown, fmt.Errorf("unsupported engine detected: %s", best.Engine.String())
	}
}

func parseAddPackageSpec(spec string) (string, string, error) {
	if spec == "" {
		return "", "", fmt.Errorf("package specification cannot be empty")
	}

	parts := strings.Split(spec, "@")
	if len(parts) == 1 {
		return parts[0], "", nil
	}
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("invalid package specification format")
}

func getConfiguredRegistry() (string, error) {
	registry := config.GetRegistry()
	if registry == "" {
		return "", fmt.Errorf("no registry configured")
	}

	return registry, nil
}

func createProjectBackup(projectPath string, engineType engines.EngineType) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	backupDir := filepath.Join(os.TempDir(), fmt.Sprintf("gpm-backup-%s", timestamp))

	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	switch engineType {
	case engines.EngineUnity:
		return backupUnityProject(projectPath, backupDir)
	default:
		return "", fmt.Errorf("backup not implemented for engine type: %s", engineType)
	}
}

func backupUnityProject(projectPath, backupDir string) (string, error) {
	manifestPath := filepath.Join(projectPath, "Packages", "manifest.json")
	if !fileExists(manifestPath) {
		// No existing manifest to backup
		return backupDir, nil
	}

	backupManifestPath := filepath.Join(backupDir, "manifest.json")
	// Validate path to prevent directory traversal
	if !strings.HasPrefix(filepath.Clean(manifestPath), projectPath) {
		return "", fmt.Errorf("invalid manifest path")
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", fmt.Errorf("failed to read manifest for backup: %w", err)
	}

	if err := os.WriteFile(backupManifestPath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write backup manifest: %w", err)
	}

	return backupDir, nil
}

func restoreFromBackup(backupPath, projectPath string, engineType engines.EngineType) error {
	switch engineType {
	case engines.EngineUnity:
		return restoreUnityProject(backupPath, projectPath)
	default:
		return fmt.Errorf("restore not implemented for engine type: %s", engineType)
	}
}

func restoreUnityProject(backupPath, projectPath string) error {
	backupManifestPath := filepath.Join(backupPath, "manifest.json")
	if !fileExists(backupManifestPath) {
		// Nothing to restore
		return nil
	}

	manifestPath := filepath.Join(projectPath, "Packages", "manifest.json")
	// Validate path to prevent directory traversal
	if !strings.HasPrefix(filepath.Clean(backupManifestPath), backupPath) {
		return fmt.Errorf("invalid backup manifest path")
	}
	data, err := os.ReadFile(backupManifestPath)
	if err != nil {
		return fmt.Errorf("failed to read backup manifest: %w", err)
	}

	return os.WriteFile(manifestPath, data, 0600)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func printAddJSON(cmd *cobra.Command, output *AddOutput) error {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON output: %w", err)
	}
	cmd.Println(string(data))
	return nil
}

func printAddHuman(cmd *cobra.Command, output *AddOutput) error {
	if !output.Changed {
		cmd.Printf("%s %s\n", styling.Info("ℹ"), output.Message)
		return nil
	}

	cmd.Println(styling.Header("📦 Package Added Successfully"))
	cmd.Println(styling.Separator())
	cmd.Printf("%s %s\n", styling.Label("Engine:"), styling.Value(output.Engine))
	cmd.Printf("%s %s\n", styling.Label("Project:"), styling.File(output.Project))
	cmd.Printf("%s %s@%s\n", styling.Label("Package:"), styling.Package(output.Package), styling.Version(output.Version))
	cmd.Printf("%s %s\n", styling.Label("Registry:"), styling.Value(output.Registry))
	if output.BackupPath != "" {
		cmd.Printf("%s %s\n", styling.Label("Backup:"), styling.File(output.BackupPath))
	}
	cmd.Println(styling.Separator())
	cmd.Printf("%s %s\n", styling.Success("✓"), output.Message)

	return nil
}
