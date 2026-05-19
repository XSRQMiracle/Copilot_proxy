package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/auth"
	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/config"
	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/proxy"
	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/web"
)

type App struct {
	mu         sync.RWMutex
	cfg        *config.Config
	configPath string
	auth       *auth.Manager
	fallback   *proxy.FallbackSelector
	proxy      *proxy.Handler
	httpClient *http.Client
	logger     *log.Logger
	deviceMu   sync.Mutex
	deviceFlow *auth.DeviceFlow
	restartCh  chan<- struct{}
	restartMu  sync.Once
}

func NewApp(cfg *config.Config, configPath string, authManager *auth.Manager, fallback *proxy.FallbackSelector, proxyHandler *proxy.Handler, httpClient *http.Client, logger *log.Logger, restartCh chan<- struct{}) *App {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &App{cfg: cfg, configPath: configPath, auth: authManager, fallback: fallback, proxy: proxyHandler, httpClient: httpClient, logger: logger, restartCh: restartCh}
}

func (a *App) HTTPServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.route)
	srv := &http.Server{
		Addr:        a.cfg.ListenAddr(),
		Handler:     responseWriterWrapper{mux},
		ReadTimeout: time.Duration(a.cfg.Server.ReadTimeoutSeconds) * time.Second,
	}
	return srv
}

type responseWriterWrapper struct {
	handler http.Handler
}

func (w responseWriterWrapper) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	isSSE := r.URL.Path == "/v1/chat/completions" ||
		r.URL.Path == "/v1/messages" ||
		strings.HasPrefix(r.URL.Path, "/v1beta/models/")
	if isSSE {
		rw.Header().Set("X-Accel-Buffering", "no")
	}
	w.handler.ServeHTTP(rw, r)
}

func (a *App) route(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/" && r.Method == http.MethodGet:
		a.health(w, r)
	case r.URL.Path == "/v1/messages":
		a.proxy.ServeAnthropicMessages(w, r)
	case strings.HasPrefix(r.URL.Path, "/v1beta/models/"):
		a.proxy.ServeGeminiModels(w, r, strings.TrimPrefix(r.URL.Path, "/v1beta/models/"))
	case strings.HasPrefix(r.URL.Path, "/api/"):
		if !a.authenticateAPI(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		a.api(w, r)
	case r.URL.Path == "/fallback" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]string{"fallback_model": a.fallback.Selected()})
	case r.URL.Path == "/favicon.ico":
		a.serveFavicon(w, r)
	case strings.HasPrefix(r.URL.Path, "/ui") || r.URL.Path == "/ui":
		a.serveFrontend(w, r)
	default:
		a.proxy.ServeHTTP(w, r)
	}
}

func (a *App) authenticateAPI(r *http.Request) bool {
	// 登录端点不需要认证
	if r.URL.Path == "/api/auth/login" {
		return true
	}
	a.mu.RLock()
	hasAdminPassword := a.cfg.HasAdminPassword()
	adminPassword := a.cfg.Security.AdminPassword
	a.mu.RUnlock()
	// 如果没有设置 admin password，允许访问（兼容旧配置）
	if !hasAdminPassword {
		return true
	}
	token := r.Header.Get("Authorization")
	token = strings.TrimPrefix(token, "Bearer ")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(adminPassword)) == 1 {
		return true
	}
	return false
}

func (a *App) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":              "running",
		"github_token_ready":  a.auth.HasGitHubToken(),
		"copilot_token_ready": a.auth.HasCopilotToken(),
		"proxy_port":          a.cfg.Server.Port,
		"ui":                  a.cfg.PublicBaseURL() + "/ui/",
	})
}

