// Package config loads mountabo's runtime configuration from the environment
// (12-factor style) and returns a concrete *Config. Secrets such as the GitHub
// client secret are read here and passed inward; they are never logged.
package config

import (
	"os"
	"time"
)

// Config is mountabo's resolved runtime configuration.
type Config struct {
	// HTTPAddr is the local address the API listens on. It binds to loopback by
	// default: mountabo is a local-first tool and must not be reachable off-box.
	HTTPAddr        string
	ShutdownTimeout time.Duration
	GitHub          GitHubConfig
}

// GitHubConfig holds the GitHub App OAuth credentials used to exchange
// authorization codes for user-to-server tokens.
type GitHubConfig struct {
	ClientID     string
	ClientSecret string
}

// Load reads configuration from the environment, applying local-first defaults.
func Load() *Config {
	return &Config{
		HTTPAddr:        env("MOUNTABO_HTTP_ADDR", "127.0.0.1:7777"),
		ShutdownTimeout: 10 * time.Second,
		GitHub: GitHubConfig{
			ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
			ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		},
	}
}

// env returns the value of key, or fallback when key is unset or empty.
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
