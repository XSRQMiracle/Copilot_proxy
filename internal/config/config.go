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
	ProxyDisabled bool     `json:"proxy_disabled"`
	DeniedVendors []string `json:"denied_vendors,omitempty"`
}

type AccountConfig struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	GitHubUserLogin string `json:"github_user_login,omitempty"`
	GitHubToken     string `json:"github_token,omitempty"`
	GitHubTokenRef  string `json:"github_token_ref,omitempty"`
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
			APIKey:        "dummy",
			AdminPassword: "admin",
		},
		Runtime: RuntimeConfig{ProxyDisabled: false, DeniedVendors: []string{"github-copilot"}},
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

	persisted := cfg
	// 深拷贝 Accounts 切片，避免持久化时修改调用者 cfg 的共享底层数组。
	persisted.Auth.Accounts = append([]AccountConfig{}, persisted.Auth.Accounts...)
	storage := EffectiveTokenStorage()
	for i := range persisted.Auth.Accounts {
		account := &persisted.Auth.Accounts[i]
		token := account.GitHubToken
		if token == "" {
			continue
		}
		switch storage {
		case TokenStorageKeyring:
			ref, err := StoreGitHubToken(account.ID, token)
			if err != nil {
				return fmt.Errorf("store token in keyring for account %s: %w", account.ID, err)
			}
			account.GitHubToken = ""
			account.GitHubTokenRef = ref
		case TokenStorageFile:
			account.GitHubToken = token
			account.GitHubTokenRef = ""
		}
	}

	raw, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

// LoadWithResolvedTokens 加载配置并解析 GitHub token。
// 桌面平台从系统 keyring 读取 github_token_ref；headless Linux 使用 0600 配置文件中的明文 token。
func LoadWithResolvedTokens(path string) (Config, string, error) {
	cfg, resolved, err := Load(path)
	if err != nil {
		return cfg, resolved, err
	}

	storage := EffectiveTokenStorage()
	var resolveErrs []error
	for i := range cfg.Auth.Accounts {
		account := &cfg.Auth.Accounts[i]
		switch storage {
		case TokenStorageKeyring:
			if account.GitHubTokenRef == "" {
				if account.GitHubToken != "" {
					account.GitHubToken = ""
					resolveErrs = append(resolveErrs, fmt.Errorf("account %q: legacy github_token is no longer supported with keyring storage; delete config.json and re-authorize", account.ID))
				}
				continue
			}
			plaintext, err := LoadGitHubToken(account.GitHubTokenRef)
			if err != nil {
				account.GitHubToken = ""
				resolveErrs = append(resolveErrs, fmt.Errorf("account %q: load keyring token: %w", account.ID, err))
				continue
			}
			account.GitHubToken = plaintext
		case TokenStorageFile:
			if account.GitHubTokenRef != "" && account.GitHubToken == "" {
				resolveErrs = append(resolveErrs, fmt.Errorf("account %q: keyring token reference is unavailable in file token storage; delete config.json and re-authorize", account.ID))
			}
		}
	}

	if len(resolveErrs) > 0 {
		return cfg, resolved, fmt.Errorf("the following accounts had token load failures and were cleared: %s",
			errors.Join(resolveErrs...))
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

// IsVendorDenied 检查 vendor 是否以 DeniedVendors 中任意一项为前缀（不区分大小写）。
func (c *Config) IsVendorDenied(vendor string) bool {
	vendor = strings.ToLower(vendor)
	for _, denied := range c.Runtime.DeniedVendors {
		if strings.HasPrefix(vendor, strings.ToLower(denied)) {
			return true
		}
	}
	return false
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
	if c.Runtime.DeniedVendors == nil {
		c.Runtime.DeniedVendors = d.Runtime.DeniedVendors
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