func (a *App) api(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/status" && r.Method == http.MethodGet:
		a.status(w, r)
	case r.URL.Path == "/api/config" && r.Method == http.MethodGet:
		a.getConfig(w, r)
	case r.URL.Path == "/api/config" && r.Method == http.MethodPut:
		a.updateConfig(w, r)
	case r.URL.Path == "/api/config/ui" && r.Method == http.MethodPatch:
		a.patchUIConfig(w, r)
	case r.URL.Path == "/api/fallback" && r.Method == http.MethodPut:
		a.updateFallback(w, r)
	case r.URL.Path == "/api/stats" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, a.proxy.Stats())
	case r.URL.Path == "/api/models" && r.Method == http.MethodGet:
		a.models(w, r)
	case r.URL.Path == "/api/quota" && r.Method == http.MethodGet:
		a.quota(w, r)
	case r.URL.Path == "/api/service" && r.Method == http.MethodPost:
		a.updateService(w, r)
	case r.URL.Path == "/api/auth/login" && r.Method == http.MethodPost:
		a.login(w, r)
	case r.URL.Path == "/api/accounts" && r.Method == http.MethodGet:
		a.listAccounts(w, r)
	case r.URL.Path == "/api/accounts" && r.Method == http.MethodPost:
		a.addAccount(w, r)
	case r.URL.Path == "/api/accounts/switch" && r.Method == http.MethodPost:
		a.switchAccount(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/accounts/") && r.Method == http.MethodDelete:
		a.deleteAccount(w, r)
	case r.URL.Path == "/api/auth/device/start" && r.Method == http.MethodPost:
		a.startDevice(w, r)
	case r.URL.Path == "/api/auth/device/poll" && r.Method == http.MethodPost:
		a.pollDevice(w, r)
	case r.URL.Path == "/api/auth/logout" && r.Method == http.MethodPost:
		a.logout(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Password string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	a.mu.RLock()
	adminPassword := a.cfg.Security.AdminPassword
	a.mu.RUnlock()
	if subtle.ConstantTimeCompare([]byte(payload.Password), []byte(adminPassword)) == 1 {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "token": payload.Password})
		return
	}
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid password"})
}

func (a *App) status(w http.ResponseWriter, _ *http.Request) {
	expiresAt := a.auth.Expiration()
	var expires any
	if !expiresAt.IsZero() {
		expires = expiresAt.Format(time.RFC3339)
	}
	account := a.auth.ActiveAccount()
	accountName := ""
	if account != nil {
		accountName = account.Name
	}
	a.mu.RLock()
	baseURL := a.cfg.PublicBaseURL()
	proxyDisabled := a.cfg.Runtime.ProxyDisabled
	a.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"github_token_ready":  a.auth.HasGitHubToken(),
		"copilot_token_ready": a.auth.HasCopilotToken(),
		"copilot_expires_at":  expires,
		"fallback_model":      a.fallback.Selected(),
		"config_path":         a.configPath,
		"base_url":            baseURL,
		"service_enabled":     !proxyDisabled,
		"active_account":      accountName,
	})
}

func (a *App) getConfig(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	cfg := *a.cfg
	a.mu.RUnlock()
	writeJSON(w, http.StatusOK, redactConfigSecrets(cfg))
}

// redactConfigSecrets 返回配置的深拷贝，清除所有账号的 GitHubToken，
// 避免明文凭据通过 API 泄漏。
// 注意：必须先深拷贝 Accounts 切片，否则会修改调用者 cfg 的共享底层数组。
func redactConfigSecrets(cfg config.Config) config.Config {
	cfg.Auth.Accounts = append([]config.AccountConfig{}, cfg.Auth.Accounts...)
	for i := range cfg.Auth.Accounts {
		cfg.Auth.Accounts[i].GitHubToken = ""
	}
	return cfg
}

func (a *App) triggerRestart() {
	a.restartMu.Do(func() {
		go func() {
			time.Sleep(300 * time.Millisecond)
			a.restartCh <- struct{}{}
		}()
	})
}

