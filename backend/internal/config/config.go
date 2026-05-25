// Package config loads mountabo's runtime configuration from the environment
// (12-factor style) and returns a concrete *Config. Secrets such as the GitHub
// client secret are read here and passed inward; they are never logged.
package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
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
// It first loads the repository-root .env (shared with the frontend) if present.
func Load() *Config {
	loadDotenv()
	return &Config{
		HTTPAddr:        env("MOUNTABO_HTTP_ADDR", "127.0.0.1:7778"),
		ShutdownTimeout: 10 * time.Second,
		GitHub: GitHubConfig{
			ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
			ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		},
	}
}

// loadDotenv loads the repository-root .env if one exists, searching upward from
// the working directory so it is found whether the binary runs from the repo
// root or from backend/. Real environment variables always win — godotenv.Load
// never overrides values already set. A shipped binary with no .env simply uses
// the OS environment.
func loadDotenv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for {
		path := filepath.Join(dir, ".env")
		if _, statErr := os.Stat(path); statErr == nil {
			if loadErr := godotenv.Load(path); loadErr != nil {
				slog.Warn("loading .env", "path", path, "err", loadErr)
			}
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return // reached the filesystem root without finding a .env
		}
		dir = parent
	}
}

// env returns the value of key, or fallback when key is unset or empty.
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
