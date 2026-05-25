package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
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

func TestSaveUsesPlainFileStorageWhenForced(t *testing.T) {
	t.Setenv(tokenStorageEnv, TokenStorageFile)
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Default()
	cfg.Auth.ActiveAccountID = "alice"
	cfg.Auth.Accounts = []AccountConfig{{
		ID:              "alice",
		Name:            "alice",
		GitHubUserLogin: "alice",
		GitHubToken:     "gho_plain",
	}}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var persisted Config
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := persisted.Auth.Accounts[0].GitHubToken; got != "gho_plain" {
		t.Fatalf("persisted token = %q, want plaintext fallback token", got)
	}
	if got := persisted.Auth.Accounts[0].GitHubTokenRef; got != "" {
		t.Fatalf("token ref = %q, want empty for file storage", got)
	}

	loaded, _, err := LoadWithResolvedTokens(path)
	if err != nil {
		t.Fatalf("LoadWithResolvedTokens() error = %v", err)
	}
	if got := loaded.Auth.Accounts[0].GitHubToken; got != "gho_plain" {
		t.Fatalf("loaded token = %q, want plaintext token", got)
	}
}

func TestSaveUsesKeyringStorageWhenForced(t *testing.T) {
	t.Setenv(tokenStorageEnv, TokenStorageKeyring)
	keyring.MockInit()
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Default()
	cfg.Auth.ActiveAccountID = "alice"
	cfg.Auth.Accounts = []AccountConfig{{
		ID:              "alice",
		Name:            "alice",
		GitHubUserLogin: "alice",
		GitHubToken:     "gho_secret",
	}}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(raw) == "" || strings.Contains(string(raw), "gho_secret") {
		t.Fatalf("persisted config leaked token: %s", raw)
	}
	var persisted Config
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := persisted.Auth.Accounts[0].GitHubToken; got != "" {
		t.Fatalf("persisted token = %q, want empty for keyring storage", got)
	}
	if got := persisted.Auth.Accounts[0].GitHubTokenRef; got != "keyring:github:alice" {
		t.Fatalf("token ref = %q, want keyring ref", got)
	}

	loaded, _, err := LoadWithResolvedTokens(path)
	if err != nil {
		t.Fatalf("LoadWithResolvedTokens() error = %v", err)
	}
	if got := loaded.Auth.Accounts[0].GitHubToken; got != "gho_secret" {
		t.Fatalf("loaded token = %q, want keyring token", got)
	}
}
