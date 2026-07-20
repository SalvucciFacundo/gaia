package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gaia/internal/core/domain"
	"gopkg.in/yaml.v3"
)

const (
	configDir  = ".config/gaia"
	configFile = "config.yaml"
)

// Load reads the configuration from ~/.config/gaia/config.yaml
func Load() (*domain.Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not get home directory: %w", err)
	}

	fullDir := filepath.Join(home, configDir)
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
			APIKeys: make(map[string]string),
		}
		defaultCfg.System.RequiresConfirmation = true
		
		data, err := yaml.Marshal(defaultCfg)
		if err != nil {
			return nil, fmt.Errorf("could not marshal default config: %w", err)
		}

		if err := ioutil.WriteFile(fullPath, data, 0600); err != nil {
			return nil, fmt.Errorf("could not write default config: %w", err)
		}
		return defaultCfg, nil
	}

	// Read existing config
	data, err := ioutil.ReadFile(fullPath)
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

	return &cfg, nil
}

// Save writes the configuration back to ~/.config/gaia/config.yaml
func Save(cfg *domain.Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not get home directory: %w", err)
	}

	fullPath := filepath.Join(home, configDir, configFile)
	
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("could not marshal config: %w", err)
	}

	return ioutil.WriteFile(fullPath, data, 0600)
}
