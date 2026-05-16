package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const EnvConfigPath = "COPILOT_PROXY_CONFIG"

type Config struct {
	Server   ServerConfig   `json:"server"`
	GitHub   GitHubConfig   `json:"github"`
	Copilot  CopilotConfig  `json:"copilot"`
	Headers  HeaderConfig   `json:"headers"`
	Fallback FallbackConfig `json:"fallback"`
	Keyring  KeyringConfig  `json:"keyring"`
	Frontend FrontendConfig `json:"frontend"`
}

type ServerConfig struct {
	Host                string `json:"host"`
	Port                int    `json:"port"`
	ReadTimeoutSeconds  int    `json:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `json:"write_timeout_seconds"`
}

type GitHubConfig struct {
	ClientID        string `json:"client_id"`
	DeviceCodeURL   string `json:"device_code_url"`
	AccessTokenURL  string `json:"access_token_url"`
	CopilotTokenURL string `json:"copilot_token_url"`
	OAuthScope      string `json:"oauth_scope"`
}

type CopilotConfig struct {
	APIBase       string `json:"api_base"`
	IntegrationID string `json:"integration_id"`
}

type HeaderConfig struct {
	EditorVersion       string `json:"editor_version"`
	EditorPluginVersion string `json:"editor_plugin_version"`
	UserAgent           string `json:"user_agent"`
}

type FallbackConfig struct {
	PreferredPrefixes []string `json:"preferred_prefixes"`
	RequiredEndpoint  string   `json:"required_endpoint"`
}

type KeyringConfig struct {
	Service string `json:"service"`
	Account string `json:"account"`
}

type FrontendConfig struct {
	Enabled bool `json:"enabled"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Host:                "0.0.0.0",
			Port:                15432,
			ReadTimeoutSeconds:  120,
			WriteTimeoutSeconds: 120,
		},
		GitHub: GitHubConfig{
			ClientID:        "Iv1.b507a08c87ecfe98",
			DeviceCodeURL:   "https://github.com/login/device/code",
			AccessTokenURL:  "https://github.com/login/oauth/access_token",
			CopilotTokenURL: "https://api.github.com/copilot_internal/v2/token",
			OAuthScope:      "read:user",
		},
		Copilot: CopilotConfig{
			APIBase:       "https://api.githubcopilot.com",
			IntegrationID: "vscode-chat",
		},
		Headers: HeaderConfig{
			EditorVersion:       "vscode/1.96.0",
			EditorPluginVersion: "copilot/1.246.0",
			UserAgent:           "GithubCopilot/1.246.0",
		},
		Fallback: FallbackConfig{
			PreferredPrefixes: []string{"gpt-4.1", "gpt-4o", "gpt-5-mini", "raptor-mini"},
			RequiredEndpoint:  "/chat/completions",
		},
		Keyring: KeyringConfig{
			Service: "copilot-proxy",
			Account: "github-token",
		},
		Frontend: FrontendConfig{Enabled: true},
	}
}

func DefaultPath() (string, error) {
	if fromEnv := strings.TrimSpace(os.Getenv(EnvConfigPath)); fromEnv != "" {
		return fromEnv, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "copilot-proxy", "config.json"), nil
}

func Load(path string) (Config, string, error) {
	resolved := path
	if resolved == "" {
		var err error
		resolved, err = DefaultPath()
		if err != nil {
			return Config{}, "", err
		}
	}

	cfg := Default()
	raw, err := os.ReadFile(resolved)
	if errors.Is(err, os.ErrNotExist) {
		if err := Save(resolved, cfg); err != nil {
			return Config{}, resolved, err
		}
		return cfg, resolved, nil
	}
	if err != nil {
		return Config{}, resolved, err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, resolved, fmt.Errorf("parse config %s: %w", resolved, err)
	}
	cfg.applyDefaults()
	return cfg, resolved, cfg.Validate()
}

func Save(path string, cfg Config) error {
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func (c Config) PublicBaseURL() string {
	host := c.Server.Host
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(c.Server.Port))
}

func (c Config) ListenAddr() string {
	return net.JoinHostPort(c.Server.Host, strconv.Itoa(c.Server.Port))
}

func (c Config) HTTPTimeout() time.Duration {
	if c.Server.ReadTimeoutSeconds <= 0 {
		return 120 * time.Second
	}
	return time.Duration(c.Server.ReadTimeoutSeconds) * time.Second
}

func (c Config) DefaultHeaders() map[string]string {
	return map[string]string{
		"Editor-Version":        c.Headers.EditorVersion,
		"Editor-Plugin-Version": c.Headers.EditorPluginVersion,
		"User-Agent":            c.Headers.UserAgent,
		"Accept":                "application/json",
	}
}

func (c *Config) applyDefaults() {
	d := Default()
	if c.Server.Host == "" {
		c.Server.Host = d.Server.Host
	}
	if c.Server.Port == 0 {
		c.Server.Port = d.Server.Port
	}
	if c.Server.ReadTimeoutSeconds == 0 {
		c.Server.ReadTimeoutSeconds = d.Server.ReadTimeoutSeconds
	}
	if c.Server.WriteTimeoutSeconds == 0 {
		c.Server.WriteTimeoutSeconds = d.Server.WriteTimeoutSeconds
	}
	if c.GitHub.ClientID == "" {
		c.GitHub.ClientID = d.GitHub.ClientID
	}
	if c.GitHub.DeviceCodeURL == "" {
		c.GitHub.DeviceCodeURL = d.GitHub.DeviceCodeURL
	}
	if c.GitHub.AccessTokenURL == "" {
		c.GitHub.AccessTokenURL = d.GitHub.AccessTokenURL
	}
	if c.GitHub.CopilotTokenURL == "" {
		c.GitHub.CopilotTokenURL = d.GitHub.CopilotTokenURL
	}
	if c.GitHub.OAuthScope == "" {
		c.GitHub.OAuthScope = d.GitHub.OAuthScope
	}
	if c.Copilot.APIBase == "" {
		c.Copilot.APIBase = d.Copilot.APIBase
	}
	if c.Copilot.IntegrationID == "" {
		c.Copilot.IntegrationID = d.Copilot.IntegrationID
	}
	if c.Headers.EditorVersion == "" {
		c.Headers.EditorVersion = d.Headers.EditorVersion
	}
	if c.Headers.EditorPluginVersion == "" {
		c.Headers.EditorPluginVersion = d.Headers.EditorPluginVersion
	}
	if c.Headers.UserAgent == "" {
		c.Headers.UserAgent = d.Headers.UserAgent
	}
	if len(c.Fallback.PreferredPrefixes) == 0 {
		c.Fallback.PreferredPrefixes = d.Fallback.PreferredPrefixes
	}
	if c.Fallback.RequiredEndpoint == "" {
		c.Fallback.RequiredEndpoint = d.Fallback.RequiredEndpoint
	}
	if c.Keyring.Service == "" {
		c.Keyring.Service = d.Keyring.Service
	}
	if c.Keyring.Account == "" {
		c.Keyring.Account = d.Keyring.Account
	}
}

func (c Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be 1-65535, got %d", c.Server.Port)
	}
	if strings.TrimSpace(c.Server.Host) == "" {
		return errors.New("server.host is required")
	}
	if strings.TrimSpace(c.Copilot.APIBase) == "" {
		return errors.New("copilot.api_base is required")
	}
	if strings.TrimSpace(c.Keyring.Service) == "" || strings.TrimSpace(c.Keyring.Account) == "" {
		return errors.New("keyring.service and keyring.account are required")
	}
	return nil
}
