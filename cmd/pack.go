package cmd

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

var packCmd = &cobra.Command{
	Use:   "pack",
	Short: "Create a package tarball",
	Long:  `Create a tarball (.tgz) from the current directory for publishing, matching npm pack behavior`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return packPackage(cmd, args)
	},
}

func readPackageJSON(filename string) (*PackageInfo, error) {
	// Security: Validate that the file is named package.json
	if filepath.Base(filename) != "package.json" {
		return nil, fmt.Errorf("invalid filename: only package.json is allowed")
	}

	// Security: Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(filename)

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filename, err)
	}

	var packageInfo PackageInfo
	if err := json.Unmarshal(data, &packageInfo); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	if packageInfo.Name == "" || packageInfo.Version == "" {
		return nil, fmt.Errorf("package.json must contain name and version")
	}

	return &packageInfo, nil
}

func packPackage(cmd *cobra.Command, args []string) error {
	packageInfo, err := readPackageJSON("package.json")
	if err != nil {
		return fmt.Errorf("failed to read package.json: %w", err)
	}

	outputFile := fmt.Sprintf("%s-%s.tgz", packageInfo.Name, packageInfo.Version)

	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	hash := sha512.New()
	fileCount := 0
	totalSize := int64(0)

	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if path == outputFile {
			return nil
		}

		if strings.HasPrefix(path, ".") && path != "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header: %w", err)
		}

		header.Name = fmt.Sprintf("package/%s", path)
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		if !info.IsDir() {
			fileData, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", path, err)
			}

			if _, err := tw.Write(fileData); err != nil {
				return fmt.Errorf("failed to write file data: %w", err)
			}

			hash.Write(fileData)
			totalSize += int64(len(fileData))
			fileCount++
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create tarball: %w", err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	hashBytes := hash.Sum(nil)
	integrity := fmt.Sprintf("sha512-%s", base64.StdEncoding.EncodeToString(hashBytes))

	fmt.Println(styling.Header("📦  GPM Package Created Successfully"))
	fmt.Println(styling.Separator())
	fmt.Printf("%s %s@%s\n", styling.Label("Package:"), styling.Package(packageInfo.Name), styling.Version(packageInfo.Version))
	fmt.Printf("%s %s\n", styling.Label("Output:"), styling.File(outputFile))
	fmt.Printf("%s %s (compressed) / %s (unpacked)\n", styling.Label("Size:"), styling.Size(fmt.Sprintf("%.1f kB", float64(fileInfo.Size())/1024)), styling.Size(fmt.Sprintf("%.1f kB", float64(totalSize)/1024)))
	fmt.Printf("%s %s\n", styling.Label("Files:"), styling.Value(fmt.Sprintf("%d", fileCount)))
	fmt.Printf("%s %s\n", styling.Label("SHA:"), styling.Hash(hex.EncodeToString(hashBytes[:20])))
	fmt.Printf("%s %s\n", styling.Label("Integrity:"), styling.Hash(integrity))
	fmt.Println(styling.Separator())
	fmt.Printf("Ready to publish with: %s\n", styling.Command(fmt.Sprintf("gpm publish %s", outputFile)))

	return nil
}
