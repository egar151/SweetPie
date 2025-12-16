package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// Load reads and parses the configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expanded := expandEnvVars(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Parse durations and decode secrets
	if err := parseConfig(&cfg); err != nil {
		return nil, fmt.Errorf("failed to process config: %w", err)
	}

	return &cfg, nil
}

// expandEnvVars replaces ${VAR} patterns with environment variable values.
func expandEnvVars(content string) string {
	return envVarPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract variable name from ${VAR}
		varName := match[2 : len(match)-1]
		if value, exists := os.LookupEnv(varName); exists {
			return value
		}
		return match // Keep original if not found
	})
}

// parseConfig processes the loaded configuration.
func parseConfig(cfg *Config) error {
	// Parse global durations
	if cfg.Global.PollingInterval != "" {
		d, err := time.ParseDuration(cfg.Global.PollingInterval)
		if err != nil {
			return fmt.Errorf("invalid polling_interval: %w", err)
		}
		cfg.Global.PollingIntervalDuration = d
	}

	if cfg.Global.StartupRetryInterval != "" {
		d, err := time.ParseDuration(cfg.Global.StartupRetryInterval)
		if err != nil {
			return fmt.Errorf("invalid startup_retry_interval: %w", err)
		}
		cfg.Global.StartupRetryIntervalDuration = d
	}

	// Parse connection settings
	for name, conn := range cfg.Connections {
		if conn.Timeout != "" {
			d, err := time.ParseDuration(conn.Timeout)
			if err != nil {
				return fmt.Errorf("connection %s: invalid timeout: %w", name, err)
			}
			conn.TimeoutDuration = d
		}

		if conn.KeepaliveInterval != "" {
			d, err := time.ParseDuration(conn.KeepaliveInterval)
			if err != nil {
				return fmt.Errorf("connection %s: invalid keepalive_interval: %w", name, err)
			}
			conn.KeepaliveIntervalDuration = d
		}

		// Decode password
		if conn.Password != "" {
			decoded, err := decodeSecret(conn.Password)
			if err != nil {
				return fmt.Errorf("connection %s: failed to decode password: %w", name, err)
			}
			conn.DecodedPassword = decoded
		}

		// Decode key passphrase
		if conn.KeyPassphrase != "" {
			decoded, err := decodeSecret(conn.KeyPassphrase)
			if err != nil {
				return fmt.Errorf("connection %s: failed to decode key_passphrase: %w", name, err)
			}
			conn.DecodedKeyPassphrase = decoded
		}

		cfg.Connections[name] = conn
	}

	// Parse rule settings
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		if rule.RetryDelay != "" {
			d, err := time.ParseDuration(rule.RetryDelay)
			if err != nil {
				return fmt.Errorf("rule %s: invalid retry_delay: %w", rule.Name, err)
			}
			rule.RetryDelayDuration = d
		}
	}

	return nil
}

// decodeSecret decodes a secret value that may be base64 encoded.
// Format: "base64:xxxxx" for base64 encoded, or plain text otherwise.
func decodeSecret(value string) (string, error) {
	if strings.HasPrefix(value, "base64:") {
		encoded := strings.TrimPrefix(value, "base64:")
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return "", fmt.Errorf("invalid base64 encoding: %w", err)
		}
		return string(decoded), nil
	}
	return value, nil
}

// EncodeSecret encodes a plain text secret to base64 format.
// This is a utility function for generating config values.
func EncodeSecret(plain string) string {
	return "base64:" + base64.StdEncoding.EncodeToString([]byte(plain))
}
