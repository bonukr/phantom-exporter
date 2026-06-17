// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds all runtime settings sourced from environment variables.
type Config struct {
	Port     string
	LogFile  string
	LogLevel string

	// Database
	DatabaseURL string // when set, takes precedence over the discrete DB_* fields
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	DBSSLMode   string
}

// Load reads configuration from the environment, applying sensible defaults.
func Load() *Config {
	return &Config{
		Port:     env("PORT", "8080"),
		LogFile:  env("LOG_FILE", "./logs/phantom-exporter.log"),
		LogLevel: env("LOG_LEVEL", "info"),

		DatabaseURL: env("DATABASE_URL", ""),
		DBHost:      env("DB_HOST", "localhost"),
		DBPort:      env("DB_PORT", "5432"),
		DBUser:      env("DB_USER", "phantom"),
		DBPassword:  env("DB_PASSWORD", "phantom"),
		DBName:      env("DB_NAME", "phantom"),
		DBSSLMode:   env("DB_SSLMODE", "disable"),
	}
}

// DSN builds a PostgreSQL connection string. DATABASE_URL wins if provided.
func (c *Config) DSN() string {
	if strings.TrimSpace(c.DatabaseURL) != "" {
		return c.DatabaseURL
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}
