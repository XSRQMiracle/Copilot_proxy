package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, resolved, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if resolved != path {
		t.Fatalf("resolved path = %q, want %q", resolved, path)
	}
	if cfg.Server.Port != 15432 {
		t.Fatalf("default port = %d, want 15432", cfg.Server.Port)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("default config was not created: %v", err)
	}
}

func TestPublicBaseURLUsesLocalhostForWildcard(t *testing.T) {
	cfg := Default()
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 18080
	if got := cfg.PublicBaseURL(); got != "http://localhost:18080" {
		t.Fatalf("PublicBaseURL() = %q", got)
	}
}

func TestValidateRejectsInvalidPort(t *testing.T) {
	cfg := Default()
	cfg.Server.Port = 70000
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid port error")
	}
}
