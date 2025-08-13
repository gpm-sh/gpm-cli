package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gpm.sh/gpm/gpm-cli/internal/api"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/styling"
)

var publishCmd = &cobra.Command{
	Use:   "publish [tarball]",
	Short: "Publish a package to GPM registry",
	Long:  `Publish a package tarball to the GPM registry`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return publish(args[0])
	},
}

func publish(tarballPath string) error {
	if _, err := os.Stat(tarballPath); os.IsNotExist(err) {
		return fmt.Errorf("tarball file not found: %s", tarballPath)
	}

	cfg := config.GetConfig()
	if cfg.Token == "" {
		return fmt.Errorf("not authenticated. Please run 'gpm login' first")
	}

	packageInfo, err := extractPackageInfo(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to extract package info: %w", err)
	}

	client := api.NewClient(cfg.Registry, cfg.Token)

	fmt.Println(styling.Header("📤 Publishing Package"))
	fmt.Println(styling.Separator())
	fmt.Printf("%s %s\n", styling.Label("Package:"), styling.Package(packageInfo.Name))
	fmt.Printf("%s %s\n", styling.Label("Version:"), styling.Version(packageInfo.Version))
	fmt.Printf("%s %s\n", styling.Label("File:"), styling.File(tarballPath))
	fmt.Println(styling.Separator())

	req := &api.PublishRequest{
		Name:       packageInfo.Name,
		Version:    packageInfo.Version,
		Visibility: "public",
	}

	resp, err := client.Publish(req, tarballPath)
	if err != nil {
		return fmt.Errorf("publish failed: %v", err)
	}

	if resp.Success {
		fmt.Println(styling.Success("✓ Package published successfully!"))
		fmt.Printf("%s %s\n", styling.Label("Package ID:"), styling.Value(resp.Data.PackageID))
		fmt.Printf("%s %s\n", styling.Label("Version ID:"), styling.Value(resp.Data.VersionID))
		fmt.Printf("%s %s\n", styling.Label("Download URL:"), styling.URL(resp.Data.DownloadURL))
		fmt.Printf("%s %s\n", styling.Label("File Size:"), styling.Size(fmt.Sprintf("%d bytes", resp.Data.FileSize)))
		fmt.Printf("%s %s\n", styling.Label("Upload Time:"), styling.Value(resp.Data.UploadTime))
	} else {
		if resp.Error != nil {
			return fmt.Errorf("publish failed: %s - %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("publish failed with unknown error")
	}

	return nil
}

func extractPackageInfo(tarballPath string) (*PackageInfo, error) {
	// Security: Validate the tarball path
	cleanPath := filepath.Clean(tarballPath)
	if !strings.HasSuffix(cleanPath, ".tgz") && !strings.HasSuffix(cleanPath, ".tar.gz") {
		return nil, fmt.Errorf("invalid file type: only .tgz and .tar.gz files are allowed")
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if header.Name == "package/package.json" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}

			var packageInfo PackageInfo
			if err := json.Unmarshal(data, &packageInfo); err != nil {
				return nil, err
			}

			return &packageInfo, nil
		}
	}

	return nil, fmt.Errorf("package.json not found in tarball")
}
