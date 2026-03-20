package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Tests  []TestCase   `yaml:"tests"`
}

type ServerConfig struct {
	URL     string        `yaml:"url"`
	Timeout time.Duration `yaml:"timeout"`
}

type TestCase struct {
	Name        string                 `yaml:"name"`
	Method      string                 `yaml:"method"`
	Params      map[string]interface{} `yaml:"params"`
	Expect      map[string]interface{} `yaml:"expect"`
	Session     bool                   `yaml:"session"`
	RequireAuth bool                   `yaml:"require_auth"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.Server.URL == "" {
		cfg.Server.URL = "http://localhost:4000/mcp"
	}
	if cfg.Server.Timeout == 0 {
		cfg.Server.Timeout = 10 * time.Second
	}

	// Default Session to true if not specified (for backwards compatibility)
	for i := range cfg.Tests {
		if !cfg.Tests[i].Session {
			cfg.Tests[i].Session = true
		}
	}

	return &cfg, nil
}
