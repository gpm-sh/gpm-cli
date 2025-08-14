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
	webLogin bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to GPM registry",
	Long:  `Login to the GPM registry with your credentials`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if webLogin {
			return loginWeb()
		}
		return loginCLI()
	},
}

func init() {
	loginCmd.Flags().BoolVarP(&webLogin, "web", "w", false, "Login via web browser")
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

func loginWeb() error {
	cfg := config.GetConfig()
	client := api.NewClient(cfg.Registry, "")

	fmt.Println(styling.Header("🌐  Web Login"))
	fmt.Println(styling.Separator())
	fmt.Println(styling.Info("Opening browser for authentication..."))

	// Request login session from server
	loginSession, err := client.StartWebLogin()
	if err != nil {
		return fmt.Errorf("failed to start web login: %w\n\n%s", err, styling.Hint("Make sure the registry supports web authentication"))
	}

	// Open browser
	loginURL := fmt.Sprintf("%s/login/cli/%s", cfg.Registry, loginSession.SessionID)
	fmt.Printf("\n%s %s\n", styling.Label("Login at:"), styling.Value(loginURL))

	if err := openBrowser(loginURL); err != nil {
		fmt.Printf("\n%s\n", styling.Warning("⚠️  Could not open browser automatically"))
		fmt.Printf("%s %s\n\n", styling.Hint("Please open this URL manually:"), styling.Command(loginURL))
	} else {
		fmt.Println(styling.Success("✓ Browser opened"))
	}

	fmt.Println(styling.Info("Waiting for authentication..."))
	fmt.Println(styling.Hint("Complete the login in your browser, then return here"))

	// Poll for completion
	token, username, err := pollForToken(client, loginSession.SessionID)
	if err != nil {
		return fmt.Errorf("authentication failed: %w\n\n%s", err, styling.Hint("Try again or use 'gpm login' for CLI authentication"))
	}

	// Save authentication
	config.ResetAuthData()
	config.SetToken(token)
	config.SetUsername(username)

	if err := config.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save configuration: %w\n\n%s", err, styling.Hint("Check file permissions in your home directory"))
	}

	fmt.Println(styling.Separator())
	fmt.Println(styling.Success("✓ Login successful!"))
	fmt.Printf("%s %s\n", styling.Label("Registry:"), styling.Value(cfg.Registry))
	fmt.Printf("%s %s\n", styling.Label("Username:"), styling.Value(username))
	fmt.Printf("%s %s\n", styling.Label("Next step:"), styling.Command("gpm publish <package>"))
	fmt.Println(styling.Separator())

	return nil
}

func openBrowser(url string) error {
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
	return exec.Command(cmd, args...).Start()
}

func pollForToken(client *api.Client, sessionID string) (string, string, error) {
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return "", "", fmt.Errorf("authentication timeout - please try again")
		case <-ticker.C:
			result, err := client.CheckWebLogin(sessionID)
			if err != nil {
				continue // Keep polling on errors
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
