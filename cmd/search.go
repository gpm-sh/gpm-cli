package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

var (
	searchLimit  int
	searchDetail bool
)

var searchCmd = &cobra.Command{
	Use:   "search <term>",
	Short: "Search for packages",
	Long: `Search for packages in the GPM registry.

Examples:
  gpm search unity
  gpm search ui --limit 20
  gpm search analytics --detail`,
	Args: cobra.ExactArgs(1),
	RunE: search,
}

func init() {
	searchCmd.Flags().IntVar(&searchLimit, "limit", 10, "Maximum number of results to show")
	searchCmd.Flags().BoolVar(&searchDetail, "detail", false, "Show detailed package information")
}

func search(cmd *cobra.Command, args []string) error {
	searchTerm := args[0]

	fmt.Println(styling.Header("🔍  Package Search"))
	fmt.Println(styling.Separator())
	fmt.Printf("%s %s\n", styling.Label("Search term:"), styling.Value(searchTerm))
	fmt.Println()

	cfg := config.GetConfig()

	// Build search URL
	searchURL := fmt.Sprintf("%s/-/v1/search?text=%s", cfg.Registry, url.QueryEscape(searchTerm))
	if searchLimit > 0 {
		searchURL += fmt.Sprintf("&size=%d", searchLimit)
	}

	resp, err := http.Get(searchURL)
	if err != nil {
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Failed to search packages: "+err.Error()),
			styling.Hint("Check your internet connection and registry URL"))
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("%s\n\n%s",
			styling.Error(fmt.Sprintf("Search failed (HTTP %d)", resp.StatusCode)),
			styling.Hint("The registry may be experiencing issues. Try again later."))
	}

	var searchResult struct {
		Objects []struct {
			Package struct {
				Name        string            `json:"name"`
				Version     string            `json:"version"`
				Description string            `json:"description"`
				Keywords    []string          `json:"keywords"`
				Author      map[string]string `json:"author"`
				License     string            `json:"license"`
				Homepage    string            `json:"homepage"`
			} `json:"package"`
			Score struct {
				Final float64 `json:"final"`
			} `json:"score"`
		} `json:"objects"`
		Total int `json:"total"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return fmt.Errorf("failed to parse search results: %w", err)
	}

	if len(searchResult.Objects) == 0 {
		fmt.Printf("%s\n\n%s\n",
			styling.Warning("No packages found matching '"+searchTerm+"'"),
			styling.Hint("Try different search terms or check spelling"))
		return nil
	}

	// Display results
	fmt.Printf("%s %d packages found\n\n", styling.Info("📦"), len(searchResult.Objects))

	for i, result := range searchResult.Objects {
		pkg := result.Package

		// Package name and version
		fmt.Printf("%s %s@%s",
			styling.Package("█"),
			styling.Package(pkg.Name),
			styling.Version(pkg.Version))

		// Score indicator
		scoreColor := styling.Muted
		if result.Score.Final > 0.7 {
			scoreColor = styling.Success
		} else if result.Score.Final > 0.4 {
			scoreColor = styling.Warning
		}
		fmt.Printf(" %s\n", scoreColor(fmt.Sprintf("(%.1f)", result.Score.Final*100)))

		// Description
		if pkg.Description != "" {
			description := pkg.Description
			if len(description) > 80 && !searchDetail {
				description = description[:77] + "..."
			}
			fmt.Printf("  %s\n", styling.Muted(description))
		}

		if searchDetail {
			// Author
			if pkg.Author["name"] != "" {
				fmt.Printf("  %s %s", styling.Label("Author:"), styling.Value(pkg.Author["name"]))
				if pkg.Author["email"] != "" {
					fmt.Printf(" <%s>", styling.Muted(pkg.Author["email"]))
				}
				fmt.Println()
			}

			// License
			if pkg.License != "" {
				fmt.Printf("  %s %s\n", styling.Label("License:"), styling.Value(pkg.License))
			}

			// Homepage
			if pkg.Homepage != "" {
				fmt.Printf("  %s %s\n", styling.Label("Homepage:"), styling.URL(pkg.Homepage))
			}

			// Keywords
			if len(pkg.Keywords) > 0 {
				fmt.Printf("  %s %s\n", styling.Label("Keywords:"), strings.Join(pkg.Keywords, ", "))
			}
		}

		// Add spacing between results
		if i < len(searchResult.Objects)-1 {
			fmt.Println()
		}
	}

	// Show footer information
	fmt.Println()
	fmt.Println(styling.Separator())

	if searchResult.Total > len(searchResult.Objects) {
		fmt.Printf("%s Showing %d of %d total results\n",
			styling.Info("📊"),
			len(searchResult.Objects),
			searchResult.Total)
		if searchLimit < searchResult.Total {
			fmt.Printf("%s Use --limit %d to see more results\n",
				styling.Hint("💡"),
				min(searchResult.Total, searchLimit*2))
		}
	}

	fmt.Printf("%s Use 'gpm info <package>' for detailed information\n",
		styling.Hint("💡"))
	fmt.Printf("%s Use 'gpm install <package>' to install a package\n",
		styling.Hint("💡"))

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
