package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/auth"
	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/config"
	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/proxy"
	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/web"
)

type App struct {
	cfg        config.Config
	configPath string
	auth       *auth.Manager
	fallback   *proxy.FallbackSelector
	proxy      *proxy.Handler
	logger     *log.Logger
	deviceMu   sync.Mutex
	deviceFlow *auth.DeviceFlow
}

func NewApp(cfg config.Config, configPath string, authManager *auth.Manager, fallback *proxy.FallbackSelector, proxyHandler *proxy.Handler, logger *log.Logger) *App {
	return &App{cfg: cfg, configPath: configPath, auth: authManager, fallback: fallback, proxy: proxyHandler, logger: logger}
}

func (a *App) HTTPServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.route)
	return &http.Server{
		Addr:         a.cfg.ListenAddr(),
		Handler:      mux,
		ReadTimeout:  time.Duration(a.cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(a.cfg.Server.WriteTimeoutSeconds) * time.Second,
	}
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
		a.api(w, r)
	case r.URL.Path == "/fallback" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]string{"fallback_model": a.fallback.Selected()})
	case a.cfg.Frontend.Enabled && (r.URL.Path == "/ui" || strings.HasPrefix(r.URL.Path, "/ui/")):
		a.frontend(w, r)
	default:
		a.proxy.ServeHTTP(w, r)
	}
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
		writeJSON(w, http.StatusOK, a.cfg)
	case r.URL.Path == "/api/config" && r.Method == http.MethodPut:
		a.updateConfig(w, r)
	case r.URL.Path == "/api/stats" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, a.proxy.Stats())
	case r.URL.Path == "/api/models" && r.Method == http.MethodGet:
		a.models(w, r)
	case r.URL.Path == "/api/quota" && r.Method == http.MethodGet:
		a.quota(w, r)
	case r.URL.Path == "/api/service" && r.Method == http.MethodPost:
		a.updateService(w, r)
	case r.URL.Path == "/api/accounts" && r.Method == http.MethodGet:
		a.accounts(w, r)
	case r.URL.Path == "/api/accounts" && r.Method == http.MethodPost:
		a.createAccount(w, r)
	case r.URL.Path == "/api/accounts/switch" && r.Method == http.MethodPost:
		a.switchAccount(w, r)
	case r.URL.Path == "/api/auth/device/start" && r.Method == http.MethodPost:
		a.startDevice(w, r)
	case r.URL.Path == "/api/auth/device/poll" && r.Method == http.MethodPost:
		a.pollDevice(w, r)
	case r.URL.Path == "/api/auth/logout" && r.Method == http.MethodPost:
		if err := a.auth.Logout(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		http.NotFound(w, r)
	}
}

func (a *App) status(w http.ResponseWriter, _ *http.Request) {
	expiresAt := a.auth.Expiration()
	var expires any
	if !expiresAt.IsZero() {
		expires = expiresAt.Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"github_token_ready":  a.auth.HasGitHubToken(),
		"copilot_token_ready": a.auth.HasCopilotToken(),
		"copilot_expires_at":  expires,
		"fallback_model":      a.fallback.Selected(),
		"config_path":         a.configPath,
		"base_url":            a.cfg.PublicBaseURL(),
		"service_enabled":     !a.cfg.Runtime.ProxyDisabled,
		"active_account":      a.auth.ActiveAccount(),
	})
}

func (a *App) updateConfig(w http.ResponseWriter, r *http.Request) {
	var next config.Config
	if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := config.Save(a.configPath, next); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	reloaded, _, err := config.Load(a.configPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !sameAccount(a.auth.ActiveAccount(), reloaded.ActiveAccount()) {
		if err := a.auth.SwitchAccount(reloaded.ActiveAccount()); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
	}
	a.cfg = reloaded
	a.proxy.UpdateConfig(reloaded)
	a.fallback.UpdateConfig(reloaded)
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved", "restart_required": "true"})
}

func sameAccount(left config.AccountConfig, right config.AccountConfig) bool {
	return left.ID == right.ID &&
		left.KeyringService == right.KeyringService &&
		left.KeyringAccount == right.KeyringAccount
}

func (a *App) updateService(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.cfg.Runtime.ProxyDisabled = !payload.Enabled
	if err := config.Save(a.configPath, a.cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.proxy.UpdateConfig(a.cfg)
	a.fallback.UpdateConfig(a.cfg)
	writeJSON(w, http.StatusOK, map[string]any{"enabled": payload.Enabled})
}

func (a *App) accounts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"active_account_id": a.cfg.Auth.ActiveAccountID,
		"accounts":          a.cfg.Auth.Accounts,
	})
}

