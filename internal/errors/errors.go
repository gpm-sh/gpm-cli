package errors

import (
	"fmt"
	"strings"
)

type GPMError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func (e *GPMError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s: %s. %s", e.Code, e.Message, e.Hint)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *GPMError) JSON() map[string]interface{} {
	result := map[string]interface{}{
		"code":    e.Code,
		"message": e.Message,
	}
	if e.Hint != "" {
		result["hint"] = e.Hint
	}
	return result
}

var (
	ErrNameScheme = func(name string) *GPMError {
		return &GPMError{
			Code:    "E_NAME_SCHEME",
			Message: fmt.Sprintf("Package name must be reverse-DNS (e.g., com.acme.foo). '%s' is not supported.", name),
			Hint:    "Rename your package.json \"name\" and retry.",
		}
	}

	ErrPlanRequired = func(plan, visibility string) *GPMError {
		return &GPMError{
			Code:    "E_PLAN_REQUIRED",
			Message: fmt.Sprintf("Your plan (%s) cannot publish %s packages.", plan, visibility),
			Hint:    "Upgrade to Pro to publish scoped-public or scoped-private.",
		}
	}

	ErrDupVersion = func(version, name string) *GPMError {
		return &GPMError{
			Code:    "E_DUP_VERSION",
			Message: fmt.Sprintf("Version %s already exists for %s.", version, name),
			Hint:    "Bump the version or use a new dist-tag.",
		}
	}

	ErrTarballInvalid = func() *GPMError {
		return &GPMError{
			Code:    "E_TARBALL_INVALID",
			Message: "Built tarball failed integrity check. Re-run without file exclusions or fix .npmignore.",
			Hint:    "Check your .npmignore file and ensure all required files are included.",
		}
	}

	ErrAuthRequired = func() *GPMError {
		return &GPMError{
			Code:    "E_AUTH_REQUIRED",
			Message: "Token is missing or invalid. Run 'gpm login' or pass --token.",
			Hint:    "Run 'gpm login' to authenticate or provide --token <your-token>.",
		}
	}

	ErrVisibilityInvalid = func(visibility string) *GPMError {
		return &GPMError{
			Code:    "E_VISIBILITY_INVALID",
			Message: fmt.Sprintf("Invalid visibility '%s'. Use one of: global-public, scoped-public, scoped-private.", visibility),
			Hint:    "Choose from: global-public, scoped-public, scoped-private.",
		}
	}

	ErrStudioUnknown = func(slug string) *GPMError {
		return &GPMError{
			Code:    "E_STUDIO_UNKNOWN",
			Message: fmt.Sprintf("Studio '%s' not found. Create it in the dashboard first.", slug),
			Hint:    "Visit the dashboard to create your studio or check the studio slug.",
		}
	}

	ErrStorageFailed = func(requestID string) *GPMError {
		return &GPMError{
			Code:    "E_STORAGE_FAILED",
			Message: "Artifact storage failed. Please retry later or contact support.",
			Hint:    fmt.Sprintf("Request ID: %s", requestID),
		}
	}

	ErrPackageJSONInvalid = func(field string) *GPMError {
		return &GPMError{
			Code:    "E_PACKAGE_JSON_INVALID",
			Message: fmt.Sprintf("Invalid package.json: %s field is required.", field),
			Hint:    "Ensure your package.json has all required fields.",
		}
	}

	ErrVersionInvalid = func(version string) *GPMError {
		return &GPMError{
			Code:    "E_VERSION_INVALID",
			Message: fmt.Sprintf("Invalid version format: %s", version),
			Hint:    "Use semantic versioning (e.g., 1.0.0, 2.1.3-beta.1).",
		}
	}

	ErrRegistryInvalid = func(registry string) *GPMError {
		return &GPMError{
			Code:    "E_REGISTRY_INVALID",
			Message: fmt.Sprintf("Invalid registry URL: %s", registry),
			Hint:    "Use a valid HTTPS URL (e.g., https://gpm.sh or https://studio.gpm.sh).",
		}
	}

	ErrNetworkFailed = func(err error) *GPMError {
		return &GPMError{
			Code:    "E_NETWORK_FAILED",
			Message: fmt.Sprintf("Network request failed: %v", err),
			Hint:    "Check your internet connection and try again.",
		}
	}
)

func IsGPMError(err error) bool {
	_, ok := err.(*GPMError)
	return ok
}

func FormatError(err error, jsonOutput bool) string {
	if gpmErr, ok := err.(*GPMError); ok {
		if jsonOutput {
			return fmt.Sprintf("{\"error\":%s}", gpmErr.JSON())
		}
		return gpmErr.Error()
	}

	if jsonOutput {
		return fmt.Sprintf("{\"error\":{\"code\":\"E_UNKNOWN\",\"message\":\"%s\"}}", err.Error())
	}
	return fmt.Sprintf("E_UNKNOWN: %s", err.Error())
}

func ValidateVisibility(visibility string) error {
	validVisibilities := []string{"global-public", "scoped-public", "scoped-private"}
	for _, v := range validVisibilities {
		if v == visibility {
			return nil
		}
	}
	return ErrVisibilityInvalid(visibility)
}

func ValidatePackageName(name string) error {
	if strings.HasPrefix(name, "@") {
		return ErrNameScheme(name)
	}

	if !strings.Contains(name, ".") {
		return ErrNameScheme(name)
	}

	return nil
}
