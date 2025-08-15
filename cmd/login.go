package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gpm.sh/gpm/gpm-cli/internal/api"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/styling"
	"gpm.sh/gpm/gpm-cli/internal/validation"
)

var (
	authType string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to GPM registry",
	Long:  `Login to the GPM registry with your credentials`,
	RunE: func(cmd *cobra.Command, args []string) error {
		switch authType {
		case "web", "":
			return loginWeb()
		case "legacy":
			return loginCLI()
		default:
			return fmt.Errorf("invalid auth-type: %s (must be 'web' or 'legacy')", authType)
		}
	},
}

func init() {
	loginCmd.Flags().StringVar(&authType, "auth-type", "web", "Authentication type: 'web' (browser-based) or 'legacy' (username/password)")
}

func loginCLI() error {
	cfg := config.GetConfig()
	client := api.NewClient(cfg.Registry, "")

	reader := bufio.NewReader(os.Stdin)

	fmt.Println(styling.Header("🔐  User Login"))
	fmt.Println(styling.Separator())

	fmt.Print(styling.Label("Username: "))
	username, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read username: %w\n\n%s", err, styling.Hint("Try running the command again or check your terminal settings"))
	}
	username = strings.TrimSpace(username)

	if err := validateUsername(username); err != nil {
		return fmt.Errorf("%s\n\n%s", styling.Error(err.Error()), styling.Hint("Username must be 3-50 characters and contain only letters, numbers, dots, underscores, and hyphens"))
	}

	fmt.Print(styling.Label("Password: "))
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read password: %w\n\n%s", err, styling.Hint("Make sure your terminal supports hidden input"))
	}
	fmt.Println()

	if len(passwordBytes) == 0 {
		return fmt.Errorf("%s\n\n%s", styling.Error("password is required"), styling.Hint("Please enter your password when prompted"))
	}

	passwordStr := string(passwordBytes)
	defer func() {
		for i := range passwordBytes {
			passwordBytes[i] = 0
		}
		passwordStr = ""
	}()

	fmt.Println(styling.Info("Authenticating..."))

	req := &api.LoginRequest{
		Name:     username,
		Password: passwordStr,
	}

	resp, err := client.Login(req)
	if err != nil {
		return handleLoginError(err)
	}

	// Reset all auth data before setting new token
	config.ResetAuthData()
	config.SetToken(resp.Token)

	// Fetch fresh user info with the new token
	userClient := api.NewClient(cfg.Registry, resp.Token)
	whoamiResp, err := userClient.Whoami()
	if err == nil {
		// Only set username if we successfully got fresh info
		config.SetUsername(whoamiResp.Username)
	}

	if err := config.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save configuration: %w\n\n%s", err, styling.Hint("Check file permissions in your home directory and try 'gpm config' to verify settings"))
	}

	fmt.Println(styling.Separator())
	fmt.Println(styling.Success("✓ Login successful!"))
	fmt.Printf("%s %s\n", styling.Label("Registry:"), styling.Value(cfg.Registry))
	if whoamiResp != nil {
		fmt.Printf("%s %s\n", styling.Label("Username:"), styling.Value(whoamiResp.Username))
	}
	fmt.Printf("%s %s\n", styling.Label("Next step:"), styling.Command("gpm publish <package>"))
	fmt.Println(styling.Separator())

	return nil
}

func openBrowser(url string) error {
	// Validate URL to prevent command injection
	if url == "" || len(url) > 2048 {
		return fmt.Errorf("invalid URL")
	}
	// Basic validation to ensure it's a proper HTTP(S) URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("invalid URL scheme")
	}

	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start() // #nosec G204 - URL validated above
}

func pollForToken(client *api.Client, sessionID string) (string, string, error) {
	timeout := time.After(10 * time.Minute)   // More realistic timeout like NPM
	ticker := time.NewTicker(5 * time.Second) // Less aggressive polling
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return "", "", fmt.Errorf("authentication timeout - please try again")
		case <-ticker.C:
			result, err := client.CheckWebLogin(sessionID)
			if err != nil {
				// Check if session expired or not found
				if strings.Contains(err.Error(), "session_expired") {
					return "", "", fmt.Errorf("authentication session expired - please try again")
				}
				if strings.Contains(err.Error(), "session_not_found") {
					return "", "", fmt.Errorf("authentication session not found - please try again")
				}
				continue // Keep polling on other errors
			}
			if result.Completed {
				return result.Token, result.Username, nil
			}
		}
	}
}

func validateUsername(username string) error {
	return validation.ValidateUsername(username)
}

func handleLoginError(err error) error {
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized"):
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Authentication failed: Invalid username or password"),
			styling.Hint("Double-check your credentials and try again. Create an account at the web dashboard if needed."))
	case strings.Contains(errStr, "403") || strings.Contains(errStr, "forbidden"):
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Access denied: Account may be suspended or you lack permissions"),
			styling.Hint("Contact your administrator or check your account status."))
	case strings.Contains(errStr, "404"):
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Registry not found or user doesn't exist"),
			styling.Hint("Check the registry URL with 'gpm config get registry' or create an account at the web dashboard."))
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection"):
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Network error: Unable to connect to registry"),
			styling.Hint("Check your internet connection and registry URL. Try 'gpm config set registry <url>' to update."))
	case strings.Contains(errStr, "500"):
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Server error: Registry is experiencing issues"),
			styling.Hint("Try again in a few minutes. If the problem persists, contact support."))
	default:
		return fmt.Errorf("%s\n\n%s",
			styling.Error(fmt.Sprintf("Login failed: %v", err)),
			styling.Hint("Run with 'gpm --verbose login' for detailed error information."))
	}
}

// NPM-style web authentication (simple browser login)
func loginWeb() error {
	cfg := config.GetConfig()
	client := api.NewClient(cfg.Registry, "")

	fmt.Println(styling.Header("🌐 GPM Web Login"))
	fmt.Println(styling.SubHeader("Authenticating via web browser..."))

	// Start web login session
	fmt.Printf("%s Starting web login session...\n", styling.Info("ℹ"))
	webLoginResp, err := client.StartWebLogin()
	if err != nil {
		return fmt.Errorf("failed to start web login: %w", err)
	}

	fmt.Printf("%s %s\n", styling.Success("✓"), "Web login session created")
	fmt.Printf("%s Opening browser to authenticate...\n", styling.Info("ℹ"))

	// Open browser to login URL
	if err := openBrowser(webLoginResp.LoginURL); err != nil {
		fmt.Printf("%s\n", styling.Warning("⚠ Could not open browser automatically"))
		fmt.Printf("%s\n\n", styling.Muted("Please manually open the following URL in your browser:"))
		fmt.Printf("%s\n\n", styling.URL(webLoginResp.LoginURL))
	}

	// Poll for completion
	fmt.Printf("%s Waiting for authentication...\n", styling.Info("⏳"))
	token, username, err := pollForToken(client, webLoginResp.SessionID)
	if err != nil {
		return fmt.Errorf("web authentication failed: %w", err)
	}

	// Save token and username to config
	cfg.Token = token
	cfg.Username = username

	if err := config.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("\n%s\n", styling.Success("🎉 Web login successful!"))
	fmt.Printf("%s %s\n", styling.Label("Logged in as:"), styling.MakeBold(username))
	fmt.Printf("%s %s\n", styling.Label("Registry:"), styling.Muted(cfg.Registry))

	return nil
}
