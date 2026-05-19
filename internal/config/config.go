package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	Security SecurityConfig `json:"security"`
	Runtime  RuntimeConfig  `json:"runtime"`
	Auth     AuthConfig     `json:"auth"`
	UI       UIConfig       `json:"ui"`
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

type SecurityConfig struct {
	APIKey        string `json:"api_key"`
	AdminPassword string `json:"admin_password,omitempty"`
}

type RuntimeConfig struct {
	ProxyDisabled bool `json:"proxy_disabled"`
}

type AccountConfig struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	GitHubUserLogin string `json:"github_user_login,omitempty"`
	GitHubToken     string `json:"github_token,omitempty"`
}

type AuthConfig struct {
	ActiveAccountID string          `json:"active_account_id"`
	Accounts        []AccountConfig `json:"accounts"`
}

type UIConfig struct {
	Language string `json:"language"`
	Theme    string `json:"theme"`
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
		Security: SecurityConfig{
			APIKey:          "dummy",
			AdminPassword:   "admin",
		},
		Runtime: RuntimeConfig{ProxyDisabled: false},
		Auth: AuthConfig{
			ActiveAccountID: "",
			Accounts:        []AccountConfig{},
		},
		UI: UIConfig{Language: "zh", Theme: "system"},
	}
}

// DefaultPath 返回默认配置文件路径（可执行程序所在目录下的 config/config.json）。
func DefaultPath() (string, error) {
	if fromEnv := strings.TrimSpace(os.Getenv(EnvConfigPath)); fromEnv != "" {
		return fromEnv, nil
	}
	exe, err := os.Executable()
	if err != nil {
		// 兜底：当前工作目录
		dir, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "config", "config.json"), nil
	}
	dir := filepath.Dir(exe)
	return filepath.Join(dir, "config", "config.json"), nil
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

	// 加密所有账号的 github_token
	encrypted := cfg
	// 深拷贝 Accounts 切片，避免加密 ciphertext 回写时修改调用者 cfg 的共享底层数组
	encrypted.Auth.Accounts = append([]AccountConfig{}, encrypted.Auth.Accounts...)
	for i := range encrypted.Auth.Accounts {
		token := encrypted.Auth.Accounts[i].GitHubToken
		if token != "" && !IsProbablyEncrypted(token) {
			ciphertext, err := EncryptToken(token)
			if err != nil {
				return fmt.Errorf("encrypt token for account %s: %w", encrypted.Auth.Accounts[i].ID, err)
			}
			encrypted.Auth.Accounts[i].GitHubToken = ciphertext
		}
	}

	raw, err := json.MarshalIndent(encrypted, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

// LoadWithDecryptedTokens 加载配置并解密所有 github_token。
// 解密失败的 token 会被清空（而非继续以密文体作为凭据使用），并通过错误返回汇总警告。
func LoadWithDecryptedTokens(path string) (Config, string, error) {
	cfg, resolved, err := Load(path)
	if err != nil {
		return cfg, resolved, err
	}

	// Reload raw to decrypt tokens
	raw, err := os.ReadFile(resolved)
	if err != nil {
		return cfg, resolved, err
	}
	// 从原始 JSON 解密 token
	var rawCfg Config
	if err := json.Unmarshal(raw, &rawCfg); err != nil {
		return cfg, resolved, err
	}

	var decryptErrs []error
	for i := range rawCfg.Auth.Accounts {
		token := rawCfg.Auth.Accounts[i].GitHubToken
		if token != "" && IsProbablyEncrypted(token) {
			log.Printf("[DEBUG] LoadWithDecryptedTokens: attempting decrypt for account %q (token prefix: %s len=%d)",
				rawCfg.Auth.Accounts[i].ID, token[:min(len(token), 20)], len(token))
			plaintext, err := DecryptToken(token)
			if err != nil {
				// 解密失败：不清除现有配置，仅把内存中的 token 置空；
				// 绝不保留密文字符串作为后续鉴权的凭据。
				cfg.Auth.Accounts[i].GitHubToken = ""
				log.Printf("[DEBUG] LoadWithDecryptedTokens: decrypt FAILED for account %q: %v",
					rawCfg.Auth.Accounts[i].ID, err)
				decryptErrs = append(decryptErrs, fmt.Errorf("account %q: %w", rawCfg.Auth.Accounts[i].ID, err))
				continue
			}
			log.Printf("[DEBUG] LoadWithDecryptedTokens: decrypt OK for account %q (plaintext len=%d)",
				rawCfg.Auth.Accounts[i].ID, len(plaintext))
			cfg.Auth.Accounts[i].GitHubToken = plaintext
		} else if token != "" && !IsProbablyEncrypted(token) {
			log.Printf("[DEBUG] LoadWithDecryptedTokens: token for account %q is not encrypted, using as-is (prefix=%s len=%d)",
				rawCfg.Auth.Accounts[i].ID, token[:min(len(token), 20)], len(token))
			cfg.Auth.Accounts[i].GitHubToken = token
		}
	}

	if len(decryptErrs) > 0 {
		return cfg, resolved, fmt.Errorf("the following accounts had token decryption failures and were cleared: %s",
			errors.Join(decryptErrs...))
	}
	return cfg, resolved, nil
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

func (c Config) ActiveAccount() *AccountConfig {
	accounts := c.Auth.Accounts
	for i := range accounts {
		if accounts[i].ID == c.Auth.ActiveAccountID {
			return &accounts[i]
		}
	}
	if len(accounts) > 0 {
		return &accounts[0]
	}
	return nil
}

func (c Config) HasAdminPassword() bool {
	return strings.TrimSpace(c.Security.AdminPassword) != ""
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
	if c.Security.APIKey == "" {
		c.Security.APIKey = d.Security.APIKey
	}
	if c.Auth.ActiveAccountID == "" {
		c.Auth.ActiveAccountID = d.Auth.ActiveAccountID
	}
	if c.Auth.Accounts == nil {
		c.Auth.Accounts = d.Auth.Accounts
	}
	if c.UI.Language == "" {
		c.UI.Language = d.UI.Language
	}
	if c.UI.Theme == "" {
		c.UI.Theme = d.UI.Theme
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
	if c.UI.Language != "" && c.UI.Language != "zh" && c.UI.Language != "en" {
		return fmt.Errorf("ui.language must be zh or en, got %q", c.UI.Language)
	}
	if c.UI.Theme != "" && c.UI.Theme != "system" && c.UI.Theme != "light" && c.UI.Theme != "dark" {
		return fmt.Errorf("ui.theme must be system, light, or dark, got %q", c.UI.Theme)
	}
	return nil
}
