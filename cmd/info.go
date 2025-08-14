package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

var (
	infoVersion string
	infoVerbose bool
	infoJSON    bool
)

var infoCmd = &cobra.Command{
	Use:   "info <package>",
	Short: "Show package information",
	Long: `Display detailed information about a package from the registry.

Examples:
  gpm info com.unity.ugui
  gpm info com.unity.ugui --version 1.0.0
  gpm info com.company.package --verbose`,
	Args: cobra.ExactArgs(1),
	RunE: info,
}

func init() {
	infoCmd.Flags().StringVar(&infoVersion, "version", "", "Show info for specific version")
	infoCmd.Flags().BoolVarP(&infoVerbose, "verbose", "v", false, "Show detailed information")
	infoCmd.Flags().BoolVar(&infoJSON, "json", false, "Output in JSON format")
}

func info(cmd *cobra.Command, args []string) error {
	packageName := args[0]

	cfg := config.GetConfig()

	// Fetch package metadata
	baseURL, err := url.Parse(cfg.Registry)
	if err != nil {
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Invalid registry URL: "+err.Error()),
			styling.Hint("Check your registry URL with 'gpm config get registry'"))
	}
	packageURL := baseURL.JoinPath(packageName).String()
	// #nosec G107 - URL is validated using url.Parse and JoinPath above
	resp, err := http.Get(packageURL)
	if err != nil {
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Failed to fetch package information: "+err.Error()),
			styling.Hint("Check your internet connection and verify the package name"))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 {
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Package not found: "+packageName),
			styling.Hint("Check the package name spelling or search with 'gpm search "+packageName+"'"))
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("%s\n\n%s",
			styling.Error(fmt.Sprintf("Registry error (HTTP %d)", resp.StatusCode)),
			styling.Hint("The registry may be experiencing issues. Try again later."))
	}

	var packageInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&packageInfo); err != nil {
		return fmt.Errorf("failed to parse package information: %w", err)
	}

	// Handle JSON output
	if infoJSON {
		return outputJSON(packageInfo)
	}

	// Display formatted output
	fmt.Println(styling.Header("ℹ️   Package Information"))
	fmt.Println(styling.Separator())

	// Display basic information
	displayBasicInfo(packageInfo)

	// Display version information
	if infoVersion != "" {
		displayVersionInfo(packageInfo, infoVersion)
	} else {
		displayLatestVersion(packageInfo)
	}

	if infoVerbose {
		displayDetailedInfo(packageInfo)
	}

	fmt.Println(styling.Separator())
	return nil
}

func displayBasicInfo(pkg map[string]interface{}) {
	name := getStringField(pkg, "name")
	description := getStringField(pkg, "description")

	fmt.Printf("%s %s\n", styling.Label("Name:"), styling.Package(name))
	if description != "" {
		fmt.Printf("%s %s\n", styling.Label("Description:"), styling.Value(description))
	}

	if displayName := getStringField(pkg, "displayName"); displayName != "" {
		fmt.Printf("%s %s\n", styling.Label("Display Name:"), styling.Value(displayName))
	}

	// Show dist-tags
	if distTags := getMapField(pkg, "dist-tags"); len(distTags) > 0 {
		fmt.Printf("%s", styling.Label("Dist-tags:"))
		first := true
		for tag, version := range distTags {
			if !first {
				fmt.Printf(",")
			}
			fmt.Printf(" %s: %s", styling.Version(tag), styling.Value(version.(string)))
			first = false
		}
		fmt.Println()
	}

	// Show modification dates
	if created := getStringField(pkg, "created"); created != "" {
		if parsedTime, err := time.Parse(time.RFC3339, created); err == nil {
			fmt.Printf("%s %s\n", styling.Label("Created:"), styling.Value(parsedTime.Format("2006-01-02 15:04:05")))
		}
	}

	if modified := getStringField(pkg, "modified"); modified != "" {
		if parsedTime, err := time.Parse(time.RFC3339, modified); err == nil {
			fmt.Printf("%s %s\n", styling.Label("Modified:"), styling.Value(parsedTime.Format("2006-01-02 15:04:05")))
		}
	}

	fmt.Println()
}

func displayLatestVersion(pkg map[string]interface{}) {
	distTags, ok := pkg["dist-tags"].(map[string]interface{})
	if !ok {
		return
	}

	latest, ok := distTags["latest"].(string)
	if !ok {
		return
	}

	fmt.Printf("%s %s\n", styling.Label("Latest Version:"), styling.Version(latest))

	versions, ok := pkg["versions"].(map[string]interface{})
	if !ok {
		return
	}

	versionInfo, ok := versions[latest].(map[string]interface{})
	if !ok {
		return
	}

	displayVersionDetails(versionInfo)
}

func displayVersionInfo(pkg map[string]interface{}, version string) {
	versions, ok := pkg["versions"].(map[string]interface{})
	if !ok {
		fmt.Printf("%s\n", styling.Error("No version information available"))
		return
	}

	versionInfo, ok := versions[version].(map[string]interface{})
	if !ok {
		fmt.Printf("%s %s\n", styling.Error("Version not found:"), styling.Version(version))

		// Show available versions
		fmt.Printf("\n%s\n", styling.Label("Available versions:"))
		for v := range versions {
			fmt.Printf("  %s\n", styling.Version(v))
		}
		return
	}

	fmt.Printf("%s %s\n", styling.Label("Version:"), styling.Version(version))
	displayVersionDetails(versionInfo)
}

