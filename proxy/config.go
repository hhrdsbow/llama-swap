package proxy

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the top-level configuration for llama-swap.
type Config struct {
	// LogLevel controls verbosity: "debug", "info", "warn", "error"
	LogLevel string `yaml:"logLevel"`

	// HealthCheckTimeout is how long to wait for a model process to become ready.
	HealthCheckTimeout Duration `yaml:"healthCheckTimeout"`

	// Models is a map of model name to model configuration.
	Models map[string]ModelConfig `yaml:"models"`

	// Groups allows aliasing multiple model names to a single logical name.
	Groups map[string]GroupConfig `yaml:"groups"`
}

// ModelConfig describes a single model backend process.
type ModelConfig struct {
	// Cmd is the full command (with arguments) used to launch the model server.
	Cmd string `yaml:"cmd"`

	// Proxy is the upstream address (e.g. "http://127.0.0.1:8080") to forward
	// requests to once the model process is running.
	Proxy string `yaml:"proxy"`

	// Env contains additional environment variables to pass to the process.
	Env []string `yaml:"env"`

	// CheckEndpoint overrides the default health-check path ("/health").
	CheckEndpoint string `yaml:"checkEndpoint"`

	// UnloadAfter is the idle duration before the model process is stopped.
	// Zero means the process is never unloaded automatically.
	UnloadAfter Duration `yaml:"unloadAfter"`

	// UseGPU marks this model as requiring a GPU slot (for mutual exclusion).
	UseGPU bool `yaml:"useGPU"`
}

// GroupConfig maps a logical group name to one or more model names.
type GroupConfig struct {
	// Members lists the model names that belong to this group.
	Members []string `yaml:"members"`

	// Swap controls whether only one member may be loaded at a time.
	Swap bool `yaml:"swap"`
}

// Duration is a yaml-deserializable wrapper around time.Duration.
type Duration struct {
	time.Duration
}

// UnmarshalYAML implements yaml.Unmarshaler for Duration so that values like
// "30s" or "5m" can be used directly in config files.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

// MarshalYAML implements yaml.Marshaler for Duration.
func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// LoadConfig reads and parses a YAML config file from the given path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// Validate performs basic sanity checks on the configuration.
func (c *Config) Validate() error {
	for name, model := range c.Models {
		if model.Cmd == "" {
			return fmt.Errorf("model %q: cmd must not be empty", name)
		}
		if model.Proxy == "" {
			return fmt.Errorf("model %q: proxy must not be empty", name)
		}
	}

	for groupName, group := range c.Groups {
		if len(group.Members) == 0 {
			return fmt.Errorf("group %q: members must not be empty", groupName)
		}
		for _, member := range group.Members {
			if _, ok := c.Models[member]; !ok {
				return fmt.Errorf("group %q: member %q is not a defined model", groupName, member)
			}
		}
	}

	return nil
}