func (a *App) createAccount(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	payload.ID = strings.TrimSpace(payload.ID)
	if payload.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	if slices.ContainsFunc(a.cfg.Auth.Accounts, func(account config.AccountConfig) bool { return account.ID == payload.ID }) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account already exists"})
		return
	}
	if strings.TrimSpace(payload.Name) == "" {
		payload.Name = payload.ID
	}
	a.cfg.Auth.Accounts = append(a.cfg.Auth.Accounts, config.AccountConfig{
		ID:             payload.ID,
		Name:           payload.Name,
		KeyringService: a.cfg.Keyring.Service,
		KeyringAccount: "github-token-" + payload.ID,
	})
	if err := config.Save(a.configPath, a.cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, a.cfg.Auth)
}

func (a *App) switchAccount(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	idx := slices.IndexFunc(a.cfg.Auth.Accounts, func(account config.AccountConfig) bool { return account.ID == payload.ID })
	if idx == -1 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
		return
	}
	account := a.cfg.Auth.Accounts[idx]
	if err := a.auth.SwitchAccount(account); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.cfg.Auth.ActiveAccountID = account.ID
	if err := config.Save(a.configPath, a.cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if a.auth.HasGitHubToken() {
		_ = a.auth.RefreshCopilotToken(r.Context())
	}
	writeJSON(w, http.StatusOK, map[string]any{"active_account_id": account.ID, "github_token_ready": a.auth.HasGitHubToken(), "copilot_token_ready": a.auth.HasCopilotToken()})
}

func (a *App) models(w http.ResponseWriter, r *http.Request) {
	token := a.auth.CopilotToken()
	if token == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Copilot token 未就绪"})
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, strings.TrimRight(a.cfg.Copilot.APIBase, "/")+"/models", nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	for k, v := range a.cfg.DefaultHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
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
	writeJSON(w, resp.StatusCode, payload)
}

func (a *App) quota(w http.ResponseWriter, r *http.Request) {
	token := a.auth.GitHubToken()
	if token == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"available": false, "message": "GitHub token 未就绪"})
		return
	}
	endpoints := []string{
		"https://api.github.com/copilot_internal/user",
		"https://api.github.com/copilot_internal/usage",
	}
	results := make([]map[string]any, 0, len(endpoints))
	var quota map[string]any
	for _, endpoint := range endpoints {
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		for k, v := range a.cfg.DefaultHeaders() {
			req.Header.Set(k, v)
		}
		req.Header.Set("Authorization", "token "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			results = append(results, map[string]any{"endpoint": endpoint, "error": err.Error()})
			continue
		}
		var body any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		results = append(results, map[string]any{"endpoint": endpoint, "status": resp.StatusCode, "body": body})
		if quota == nil && resp.StatusCode == http.StatusOK {
			if summary, ok := quotaSummaryFromBody(body); ok {
				quota = summary
			}
		}
	}
	if quota != nil {
		quota["probes"] = results
		writeJSON(w, http.StatusOK, quota)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"available": false,
		"message":   "GitHub does not expose a stable public personal Copilot remaining-quota endpoint; these are best-effort probe results.",
		"probes":    results,
	})
}

func quotaSummaryFromBody(body any) (map[string]any, bool) {
	payload, ok := body.(map[string]any)
	if !ok {
		return nil, false
	}
	snapshots, ok := payload["quota_snapshots"].(map[string]any)
	if !ok || len(snapshots) == 0 {
		return nil, false
	}
	keys := []string{"premium_interactions", "chat", "completions"}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		raw, ok := snapshots[key].(map[string]any)
		if !ok {
			continue
		}
		remaining := quotaNumber(raw["remaining"])
		if remaining == "" {
			remaining = quotaNumber(raw["quota_remaining"])
		}
		entitlement := quotaNumber(raw["entitlement"])
		percent := quotaNumber(raw["percent_remaining"])
		unlimited, _ := raw["unlimited"].(bool)
		if unlimited {
			parts = append(parts, key+": unlimited")
			continue
		}
		if remaining == "" {
			continue
		}
		text := key + ": " + remaining + " remaining"
		if entitlement != "" && entitlement != "0" {
			text += "/" + entitlement
		}
		if percent != "" {
			text += " (" + percent + "%)"
		}
		parts = append(parts, text)
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

func quotaNumber(value any) string {
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

func (a *App) startDevice(w http.ResponseWriter, r *http.Request) {
	flow, err := a.auth.StartDeviceFlow(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	a.deviceMu.Lock()
	a.deviceFlow = &flow
	a.deviceMu.Unlock()
	_ = auth.OpenBrowser(flow.VerificationURI)
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
	_, err := a.auth.PollDeviceFlow(r.Context(), flow.DeviceCode)
	if err != nil {
		var pending auth.OAuthPendingError
		if errors.As(err, &pending) {
			writeJSON(w, http.StatusAccepted, map[string]string{"status": pending.Error()})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if err := a.auth.RefreshCopilotToken(context.Background()); err != nil && a.logger != nil {
		a.logger.Printf("[!] Copilot Token 刷新失败: %v", err)
	}
	a.deviceMu.Lock()
	a.deviceFlow = nil
	a.deviceMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "authorized"})
}

func (a *App) frontend(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/ui" {
		http.Redirect(w, r, "/ui/", http.StatusTemporaryRedirect)
		return
	}
	dist, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
