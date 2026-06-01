// Package config loads worker runtime config from the environment.
package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

// Config is the validated worker config.
type Config struct {
	Env      string
	RedisURL string

	// Asynq
	Queues      map[string]int
	Concurrency int

	// MarkItDown
	MarkItDownBin     string
	MarkItDownTimeout time.Duration

	// Extraction
	EnabledMimeTypes []string
}

// Load returns the validated config.
func Load() (Config, error) {
	cfg := Config{
		Env:      getOr("GO_ENV", "development"),
		RedisURL: getOr("REDIS_URL", "redis://localhost:6379/0"),

		// Default to one queue, all weight on it. We split priority
		// queues in M5+ when extraction priorities matter.
		Queues:      map[string]int{"default": 1},
		Concurrency: parseIntOr("WORKER_CONCURRENCY", 4),

		MarkItDownBin:     getOr("MARKITDOWN_BIN", "markitdown"),
		MarkItDownTimeout: parseDurationOr("MARKITDOWN_TIMEOUT", 5*time.Minute),

		EnabledMimeTypes: []string{getOr("EXTRACTION_ENABLED_FORMATS", "application/pdf")},
	}
	if cfg.Concurrency < 1 {
		return Config{}, errors.New("WORKER_CONCURRENCY must be >= 1")
	}
	if cfg.MarkItDownTimeout <= 0 {
		return Config{}, errors.New("MARKITDOWN_TIMEOUT must be positive")
	}
	return cfg, nil
}

func getOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func parseDurationOr(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
