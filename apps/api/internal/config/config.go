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
}

// Load reads configuration from the environment and returns it validated.
func Load() (Config, error) {
	cfg := Config{
		Env:          getOr("GO_ENV", "development"),
		HTTPAddr:     getOr("HTTP_ADDR", ":8080"),
		AuthProvider: Provider(strings.ToLower(getOr("AUTH_PROVIDER", "local"))),
		ClerkIssuer:  os.Getenv("CLERK_ISSUER"),
		ClerkAud:     os.Getenv("CLERK_AUDIENCE"),
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
	return nil
}

func getOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
