package config

import (
	"log/slog"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("LDAP_HOST", "ldap.example.org")
	t.Setenv("LDAP_BASE_DN", "dc=example,dc=org")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr default: got %q", cfg.ListenAddr)
	}
	if cfg.LDAPPort != 636 {
		t.Errorf("LDAPPort default: got %d", cfg.LDAPPort)
	}
	if cfg.AuthMode != AuthModeGateway {
		t.Errorf("AuthMode default: got %q", cfg.AuthMode)
	}
}

func TestLoad_InvalidAuthMode(t *testing.T) {
	t.Setenv("LDAP_HOST", "ldap.example.org")
	t.Setenv("LDAP_BASE_DN", "dc=example,dc=org")
	t.Setenv("AUTH_MODE", "bogus")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for bogus AUTH_MODE")
	}
}

func TestLoad_MissingHost(t *testing.T) {
	t.Setenv("LDAP_HOST", "")
	t.Setenv("LDAP_BASE_DN", "dc=example,dc=org")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when LDAP_HOST is empty")
	}
}

func TestLoad_LogLevelDefault(t *testing.T) {
	t.Setenv("LDAP_HOST", "ldap.example.org")
	t.Setenv("LDAP_BASE_DN", "dc=example,dc=org")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel default: got %v, want INFO", cfg.LogLevel)
	}
}

func TestLoad_LogLevelValues(t *testing.T) {
	tests := []struct {
		env  string
		want slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"debug", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"bogus", slog.LevelInfo}, // unknown defaults to INFO
	}
	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			t.Setenv("LDAP_HOST", "ldap.example.org")
			t.Setenv("LDAP_BASE_DN", "dc=example,dc=org")
			t.Setenv("LOG_LEVEL", tt.env)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.LogLevel != tt.want {
				t.Errorf("LOG_LEVEL=%q: got %v, want %v", tt.env, cfg.LogLevel, tt.want)
			}
		})
	}
}

func TestLoad_DevModeRejectsBindPassword(t *testing.T) {
	t.Setenv("LDAP_HOST", "ldap.example.org")
	t.Setenv("LDAP_BASE_DN", "dc=example,dc=org")
	t.Setenv("DEV_MODE", "true")
	t.Setenv("LDAP_BIND_PASSWORD", "secret")

	if _, err := Load(); err == nil {
		t.Fatal("expected DEV_MODE + bind password to fail validation")
	}
}
