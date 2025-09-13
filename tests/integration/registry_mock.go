package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// RegistryMock provides npm-compatible registry responses for integration tests
type RegistryMock struct {
	server   *httptest.Server
	packages map[string]*PackageDoc
	users    map[string]*User
}

// PackageDoc represents a complete npm-style package document
type PackageDoc struct {
	ID          string                     `json:"_id"`
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	DistTags    map[string]string          `json:"dist-tags"`
	Versions    map[string]*PackageVersion `json:"versions"`
	Time        map[string]string          `json:"time"`
	Users       map[string]bool            `json:"users,omitempty"`
	Maintainers []Maintainer               `json:"maintainers,omitempty"`
	Author      *Person                    `json:"author,omitempty"`
	Repository  *Repository                `json:"repository,omitempty"`
	Homepage    string                     `json:"homepage,omitempty"`
	Keywords    []string                   `json:"keywords,omitempty"`
	License     string                     `json:"license,omitempty"`
	ReadmeFile  string                     `json:"readme,omitempty"`
	Access      string                     `json:"access,omitempty"` // public, scoped, private
}

// PackageVersion represents a specific version in npm format
type PackageVersion struct {
	Name             string            `json:"name"`
	Version          string            `json:"version"`
	Description      string            `json:"description"`
	Main             string            `json:"main,omitempty"`
	Scripts          map[string]string `json:"scripts,omitempty"`
	Dependencies     map[string]string `json:"dependencies,omitempty"`
	DevDependencies  map[string]string `json:"devDependencies,omitempty"`
	PeerDependencies map[string]string `json:"peerDependencies,omitempty"`
	Author           *Person           `json:"author,omitempty"`
	License          string            `json:"license,omitempty"`
	Repository       *Repository       `json:"repository,omitempty"`
	Homepage         string            `json:"homepage,omitempty"`
	Keywords         []string          `json:"keywords,omitempty"`
	Dist             *Dist             `json:"dist"`

	// Unity-specific fields
	Unity       string `json:"unity,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Category    string `json:"category,omitempty"`
}

// Dist represents distribution metadata
type Dist struct {
	Integrity    string `json:"integrity"`
	Shasum       string `json:"shasum"`
	Tarball      string `json:"tarball"`
	FileCount    int    `json:"fileCount,omitempty"`
	UnpackedSize int    `json:"unpackedSize,omitempty"`
}

// Person represents author/maintainer information
type Person struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// Maintainer represents package maintainer
type Maintainer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Repository represents repository information
type Repository struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// User represents a registry user
type User struct {
	Username string `json:"name"`
	Email    string `json:"email"`
	Token    string `json:"token"`
}

// NewRegistryMock creates a new mock registry server
func NewRegistryMock() *RegistryMock {
	rm := &RegistryMock{
		packages: make(map[string]*PackageDoc),
		users:    make(map[string]*User),
	}

	rm.server = httptest.NewServer(http.HandlerFunc(rm.handler))
	return rm
}

// Close shuts down the mock server
func (rm *RegistryMock) Close() {
	rm.server.Close()
}

// URL returns the mock server URL
func (rm *RegistryMock) URL() string {
	return rm.server.URL
}

// AddPackage adds a package to the mock registry
func (rm *RegistryMock) AddPackage(pkg *PackageDoc) {
	rm.packages[pkg.Name] = pkg
}

// AddUser adds a user to the mock registry
func (rm *RegistryMock) AddUser(user *User) {
	rm.users[user.Username] = user
}

// handler handles HTTP requests to the mock registry
func (rm *RegistryMock) handler(w http.ResponseWriter, r *http.Request) {
	// Set common headers
	w.Header().Set("Content-Type", "application/json")

	// Handle authentication endpoints
	if strings.HasPrefix(r.URL.Path, "/-/user/org.couchdb.user:") {
		rm.handleAuth(w, r)
		return
	}

	if r.URL.Path == "/-/whoami" {
		rm.handleWhoami(w, r)
		return
	}

	// Handle package endpoints
	if !strings.HasPrefix(r.URL.Path, "/-/") {
		rm.handlePackage(w, r)
		return
	}

	// Handle search
	if r.URL.Path == "/-/v1/search" {
		rm.handleSearch(w, r)
		return
	}

	// Default 404
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "Not found",
	})
}

// handlePackage handles package document requests
func (rm *RegistryMock) handlePackage(w http.ResponseWriter, r *http.Request) {
	packageName := strings.TrimPrefix(r.URL.Path, "/")
	packageName = strings.TrimSuffix(packageName, "/")

	pkg, exists := rm.packages[packageName]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Not found",
		})
		return
	}

	// Check access permissions
	if rm.requiresAuth(pkg) && !rm.isAuthenticated(r) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Authentication required",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(pkg)
}

// handleAuth handles npm-style authentication
func (rm *RegistryMock) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var authReq map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&authReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	username, _ := authReq["name"].(string)
	password, _ := authReq["password"].(string)

	// Simple auth check (for testing purposes)
	user, exists := rm.users[username]
	if !exists || password != "validpass" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid credentials",
		})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"id":    fmt.Sprintf("org.couchdb.user:%s", username),
		"rev":   "1-abc123",
		"token": user.Token,
	})
}

// handleWhoami handles user info requests
func (rm *RegistryMock) handleWhoami(w http.ResponseWriter, r *http.Request) {
	if !rm.isAuthenticated(r) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Authentication required",
		})
		return
	}

	token := rm.extractToken(r)
	for _, user := range rm.users {
		if user.Token == token {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"username": user.Username,
			})
			return
		}
	}

	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "Invalid token",
	})
}

// handleSearch handles package search requests
func (rm *RegistryMock) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("text")

	var results []map[string]interface{}

	for _, pkg := range rm.packages {
		if strings.Contains(strings.ToLower(pkg.Name), strings.ToLower(query)) ||
			strings.Contains(strings.ToLower(pkg.Description), strings.ToLower(query)) {

			results = append(results, map[string]interface{}{
				"package": map[string]interface{}{
					"name":        pkg.Name,
					"description": pkg.Description,
					"version":     pkg.DistTags["latest"],
					"keywords":    pkg.Keywords,
					"author":      pkg.Author,
				},
				"score": map[string]interface{}{
					"final": 0.8,
					"detail": map[string]float64{
						"quality":     0.8,
						"popularity":  0.7,
						"maintenance": 0.9,
					},
				},
			})
		}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"objects": results,
		"total":   len(results),
		"time":    "Wed Oct 11 2023 12:34:56 GMT+0000 (UTC)",
	})
}

// requiresAuth checks if a package requires authentication
func (rm *RegistryMock) requiresAuth(pkg *PackageDoc) bool {
	return pkg.Access == "private" || pkg.Access == "scoped"
}

// isAuthenticated checks if the request has valid authentication
func (rm *RegistryMock) isAuthenticated(r *http.Request) bool {
	token := rm.extractToken(r)
	if token == "" {
		return false
	}

	for _, user := range rm.users {
		if user.Token == token {
			return true
		}
	}

	return false
}

// extractToken extracts bearer token from Authorization header
func (rm *RegistryMock) extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

// CreateTestPackage creates a package for testing
func CreateTestPackage(name, version string, access string) *PackageDoc {
	return &PackageDoc{
		ID:          name,
		Name:        name,
		Description: fmt.Sprintf("Test package %s", name),
		DistTags: map[string]string{
			"latest": version,
		},
		Versions: map[string]*PackageVersion{
			version: {
				Name:        name,
				Version:     version,
				Description: fmt.Sprintf("Test package %s version %s", name, version),
				Main:        "index.js",
				Dist: &Dist{
					Integrity:    "sha512-abc123...",
					Shasum:       "abc123",
					Tarball:      fmt.Sprintf("http://registry.test/%s/-/%s-%s.tgz", name, name, version),
					FileCount:    5,
					UnpackedSize: 1024,
				},
			},
		},
		Time: map[string]string{
			"created":  "2023-01-01T00:00:00.000Z",
			"modified": "2023-01-01T00:00:00.000Z",
			version:    "2023-01-01T00:00:00.000Z",
		},
		Author: &Person{
			Name:  "Test Author",
			Email: "test@example.com",
		},
		License:  "MIT",
		Keywords: []string{"test", "gpm"},
		Access:   access,
	}
}

// CreateUnityPackage creates a Unity-specific test package
func CreateUnityPackage(name, version, unityVersion string) *PackageDoc {
	pkg := CreateTestPackage(name, version, "public")
	pkg.Versions[version].Unity = unityVersion
	pkg.Versions[version].DisplayName = "Test Unity Package"
	pkg.Versions[version].Category = "Unity"
	return pkg
}
