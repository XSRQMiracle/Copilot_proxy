package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/config"
)

type OAuthPendingError string

func (e OAuthPendingError) Error() string { return string(e) }

type Manager struct {
	cfg          *config.Config
	configPath   string
	httpClient   *http.Client
	oauth        OAuthClient
	mu           sync.RWMutex
	activeID     string
	copilotToken string
	expiresAt    time.Time
}

func NewManager(cfg *config.Config, configPath string, httpClient *http.Client) *Manager {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	activeID := ""
	if account := cfg.ActiveAccount(); account != nil {
		activeID = account.ID
	}
	return &Manager{
		cfg:        cfg,
		configPath: configPath,
		httpClient: httpClient,
		oauth:      NewOAuthClient(*cfg, httpClient),
		activeID:   activeID,
	}
}

func (m *Manager) ActiveAccount() *config.AccountConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeAccountLocked()
}

func (m *Manager) activeAccountLocked() *config.AccountConfig {
	if m.activeID == "" {
		return m.cfg.ActiveAccount()
	}
	for i := range m.cfg.Auth.Accounts {
		if m.cfg.Auth.Accounts[i].ID == m.activeID {
			return &m.cfg.Auth.Accounts[i]
		}
	}
	return m.cfg.ActiveAccount()
}

func (m *Manager) GitHubToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	account := m.activeAccountLocked()
	if account == nil {
		return ""
	}
	return account.GitHubToken
}

func (m *Manager) CopilotToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.copilotToken
}

func (m *Manager) HasGitHubToken() bool {
	return m.GitHubToken() != ""
}

func (m *Manager) HasCopilotToken() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.copilotToken != ""
}

func (m *Manager) Expiration() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.expiresAt
}

func (m *Manager) SwitchAccount(ctx context.Context, id string) error {
	m.mu.Lock()
	found := false
	for i := range m.cfg.Auth.Accounts {
		if m.cfg.Auth.Accounts[i].ID == id {
			m.activeID = id
			m.cfg.Auth.ActiveAccountID = id
			found = true
			break
		}
	}
	m.mu.Unlock()

	if !found {
		return fmt.Errorf("account %q not found", id)
	}

	if m.HasGitHubToken() {
		if err := m.RefreshCopilotToken(ctx); err != nil {
			return fmt.Errorf("refresh copilot token after switch: %w", err)
		}
	}
	return config.Save(m.configPath, *m.cfg)
}

