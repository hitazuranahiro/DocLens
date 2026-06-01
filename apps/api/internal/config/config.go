// Package config loads and validates the API's runtime configuration.
//
// Configuration is sourced from environment variables. Defaults are
// developer-friendly; production deployments must set every variable.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Provider identifies the configured authentication adapter.
type Provider string

const (
	ProviderLocal Provider = "local"
	ProviderClerk Provider = "clerk"
)

// Config is the validated runtime configuration.
type Config struct {
	Env          string
	HTTPAddr     string
	AuthProvider Provider
	ClerkIssuer  string
	ClerkAud     string

	// Postgres
	DatabaseURL string

	// Redis (asynq broker, future cache)
	RedisURL string

	// Object storage
	S3Endpoint        string
	S3Region          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3UsePathStyle    bool
	S3BucketRaw       string
	S3BucketArtifacts string

	// Upload knobs
	EnabledMimeTypes []string
	UploadPresignTTL time.Duration

	// Sweep cron
	SweepInterval     time.Duration
	UploadSweepWindow time.Duration
}

// Load reads configuration from the environment and returns it validated.
func Load() (Config, error) {
	cfg := Config{
		Env:          getOr("GO_ENV", "development"),
		HTTPAddr:     getOr("HTTP_ADDR", ":8080"),
		AuthProvider: Provider(strings.ToLower(getOr("AUTH_PROVIDER", "local"))),
		ClerkIssuer:  os.Getenv("CLERK_ISSUER"),
		ClerkAud:     os.Getenv("CLERK_AUDIENCE"),

		DatabaseURL: getOr("DATABASE_URL",
			"postgres://doclens:doclens@localhost:5432/doclens?sslmode=disable"),

		RedisURL: getOr("REDIS_URL", "redis://localhost:6379/0"),

		S3Endpoint:        getOr("S3_ENDPOINT", "http://localhost:9000"),
		S3Region:          getOr("S3_REGION", "us-east-1"),
		S3AccessKeyID:     getOr("S3_ACCESS_KEY", "doclens"),
		S3SecretAccessKey: getOr("S3_SECRET_KEY", "doclens-dev-secret"),
		S3UsePathStyle:    strings.EqualFold(getOr("S3_USE_PATH_STYLE", "true"), "true"),
		S3BucketRaw:       getOr("S3_BUCKET_RAW", "doclens-raw"),
		S3BucketArtifacts: getOr("S3_BUCKET_ARTIFACTS", "doclens-artifacts"),

		EnabledMimeTypes: parseCSV(getOr("EXTRACTION_ENABLED_FORMATS", "application/pdf")),
		UploadPresignTTL: parseDurationOr("UPLOAD_PRESIGN_TTL", 5*time.Minute),

		SweepInterval:     parseDurationOr("UPLOAD_SWEEP_INTERVAL", 15*time.Minute),
		UploadSweepWindow: parseDurationOr("UPLOAD_SWEEP_WINDOW", 24*time.Hour),
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	switch c.AuthProvider {
	case ProviderLocal:
		if c.Env == "production" {
			return errors.New("AUTH_PROVIDER=local is not allowed when GO_ENV=production")
		}
	case ProviderClerk:
		if c.ClerkIssuer == "" {
			return errors.New("CLERK_ISSUER is required when AUTH_PROVIDER=clerk")
		}
	default:
		return fmt.Errorf("unsupported AUTH_PROVIDER=%q", c.AuthProvider)
	}
	if c.S3BucketRaw == "" {
		return errors.New("S3_BUCKET_RAW is required")
	}
	if c.S3BucketArtifacts == "" {
		return errors.New("S3_BUCKET_ARTIFACTS is required")
	}
	if len(c.EnabledMimeTypes) == 0 {
		return errors.New("EXTRACTION_ENABLED_FORMATS must list at least one MIME type")
	}
	if c.UploadPresignTTL <= 0 {
		return errors.New("UPLOAD_PRESIGN_TTL must be positive")
	}
	return nil
}

func getOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
