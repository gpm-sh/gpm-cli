package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/engines"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

var (
	detectOutputJSON bool
	detectProjectDir string
)

// detectCmd represents the detect command
var detectCmd = &cobra.Command{
	Use:   "detect [directory]",
	Short: "Detect game engine projects",
	Long: `Detect game engine projects in the current or specified directory.

GPM can automatically detect Unity, Unreal Engine, Godot, and Cocos Creator
projects based on their file structure and configuration files.

Detection Criteria:
  Unity        - Assets/, ProjectSettings/, Packages/manifest.json
  Unreal       - *.uproject files, Content/, Config/
  Godot        - project.godot file, .tscn files
  Cocos Creator - project.json, assets/ directory

Confidence Levels:
  Maximum   - Very specific indicators (e.g., .uproject files)
  High      - Multiple strong indicators
  Medium    - Some indicators present
  Low       - Weak or few indicators
  None      - No indicators found

Examples:
  gpm detect                    # Detect in current directory
  gpm detect /path/to/project   # Detect in specific directory
  gpm detect --json            # Output results as JSON`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir := detectProjectDir
		if len(args) > 0 {
			projectDir = args[0]
		}

		if projectDir == "" {
			var err error
			projectDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Detect engines
		results, err := engines.DetectEngine(projectDir)
		if err != nil {
			return fmt.Errorf("detection failed: %w", err)
		}

		if detectOutputJSON {
			return outputDetectionJSON(results)
		}

		return outputDetectionHuman(results, projectDir)
	},
}

func init() {

	detectCmd.Flags().BoolVar(&detectOutputJSON, "json", false, "Output results in JSON format")
	detectCmd.Flags().StringVar(&detectProjectDir, "project-dir", "", "Project directory to scan (default: current directory)")
}

func outputDetectionJSON(results engines.DetectionResults) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func outputDetectionHuman(results engines.DetectionResults, projectDir string) error {
	fmt.Println(styling.Header("🔍 Game Engine Detection"))
	fmt.Println(styling.Separator())
	fmt.Printf("%s %s\n", styling.Label("Scanned Directory:"), styling.File(projectDir))
	fmt.Println(styling.Separator())

	if len(results) == 0 {
		fmt.Println(styling.Warning("❌ No game engine projects detected"))
		fmt.Println()
		fmt.Println(styling.Hint("GPM looks for:"))
		fmt.Println(styling.Value("  • Unity: Assets/, ProjectSettings/, Packages/manifest.json"))
		fmt.Println(styling.Value("  • Unreal: *.uproject files, Content/, Config/"))
		fmt.Println(styling.Value("  • Godot: project.godot file, .tscn files"))
		fmt.Println(styling.Value("  • Cocos Creator: project.json, assets/ directory"))
		return nil
	}

	// Show best result first
	best := results.Best()
	fmt.Printf("%s %s\n", styling.Label("Best Match:"), getEngineIcon(best.Engine)+" "+styling.Value(best.Engine.String()))
	fmt.Printf("%s %s\n", styling.Label("Confidence:"), getConfidenceStyle(best.Confidence))

	if best.Version != "" {
		fmt.Printf("%s %s\n", styling.Label("Version:"), styling.Value(best.Version))
	}

	if len(best.Details) > 0 {
		fmt.Printf("%s\n", styling.Label("Details:"))
		for key, value := range best.Details {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}

	fmt.Println(styling.Separator())

	// Show all results
	fmt.Println(styling.Header("All Detection Results:"))

	for i, result := range results {
		if i > 0 {
			fmt.Println()
		}

		fmt.Printf("%s %s %s\n",
			getEngineIcon(result.Engine),
			styling.Value(result.Engine.String()),
			getConfidenceStyle(result.Confidence))

		if result.Version != "" {
			fmt.Printf("    Version: %s\n", result.Version)
		}

		if len(result.Details) > 0 {
			fmt.Printf("    Details:\n")
			for key, value := range result.Details {
				fmt.Printf("      %s: %v\n", key, value)
			}
		}
	}

	// Show recommendations
	fmt.Println()
	fmt.Println(styling.Separator())

	if results.HasAmbiguous() {
		fmt.Println(styling.Warning("⚠️  Multiple high-confidence engines detected"))
		fmt.Println(styling.Hint("Use explicit engine flags when installing packages:"))
		fmt.Println(styling.Value("  gpm install --unity package-name"))
		fmt.Println(styling.Value("  gpm install --unreal package-name"))
	} else if best.Confidence >= engines.ConfidenceMedium {
		fmt.Println(styling.Success("✅ Engine detection successful"))
		fmt.Println(styling.Hint("You can now install packages:"))
		fmt.Printf("%s  gpm install package-name\n", styling.Value(""))
	} else {
		fmt.Println(styling.Warning("⚠️  Low confidence detection"))
		fmt.Println(styling.Hint("Consider using explicit engine flags:"))
		fmt.Printf("%s  gpm install --unity package-name\n", styling.Value(""))
	}

	return nil
}

func getEngineIcon(engine engines.EngineType) string {
	switch engine {
	case engines.EngineUnity:
		return "🎮"
	case engines.EngineUnreal:
		return "🚀"
	case engines.EngineGodot:
		return "🎲"
	case engines.EngineCocos:
		return "🥥"
	default:
		return "❓"
	}
}

func getConfidenceStyle(confidence engines.ConfidenceLevel) string {
	switch {
	case confidence >= engines.ConfidenceMax:
		return styling.Success("Maximum")
	case confidence >= engines.ConfidenceHigh:
		return styling.Success("High")
	case confidence >= engines.ConfidenceMedium:
		return styling.Warning("Medium")
	case confidence >= engines.ConfidenceLow:
		return styling.Error("Low")
	default:
		return styling.Error("None")
	}
}
