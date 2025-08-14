package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/styling"
	"gpm.sh/gpm/gpm-cli/internal/validation"
)

type PackageJSON struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	DisplayName  string            `json:"displayName,omitempty"`
	Description  string            `json:"description,omitempty"`
	Unity        string            `json:"unity,omitempty"`
	License      string            `json:"license,omitempty"`
	Author       interface{}       `json:"author,omitempty"`
	Keywords     []string          `json:"keywords,omitempty"`
	Category     string            `json:"category,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Repository   interface{}       `json:"repository,omitempty"`
	Bugs         interface{}       `json:"bugs,omitempty"`
	Homepage     string            `json:"homepage,omitempty"`
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new UPM-compatible package",
	Long: `Initialize a new Unity Package Manager (UPM) compatible package.

This command will guide you through creating a package.json file with all the
necessary fields for Unity Package Manager compatibility and GPM registry publishing.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolP("yes", "y", false, "Accept all defaults and skip prompts")
	initCmd.Flags().StringP("name", "n", "", "Package name (reverse DNS format)")
	initCmd.Flags().String("version", "1.0.0", "Package version")
	initCmd.Flags().StringP("description", "d", "", "Package description")
	initCmd.Flags().String("unity", "2021.3", "Minimum Unity version")
	initCmd.Flags().String("license", "MIT", "Package license")
}

func runInit(cmd *cobra.Command, args []string) error {
	acceptDefaults, _ := cmd.Flags().GetBool("yes")

	fmt.Println(styling.Header("🚀 GPM Package Initialization"))
	fmt.Println(styling.Info("This utility will walk you through creating a package.json file."))
	fmt.Println(styling.Info("It only covers the most common items, and tries to guess sensible defaults."))
	fmt.Println()

	pkg := &PackageJSON{
		Dependencies: make(map[string]string),
	}

	var err error
	if acceptDefaults {
		err = setDefaults(cmd, pkg)
	} else {
		err = promptForValues(cmd, pkg)
	}

	if err != nil {
		return fmt.Errorf("%s", styling.Error("failed to collect package information: "+err.Error()))
	}

	err = writePackageJSONInit(pkg)
	if err != nil {
		return fmt.Errorf("%s", styling.Error("failed to write package.json: "+err.Error()))
	}

	err = createDirectoryStructure()
	if err != nil {
		return fmt.Errorf("%s", styling.Error("failed to create directory structure: "+err.Error()))
	}

	fmt.Println()
	fmt.Println(styling.Success("✅ Package initialized successfully!"))
	fmt.Println(styling.Info("Next steps:"))
	fmt.Println("  • Add your scripts to Runtime/ directory")
	fmt.Println("  • Run 'gpm pack' to create a tarball")
	fmt.Println("  • Run 'gpm publish' to publish to registry")

	return nil
}

func setDefaults(cmd *cobra.Command, pkg *PackageJSON) error {
	var err error

	pkg.Name, err = getDefaultName(cmd)
	if err != nil {
		return err
	}

	pkg.Version, _ = cmd.Flags().GetString("version")
	pkg.Description, _ = cmd.Flags().GetString("description")
	pkg.Unity, _ = cmd.Flags().GetString("unity")
	pkg.License, _ = cmd.Flags().GetString("license")

	if pkg.Description == "" {
		pkg.Description = "A Unity package"
	}

	pkg.DisplayName = generateDisplayName(pkg.Name)
	pkg.Category = "Libraries"
	pkg.Keywords = []string{"unity", "package"}

	return nil
}

