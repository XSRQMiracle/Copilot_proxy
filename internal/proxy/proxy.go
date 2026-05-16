package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/auth"
	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/config"
)

var excludedResponseHeaders = map[string]struct{}{
	"content-encoding":  {},
	"connection":        {},
	"transfer-encoding": {},
	"content-length":    {},
}

type Handler struct {
	cfg      config.Config
	auth     *auth.Manager
	fallback *FallbackSelector
	client   *http.Client
	logger   *log.Logger
}

func NewHandler(cfg config.Config, authManager *auth.Manager, fallback *FallbackSelector, client *http.Client, logger *log.Logger) *Handler {
	if client == nil {
		client = http.DefaultClient
	}
	return &Handler{cfg: cfg, auth: authManager, fallback: fallback, client: client, logger: logger}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := h.auth.CopilotToken()
	if token == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Copilot token 未就绪，请检查授权状态"})
		return
	}

	upstreamURL, err := h.upstreamURL(r.URL.Path, r.URL.RawQuery)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	body = cleanBody(body)

	resp, err := h.forward(r.Context(), r.Method, upstreamURL, token, r.Header.Get("Content-Type"), body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		retryResp, retryErr := h.tryFallback(r.Context(), r.Method, upstreamURL, token, r.Header.Get("Content-Type"), body, resp)
		if retryErr == nil && retryResp != nil {
			resp.Body.Close()
			resp = retryResp
			defer resp.Body.Close()
		}
	}

	copyResponse(w, resp)
}

func (h *Handler) upstreamURL(path string, rawQuery string) (string, error) {
	normalized := strings.TrimPrefix(path, "/")
	normalized = strings.TrimPrefix(normalized, "v1/")
	base, err := url.Parse(h.cfg.Copilot.APIBase)
	if err != nil {
		return "", err
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/" + normalized
	base.RawQuery = rawQuery
	return base.String(), nil
}

func (h *Handler) forward(ctx context.Context, method, upstreamURL, token, contentType string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range h.cfg.DefaultHeaders() {
		req.Header.Set(k, v)
	}
	if contentType == "" {
		contentType = "application/json"
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Copilot-Integration-Id", h.cfg.Copilot.IntegrationID)
	return h.client.Do(req)
}

func cleanBody(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return raw
	}
	delete(data, "api_key")
	delete(data, "api_base")
	cleaned, err := json.Marshal(data)
	if err != nil {
		return raw
	}
	return cleaned
}

func (h *Handler) tryFallback(ctx context.Context, method, upstreamURL, token, contentType string, body []byte, first *http.Response) (*http.Response, error) {
	snapshot, err := io.ReadAll(first.Body)
	if err != nil {
		return nil, err
	}
	first.Body = io.NopCloser(bytes.NewReader(snapshot))
	if !isModelNotSupported(first.StatusCode, snapshot) {
		return nil, errors.New("not a model_not_supported response")
	}

	var requestBody map[string]any
	if err := json.Unmarshal(body, &requestBody); err != nil {
		return nil, err
	}
	requestedModel, _ := requestBody["model"].(string)
	selected := h.fallback.Selected()
	if selected == "" {
		headers := h.cfg.DefaultHeaders()
		headers["Authorization"] = "Bearer " + token
		var err error
		selected, err = h.fallback.Choose(ctx, strings.TrimRight(h.cfg.Copilot.APIBase, "/")+"/models", headers)
		if err != nil {
			return nil, err
		}
	}
	if selected == "" {
		return nil, errors.New("no fallback model available")
	}
	requestBody["model"] = selected
	retryBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	if h.logger != nil {
		h.logger.Printf("[!] 模型不可用: %s，回退到: %s", requestedModel, selected)
	}
	return h.forward(ctx, method, upstreamURL, token, contentType, retryBody)
}

func isModelNotSupported(status int, body []byte) bool {
	if status != http.StatusBadRequest {
		return false
	}
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if payload.Error.Code == "model_not_supported" {
			return true
		}
		if strings.Contains(strings.ToLower(payload.Error.Message), "not supported") {
			return true
		}
	}
	return strings.Contains(strings.ToLower(string(body)), "not supported")
}

func copyResponse(w http.ResponseWriter, resp *http.Response) {
	for k, values := range resp.Header {
		if _, excluded := excludedResponseHeaders[strings.ToLower(k)]; excluded {
			continue
		}
		for _, value := range values {
			w.Header().Add(k, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
