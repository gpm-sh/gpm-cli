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
	Access      string   `json:"access"`
	Tag         string   `json:"tag,omitempty"`
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

// PackageMetadata represents complete package metadata from registry
type PackageMetadata struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	DistTags    map[string]string          `json:"dist-tags,omitempty"`
	Versions    map[string]*PackageVersion `json:"versions,omitempty"`
	Time        map[string]string          `json:"time,omitempty"`
	Author      interface{}                `json:"author,omitempty"`
	License     string                     `json:"license,omitempty"`
	Repository  interface{}                `json:"repository,omitempty"`
	Homepage    string                     `json:"homepage,omitempty"`
	Keywords    []string                   `json:"keywords,omitempty"`
}

// PackageVersion represents a specific version of a package
type PackageVersion struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description,omitempty"`
	Author       interface{}       `json:"author,omitempty"`
	License      string            `json:"license,omitempty"`
	Repository   interface{}       `json:"repository,omitempty"`
	Homepage     string            `json:"homepage,omitempty"`
	Keywords     []string          `json:"keywords,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Dist         *PackageDist      `json:"dist,omitempty"`
	Unity        string            `json:"unity,omitempty"`
	DisplayName  string            `json:"displayName,omitempty"`
	Category     string            `json:"category,omitempty"`
}

// PackageDist represents distribution metadata for a package version
type PackageDist struct {
	Integrity string `json:"integrity,omitempty"`
	Shasum    string `json:"shasum,omitempty"`
	Tarball   string `json:"tarball,omitempty"`
	FileSize  int64  `json:"fileSize,omitempty"`
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
	Email    string `json:"email,omitempty"`
	Type     string `json:"type,omitempty"`
}

type RegisterRequest struct {
	ID       string `json:"_id"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Type     string `json:"type"`
}

type LoginResponse struct {
	OK    bool   `json:"ok"`
	ID    string `json:"id"`
	Rev   string `json:"rev"`
	Token string `json:"token"`
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
	Username string `json:"username"`
}

