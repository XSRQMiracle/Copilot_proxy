package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"path"
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
	a.cfg = reloaded
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved", "restart_required": "true"})
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