func (a *App) updateConfig(w http.ResponseWriter, r *http.Request) {
	var next config.Config
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Preserve existing non-empty tokens when the incoming payload has empty tokens.
	// The GET /api/config endpoint redacts tokens for security, so the settings form
	// always submits empty token values. Without this guard, saving settings would
	// permanently wipe the encrypted token from disk.
	a.mu.RLock()
	existingAccounts := append([]config.AccountConfig{}, a.cfg.Auth.Accounts...)
	activeAccountID := a.cfg.Auth.ActiveAccountID
	a.mu.RUnlock()
	for i := range next.Auth.Accounts {
		if next.Auth.Accounts[i].GitHubToken == "" {
			for j := range existingAccounts {
				if existingAccounts[j].ID == next.Auth.Accounts[i].ID {
					if existingAccounts[j].GitHubToken != "" {
						next.Auth.Accounts[i].GitHubToken = existingAccounts[j].GitHubToken
					}
					break
				}
			}
		}
	}

	if err := config.Save(a.configPath, next); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	reloaded, _, err := config.LoadWithDecryptedTokens(a.configPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if activeAccountID != reloaded.Auth.ActiveAccountID {
		if err := a.auth.SwitchAccount(r.Context(), reloaded.Auth.ActiveAccountID); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
	}
	a.mu.Lock()
	*a.cfg = reloaded
	a.mu.Unlock()
	a.proxy.UpdateConfig(reloaded)
	a.fallback.UpdateConfig(reloaded)
	a.refreshFallbackChoice(r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
	if r.URL.Query().Get("restart") != "false" {
		a.triggerRestart()
	}
}

// patchUIConfig 轻量更新 ui.* 字段，不触发重启、不重新加载 token、不刷新 fallback。
// 直接读磁盘 JSON → 改 ui 字段 → 写回，零额外开销。
func (a *App) patchUIConfig(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Theme    *string `json:"theme,omitempty"`
		Language *string `json:"language,omitempty"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	raw, err := os.ReadFile(a.configPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var cfg config.Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if payload.Theme != nil {
		cfg.UI.Theme = *payload.Theme
	}
	if payload.Language != nil {
		cfg.UI.Language = *payload.Language
	}

	if cfg.UI.Theme != "" && cfg.UI.Theme != "system" && cfg.UI.Theme != "light" && cfg.UI.Theme != "dark" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid ui.theme: %q", cfg.UI.Theme)})
		return
	}
	if cfg.UI.Language != "" && cfg.UI.Language != "zh" && cfg.UI.Language != "en" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid ui.language: %q", cfg.UI.Language)})
		return
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := os.WriteFile(a.configPath, append(out, '\n'), 0600); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	a.mu.Lock()
	a.cfg.UI = cfg.UI
	a.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (a *App) updateFallback(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		PreferredPrefixes []string `json:"preferred_prefixes"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	cleaned := make([]string, 0, len(payload.PreferredPrefixes))
	for _, pref := range payload.PreferredPrefixes {
		pref = strings.TrimSpace(pref)
		if pref != "" {
			cleaned = append(cleaned, pref)
		}
	}
	a.mu.Lock()
	a.cfg.Fallback.PreferredPrefixes = cleaned
	snapshot := *a.cfg
	a.mu.Unlock()
	if err := config.Save(a.configPath, snapshot); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	reloaded, _, err := config.LoadWithDecryptedTokens(a.configPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.mu.Lock()
	*a.cfg = reloaded
	preferredPrefixes := append([]string{}, a.cfg.Fallback.PreferredPrefixes...)
	a.mu.Unlock()
	a.fallback.UpdateConfig(reloaded)
	a.refreshFallbackChoice(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "saved",
		"preferred_prefixes": preferredPrefixes,
		"fallback_model":     a.fallback.Selected(),
	})
}

func (a *App) refreshFallbackChoice(r *http.Request) {
	if token := a.auth.CopilotToken(); token != "" {
		a.mu.RLock()
		headers := a.cfg.DefaultHeaders()
		apiBase := a.cfg.Copilot.APIBase
		a.mu.RUnlock()
		headers["Authorization"] = "Bearer " + token
		if _, err := a.fallback.Choose(r.Context(), strings.TrimRight(apiBase, "/")+"/models", headers); err != nil {
			a.logger.Printf("[!] fallback model refresh: %v", err)
		}
	}
}

func (a *App) updateService(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.mu.Lock()
	a.cfg.Runtime.ProxyDisabled = !payload.Enabled
	snapshot := *a.cfg
	a.mu.Unlock()
	if err := config.Save(a.configPath, snapshot); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.proxy.UpdateConfig(snapshot)
	a.fallback.UpdateConfig(snapshot)
	writeJSON(w, http.StatusOK, map[string]any{"enabled": payload.Enabled})
}

func (a *App) listAccounts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"active_account_id": a.auth.ActiveAccountID(),
		"accounts":          a.auth.ListAccounts(),
	})
}