func (m *Manager) AddAccount(ctx context.Context, githubToken string) (*config.AccountConfig, error) {
	if githubToken == "" {
		return nil, errors.New("github token is required")
	}

	login, err := m.githubLogin(ctx, githubToken)
	if err != nil {
		return nil, fmt.Errorf("get github login: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.cfg.Auth.Accounts {
		if strings.EqualFold(m.cfg.Auth.Accounts[i].GitHubUserLogin, login) {
			m.cfg.Auth.Accounts[i].GitHubToken = githubToken
			account := &m.cfg.Auth.Accounts[i]
			m.activeID = account.ID
			m.cfg.Auth.ActiveAccountID = account.ID
			if err := config.Save(m.configPath, *m.cfg); err != nil {
				return nil, err
			}
			return account, nil
		}
	}

	account := config.AccountConfig{
		ID:              login,
		Name:            login,
		GitHubUserLogin: login,
		GitHubToken:     githubToken,
	}
	m.cfg.Auth.Accounts = append(m.cfg.Auth.Accounts, account)
	m.activeID = account.ID
	m.cfg.Auth.ActiveAccountID = account.ID

	if err := config.Save(m.configPath, *m.cfg); err != nil {
		return nil, err
	}
	return &account, nil
}

func (m *Manager) RemoveAccount(ctx context.Context, id string) error {
	m.mu.Lock()
	idx := -1
	for i := range m.cfg.Auth.Accounts {
		if m.cfg.Auth.Accounts[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		m.mu.Unlock()
		return fmt.Errorf("account %q not found", id)
	}

	m.cfg.Auth.Accounts = append(m.cfg.Auth.Accounts[:idx], m.cfg.Auth.Accounts[idx+1:]...)

	if m.activeID == id {
		if len(m.cfg.Auth.Accounts) > 0 {
			m.activeID = m.cfg.Auth.Accounts[0].ID
			m.cfg.Auth.ActiveAccountID = m.activeID
		} else {
			m.activeID = ""
			m.cfg.Auth.ActiveAccountID = ""
			m.copilotToken = ""
			m.expiresAt = time.Time{}
		}
	}
	m.mu.Unlock()

	if err := config.Save(m.configPath, *m.cfg); err != nil {
		return err
	}

	if m.HasGitHubToken() && m.activeID != "" {
		if err := m.RefreshCopilotToken(ctx); err != nil {
			return fmt.Errorf("refresh copilot token after remove: %w", err)
		}
	}
	return nil
}

func (m *Manager) RefreshCopilotToken(ctx context.Context) error {
	githubToken := m.GitHubToken()
	if githubToken == "" {
		return errors.New("github token is not available")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.cfg.GitHub.CopilotTokenURL, nil)
	if err != nil {
		return err
	}
	for k, v := range m.cfg.DefaultHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "token "+githubToken)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var payload struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	if payload.Token == "" {
		if payload.Message != "" {
			return errors.New(payload.Message)
		}
		return fmt.Errorf("copilot token request failed with status %d", resp.StatusCode)
	}

	expiresAt := time.Unix(payload.ExpiresAt, 0)
	m.mu.Lock()
	m.copilotToken = payload.Token
	m.expiresAt = expiresAt
	m.mu.Unlock()
	return nil
}

func (m *Manager) StartRefreshLoop(ctx context.Context, every time.Duration, logf func(string, ...any)) {
	if every <= 0 {
		every = 25 * time.Minute
	}
	ticker := time.NewTicker(every)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if logf != nil {
					logf("[~] Auto-refreshing Copilot Token...")
				}
				if err := m.RefreshCopilotToken(ctx); err != nil && logf != nil {
					logf("[!] Copilot Token refresh failed: %v", err)
				}
			}
		}
	}()
}

func (m *Manager) ListAccounts() []config.AccountConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]config.AccountConfig, len(m.cfg.Auth.Accounts))
	for i := range m.cfg.Auth.Accounts {
		result[i] = m.cfg.Auth.Accounts[i]
		result[i].GitHubToken = ""
	}
	return result
}

func (m *Manager) ActiveAccountID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeID
}

func (m *Manager) StartDeviceFlow(ctx context.Context) (DeviceFlow, error) {
	return m.oauth.StartDeviceFlow(ctx)
}

func (m *Manager) PollAccessToken(ctx context.Context, deviceCode string) (string, string, error) {
	return m.oauth.PollAccessToken(ctx, deviceCode)
}

func (m *Manager) WaitForAccessToken(ctx context.Context, flow DeviceFlow, onWait func(time.Duration)) (string, error) {
	return m.oauth.WaitForAccessToken(ctx, flow, onWait)
}

func (m *Manager) githubLogin(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return "", err
	}
	for k, v := range m.cfg.DefaultHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "token "+token)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var payload struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.Login) == "" {
		return "", fmt.Errorf("github user request returned status %d without login", resp.StatusCode)
	}
	return payload.Login, nil
}

func OpenBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		if os.Getenv("DISPLAY") == "" {
			return fmt.Errorf("no display available (headless), URL: %s", target)
		}
		if _, err := exec.LookPath("xdg-open"); err != nil {
			return fmt.Errorf("xdg-open not found, URL: %s", target)
		}
		cmd = exec.Command("xdg-open", target)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%w: %s", err, stderr.String())
		}
		return err
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}