func displayVersionDetails(versionInfo map[string]interface{}) {
	if author := getMapField(versionInfo, "author"); author != nil {
		if name := getStringField(author, "name"); name != "" {
			fmt.Printf("%s %s", styling.Label("Author:"), styling.Value(name))
			if email := getStringField(author, "email"); email != "" {
				fmt.Printf(" <%s>", styling.Muted(email))
			}
			fmt.Println()
		}
	}

	// Show all maintainers
	if maintainers := getArrayOfObjects(versionInfo, "maintainers"); len(maintainers) > 0 {
		fmt.Printf("%s", styling.Label("Maintainers:"))
		for i, maintainer := range maintainers {
			if i > 0 {
				fmt.Printf(",")
			}
			name := getStringField(maintainer, "name")
			email := getStringField(maintainer, "email")
			fmt.Printf(" %s", styling.Value(name))
			if email != "" {
				fmt.Printf(" <%s>", styling.Muted(email))
			}
		}
		fmt.Println()
	}

	if license := getStringField(versionInfo, "license"); license != "" {
		fmt.Printf("%s %s\n", styling.Label("License:"), styling.Value(license))
	}

	if homepage := getStringField(versionInfo, "homepage"); homepage != "" {
		fmt.Printf("%s %s\n", styling.Label("Homepage:"), styling.URL(homepage))
	}

	if repository := getMapField(versionInfo, "repository"); repository != nil {
		if url := getStringField(repository, "url"); url != "" {
			fmt.Printf("%s %s\n", styling.Label("Repository:"), styling.URL(url))
		}
	}

	if keywords := getArrayField(versionInfo, "keywords"); len(keywords) > 0 {
		fmt.Printf("%s %s\n", styling.Label("Keywords:"), strings.Join(keywords, ", "))
	}

	if unity := getStringField(versionInfo, "unity"); unity != "" {
		fmt.Printf("%s %s\n", styling.Label("Unity Version:"), styling.Value(unity))
	}

	// Display dist information
	if dist := getMapField(versionInfo, "dist"); dist != nil {
		fmt.Printf("\n%s\n", styling.SubHeader("Distribution:"))

		if tarball := getStringField(dist, "tarball"); tarball != "" {
			fmt.Printf("  %s %s\n", styling.Label("Tarball:"), styling.URL(tarball))
		}

		if integrity := getStringField(dist, "integrity"); integrity != "" {
			fmt.Printf("  %s %s\n", styling.Label("Integrity:"), styling.Value(integrity))
		}

		if size := dist["size"]; size != nil {
			if sizeFloat, ok := size.(float64); ok {
				fmt.Printf("  %s %s\n", styling.Label("Size:"), styling.Value(formatSize(int64(sizeFloat))))
			}
		}
	}

	// Check for deprecation
	if deprecated := getStringField(versionInfo, "deprecated"); deprecated != "" {
		fmt.Printf("\n%s %s\n", styling.Error("⚠️  DEPRECATED:"), styling.Value(deprecated))
	}

	// Display dependencies
	if deps := getMapField(versionInfo, "dependencies"); len(deps) > 0 {
		fmt.Printf("\n%s\n", styling.SubHeader("Dependencies:"))
		for name, version := range deps {
			if versionStr, ok := version.(string); ok {
				fmt.Printf("  %s@%s\n", styling.Package(name), styling.Version(versionStr))
			}
		}
	}

	fmt.Println()
}

func displayDetailedInfo(pkg map[string]interface{}) {
	fmt.Printf("%s\n", styling.SubHeader("All Versions:"))

	versions, ok := pkg["versions"].(map[string]interface{})
	if !ok {
		fmt.Printf("%s\n", styling.Muted("No version information available"))
		return
	}

	// Show time information from package level
	if timeInfo := getMapField(pkg, "time"); len(timeInfo) > 0 {
		fmt.Printf("\n%s\n", styling.SubHeader("Version History:"))
		for version, timestamp := range timeInfo {
			if version == "created" || version == "modified" {
				continue
			}
			fmt.Printf("  %s", styling.Version(version))
			if timeStr, ok := timestamp.(string); ok {
				if parsedTime, err := time.Parse(time.RFC3339, timeStr); err == nil {
					fmt.Printf(" - %s", styling.Muted(parsedTime.Format("2006-01-02 15:04:05")))
				}
			}

			// Check if this version is deprecated
			if versionData, ok := versions[version].(map[string]interface{}); ok {
				if deprecated := getStringField(versionData, "deprecated"); deprecated != "" {
					fmt.Printf(" %s", styling.Error("[DEPRECATED]"))
				}
			}
			fmt.Println()
		}
	} else {
		// Fallback to old format
		for version, versionData := range versions {
			versionMap, ok := versionData.(map[string]interface{})
			if !ok {
				continue
			}

			fmt.Printf("  %s", styling.Version(version))

			if created := getStringField(versionMap, "created"); created != "" {
				if parsedTime, err := time.Parse(time.RFC3339, created); err == nil {
					fmt.Printf(" (%s)", styling.Muted(parsedTime.Format("2006-01-02")))
				}
			}

			if deprecated := getStringField(versionMap, "deprecated"); deprecated != "" {
				fmt.Printf(" %s", styling.Error("[DEPRECATED]"))
			}

			fmt.Println()
		}
	}
}

func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getMapField(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key].(map[string]interface{}); ok {
		return val
	}
	return nil
}

func getArrayField(m map[string]interface{}, key string) []string {
	if val, ok := m[key].([]interface{}); ok {
		var result []string
		for _, item := range val {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}

func getArrayOfObjects(m map[string]interface{}, key string) []map[string]interface{} {
	if val, ok := m[key].([]interface{}); ok {
		var result []map[string]interface{}
		for _, item := range val {
			if obj, ok := item.(map[string]interface{}); ok {
				result = append(result, obj)
			}
		}
		return result
	}
	return nil
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func outputJSON(packageInfo map[string]interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(packageInfo)
}
