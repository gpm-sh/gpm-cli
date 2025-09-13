package cmd

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1" // #nosec G505 - Required for npm compatibility
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/filtering"
	"gpm.sh/gpm/gpm-cli/internal/packaging"
	"gpm.sh/gpm/gpm-cli/internal/styling"
	"gpm.sh/gpm/gpm-cli/internal/validation"
)

var (
	packDryRun        bool
	packJSON          bool
	packDestination   string
	packScope         string
	packIgnoreScripts bool
)

var packCmd = &cobra.Command{
	Use:   "pack [package-spec...]",
	Short: "Create a package tarball",
	Long: `Create a tarball (.tgz) from packages for publishing, matching npm pack behavior.

Package Specs:
  Current directory (default)     # gpm pack
  Package folders                 # gpm pack ./my-package ./another-package  
  Existing tarballs              # gpm pack package.tgz another.tgz

Examples:
  gpm pack                       # Pack current directory
  gpm pack ./my-package          # Pack specific folder
  gpm pack package.tgz           # Repack existing tarball
  gpm pack --dry-run             # Show what would be packed
  gpm pack --json                # Output in JSON format
  gpm pack --pack-destination /tmp  # Output to specific directory
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return packPackages(cmd, args)
	},
}

func init() {
	packCmd.Flags().BoolVar(&packDryRun, "dry-run", false, "Simulate pack without creating tarball")
	packCmd.Flags().BoolVar(&packJSON, "json", false, "Output results in JSON format")
	packCmd.Flags().StringVar(&packDestination, "pack-destination", "", "Specify output directory (default: current directory)")
	packCmd.Flags().StringVar(&packScope, "scope", "", "Scope for scoped packages (e.g., @myscope)")
	packCmd.Flags().BoolVar(&packIgnoreScripts, "ignore-scripts", false, "Skip running package scripts during packing")
}

type PackResult struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Filename     string   `json:"filename"`
	Files        []string `json:"files"`
	FileCount    int      `json:"fileCount"`
	UnpackedSize int64    `json:"unpackedSize"`
	PackedSize   int64    `json:"packedSize"`
	Sha1         string   `json:"sha1"`
	Sha512       string   `json:"sha512"`
	Integrity    string   `json:"integrity"`
}

type PackOutput struct {
	Results []PackResult `json:"results,omitempty"`
	Success bool         `json:"success"`
	Error   string       `json:"error,omitempty"`
}

func packPackages(cmd *cobra.Command, args []string) error {
	var packageSpecs []string
	if len(args) == 0 {
		packageSpecs = []string{"."}
	} else {
		packageSpecs = args
	}

	// npm behavior: validate all manifests first before creating any tarballs
	type packageManifest struct {
		spec         string
		pkg          *validation.PackageJSON
		sourceDir    string
		filterResult *filtering.FilterResult
	}

	var manifests []packageManifest
	var validationErrors []string

	// First pass: validate all packages
	for _, spec := range packageSpecs {
		specType := packaging.DetectPackageSpecType(spec)
		if specType == "tarball" {
			// For tarballs, we'll handle them separately as they don't need validation
			continue
		}

		if specType == "folder_no_package_json" {
			validationErrors = append(validationErrors, fmt.Sprintf("%s: no package.json found", spec))
			continue
		}

		if specType == "unknown" {
			validationErrors = append(validationErrors, fmt.Sprintf("%s: invalid package spec", spec))
			continue
		}

		validationResult, err := validation.ValidatePackage(spec)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("%s: validation failed: %v", spec, err))
			continue
		}

		// npm error message: "Invalid package, must have name and version" - check this first
		if validationResult.Package.Name == "" || validationResult.Package.Version == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("%s: Invalid package, must have name and version", spec))
			continue
		}

		if !validationResult.Valid {
			validationErrors = append(validationErrors, fmt.Sprintf("%s: package validation failed", spec))
			continue
		}

		// Handle scope configuration for pack
		if packScope != "" {
			if !strings.HasPrefix(packScope, "@") {
				validationErrors = append(validationErrors, fmt.Sprintf("%s: scope must start with @ (e.g., @myscope)", spec))
				continue
			}
		}

		filterEngine, err := filtering.NewFileFilterEngine(spec)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("%s: failed to create file filter: %v", spec, err))
			continue
		}

		filterResult, err := filterEngine.FilterFiles()
		if err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("%s: failed to filter files: %v", spec, err))
			continue
		}

		manifests = append(manifests, packageManifest{
			spec:         spec,
			pkg:          validationResult.Package,
			sourceDir:    spec,
			filterResult: filterResult,
		})
	}

	// If any validation errors, bail early
	if len(validationErrors) > 0 {
		for _, err := range validationErrors {
			if !packJSON {
				fmt.Printf("%s %s\n", styling.Error("✗"), err)
			}
		}
		return fmt.Errorf("failed to validate %d package(s)", len(validationErrors))
	}

	// Second pass: create tarballs
	var results []PackResult
	var allErrors []string
	processedFiles := make(map[string]bool) // Track processed files to handle overwrites

	// Handle regular packages
	for _, manifest := range manifests {
		var result *PackResult
		var err error

		if packDryRun {
			result, err = createDryRunResult(manifest.pkg, manifest.filterResult)
		} else {
			result, err = createPackage(manifest.sourceDir, manifest.pkg, manifest.filterResult, nil)
		}

		if err != nil {
			allErrors = append(allErrors, fmt.Sprintf("%s: %v", manifest.spec, err))
			continue
		}

		// npm pack behavior: overwrite if same filename already processed
		if processedFiles[result.Filename] {
			// Remove previous result with same filename
			for i, r := range results {
				if r.Filename == result.Filename {
					results = append(results[:i], results[i+1:]...)
					break
				}
			}
		}
		processedFiles[result.Filename] = true
		results = append(results, *result)
	}

	// Handle tarballs separately
	for _, spec := range packageSpecs {
		if packaging.DetectPackageSpecType(spec) == "tarball" {
			result, err := repackTarball(spec)
			if err != nil {
				allErrors = append(allErrors, fmt.Sprintf("%s: %v", spec, err))
				continue
			}

			if processedFiles[result.Filename] {
				for i, r := range results {
					if r.Filename == result.Filename {
						results = append(results[:i], results[i+1:]...)
						break
					}
				}
			}
			processedFiles[result.Filename] = true
			results = append(results, *result)
		}
	}

	if packJSON {
		output := PackOutput{
			Results: results,
			Success: len(allErrors) == 0,
		}
		if len(allErrors) > 0 {
			output.Error = strings.Join(allErrors, "; ")
		}

		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON output: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// npm pack behavior: print filenames to stdout with @ and / replaced (even in dry-run)
	for _, result := range results {
		// Match npm: tar.filename.replace(/^@/, '').replace(/\//, '-')
		filename := result.Filename
		filename = strings.TrimPrefix(filename, "@")
		filename = strings.ReplaceAll(filename, "/", "-")
		fmt.Println(filename)
	}

	if len(allErrors) > 0 {
		for _, err := range allErrors {
			fmt.Printf("%s %s\n", styling.Error("✗"), err)
		}
		return fmt.Errorf("failed to pack %d package(s)", len(allErrors))
	}

	return nil
}

func createDryRunResult(pkg *validation.PackageJSON, filterResult *filtering.FilterResult) (*PackResult, error) {
	result := &PackResult{
		Name:         pkg.Name,
		Version:      pkg.Version,
		Filename:     fmt.Sprintf("%s-%s.tgz", pkg.Name, pkg.Version),
		FileCount:    filterResult.FileCount,
		UnpackedSize: filterResult.TotalSize,
	}

	for _, file := range filterResult.Files {
		if !file.IsDir {
			result.Files = append(result.Files, file.RelativePath)
		}
	}

	if !packJSON {
		fmt.Println(styling.Header("🧪 Dry Run - Would Pack"))
		fmt.Println(styling.Separator())
		fmt.Printf("%s %s@%s\n", styling.Label("Package:"), styling.Package(pkg.Name), styling.Version(pkg.Version))
		fmt.Printf("%s %s\n", styling.Label("Output:"), styling.File(result.Filename))
		fmt.Printf("%s %s\n", styling.Label("Files:"), styling.Value(fmt.Sprintf("%d", result.FileCount)))
		fmt.Printf("%s %s\n", styling.Label("Unpacked Size:"), styling.Size(fmt.Sprintf("%.1f kB", float64(result.UnpackedSize)/1024)))
		fmt.Println(styling.Separator())

		if len(result.Files) > 0 {
			fmt.Println(styling.Info("📋 Files to be included:"))
			for _, file := range result.Files {
				fmt.Printf("  %s\n", file)
			}
		}
	}

	return result, nil
}

func createPackage(sourceDir string, pkg *validation.PackageJSON, filterResult *filtering.FilterResult, cleanup func()) (*PackResult, error) {
	if cleanup != nil {
		defer cleanup()
	}

	outputDir := packDestination
	if outputDir == "" {
		outputDir = "."
	}

	if !packaging.IsValidPackageNameForFilename(pkg.Name) || !packaging.IsValidVersionForFilename(pkg.Version) {
		return nil, fmt.Errorf("invalid package name or version for filename")
	}

	outputFile := fmt.Sprintf("%s-%s.tgz", pkg.Name, pkg.Version)
	outputPath := filepath.Join(outputDir, outputFile)
	cleanOutputPath := filepath.Clean(outputPath)

	file, err := os.Create(cleanOutputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() { _ = file.Close() }()

	gw := gzip.NewWriter(file)
	defer func() { _ = gw.Close() }()

	tw := tar.NewWriter(gw)
	defer func() { _ = tw.Close() }()

	sha1Hash := sha1.New() // #nosec G401 - Required for npm compatibility
	sha512Hash := sha512.New()

	var filePaths []string

	for _, filteredFile := range filterResult.Files {
		if filteredFile.IsDir {
			continue
		}

		filePaths = append(filePaths, filteredFile.RelativePath)

		relativePath := strings.ReplaceAll(filteredFile.RelativePath, "\\", "/")

		info, err := os.Stat(filteredFile.AbsolutePath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat file %s: %w", filteredFile.RelativePath, err)
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return nil, fmt.Errorf("failed to create tar header: %w", err)
		}

		header.Name = fmt.Sprintf("package/%s", relativePath)
		if err := tw.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("failed to write tar header: %w", err)
		}

		fileData, err := os.ReadFile(filteredFile.AbsolutePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filteredFile.RelativePath, err)
		}

		if _, err := tw.Write(fileData); err != nil {
			return nil, fmt.Errorf("failed to write file data: %w", err)
		}

		sha1Hash.Write(fileData) // #nosec G401 - Required for npm compatibility
		sha512Hash.Write(fileData)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	sha1Bytes := sha1Hash.Sum(nil) // #nosec G401 - Required for npm compatibility
	sha512Bytes := sha512Hash.Sum(nil)
	integrity := fmt.Sprintf("sha512-%s", base64.StdEncoding.EncodeToString(sha512Bytes))

	result := &PackResult{
		Name:         pkg.Name,
		Version:      pkg.Version,
		Filename:     outputFile,
		Files:        filePaths,
		FileCount:    filterResult.FileCount,
		UnpackedSize: filterResult.TotalSize,
		PackedSize:   fileInfo.Size(),
		Sha1:         hex.EncodeToString(sha1Bytes), // #nosec G401 - Required for npm compatibility
		Sha512:       hex.EncodeToString(sha512Bytes),
		Integrity:    integrity,
	}

	// Output is handled in packPackages function to match npm behavior

	return result, nil
}

//nolint:unused
func printPackResult(result PackResult) {
	fmt.Println(styling.Header("📦  GPM Package Created Successfully"))
	fmt.Println(styling.Separator())
	fmt.Printf("%s %s@%s\n", styling.Label("Package:"), styling.Package(result.Name), styling.Version(result.Version))
	fmt.Printf("%s %s\n", styling.Label("Output:"), styling.File(result.Filename))
	fmt.Printf("%s %s (compressed) / %s (unpacked)\n", styling.Label("Size:"),
		styling.Size(fmt.Sprintf("%.1f kB", float64(result.PackedSize)/1024)),
		styling.Size(fmt.Sprintf("%.1f kB", float64(result.UnpackedSize)/1024)))
	fmt.Printf("%s %s\n", styling.Label("Files:"), styling.Value(fmt.Sprintf("%d", result.FileCount)))
	if len(result.Sha1) >= 20 {
		fmt.Printf("%s %s\n", styling.Label("SHA1:"), styling.Hash(result.Sha1[:20]))
	} else {
		fmt.Printf("%s %s\n", styling.Label("SHA1:"), styling.Hash(result.Sha1))
	}
	fmt.Printf("%s %s\n", styling.Label("Integrity:"), styling.Hash(result.Integrity))
	fmt.Println(styling.Separator())
	fmt.Printf("Ready to publish with: %s\n", styling.Command(fmt.Sprintf("gpm publish %s", result.Filename)))
}

func repackTarball(tarballPath string) (*PackResult, error) {
	packageInfo, err := packaging.ExtractPackageInfo(tarballPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract package info: %w", err)
	}

	info, err := os.Stat(tarballPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat tarball: %w", err)
	}

	result := &PackResult{
		Name:       packageInfo.Name,
		Version:    packageInfo.Version,
		Filename:   filepath.Base(tarballPath),
		PackedSize: info.Size(),
	}

	// Output is handled in packPackages function to match npm behavior

	return result, nil
}