func (a *App) addAccount(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		GitHubToken string `json:"github_token"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	account, err := a.auth.AddAccount(r.Context(), payload.GitHubToken)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if err := a.auth.RefreshCopilotToken(r.Context()); err != nil {
		a.logger.Printf("[!] Copilot Token 刷新失败: %v", err)
	}
	safe := *account
	safe.GitHubToken = ""
	writeJSON(w, http.StatusOK, safe)
}

func (a *App) switchAccount(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID string `json:"id"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := a.auth.SwitchAccount(r.Context(), payload.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"active_account_id":   a.auth.ActiveAccountID(),
		"github_token_ready":  a.auth.HasGitHubToken(),
		"copilot_token_ready": a.auth.HasCopilotToken(),
	})
}

func (a *App) deleteAccount(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/accounts/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account id is required"})
		return
	}
	if err := a.auth.RemoveAccount(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) startDevice(w http.ResponseWriter, r *http.Request) {
	flow, err := a.auth.StartDeviceFlow(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	a.deviceMu.Lock()
	a.deviceFlow = &flow
	a.deviceMu.Unlock()
	writeJSON(w, http.StatusOK, flow)
}

func (a *App) pollDevice(w http.ResponseWriter, r *http.Request) {
	a.deviceMu.Lock()
	flow := a.deviceFlow
	a.deviceMu.Unlock()
	if flow == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device flow not started"})
		return
	}
	token, oauthErr, err := a.auth.PollAccessToken(r.Context(), flow.DeviceCode)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if oauthErr != "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": oauthErr})
		return
	}
	if _, err := a.auth.AddAccount(r.Context(), token); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if err := a.auth.RefreshCopilotToken(context.Background()); err != nil && a.logger != nil {
		a.logger.Printf("[!] Copilot Token 刷新失败: %v", err)
	}
	a.deviceMu.Lock()
	a.deviceFlow = nil
	a.deviceMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"status": "authorized"})
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	account := a.auth.ActiveAccount()
	if account == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if err := a.auth.RemoveAccount(r.Context(), account.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) models(w http.ResponseWriter, r *http.Request) {
	token := a.auth.CopilotToken()
	if token == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Copilot token not ready"})
		return
	}
	a.mu.RLock()
	apiBase := a.cfg.Copilot.APIBase
	headers := a.cfg.DefaultHeaders()
	requiredEndpoint := a.cfg.Fallback.RequiredEndpoint
	a.mu.RUnlock()
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, strings.TrimRight(apiBase, "/")+"/models", nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	var payload any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, resp.StatusCode, enrichModelAvailability(payload, requiredEndpoint))
}

func enrichModelAvailability(payload any, requiredEndpoint string) any {
	root, ok := payload.(map[string]any)
	if !ok {
		return payload
	}
	for _, key := range []string{"data", "models"} {
		items, ok := root[key].([]any)
		if !ok {
			continue
		}
		enriched := make([]any, 0, len(items))
		for _, item := range items {
			model, ok := item.(map[string]any)
			if !ok {
				enriched = append(enriched, item)
				continue
			}
			copy := make(map[string]any, len(model)+1)
			maps.Copy(copy, model)
			copy["available"] = modelAvailable(copy, requiredEndpoint)
			enriched = append(enriched, copy)
		}
		root[key] = enriched
	}
	return root
}

func modelAvailable(model map[string]any, requiredEndpoint string) bool {
	if enabled, ok := model["model_picker_enabled"].(bool); ok && !enabled {
		return false
	}
	if policy, ok := model["policy"].(map[string]any); ok {
		if state, _ := policy["state"].(string); state != "" && state == "disabled" {
			return false
		}
	}
	if requiredEndpoint == "" {
		return true
	}
	raw, ok := model["supported_endpoints"].([]any)
	if !ok || len(raw) == 0 {
		return true
	}
	for _, item := range raw {
		if endpoint, ok := item.(string); ok && endpoint == requiredEndpoint {
			return true
		}
	}
	return false
}

