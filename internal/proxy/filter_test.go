package proxy

import (
	"testing"

	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/config"
)

func TestIsModelAllowed_NoVendor(t *testing.T) {
	cfg := config.Default()
	if !IsModelAllowed("gpt-4.1", &cfg) {
		t.Error("IsModelAllowed(gpt-4.1) = false, want true (no vendor prefix)")
	}
}

func TestIsModelAllowed_AllowedVendor(t *testing.T) {
	cfg := config.Default()
	if !IsModelAllowed("openai/gpt-4.1", &cfg) {
		t.Error("IsModelAllowed(openai/gpt-4.1) = false, want true")
	}
}

func TestIsModelAllowed_DeniedVendor(t *testing.T) {
	cfg := config.Default()
	if IsModelAllowed("github-copilot/gpt-5-mini", &cfg) {
		t.Error("IsModelAllowed(github-copilot/gpt-5-mini) = true, want false")
	}
}

func TestIsModelAllowed_DeniedVendorCustom(t *testing.T) {
	cfg := config.Default()
	cfg.Runtime.DeniedVendors = []string{"my-blocked"}
	if IsModelAllowed("my-blocked/gpt-5", &cfg) {
		t.Error("IsModelAllowed(my-blocked/gpt-5) = true, want false")
	}
	if !IsModelAllowed("openai/gpt-5", &cfg) {
		t.Error("IsModelAllowed(openai/gpt-5) = false, want true")
	}
}

func TestIsModelAllowed_NilConfig(t *testing.T) {
	if !IsModelAllowed("github-copilot/gpt-5", nil) {
		t.Error("IsModelAllowed(..., nil) = false, want true (nil cfg is safe)")
	}
}

func TestExtractVendor(t *testing.T) {
	tests := []struct {
		model  string
		vendor string
	}{
		{"github-copilot/gpt-5-mini", "github-copilot"},
		{"openai/gpt-4.1", "openai"},
		{"gpt-4.1", ""},
		{"", ""},
		{"a/b/c", "a"},
	}
	for _, tt := range tests {
		if got := extractVendor(tt.model); got != tt.vendor {
			t.Errorf("extractVendor(%q) = %q, want %q", tt.model, got, tt.vendor)
		}
	}
}
