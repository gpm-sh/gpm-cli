package engines

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// EngineType represents the type of game engine
type EngineType string

const (
	EngineUnity   EngineType = "unity"
	EngineUnreal  EngineType = "unreal"
	EngineGodot   EngineType = "godot"
	EngineCocos   EngineType = "cocos"
	EngineUnknown EngineType = "unknown"
)

// ConfidenceLevel represents how confident we are about engine detection
type ConfidenceLevel int

const (
	ConfidenceNone   ConfidenceLevel = 0
	ConfidenceLow    ConfidenceLevel = 25
	ConfidenceMedium ConfidenceLevel = 50
	ConfidenceHigh   ConfidenceLevel = 75
	ConfidenceMax    ConfidenceLevel = 100
)

// DetectionResult contains the result of engine detection
type DetectionResult struct {
	Engine      EngineType      `json:"engine"`
	Confidence  ConfidenceLevel `json:"confidence"`
	Version     string          `json:"version,omitempty"`
	ProjectPath string          `json:"project_path"`
	Details     map[string]any  `json:"details,omitempty"`
}

// DetectionResults holds multiple detection results
type DetectionResults []*DetectionResult

// Best returns the detection result with highest confidence
func (dr DetectionResults) Best() *DetectionResult {
	if len(dr) == 0 {
		return &DetectionResult{Engine: EngineUnknown, Confidence: ConfidenceNone}
	}

	sort.Slice(dr, func(i, j int) bool {
		return dr[i].Confidence > dr[j].Confidence
	})

	return dr[0]
}

// HasAmbiguous returns true if there are multiple high-confidence results
func (dr DetectionResults) HasAmbiguous() bool {
	if len(dr) < 2 {
		return false
	}

	sort.Slice(dr, func(i, j int) bool {
		return dr[i].Confidence > dr[j].Confidence
	})

	return dr[0].Confidence >= ConfidenceHigh && dr[1].Confidence >= ConfidenceHigh
}

// DetectEngine scans the given directory for game engine projects
func DetectEngine(projectPath string) (DetectionResults, error) {
	if projectPath == "" {
		var err error
		projectPath, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	var results DetectionResults

	// Check for Unreal Engine (highest confidence)
	if result := detectUnreal(projectPath); result.Confidence > ConfidenceNone {
		results = append(results, result)
	}

	// Check for Cocos Creator (high confidence)
	if result := detectCocos(projectPath); result.Confidence > ConfidenceNone {
		results = append(results, result)
	}

	// Check for Unity (medium-high confidence)
	if result := detectUnity(projectPath); result.Confidence > ConfidenceNone {
		results = append(results, result)
	}

	// Check for Godot (medium confidence)
	if result := detectGodot(projectPath); result.Confidence > ConfidenceNone {
		results = append(results, result)
	}

	return results, nil
}

// detectUnreal checks for Unreal Engine project indicators
func detectUnreal(projectPath string) *DetectionResult {
	result := &DetectionResult{
		Engine:      EngineUnreal,
		Confidence:  ConfidenceNone,
		ProjectPath: projectPath,
		Details:     make(map[string]any),
	}

	// Look for .uproject files
	uprojectFiles, err := filepath.Glob(filepath.Join(projectPath, "*.uproject"))
	if err != nil || len(uprojectFiles) == 0 {
		return result
	}

	// Parse the first .uproject file
	uprojectPath := uprojectFiles[0]
	// Validate path to prevent directory traversal
	if !strings.HasPrefix(filepath.Clean(uprojectPath), projectPath) {
		return result
	}
	data, err := os.ReadFile(uprojectPath)
	if err != nil {
		return result
	}

	var uproject map[string]any
	if err := json.Unmarshal(data, &uproject); err != nil {
		return result
	}

	result.Confidence = ConfidenceMax
	result.Details["uproject_file"] = filepath.Base(uprojectPath)

	if engineAssoc, ok := uproject["EngineAssociation"].(string); ok {
		result.Version = engineAssoc
		result.Details["engine_association"] = engineAssoc
	}

	// Verify supporting directories
	contentDir := filepath.Join(projectPath, "Content")
	configDir := filepath.Join(projectPath, "Config")

	if dirExists(contentDir) {
		result.Details["has_content_dir"] = true
	}
	if dirExists(configDir) {
		result.Details["has_config_dir"] = true
	}

	return result
}

// detectCocos checks for Cocos Creator project indicators
func detectCocos(projectPath string) *DetectionResult {
	result := &DetectionResult{
		Engine:      EngineCocos,
		Confidence:  ConfidenceNone,
		ProjectPath: projectPath,
		Details:     make(map[string]any),
	}

	// Look for project.json
	projectJsonPath := filepath.Join(projectPath, "project.json")
	if !fileExists(projectJsonPath) {
		return result
	}

	// Check for assets directory
	assetsDir := filepath.Join(projectPath, "assets")
	if !dirExists(assetsDir) {
		return result
	}

	// Parse project.json
	// Validate path to prevent directory traversal
	if !strings.HasPrefix(filepath.Clean(projectJsonPath), projectPath) {
		return result
	}
	data, err := os.ReadFile(projectJsonPath)
	if err != nil {
		return result
	}

	var project map[string]any
	if err := json.Unmarshal(data, &project); err != nil {
		return result
	}

	result.Confidence = ConfidenceHigh
	result.Details["has_project_json"] = true
	result.Details["has_assets_dir"] = true

	if version, ok := project["version"].(string); ok {
		result.Version = version
		result.Details["project_version"] = version
	}

	// Check for .meta files in assets (Cocos Creator specific)
	metaFiles, err := filepath.Glob(filepath.Join(assetsDir, "*.meta"))
	if err == nil && len(metaFiles) > 0 {
		result.Details["has_meta_files"] = len(metaFiles)
	}

	return result
}

// detectUnity checks for Unity project indicators
func detectUnity(projectPath string) *DetectionResult {
	result := &DetectionResult{
		Engine:      EngineUnity,
		Confidence:  ConfidenceNone,
		ProjectPath: projectPath,
		Details:     make(map[string]any),
	}

	// Check for Unity project structure
	assetsDir := filepath.Join(projectPath, "Assets")
	projectSettingsDir := filepath.Join(projectPath, "ProjectSettings")
	manifestPath := filepath.Join(projectPath, "Packages", "manifest.json")

	if !dirExists(assetsDir) || !dirExists(projectSettingsDir) {
		return result
	}

	result.Confidence = ConfidenceMedium
	result.Details["has_assets_dir"] = true
	result.Details["has_project_settings"] = true

	// Check for manifest.json
	if fileExists(manifestPath) {
		result.Confidence = ConfidenceHigh
		result.Details["has_manifest"] = true
	}

	// Try to get Unity version
	versionPath := filepath.Join(projectSettingsDir, "ProjectVersion.txt")
	if fileExists(versionPath) {
		// Validate path to prevent directory traversal
		if !strings.HasPrefix(filepath.Clean(versionPath), projectPath) {
			return result
		}
		if data, err := os.ReadFile(versionPath); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "m_EditorVersion:") {
					version := strings.TrimSpace(strings.TrimPrefix(line, "m_EditorVersion:"))
					result.Version = version
					result.Details["editor_version"] = version
					break
				}
			}
		}
	}

	// Check for .meta files in Assets
	metaFiles, err := filepath.Glob(filepath.Join(assetsDir, "*.meta"))
	if err == nil && len(metaFiles) > 0 {
		result.Details["has_meta_files"] = len(metaFiles)
	}

	return result
}

