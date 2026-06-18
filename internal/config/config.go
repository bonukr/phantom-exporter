// Package config loads runtime configuration from environment variables.
package config

import (
	"os"
	"strings"
)

// Config holds all runtime settings sourced from environment variables.
type Config struct {
	Port     string
	LogFile  string
	LogLevel string

	// SettingsDir is the directory holding one YAML file per metric group.
	SettingsDir string
}

// Load reads configuration from the environment, applying sensible defaults.
func Load() *Config {
	return &Config{
		Port:        env("H2H_PORT", "8080"),
		LogFile:     env("H2H_LOG_FILE", "./logs/phantom-exporter.log"),
		LogLevel:    env("H2H_LOG_LEVEL", "info"),
		SettingsDir: env("H2H_SETTINGS_DIR", "./settings"),
	}
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}
