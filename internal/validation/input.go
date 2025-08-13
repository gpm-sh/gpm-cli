package validation

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

const (
	MaxUsernameLength    = 50
	MinUsernameLength    = 3
	MaxPasswordLength    = 128
	MinPasswordLength    = 8
	MaxPackageNameLength = 214
	MaxEmailLength       = 254
)

var (
	// Package name patterns
	upmPackageNameRegex = regexp.MustCompile(`^[a-z][a-z0-9\-]*(\.[a-z][a-z0-9\-]*)*$`)
	npmPackageNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

	// Username pattern (alphanumeric, dots, underscores, hyphens)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

	// Email pattern (basic validation)
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	// Version pattern (semantic versioning)
	semverRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
)

// ValidationError represents a validation error with field context
type ValidationError struct {
	Field   string
	Message string
	Value   string
}

func (e ValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("%s: %s (got: %s)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// SanitizeInput removes dangerous characters and normalizes input
func SanitizeInput(input string) string {
	// Remove null bytes and control characters
	sanitized := strings.Map(func(r rune) rune {
		if r == 0 || (unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t') {
			return -1
		}
		return r
	}, input)

	// Trim whitespace
	sanitized = strings.TrimSpace(sanitized)

	return sanitized
}

// ValidateUsername validates and sanitizes username input
func ValidateUsername(username string) error {
	username = SanitizeInput(username)

	if len(username) < MinUsernameLength {
		return ValidationError{
			Field:   "username",
			Message: fmt.Sprintf("must be at least %d characters long", MinUsernameLength),
			Value:   username,
		}
	}

	if len(username) > MaxUsernameLength {
		return ValidationError{
			Field:   "username",
			Message: fmt.Sprintf("must be no more than %d characters long", MaxUsernameLength),
			Value:   username,
		}
	}

	if !usernameRegex.MatchString(username) {
		return ValidationError{
			Field:   "username",
			Message: "can only contain letters, numbers, dots, underscores, and hyphens",
			Value:   username,
		}
	}

	// Check for reserved usernames
	reserved := []string{"admin", "root", "api", "www", "ftp", "mail", "test", "user", "guest"}
	for _, r := range reserved {
		if strings.EqualFold(username, r) {
			return ValidationError{
				Field:   "username",
				Message: "is reserved and cannot be used",
				Value:   username,
			}
		}
	}

	return nil
}

// ValidatePassword validates password strength
func ValidatePassword(password []byte) error {
	if len(password) < MinPasswordLength {
		return ValidationError{
			Field:   "password",
			Message: fmt.Sprintf("must be at least %d characters long", MinPasswordLength),
		}
	}

	if len(password) > MaxPasswordLength {
		return ValidationError{
			Field:   "password",
			Message: fmt.Sprintf("must be no more than %d characters long", MaxPasswordLength),
		}
	}

	passwordStr := string(password)

	// Check for at least one letter and one number
	hasLetter := false
	hasNumber := false

	for _, char := range passwordStr {
		if unicode.IsLetter(char) {
			hasLetter = true
		}
		if unicode.IsNumber(char) {
			hasNumber = true
		}
		if hasLetter && hasNumber {
			break
		}
	}

	if !hasLetter || !hasNumber {
		return ValidationError{
			Field:   "password",
			Message: "must contain at least one letter and one number",
		}
	}

	return nil
}

// ValidateEmail validates email format
func ValidateEmail(email string) error {
	email = SanitizeInput(email)

	if len(email) == 0 {
		return ValidationError{
			Field:   "email",
			Message: "is required",
		}
	}

	if len(email) > MaxEmailLength {
		return ValidationError{
			Field:   "email",
			Message: fmt.Sprintf("must be no more than %d characters long", MaxEmailLength),
			Value:   email,
		}
	}

	if !emailRegex.MatchString(email) {
		return ValidationError{
			Field:   "email",
			Message: "must be a valid email address",
			Value:   email,
		}
	}

	return nil
}

// ValidatePackageName validates package name according to npm and UPM rules
func ValidatePackageName(name string) error {
	name = SanitizeInput(name)

	if len(name) == 0 {
		return ValidationError{
			Field:   "package name",
			Message: "is required",
		}
	}

	if len(name) > MaxPackageNameLength {
		return ValidationError{
			Field:   "package name",
			Message: fmt.Sprintf("must be no more than %d characters long", MaxPackageNameLength),
			Value:   name,
		}
	}

	// Check if it's a reverse-DNS name (UPM style)
	if strings.Contains(name, ".") {
		if !upmPackageNameRegex.MatchString(name) {
			return ValidationError{
				Field:   "package name",
				Message: "reverse-DNS names must use lowercase letters, numbers, and hyphens, separated by dots",
				Value:   name,
			}
		}

		// Validate each segment
		segments := strings.Split(name, ".")
		if len(segments) < 2 {
			return ValidationError{
				Field:   "package name",
				Message: "reverse-DNS names must have at least two segments (e.g., com.company.package)",
				Value:   name,
			}
		}

		for _, segment := range segments {
			if len(segment) == 0 {
				return ValidationError{
					Field:   "package name",
					Message: "cannot have empty segments",
					Value:   name,
				}
			}
		}
	} else {
		// npm-style name
		if !npmPackageNameRegex.MatchString(name) {
			return ValidationError{
				Field:   "package name",
				Message: "must start with a letter or number and contain only letters, numbers, dots, underscores, and hyphens",
				Value:   name,
			}
		}
	}

	return nil
}

// ValidateVersion validates semantic version format
func ValidateVersion(version string) error {
	version = SanitizeInput(version)

	if len(version) == 0 {
		return ValidationError{
			Field:   "version",
			Message: "is required",
		}
	}

	if !semverRegex.MatchString(version) {
		return ValidationError{
			Field:   "version",
			Message: "must be a valid semantic version (e.g., 1.0.0, 2.1.0-beta.1)",
			Value:   version,
		}
	}

	return nil
}

// ValidateURL validates URL format
func ValidateURL(urlStr string) error {
	urlStr = SanitizeInput(urlStr)

	if len(urlStr) == 0 {
		return ValidationError{
			Field:   "URL",
			Message: "is required",
		}
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ValidationError{
			Field:   "URL",
			Message: "must be a valid URL",
			Value:   urlStr,
		}
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return ValidationError{
			Field:   "URL",
			Message: "must use http or https protocol",
			Value:   urlStr,
		}
	}

	if parsedURL.Host == "" {
		return ValidationError{
			Field:   "URL",
			Message: "must have a valid hostname",
			Value:   urlStr,
		}
	}

	return nil
}

// ValidateUserType validates user type input
func ValidateUserType(userType string) error {
	userType = SanitizeInput(userType)
	userType = strings.ToLower(userType)

	validTypes := []string{"user", "studio"}
	for _, validType := range validTypes {
		if userType == validType {
			return nil
		}
	}

	return ValidationError{
		Field:   "user type",
		Message: "must be either 'user' or 'studio'",
		Value:   userType,
	}
}

// ValidateSearchTerm validates search query input
func ValidateSearchTerm(term string) error {
	term = SanitizeInput(term)

	if len(term) == 0 {
		return ValidationError{
			Field:   "search term",
			Message: "is required",
		}
	}

	if len(term) < 2 {
		return ValidationError{
			Field:   "search term",
			Message: "must be at least 2 characters long",
			Value:   term,
		}
	}

	if len(term) > 100 {
		return ValidationError{
			Field:   "search term",
			Message: "must be no more than 100 characters long",
			Value:   term,
		}
	}

	return nil
}

// ValidateLimit validates numeric limit parameters
func ValidateLimit(limit int, fieldName string, min, max int) error {
	if limit < min {
		return ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must be at least %d", min),
			Value:   fmt.Sprintf("%d", limit),
		}
	}

	if limit > max {
		return ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must be no more than %d", max),
			Value:   fmt.Sprintf("%d", limit),
		}
	}

	return nil
}