// detectGodot checks for Godot project indicators
func detectGodot(projectPath string) *DetectionResult {
	result := &DetectionResult{
		Engine:      EngineGodot,
		Confidence:  ConfidenceNone,
		ProjectPath: projectPath,
		Details:     make(map[string]any),
	}

	// Look for project.godot
	projectGodotPath := filepath.Join(projectPath, "project.godot")
	if !fileExists(projectGodotPath) {
		return result
	}

	result.Confidence = ConfidenceMedium
	result.Details["has_project_godot"] = true

	// Parse project.godot (INI format)
	// Validate path to prevent directory traversal
	if !strings.HasPrefix(filepath.Clean(projectGodotPath), projectPath) {
		return result
	}
	data, err := os.ReadFile(projectGodotPath)
	if err != nil {
		return result
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "config_version=") {
			configVersion := strings.TrimPrefix(line, "config_version=")
			result.Details["config_version"] = configVersion

			// Determine Godot version from config_version
			switch configVersion {
			case "3":
				result.Version = "3.x"
			case "4":
				result.Version = "4.x"
			default:
				result.Version = "unknown"
			}
			break
		}
	}

	// Check for .tscn files
	tscnFiles, err := filepath.Glob(filepath.Join(projectPath, "*.tscn"))
	if err == nil && len(tscnFiles) > 0 {
		result.Details["has_scene_files"] = len(tscnFiles)
	}

	// Check for .import directory
	importDir := filepath.Join(projectPath, ".import")
	if dirExists(importDir) {
		result.Details["has_import_dir"] = true
	}

	return result
}

// Helper functions
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// String returns a human-readable string for the engine type
func (e EngineType) String() string {
	switch e {
	case EngineUnity:
		return "Unity"
	case EngineUnreal:
		return "Unreal Engine"
	case EngineGodot:
		return "Godot"
	case EngineCocos:
		return "Cocos Creator"
	default:
		return "Unknown"
	}
}

// String returns a human-readable string for confidence level
func (c ConfidenceLevel) String() string {
	switch {
	case c >= ConfidenceMax:
		return "Maximum"
	case c >= ConfidenceHigh:
		return "High"
	case c >= ConfidenceMedium:
		return "Medium"
	case c >= ConfidenceLow:
		return "Low"
	default:
		return "None"
	}
}