func promptForValues(cmd *cobra.Command, pkg *PackageJSON) error {
	reader := bufio.NewReader(os.Stdin)

	var err error
	pkg.Name, err = promptForName(cmd, reader)
	if err != nil {
		return err
	}

	pkg.Version = promptWithDefault(reader, "version", "1.0.0")
	pkg.DisplayName = promptWithDefault(reader, "displayName", generateDisplayName(pkg.Name))
	pkg.Description = promptWithDefault(reader, "description", "A Unity package")
	pkg.Unity = promptWithDefault(reader, "unity", "2021.3")
	pkg.License = promptWithDefault(reader, "license", "MIT")
	pkg.Category = promptWithDefault(reader, "category", "Libraries")

	keywords := promptWithDefault(reader, "keywords", "unity,package")
	if keywords != "" {
		pkg.Keywords = strings.Split(keywords, ",")
		for i, keyword := range pkg.Keywords {
			pkg.Keywords[i] = strings.TrimSpace(keyword)
		}
	}

	return nil
}

func getDefaultName(cmd *cobra.Command) (string, error) {
	name, _ := cmd.Flags().GetString("name")
	if name != "" {
		return name, validation.ValidatePackageName(name)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dirName := filepath.Base(cwd)
	if !strings.Contains(dirName, ".") {
		return "com.company." + dirName, nil
	}

	return dirName, nil
}

func promptForName(cmd *cobra.Command, reader *bufio.Reader) (string, error) {
	defaultName, _ := getDefaultName(cmd)

	for {
		name := promptWithDefault(reader, "name", defaultName)

		err := validation.ValidatePackageName(name)
		if err == nil {
			return name, nil
		}

		fmt.Println(styling.Warning("Invalid package name: " + err.Error()))
		fmt.Println(styling.Info("Package name should be in reverse DNS format (e.g., com.company.package)"))
	}
}

func promptWithDefault(reader *bufio.Reader, field, defaultValue string) string {
	fmt.Printf("%s (%s): ", field, defaultValue)

	input, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}

	return input
}

func generateDisplayName(name string) string {
	parts := strings.Split(name, ".")
	if len(parts) == 0 {
		return name
	}

	lastPart := parts[len(parts)-1]
	words := regexp.MustCompile(`[-_]`).Split(lastPart, -1)

	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}

	return strings.Join(words, " ")
}

func writePackageJSONInit(pkg *PackageJSON) error {
	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile("package.json", data, 0600)
}

func createDirectoryStructure() error {
	dirs := []string{
		"Runtime",
		"Runtime/Scripts",
		"Editor",
		"Tests",
		"Tests/Runtime",
		"Tests/Editor",
		"Documentation~",
	}

	for _, dir := range dirs {
		cleanDir := filepath.Clean(dir)
		if !strings.HasPrefix(cleanDir, ".") && !strings.Contains(cleanDir, "..") {
			err := os.MkdirAll(cleanDir, 0750)
			if err != nil {
				return err
			}
		}
	}

	asmdefContent := `{
    "name": "%s",
    "references": [],
    "includePlatforms": [],
    "excludePlatforms": [],
    "allowUnsafeCode": false,
    "overrideReferences": false,
    "precompiledReferences": [],
    "autoReferenced": true,
    "defineConstraints": [],
    "versionDefines": [],
    "noEngineReferences": false
}`

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	asmdefName := filepath.Base(cwd)
	asmdefPath := filepath.Join("Runtime", asmdefName+".asmdef")
	cleanAsmdefPath := filepath.Clean(asmdefPath)

	if strings.HasPrefix(cleanAsmdefPath, "Runtime/") {
		content := fmt.Sprintf(asmdefContent, asmdefName)
		err = os.WriteFile(cleanAsmdefPath, []byte(content), 0600)
		if err != nil {
			return err
		}
	}

	readmeContent := `# %s

A Unity package for game development.

## Installation

Add this package to your Unity project using the Package Manager:

1. Open the Package Manager (Window > Package Manager)
2. Click the '+' button and select "Add package from git URL"
3. Enter the package URL

## Usage

Documentation and examples coming soon.

## License

%s
`

	pkg := &PackageJSON{}
	if data, err := os.ReadFile("package.json"); err == nil {
		_ = json.Unmarshal(data, pkg) // Ignore error - fallback to defaults if invalid
	}

	readmeText := fmt.Sprintf(readmeContent, pkg.DisplayName, pkg.License)
	return os.WriteFile("README.md", []byte(readmeText), 0600)
}