func (a *App) quota(w http.ResponseWriter, r *http.Request) {
	token := a.auth.GitHubToken()
	if token == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"available": false,
			"reason":    "quota_probe_failed",
			"message":   "GitHub token not ready",
		})
		return
	}
	endpoints := []string{
		"https://api.github.com/copilot_internal/user",
		"https://api.github.com/copilot_internal/usage",
	}
	results := make([]map[string]any, 0, len(endpoints))
	var quota map[string]any
	hadOK := false
	a.mu.RLock()
	headers := a.cfg.DefaultHeaders()
	a.mu.RUnlock()
	for _, endpoint := range endpoints {
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("Authorization", "token "+token)
		resp, err := a.httpClient.Do(req)
		if err != nil {
			results = append(results, map[string]any{"endpoint": endpoint, "error": err.Error()})
			continue
		}
		var body any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		results = append(results, map[string]any{"endpoint": endpoint, "status": resp.StatusCode, "body": body})
		if resp.StatusCode == http.StatusOK {
			hadOK = true
		}
		if quota == nil && resp.StatusCode == http.StatusOK {
			if summary, ok := quotaSummaryFromBody(body); ok {
				quota = summary
			}
		}
	}
	if quota != nil {
		quota["reason"] = "quota_ok"
		quota["probes"] = results
		writeJSON(w, http.StatusOK, quota)
		return
	}
	reason := "quota_probe_failed"
	message := "Unable to determine quota from GitHub probe endpoints. This is expected for some accounts and is not an error."
	if hadOK {
		reason = "quota_unrecognized"
		message = "GitHub returned quota data, but this version could not recognize its shape. This can happen when GitHub changes Copilot quota fields."
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"available": false,
		"reason":    reason,
		"message":   message,
		"probes":    results,
	})
}

func quotaSummaryFromBody(body any) (map[string]any, bool) {
	snapshots := findQuotaSnapshots(body)
	if len(snapshots) == 0 {
		return nil, false
	}
	// Enrich with monthly_quotas as entitlement for flat quota maps (Free accounts)
	if payload, ok := body.(map[string]any); ok {
		if monthly, ok := payload["monthly_quotas"].(map[string]any); ok {
			for key, val := range monthly {
				if snapshot, ok := snapshots[key].(map[string]any); ok {
					if _, hasEnt := snapshot["entitlement"]; !hasEnt {
						snapshot["entitlement"] = val
					}
				}
			}
		}
	}
	keys := orderedQuotaKeys(snapshots)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		raw := normalizeQuotaSnapshot(snapshots[key])
		if len(raw) == 0 {
			continue
		}
		remaining := numberAsString(raw["remaining"])
		entitlement := numberAsString(raw["entitlement"])
		percent := numberAsString(raw["percent_remaining"])
		unlimited, _ := raw["unlimited"].(bool)
		if unlimited {
			parts = append(parts, key+": unlimited")
			continue
		}
		if remaining == "" && percent == "" && entitlement == "" {
			continue
		}
		text := key + ": "
		if remaining != "" {
			text += remaining + " remaining"
		} else {
			text += "quota available"
		}
		if entitlement != "" && entitlement != "0" {
			text += "/" + entitlement
		}
		if percent != "" {
			text += " (" + percent + "%)"
		}
		parts = append(parts, text)
		snapshots[key] = raw
	}
	if len(parts) == 0 {
		return nil, false
	}
	return map[string]any{
		"available": true,
		"message":   strings.Join(parts, " · "),
		"snapshots": snapshots,
	}, true
}

func findQuotaSnapshots(body any) map[string]any {
	payload, ok := body.(map[string]any)
	if !ok {
		return nil
	}
	containerKeys := []string{"quota_snapshots", "quotaSnapshots", "usage", "quotas", "limited_user_quotas"}
	for _, key := range containerKeys {
		if snapshots := quotaMapFromValue(payload[key]); len(snapshots) > 0 {
			return snapshots
		}
	}
	for _, value := range payload {
		child, ok := value.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range containerKeys {
			if snapshots := quotaMapFromValue(child[key]); len(snapshots) > 0 {
				return snapshots
			}
		}
	}
	if snapshots := quotaMapFromValue(payload); len(snapshots) > 0 {
		return snapshots
	}
	return nil
}

