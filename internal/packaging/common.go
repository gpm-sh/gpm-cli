package packaging

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func DetectPackageSpecType(packageSpec string) string {
	if strings.HasSuffix(packageSpec, ".tgz") || strings.HasSuffix(packageSpec, ".tar.gz") {
		return "tarball"
	}

	if stat, err := os.Stat(packageSpec); err == nil && stat.IsDir() {
		packageJSONPath := filepath.Join(packageSpec, "package.json")
		if _, err := os.Stat(packageJSONPath); err == nil {
			return "folder"
		}
		return "folder_no_package_json"
	}

	return "unknown"
}

func ExtractPackageInfo(tarballPath string) (*PackageInfo, error) {
	cleanPath := filepath.Clean(tarballPath)
	if !strings.HasSuffix(cleanPath, ".tgz") && !strings.HasSuffix(cleanPath, ".tar.gz") {
		return nil, fmt.Errorf("invalid file type: only .tgz and .tar.gz files are allowed")
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gzr.Close() }()

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

func IsValidPackageNameForFilename(name string) bool {
	if len(name) == 0 || len(name) > 214 {
		return false
	}

	// For non-scoped packages, don't allow / or @
	if strings.Contains(name, "/") || strings.Contains(name, "@") {
		return false
	}

	return isValidBasicName(name)
}

func IsValidVersionForFilename(version string) bool {
	if len(version) == 0 || len(version) > 50 {
		return false
	}

	for _, char := range version {
		if !isValidVersionChar(char) {
			return false
		}
	}

	return true
}

func isValidBasicName(name string) bool {
	if len(name) == 0 {
		return false
	}

	for _, char := range name {
		if !isValidNameChar(char) {
			return false
		}
	}

	return true
}

func isValidNameChar(char rune) bool {
	return (char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9') ||
		char == '.' || char == '-' || char == '_'
}

func isValidVersionChar(char rune) bool {
	return (char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9') ||
		char == '.' || char == '-'
}
