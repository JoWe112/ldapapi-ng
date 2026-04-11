// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// AuthMode describes how the API authenticates incoming requests.
type AuthMode string

const (
	// AuthModeGateway means an API gateway (e.g. KrakenD) handles auth upstream.
	// The API itself does not enforce authentication — a NetworkPolicy must
	// restrict ingress to the gateway only.
	AuthModeGateway AuthMode = "gateway"
	// AuthModeStandalone means the API enforces HTTP Basic Auth itself,
	// validated by performing an LDAP bind with the submitted credentials.
	AuthModeStandalone AuthMode = "standalone"
)

// Config holds all runtime configuration.
type Config struct {
	// HTTP server
	ListenAddr string

	// LDAP
	LDAPHost       string
	LDAPPort       int
	LDAPBaseDN     string
	LDAPBindDN     string
	LDAPBindPass   string
	LDAPUserFilter string
	LDAPCACertPath string
	LDAPTimeout    time.Duration

	// Auth
	AuthMode AuthMode

	// Swagger
	SwaggerEnabled bool

	// Dev mode
	DevMode bool
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:     getEnv("LISTEN_ADDR", ":8080"),
		LDAPHost:       os.Getenv("LDAP_HOST"),
		LDAPPort:       getEnvInt("LDAP_PORT", 636),
		LDAPBaseDN:     os.Getenv("LDAP_BASE_DN"),
		LDAPBindDN:     os.Getenv("LDAP_BIND_DN"),
		LDAPBindPass:   os.Getenv("LDAP_BIND_PASSWORD"),
		LDAPUserFilter: getEnv("LDAP_USER_FILTER", "(uid=%s)"),
		LDAPCACertPath: os.Getenv("LDAP_CA_CERT_PATH"),
		LDAPTimeout:    getEnvDuration("LDAP_TIMEOUT", 10*time.Second),
		AuthMode:       AuthMode(strings.ToLower(getEnv("AUTH_MODE", "gateway"))),
		SwaggerEnabled: getEnvBool("SWAGGER_ENABLED", false),
		DevMode:        getEnvBool("DEV_MODE", false),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.LDAPHost == "" {
		return errors.New("LDAP_HOST is required")
	}
	if c.LDAPBaseDN == "" {
		return errors.New("LDAP_BASE_DN is required")
	}
	switch c.AuthMode {
	case AuthModeGateway, AuthModeStandalone:
	default:
		return fmt.Errorf("AUTH_MODE must be 'gateway' or 'standalone', got %q", c.AuthMode)
	}
	// Prevent DEV_MODE from being enabled alongside real credentials.
	if c.DevMode && c.LDAPBindPass != "" {
		return errors.New("DEV_MODE=true must not be set when LDAP_BIND_PASSWORD is configured")
	}
	return nil
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
