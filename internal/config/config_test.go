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

func TestDefaultDeniedVendors(t *testing.T) {
	cfg := Default()
	if len(cfg.Runtime.DeniedVendors) != 1 {
		t.Fatalf("len(DeniedVendors) = %d, want 1", len(cfg.Runtime.DeniedVendors))
	}
	if cfg.Runtime.DeniedVendors[0] != "github-copilot" {
		t.Fatalf("DeniedVendors[0] = %q, want %q", cfg.Runtime.DeniedVendors[0], "github-copilot")
	}
}

func TestIsVendorDenied(t *testing.T) {
	cfg := Default()
	tests := []struct {
		vendor string
		want   bool
	}{
		{"github-copilot", true},
		{"GitHub-Copilot", true}, // case insensitive
		{"openai", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := cfg.IsVendorDenied(tt.vendor); got != tt.want {
			t.Errorf("IsVendorDenied(%q) = %v, want %v", tt.vendor, got, tt.want)
		}
	}
}

func TestIsVendorDeniedCustom(t *testing.T) {
	cfg := Default()
	cfg.Runtime.DeniedVendors = []string{"custom-vendor", "blocked"}
	tests := []struct {
		vendor string
		want   bool
	}{
		{"custom-vendor", true},
		{"blocked", true},
		{"custom-vendor/gpt", true},
		{"openai", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := cfg.IsVendorDenied(tt.vendor); got != tt.want {
			t.Errorf("IsVendorDenied(%q) = %v, want %v", tt.vendor, got, tt.want)
		}
	}
}