func quotaMapFromValue(value any) map[string]any {
	items, ok := value.(map[string]any)
	if !ok || len(items) == 0 {
		return nil
	}
	if isQuotaSnapshot(items) {
		return map[string]any{"quota": normalizeQuotaSnapshot(items)}
	}
	result := make(map[string]any)
	for key, item := range items {
		if snapshot, ok := item.(map[string]any); ok && isQuotaSnapshot(snapshot) {
			result[normalizeQuotaKey(key)] = normalizeQuotaSnapshot(snapshot)
		}
	}
	if len(result) == 0 && isFlatQuotaMap(items) {
		for key, val := range items {
			if num, ok := val.(float64); ok {
				result[normalizeQuotaKey(key)] = map[string]any{"remaining": num}
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func isQuotaSnapshot(snapshot map[string]any) bool {
	for _, key := range []string{"remaining", "quota_remaining", "remaining_quota", "entitlement", "limit", "total", "quota", "used", "consumed", "percent_remaining", "remaining_percent", "unlimited"} {
		if _, ok := snapshot[key]; ok {
			return true
		}
	}
	return false
}

func normalizeQuotaSnapshot(value any) map[string]any {
	raw, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	normalized := make(map[string]any, len(raw))
	maps.Copy(normalized, raw)
	copyFirstQuotaField(normalized, raw, "remaining", "quota_remaining", "remaining_quota")
	copyFirstQuotaField(normalized, raw, "entitlement", "limit", "total", "quota")
	copyFirstQuotaField(normalized, raw, "percent_remaining", "remaining_percent")
	copyFirstQuotaField(normalized, raw, "used", "consumed")
	return normalized
}

func copyFirstQuotaField(dst, src map[string]any, canonical string, aliases ...string) {
	if _, ok := dst[canonical]; ok {
		return
	}
	for _, alias := range aliases {
		if value, ok := src[alias]; ok {
			dst[canonical] = value
			return
		}
	}
}

func isFlatQuotaMap(items map[string]any) bool {
	if len(items) == 0 {
		return false
	}
	for _, val := range items {
		switch val.(type) {
		case float64:
		default:
			return false
		}
	}
	return true
}

func orderedQuotaKeys(snapshots map[string]any) []string {
	known := []string{"premium_interactions", "chat", "completions"}
	result := make([]string, 0, len(snapshots))
	seen := make(map[string]bool, len(snapshots))
	for _, key := range known {
		if _, ok := snapshots[key]; ok {
			result = append(result, key)
			seen[key] = true
		}
	}
	otherKeys := make([]string, 0, len(snapshots))
	for key := range snapshots {
		if !seen[key] {
			otherKeys = append(otherKeys, key)
		}
	}
	sort.Strings(otherKeys)
	result = append(result, otherKeys...)
	return result
}

func normalizeQuotaKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, " ", "_")
	return strings.ToLower(key)
}

func numberAsString(value any) string {
	switch v := value.(type) {
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%.0f", v)
		}
		return fmt.Sprintf("%.1f", v)
	default:
		return ""
	}
}

func (a *App) serveFrontend(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/ui" {
		http.Redirect(w, r, "/ui/", http.StatusTemporaryRedirect)
		return
	}
	dist, _, err := web.DistFS()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if web.DistFromDisk() {
		w.Header().Set("Cache-Control", "no-cache")
	}
	name := strings.TrimPrefix(r.URL.Path, "/ui/")
	if name == "" || strings.HasSuffix(r.URL.Path, "/") {
		name = "index.html"
	}
	name = path.Clean(name)
	if strings.HasPrefix(name, "..") {
		http.NotFound(w, r)
		return
	}
	http.ServeFileFS(w, r, dist, name)
}

func (a *App) serveFavicon(w http.ResponseWriter, r *http.Request) {
	data, err := web.Favicon()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "max-age=86400, public")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
