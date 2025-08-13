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

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a new user account",
	Long: `Register a new user account in the GPM registry.
This will create a new user with the specified username and password.`,
	RunE: register,
}

func register(cmd *cobra.Command, args []string) error {
	cfg := config.GetConfig()
	reader := bufio.NewReader(os.Stdin)

	fmt.Println(styling.Header("🔐  User Registration"))
	fmt.Println(styling.Separator())

	fmt.Print(styling.Label("Username: "))
	username, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read username: %w\n\n%s", err, styling.Hint("Try running the command again"))
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

	if err := validatePassword(passwordBytes); err != nil {
		return fmt.Errorf("%s\n\n%s", styling.Error(err.Error()), styling.Hint("Use a strong password with at least 8 characters, including letters and numbers"))
	}

	fmt.Print(styling.Label("Confirm Password: "))
	confirmPasswordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read confirm password: %w\n\n%s", err, styling.Hint("Make sure your terminal supports hidden input"))
	}
	fmt.Println()

	if string(passwordBytes) != string(confirmPasswordBytes) {
		return fmt.Errorf("%s\n\n%s", styling.Error("passwords do not match"), styling.Hint("Please ensure both password entries are identical"))
	}

	passwordStr := string(passwordBytes)
	defer func() {
		for i := range passwordBytes {
			passwordBytes[i] = 0
		}
		for i := range confirmPasswordBytes {
			confirmPasswordBytes[i] = 0
		}
		passwordStr = ""
	}()

	fmt.Println(styling.Info("Creating account..."))

	client := api.NewClient(cfg.Registry, "")

	req := &api.RegisterRequest{
		Name:     username,
		Password: passwordStr,
		Type:     "user",
	}

	resp, err := client.Register(req)
	if err != nil {
		return handleRegisterError(err)
	}

	fmt.Println(styling.Separator())
	fmt.Println(styling.Success("✓ Account created successfully!"))
	fmt.Printf("%s %s\n", styling.Label("Username:"), styling.Value(resp.User.Username))
	fmt.Printf("%s %s\n", styling.Label("Registry:"), styling.Value(cfg.Registry))
	fmt.Printf("%s %s\n", styling.Label("Next step:"), styling.Command("gpm login"))
	fmt.Println(styling.Separator())

	return nil
}

func validatePassword(password []byte) error {
	return validation.ValidatePassword(password)
}

func handleRegisterError(err error) error {
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "409") || strings.Contains(errStr, "conflict") || strings.Contains(errStr, "exists"):
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Registration failed: Username already exists"),
			styling.Hint("Try a different username or use 'gpm login' if you already have an account."))
	case strings.Contains(errStr, "400") || strings.Contains(errStr, "bad request"):
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Registration failed: Invalid user data"),
			styling.Hint("Check that your username follows the required format and try again."))
	case strings.Contains(errStr, "403") || strings.Contains(errStr, "forbidden"):
		return fmt.Errorf("%s\n\n%s",
			styling.Error("Registration failed: Registration may be disabled"),
			styling.Hint("Contact your administrator for account creation assistance."))
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
			styling.Error(fmt.Sprintf("Registration failed: %v", err)),
			styling.Hint("Run with 'gpm --verbose register' for detailed error information."))
	}
}
