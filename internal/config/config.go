package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Registry string `mapstructure:"registry"`
	Token    string `mapstructure:"token"`
	Username string `mapstructure:"username"`
}

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

var config *Config

func InitConfig() {
	// Set default values
	viper.SetDefault("registry", "https://registry.gpm.sh")
	viper.SetDefault("token", "")
	viper.SetDefault("username", "")

	// Set config file name and type
	viper.SetConfigName(".gpmrc")
	viper.SetConfigType("yaml")

	// Set config file location to user's home directory
	// Check HOME environment variable first (for tests)
	home := os.Getenv("HOME")
	if home == "" {
		// Fall back to os.UserHomeDir() if HOME is not set
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			// If we can't get home dir, use current directory
			viper.AddConfigPath(".")
		} else {
			viper.AddConfigPath(home)
		}
	} else {
		viper.AddConfigPath(home)
	}

	// Always initialize config struct
	config = &Config{}

	// Try to read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Only log non-ConfigFileNotFound errors
			fmt.Printf("Warning: Error reading config file: %v\n", err)
		}
		// Continue with defaults even if config file is not found
	}

	// Unmarshal config into struct
	if err := viper.Unmarshal(config); err != nil {
		fmt.Printf("Warning: Error unmarshaling config: %v\n", err)
		// Continue with defaults if unmarshaling fails
	}

}

func GetConfig() *Config {
	if config == nil {
		InitConfig()
	}

	// Ensure config is never nil
	if config == nil {
		// Fallback to default config if InitConfig fails
		config = &Config{
			Registry: "https://registry.gpm.sh",
			Token:    "",
			Username: "",
		}
	}

	return config
}

func SaveConfig() error {
	cfg := GetConfig()

	if err := validateConfig(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	viper.Set("registry", cfg.Registry)
	viper.Set("token", cfg.Token)
	viper.Set("username", cfg.Username)

	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		configFile = home + "/.gpmrc"
	}

	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	if err := os.Chmod(configFile, 0600); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	return nil
}

func SetRegistry(registry string) {
	cfg := GetConfig()
	cfg.Registry = registry
}

func SetToken(token string) {
	cfg := GetConfig()
	cfg.Token = token
}

func SetUsername(username string) {
	cfg := GetConfig()
	cfg.Username = username
}

func ResetAuthData() {
	cfg := GetConfig()
	cfg.Token = ""
	cfg.Username = ""
}

func GetRegistry() string {
	cfg := GetConfig()
	return cfg.Registry
}

func GetToken() string {
	cfg := GetConfig()
	return cfg.Token
}

func GetUsername() string {
	cfg := GetConfig()
	return cfg.Username
}

// SetConfigForTesting allows tests to override the global config
func SetConfigForTesting(testConfig *Config) {
	config = testConfig
}

// ResetConfigForTesting resets the global config to nil for testing
func ResetConfigForTesting() {
	config = nil
}

func validateConfig(cfg *Config) error {
	if cfg.Registry != "" {
		if _, err := url.Parse(cfg.Registry); err != nil {
			return ValidationError{Field: "registry", Message: "invalid URL format"}
		}
		if !strings.HasPrefix(cfg.Registry, "http://") && !strings.HasPrefix(cfg.Registry, "https://") {
			return ValidationError{Field: "registry", Message: "registry URL must use http or https"}
		}
	}

	if cfg.Username != "" {
		if len(cfg.Username) < 3 || len(cfg.Username) > 50 {
			return ValidationError{Field: "username", Message: "username must be between 3 and 50 characters"}
		}
		if matched, _ := regexp.MatchString(`^[a-zA-Z0-9._-]+$`, cfg.Username); !matched {
			return ValidationError{Field: "username", Message: "username can only contain letters, numbers, dots, underscores, and hyphens"}
		}
	}

	return nil
}
