package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gpm.sh/gpm/gpm-cli/internal/api"
	"gpm.sh/gpm/gpm-cli/internal/config"
	"gpm.sh/gpm/gpm-cli/internal/styling"
	"gpm.sh/gpm/gpm-cli/internal/validation"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to GPM registry",
	Long:  `Login to the GPM registry with your credentials`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return login()
	},
}

func login() error {
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

	config.SetToken(resp.Token)
	// Don't set username here - we'll get it from whoami if needed

	if err := config.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save configuration: %w\n\n%s", err, styling.Hint("Check file permissions in your home directory and try 'gpm config' to verify settings"))
	}

	fmt.Println(styling.Separator())
	fmt.Println(styling.Success("✓ Login successful!"))
	fmt.Printf("%s %s\n", styling.Label("Registry:"), styling.Value(cfg.Registry))
	fmt.Printf("%s %s\n", styling.Label("Next step:"), styling.Command("gpm publish <package>"))
	fmt.Println(styling.Separator())

	return nil
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
