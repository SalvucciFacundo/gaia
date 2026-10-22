package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"gaia/internal/core/domain"
	"gopkg.in/yaml.v3"
)

const (
	configDir  = ".config/gaia"
	configFile = "config.yaml"
)

var profileNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// BaseHome returns the root GAIA home directory ($GAIA_HOME or ~/.config/gaia).
func BaseHome() string {
	if home := os.Getenv("GAIA_HOME"); home != "" {
		return home
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return configDir
	}
	return filepath.Join(userHome, configDir)
}

// GaiaHome is an alias for BaseHome.
func GaiaHome() string {
	return BaseHome()
}

// ValidateProfileName validates that a profile name conforms to security and syntax constraints:
// - Length between 1 and 32 characters
// - Matches ^[a-zA-Z0-9_-]+$
func ValidateProfileName(name string) error {
	if len(name) < 1 {
		return fmt.Errorf("profile name cannot be empty")
	}
	if len(name) > 32 {
		return fmt.Errorf("profile name %q exceeds maximum length of 32 characters", name)
	}
	if !profileNameRegex.MatchString(name) {
		return fmt.Errorf("profile name %q contains invalid characters (must match %s)", name, profileNameRegex.String())
	}
	return nil
}

// Load reads the configuration from ~/.config/gaia/config.yaml
func Load() (*domain.Config, error) {
	fullDir := BaseHome()
	fullPath := filepath.Join(fullDir, configFile)

	// Create directory if it doesn't exist
	if _, err := os.Stat(fullDir); os.IsNotExist(err) {
		if err := os.MkdirAll(fullDir, 0700); err != nil {
			return nil, fmt.Errorf("could not create config directory: %w", err)
		}
	}

	// Create default config if it doesn't exist
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		defaultCfg := &domain.Config{
			APIKeys:        make(map[string]string),
			CurrentProfile: "assistant",
			Profiles: map[string]domain.ProfileConfig{
				"assistant": {Enabled: true},
				"dev":       {Enabled: false},
				"art":       {Enabled: false},
			},
			Browser: domain.BrowserConfig{
				Timeout:        30 * time.Second,
				ViewportWidth:  1280,
				ViewportHeight: 720,
			},
		}
		defaultCfg.System.RequiresConfirmation = true

		data, err := yaml.Marshal(defaultCfg)
		if err != nil {
			return nil, fmt.Errorf("could not marshal default config: %w", err)
		}

		if err := os.WriteFile(fullPath, data, 0600); err != nil {
			return nil, fmt.Errorf("could not write default config: %w", err)
		}
		return defaultCfg, nil
	}

	// Read existing config
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	var cfg domain.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("could not unmarshal config: %w", err)
	}

	// Apply defaults for new fields
	if cfg.Budget.MaxIterations <= 0 {
		cfg.Budget = domain.DefaultBudget()
	}
	if cfg.LLM.TrustMode == "" {
		cfg.LLM.TrustMode = string(domain.TrustAlways)
	}
	if cfg.LLM.FallbackChain == nil {
		cfg.LLM.FallbackChain = []string{}
	}
	if cfg.Terminal.Backend == "" {
		cfg.Terminal.Backend = "local"
	}
	if cfg.MCP.Servers == nil {
		cfg.MCP.Servers = []domain.MCPServerConfig{}
	}
	if cfg.CurrentProfile == "" {
		cfg.CurrentProfile = "assistant"
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]domain.ProfileConfig{
			"assistant": {Enabled: true},
			"dev":       {Enabled: false},
			"art":       {Enabled: false},
		}
	}

	// Apply Multimodal environment variables overrides
	if envProvider := os.Getenv("MULTIMODAL_PROVIDER"); envProvider != "" {
		cfg.Multimodal.Provider = envProvider
	}
	if envModel := os.Getenv("MULTIMODAL_MODEL"); envModel != "" {
		cfg.Multimodal.Model = envModel
	}
	if envKey := os.Getenv("MULTIMODAL_API_KEY"); envKey != "" {
		cfg.Multimodal.APIKey = envKey
	}

	// Set multimodal defaults if not provided
	if cfg.Multimodal.Provider == "" {
		cfg.Multimodal.Provider = "gemini"
	}
	if cfg.Multimodal.Model == "" {
		if cfg.Multimodal.Provider == "gemini" {
			cfg.Multimodal.Model = "gemini-2.5-flash"
		} else if cfg.Multimodal.Provider == "openai" {
			cfg.Multimodal.Model = "gpt-4o-mini"
		} else {
			cfg.Multimodal.Model = "claude-3-5-sonnet"
		}
	}

	// Apply Browser defaults
	if cfg.Browser.Timeout <= 0 {
		cfg.Browser.Timeout = 30 * time.Second
	}
	if cfg.Browser.ViewportWidth <= 0 {
		cfg.Browser.ViewportWidth = 1280
	}
	if cfg.Browser.ViewportHeight <= 0 {
		cfg.Browser.ViewportHeight = 720
	}

	return &cfg, nil
}

// Save writes the configuration back to ~/.config/gaia/config.yaml
func Save(cfg *domain.Config) error {
	fullPath := filepath.Join(BaseHome(), configFile)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("could not marshal config: %w", err)
	}

	return os.WriteFile(fullPath, data, 0600)
}

// Config type aliases for audio and gateway subsystems.
type TelegramConfig = domain.TelegramConfig
type DiscordConfig = domain.DiscordConfig
type SlackConfig = domain.SlackConfig
type WhatsAppConfig = domain.WhatsAppConfig
type TTSConfig = domain.TTSConfig
type AudioConfig = domain.AudioConfig
type VisionConfig = domain.VisionConfig
type GatewayConfig = domain.GatewayConfig
type BrowserConfig = domain.BrowserConfig
