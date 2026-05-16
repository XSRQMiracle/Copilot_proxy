package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/config"
	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/keyring"
)

type Manager struct {
	cfg          config.Config
	store        keyring.Store
	oauth        OAuthClient
	httpClient   *http.Client
	mu           sync.RWMutex
	githubToken  string
	copilotToken string
	expiresAt    time.Time
}

func NewManager(cfg config.Config, store keyring.Store, httpClient *http.Client) *Manager {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Manager{
		cfg:        cfg,
		store:      store,
		oauth:      NewOAuthClient(cfg, httpClient),
		httpClient: httpClient,
	}
}

func (m *Manager) LoadGitHubToken() error {
	token, err := m.store.Get()
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.githubToken = token
	m.mu.Unlock()
	return nil
}

func (m *Manager) HasGitHubToken() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.githubToken != ""
}

func (m *Manager) HasCopilotToken() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.copilotToken != ""
}

func (m *Manager) CopilotToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.copilotToken
}

func (m *Manager) Expiration() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.expiresAt
}

func (m *Manager) SaveGitHubToken(token string) error {
	if token == "" {
		return errors.New("empty github token")
	}
	if err := m.store.Set(token); err != nil {
		return err
	}
	m.mu.Lock()
	m.githubToken = token
	m.mu.Unlock()
	return nil
}

func (m *Manager) Logout() error {
	if err := m.store.Delete(); err != nil {
		return err
	}
	m.mu.Lock()
	m.githubToken = ""
	m.copilotToken = ""
	m.expiresAt = time.Time{}
	m.mu.Unlock()
	return nil
}

func (m *Manager) RefreshCopilotToken(ctx context.Context) error {
	m.mu.RLock()
	githubToken := m.githubToken
	m.mu.RUnlock()
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
					logf("[~] 自动刷新 Copilot Token...")
				}
				if err := m.RefreshCopilotToken(ctx); err != nil && logf != nil {
					logf("[!] Copilot Token 刷新失败: %v", err)
				}
			}
		}
	}()
}

func (m *Manager) StartDeviceFlow(ctx context.Context) (DeviceFlow, error) {
	return m.oauth.StartDeviceFlow(ctx)
}

func (m *Manager) PollDeviceFlow(ctx context.Context, deviceCode string) (string, error) {
	token, oauthErr, err := m.oauth.PollAccessToken(ctx, deviceCode)
	if err != nil {
		return "", err
	}
	if oauthErr != "" {
		return "", OAuthPendingError(oauthErr)
	}
	if err := m.SaveGitHubToken(token); err != nil {
		return "", err
	}
	return token, nil
}

type OAuthPendingError string

func (e OAuthPendingError) Error() string { return string(e) }

func (m *Manager) InteractiveLogin(ctx context.Context, logf func(string, ...any)) error {
	flow, err := m.oauth.StartDeviceFlow(ctx)
	if err != nil {
		return err
	}
	if logf != nil {
		logf("\n==================================================")
		logf("  GitHub Copilot 授权")
		logf("==================================================")
		logf("请打开: %s", flow.VerificationURI)
		logf("输入验证码: %s", flow.UserCode)
		logf("==================================================")
	}
	_ = OpenBrowser(flow.VerificationURI)
	token, err := m.oauth.WaitForAccessToken(ctx, flow, func(remaining time.Duration) {
		if logf != nil {
			logf("等待授权中... 剩余约 %s", remaining.Round(time.Second))
		}
	})
	if err != nil {
		return err
	}
	if err := m.SaveGitHubToken(token); err != nil {
		return err
	}
	return nil
}

func OpenBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
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
	return nil
}
