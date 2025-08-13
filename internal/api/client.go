package api

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"archive/tar"
	"compress/gzip"

	gpmerrors "gpm.sh/gpm/gpm-cli/internal/errors"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type PublishRequest struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Visibility  string   `json:"visibility"`
	Studio      string   `json:"studio,omitempty"`
	Description string   `json:"description,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	License     string   `json:"license,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Repository  string   `json:"repository,omitempty"`
	Author      string   `json:"author,omitempty"`
}

type PackageInfo struct {
	Name    string                 `json:"name"`
	Version string                 `json:"version"`
	RawData map[string]interface{} `json:"-"` // Store the raw package.json data
}

type PublishData struct {
	PackageID   string `json:"packageId"`
	VersionID   string `json:"versionId"`
	DownloadURL string `json:"downloadUrl"`
	FileSize    int64  `json:"fileSize"`
	UploadTime  string `json:"uploadTime"`
}

type PublishResponse struct {
	Success bool           `json:"success"`
	Data    PublishData    `json:"data,omitempty"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

type LoginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Type     string `json:"type"`
}

type RegisterRequest struct {
	ID       string `json:"_id"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Type     string `json:"type"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
	} `json:"user"`
}

type RegisterResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	User    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
	} `json:"user,omitempty"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type WhoamiResponse struct {
	User struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
	} `json:"user"`
	Studio struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"studio,omitempty"`
	Plan struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"plan,omitempty"`
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) SetToken(token string) {
	c.token = token
}

func (c *Client) Login(req *LoginRequest) (*LoginResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal login request: %w", err)
	}

	resp, err := c.makeRequest("POST", "/-/v1/login", data, map[string]string{
		"Content-Type": "application/json",
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return nil, fmt.Errorf("failed to decode login response: %w", err)
	}

	return &loginResp, nil
}

func (c *Client) Register(req *RegisterRequest) (*RegisterResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal register request: %w", err)
	}

	resp, err := c.makeRequest("POST", "/-/v1/register", data, map[string]string{
		"Content-Type": "application/json",
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var registerResp RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&registerResp); err != nil {
		return nil, fmt.Errorf("failed to decode register response: %w", err)
	}

	return &registerResp, nil
}

func (c *Client) Whoami() (*WhoamiResponse, error) {
	resp, err := c.makeRequest("GET", "/-/whoami", nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var whoamiResp WhoamiResponse
	if err := json.NewDecoder(resp.Body).Decode(&whoamiResp); err != nil {
		return nil, fmt.Errorf("failed to decode whoami response: %w", err)
	}

	return &whoamiResp, nil
}

func (c *Client) Publish(req *PublishRequest, tarballPath string) (*PublishResponse, error) {
	// First extract the actual package.json from the tarball
	packageInfo, err := extractPackageInfo(tarballPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract package info: %w", err)
	}

	// Security: Validate the tarball path
	cleanPath := filepath.Clean(tarballPath)
	if !strings.HasSuffix(cleanPath, ".tgz") && !strings.HasSuffix(cleanPath, ".tar.gz") {
		return nil, fmt.Errorf("invalid file type: only .tgz and .tar.gz files are allowed")
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open tarball: %w", err)
	}
	defer file.Close()

	// Read tarball data
	tarballData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read tarball: %w", err)
	}

	// Create npm publish format request using the actual package.json data
	npmRequest := map[string]interface{}{
		"_id":  packageInfo.Name,
		"name": packageInfo.Name,
		"versions": map[string]interface{}{
			packageInfo.Version: packageInfo.RawData,
		},
		"_attachments": map[string]interface{}{
			fmt.Sprintf("%s-%s.tgz", packageInfo.Name, packageInfo.Version): map[string]interface{}{
				"content_type": "application/octet-stream",
				"data":         base64.StdEncoding.EncodeToString(tarballData),
				"length":       len(tarballData),
			},
		},
		"time": map[string]interface{}{
			"created":  time.Now().Format(time.RFC3339),
			"modified": time.Now().Format(time.RFC3339),
		},
		"maintainers":    []interface{}{},
		"readme":         "A Unity Package Manager compatible package",
		"readmeFilename": "README.md",
	}

	// Marshal the npm request
	requestBody, err := json.Marshal(npmRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal npm request: %w", err)
	}

	// Send the request to the npm publish endpoint
	resp, err := c.makeRequest("PUT", "/"+packageInfo.Name, requestBody, map[string]string{
		"Content-Type": "application/json",
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var publishResp PublishResponse
	if err := json.NewDecoder(resp.Body).Decode(&publishResp); err != nil {
		return nil, fmt.Errorf("failed to decode publish response: %w", err)
	}

	return &publishResp, nil
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

			// Also extract the raw package.json data
			var rawData map[string]interface{}
			if err := json.Unmarshal(data, &rawData); err != nil {
				return nil, err
			}

			// Add dist information to the raw data
			rawData["dist"] = map[string]interface{}{
				"integrity": fmt.Sprintf("sha512-%s", generateSHA512(data)),
				"shasum":    generateSHA256(data),
				"tarball":   fmt.Sprintf("https://registry.npmjs.org/%s/-/%s-%s.tgz", packageInfo.Name, packageInfo.Name, packageInfo.Version),
			}

			packageInfo.RawData = rawData

			return &packageInfo, nil
		}
	}

	return nil, fmt.Errorf("package.json not found in tarball")
}

// Helper function to generate SHA512 hash
func generateSHA512(data []byte) string {
	hash := sha512.Sum512(data)
	return base64.StdEncoding.EncodeToString(hash[:])
}

// Helper function to generate SHA256 hash
func generateSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (c *Client) makeRequest(method, endpoint string, body []byte, headers map[string]string) (*http.Response, error) {
	url := c.baseURL + endpoint

	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewReader(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, gpmerrors.ErrNetworkFailed(err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		var apiError struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal(body, &apiError); err == nil && apiError.Error.Code != "" {
			return nil, &gpmerrors.GPMError{
				Code:    apiError.Error.Code,
				Message: apiError.Error.Message,
			}
		}

		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

func (c *Client) ValidateRegistry() error {
	parsedURL, err := url.Parse(c.baseURL)
	if err != nil {
		return gpmerrors.ErrRegistryInvalid(c.baseURL)
	}

	if parsedURL.Scheme != "https" {
		return gpmerrors.ErrRegistryInvalid(c.baseURL)
	}

	return nil
}
