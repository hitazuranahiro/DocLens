// Package config loads worker runtime config from the environment.
package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the validated worker config.
type Config struct {
	Env      string
	RedisURL string

	// Postgres
	DatabaseURL string

	// Object storage
	S3Endpoint        string
	S3Region          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3UsePathStyle    bool
	S3BucketRaw       string
	S3BucketArtifacts string

	// Asynq
	Queues      map[string]int
	Concurrency int

	// MarkItDown
	MarkItDownBin     string
	MarkItDownTimeout time.Duration

	// Thumbnailer
	ThumbnailerBin string

	// Extraction
	EnabledMimeTypes []string

	// Observability
	SentryDSN string
	Release   string

	// OTel collector endpoint. Empty disables tracing.
	OTelEndpoint   string
	OTelInsecure   bool
	OTelSampleRate float64
}

// Load returns the validated config.
func Load() (Config, error) {
	cfg := Config{
		Env:      getOr("GO_ENV", "development"),
		RedisURL: getOr("REDIS_URL", "redis://localhost:6379/0"),

		DatabaseURL: getOr("DATABASE_URL",
			"postgres://doclens:doclens@localhost:5432/doclens?sslmode=disable"),

		S3Endpoint:        getOr("S3_ENDPOINT", "http://localhost:9000"),
		S3Region:          getOr("S3_REGION", "us-east-1"),
		S3AccessKeyID:     getOr("S3_ACCESS_KEY", "doclens"),
		S3SecretAccessKey: getOr("S3_SECRET_KEY", "doclens-dev-secret"),
		S3UsePathStyle:    strings.EqualFold(getOr("S3_USE_PATH_STYLE", "true"), "true"),
		S3BucketRaw:       getOr("S3_BUCKET_RAW", "doclens-raw"),
		S3BucketArtifacts: getOr("S3_BUCKET_ARTIFACTS", "doclens-artifacts"),

		Queues:      map[string]int{"default": 1},
		Concurrency: parseIntOr("WORKER_CONCURRENCY", 4),

		MarkItDownBin:     getOr("MARKITDOWN_BIN", "markitdown"),
		MarkItDownTimeout: parseDurationOr("MARKITDOWN_TIMEOUT", 5*time.Minute),

		ThumbnailerBin: getOr("THUMBNAILER_BIN", "pdftoppm"),

		EnabledMimeTypes: parseCSV(getOr("EXTRACTION_ENABLED_FORMATS", "application/pdf")),

		SentryDSN: os.Getenv("SENTRY_DSN"),
		Release:   getOr("APP_RELEASE", ""),

		OTelEndpoint:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		OTelInsecure:   strings.EqualFold(getOr("OTEL_EXPORTER_OTLP_INSECURE", "true"), "true"),
		OTelSampleRate: parseFloatOr("OTEL_TRACES_SAMPLER_ARG", 0.1),
	}
	if cfg.Concurrency < 1 {
		return Config{}, errors.New("WORKER_CONCURRENCY must be >= 1")
	}
	if cfg.MarkItDownTimeout <= 0 {
		return Config{}, errors.New("MARKITDOWN_TIMEOUT must be positive")
	}
	if len(cfg.EnabledMimeTypes) == 0 {
		return Config{}, errors.New("EXTRACTION_ENABLED_FORMATS must list at least one MIME type")
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

func parseCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, strings.ToLower(p))
		}
	}
	return out
}


func parseFloatOr(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}