// OAuth 2.0 Authorization Code with PKCE structures
type OAuthAuthorizationRequest struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	ResponseType        string `json:"response_type"`
	Scope               string `json:"scope"`
	State               string `json:"state"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
}

type OAuthTokenRequest struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	ClientID     string `json:"client_id"`
	CodeVerifier string `json:"code_verifier"`
}

type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type OAuthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
	State            string `json:"state,omitempty"`
}

// Legacy web login structures (deprecated - use OAuth instead)
type WebLoginRequest struct {
	SessionID string `json:"sessionId"`
}

type WebLoginResponse struct {
	SessionID string `json:"sessionId"`
	LoginURL  string `json:"loginUrl"`
}

type WebLoginStatus struct {
	Completed bool   `json:"completed"`
	Token     string `json:"token,omitempty"`
	Username  string `json:"username,omitempty"`
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

func (c *Client) GetPackageInfo(name, version string) (*PackageInfo, error) {
	endpoint := fmt.Sprintf("/-/v1/packages/%s", name)
	if version != "" && version != "latest" {
		endpoint = fmt.Sprintf("/-/v1/packages/%s/%s", name, version)
	}

	resp, err := c.makeRequest("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var info PackageInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode package info: %w", err)
	}

	return &info, nil
}

// GetPackageMetadata retrieves complete package metadata including all versions and dist-tags
func (c *Client) GetPackageMetadata(name string) (*PackageMetadata, error) {
	// Try registry-specific endpoint first
	endpoint := fmt.Sprintf("/%s", name)

	resp, err := c.makeRequest("GET", endpoint, nil, nil)
	if err != nil {
		// Check for 404 to provide better error message
		if resp != nil && resp.StatusCode == 404 {
			return nil, fmt.Errorf("package '%s' not found", name)
		}
		return nil, fmt.Errorf("failed to fetch package metadata: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("package '%s' not found", name)
	}

	var metadata PackageMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode package metadata: %w", err)
	}

	// Validate that we got a valid package
	if metadata.Name == "" {
		return nil, fmt.Errorf("invalid package response: missing name")
	}

	return &metadata, nil
}

// CheckPackageExists checks if a package exists in the registry
func (c *Client) CheckPackageExists(name string) (bool, error) {
	_, err := c.GetPackageMetadata(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetPackageVersions returns all available versions for a package
func (c *Client) GetPackageVersions(name string) ([]string, error) {
	metadata, err := c.GetPackageMetadata(name)
	if err != nil {
		return nil, err
	}

	var versions []string
	for version := range metadata.Versions {
		versions = append(versions, version)
	}

	return versions, nil
}

// ResolvePackageVersion resolves a version specification to a concrete version
func (c *Client) ResolvePackageVersion(name, versionSpec string) (string, error) {
	metadata, err := c.GetPackageMetadata(name)
	if err != nil {
		return "", err
	}

	// If no version specified, or "latest", use latest dist-tag
	if versionSpec == "" || versionSpec == "latest" {
		if metadata.DistTags == nil {
			return "", fmt.Errorf("package '%s' has no dist-tags - no default version available", name)
		}

		latestVersion, exists := metadata.DistTags["latest"]
		if !exists || latestVersion == "" {
			return "", fmt.Errorf("package '%s' has no 'latest' dist-tag - no default version available", name)
		}

		// Verify the latest version actually exists
		if metadata.Versions == nil || metadata.Versions[latestVersion] == nil {
			return "", fmt.Errorf("package '%s' latest version '%s' is invalid", name, latestVersion)
		}

		return latestVersion, nil
	}

	// If specific version requested, verify it exists
	if metadata.Versions == nil || metadata.Versions[versionSpec] == nil {
		return "", fmt.Errorf("version '%s' not available for package '%s'", versionSpec, name)
	}

	return versionSpec, nil
}

func (c *Client) Login(req *LoginRequest) (*LoginResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal login request: %w", err)
	}

	resp, err := c.makeRequest("POST", "/-/v1/login", data, map[string]string{
		"Content-Type": "application/json",
	})
	if err == nil {
		defer func() { _ = resp.Body.Close() }()

		var loginResp LoginResponse
		if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
			return nil, fmt.Errorf("failed to decode login response: %w", err)
		}

		return &loginResp, nil
	}

	// Fallback to npm/verdaccio CouchDB-style login if primary endpoint failed
	npmResp, npmErr := c.npmCompatibleLogin(req)
	if npmErr == nil {
		return npmResp, nil
	}

	return nil, err
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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

	var whoamiResp WhoamiResponse
	if err := json.NewDecoder(resp.Body).Decode(&whoamiResp); err != nil {
		return nil, fmt.Errorf("failed to decode whoami response: %w", err)
	}

	return &whoamiResp, nil
}

// OAuth 2.0 Authorization Code with PKCE methods
func (c *Client) StartOAuthFlow(authorizationURL string) (string, error) {
	// Open browser to authorization URL
	return authorizationURL, nil
}

func (c *Client) ExchangeCodeForToken(code, clientID, redirectURI, codeVerifier string) (*OAuthTokenResponse, error) {
	tokenRequest := OAuthTokenRequest{
		GrantType:    "authorization_code",
		Code:         code,
		RedirectURI:  redirectURI,
		ClientID:     clientID,
		CodeVerifier: codeVerifier,
	}

	reqBody, err := json.Marshal(tokenRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token request: %w", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	resp, err := c.makeRequest("POST", "/oauth/token", reqBody, headers)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var oauthErr OAuthErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&oauthErr); err != nil {
			return nil, fmt.Errorf("oauth token exchange failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("oauth error: %s - %s", oauthErr.Error, oauthErr.ErrorDescription)
	}

	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// Legacy web login methods (deprecated - use OAuth instead)
func (c *Client) StartWebLogin() (*WebLoginResponse, error) {
	resp, err := c.makeRequest("POST", "/-/v1/login/web", nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var webLoginResp WebLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&webLoginResp); err != nil {
		return nil, fmt.Errorf("failed to decode web login response: %w", err)
	}

	return &webLoginResp, nil
}

func (c *Client) CheckWebLogin(sessionID string) (*WebLoginStatus, error) {
	endpoint := fmt.Sprintf("/-/v1/login/web/%s", sessionID)
	resp, err := c.makeRequest("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var status WebLoginStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode web login status: %w", err)
	}

	return &status, nil
}

func (c *Client) Publish(req *PublishRequest, tarballPath string) (*PublishResponse, error) {
	// Security: Validate the tarball path
	cleanPath := filepath.Clean(tarballPath)
	if !strings.HasSuffix(cleanPath, ".tgz") && !strings.HasSuffix(cleanPath, ".tar.gz") {
		return nil, fmt.Errorf("invalid file type: only .tgz and .tar.gz files are allowed")
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open tarball: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Read tarball data
	tarballData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read tarball: %w", err)
	}

	// First extract the actual package.json from the tarball
	packageInfo, err := extractPackageInfoWithTarballData(tarballData)
	if err != nil {
		return nil, fmt.Errorf("failed to extract package info: %w", err)
	}

	// Create npm publish format request using the actual package.json data
	npmRequest := map[string]interface{}{
		"_id":    packageInfo.Name,
		"name":   packageInfo.Name,
		"access": req.Access,
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
	defer func() { _ = resp.Body.Close() }()

	// Read response body for flexible handling
	respBody, _ := io.ReadAll(resp.Body)

	// Try to decode into our structured response first
	var publishResp PublishResponse
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &publishResp); err == nil {
			// If server explicitly says success, return it
			if publishResp.Success {
				return &publishResp, nil
			}
		}
	}

	// Try to decode npm-compatible response format
	var npmResp struct {
		OK  bool   `json:"ok"`
		ID  string `json:"id"`
		Rev string `json:"rev"`
	}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &npmResp); err == nil && npmResp.OK {
			// Convert npm response to our format
			return &PublishResponse{
				Success: true,
				Data: PublishData{
					PackageID:   npmResp.ID,
					VersionID:   npmResp.Rev,
					DownloadURL: "", // Not provided in npm format
					FileSize:    0,  // Not provided in npm format
					UploadTime:  "", // Not provided in npm format
				},
			}, nil
		}
	}

	// If we reached here and the HTTP status is 2xx, treat as success for npm-compatible registries
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &PublishResponse{Success: true}, nil
	}

	// Otherwise, include body for easier debugging
	return nil, fmt.Errorf("unexpected publish response (status %d): %s", resp.StatusCode, string(respBody))
}

func extractPackageInfoWithTarballData(tarballData []byte) (*PackageInfo, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(tarballData))
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

			// Also extract the raw package.json data
			var rawData map[string]interface{}
			if err := json.Unmarshal(data, &rawData); err != nil {
				return nil, err
			}

			// Add dist information to the raw data - hash the tarball, not the package.json
			rawData["dist"] = map[string]interface{}{
				"integrity": fmt.Sprintf("sha512-%s", generateSHA512(tarballData)),
				"shasum":    generateSHA256(tarballData),
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

// npmCompatibleLogin attempts npm/verdaccio-style login by PUTing to
// /-/user/org.couchdb.user:<username> with a CouchDB-style payload.
func (c *Client) npmCompatibleLogin(req *LoginRequest) (*LoginResponse, error) {
	username := strings.TrimSpace(req.Name)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	payload := map[string]interface{}{
		"name":     username,
		"password": req.Password,
		"type":     "user",
	}
	if req.Email != "" {
		payload["email"] = req.Email
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal npm login payload: %w", err)
	}

	endpoint := "/-/user/org.couchdb.user:" + url.PathEscape(username)
	resp, err := c.makeRequest("PUT", endpoint, body, map[string]string{
		"Content-Type": "application/json",
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// Verdaccio responses may include token or ok:true
	var res struct {
		Token string `json:"token"`
		OK    bool   `json:"ok"`
		ID    string `json:"id"`
		Rev   string `json:"rev"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode npm login response: %w", err)
	}

	if res.Token == "" && !res.OK {
		return nil, fmt.Errorf("npm-compatible login failed")
	}

	return &LoginResponse{OK: res.OK || res.Token != "", ID: res.ID, Rev: res.Rev, Token: res.Token}, nil
}
