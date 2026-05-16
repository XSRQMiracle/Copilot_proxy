package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	stats    *Stats
}

func NewHandler(cfg config.Config, authManager *auth.Manager, fallback *FallbackSelector, client *http.Client, logger *log.Logger, stats *Stats) *Handler {
	if client == nil {
		client = http.DefaultClient
	}
	if stats == nil {
		stats = NewStats(200)
	}
	return &Handler{cfg: cfg, auth: authManager, fallback: fallback, client: client, logger: logger, stats: stats}
}

func (h *Handler) UpdateConfig(cfg config.Config) {
	h.cfg = cfg
}

func (h *Handler) Stats() StatsSnapshot {
	return h.stats.Snapshot()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !h.cfg.Runtime.ProxyDisabled && !h.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid api key"})
		h.record(RequestRecord{Time: start, Protocol: "openai", Method: r.Method, Path: r.URL.Path, Status: http.StatusUnauthorized, DurationMs: time.Since(start).Milliseconds(), Error: "invalid api key"})
		return
	}
	if h.cfg.Runtime.ProxyDisabled {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "proxy service is disabled"})
		h.record(RequestRecord{Time: start, Protocol: "openai", Method: r.Method, Path: r.URL.Path, Status: http.StatusServiceUnavailable, DurationMs: time.Since(start).Milliseconds(), Error: "proxy disabled"})
		return
	}
	token := h.auth.CopilotToken()
	if token == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Copilot token 未就绪，请检查授权状态"})
		h.record(RequestRecord{Time: start, Protocol: "openai", Method: r.Method, Path: r.URL.Path, Status: http.StatusServiceUnavailable, DurationMs: time.Since(start).Milliseconds(), Error: "missing copilot token"})
		return
	}

	upstreamURL, err := h.upstreamURL(r.URL.Path, r.URL.RawQuery)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		h.record(RequestRecord{Time: start, Protocol: "openai", Method: r.Method, Path: r.URL.Path, Status: http.StatusBadGateway, DurationMs: time.Since(start).Milliseconds(), Error: err.Error()})
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		h.record(RequestRecord{Time: start, Protocol: "openai", Method: r.Method, Path: r.URL.Path, Status: http.StatusBadRequest, DurationMs: time.Since(start).Milliseconds(), Error: err.Error()})
		return
	}
	body = cleanBody(body)
	model := modelFromJSONBody(body)

	resp, err := h.forward(r.Context(), r.Method, upstreamURL, token, r.Header.Get("Content-Type"), body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		h.record(RequestRecord{Time: start, Protocol: "openai", Method: r.Method, Path: r.URL.Path, Model: model, Status: http.StatusBadGateway, DurationMs: time.Since(start).Milliseconds(), Error: err.Error()})
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

	h.copyAndRecordResponse(w, resp, RequestRecord{Time: start, Protocol: "openai", Method: r.Method, Path: r.URL.Path, Model: model, DurationMs: time.Since(start).Milliseconds()})
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
	ensureStreamUsage(data)
	cleaned, err := json.Marshal(data)
	if err != nil {
		return raw
	}
	return cleaned
}

func ensureStreamUsage(payload map[string]any) {
	stream, _ := payload["stream"].(bool)
	if !stream {
		return
	}
	options, _ := payload["stream_options"].(map[string]any)
	if options == nil {
		options = map[string]any{}
	}
	options["include_usage"] = true
	payload["stream_options"] = options
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

func (h *Handler) copyAndRecordResponse(w http.ResponseWriter, resp *http.Response, record RequestRecord) {
	record.Status = resp.StatusCode
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		flusher, _ := w.(http.Flusher)
		for k, values := range resp.Header {
			if _, excluded := excludedResponseHeaders[strings.ToLower(k)]; excluded {
				continue
			}
			for _, value := range values {
				w.Header().Add(k, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			_, _ = fmt.Fprintln(w, line)
			flush(flusher)
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
				if data != "" && data != "[DONE]" {
					var chunk map[string]any
					if err := json.Unmarshal([]byte(data), &chunk); err == nil {
						if model := stringValue(chunk["model"]); model != "" {
							record.Model = stringValue(chunk["model"])
						}
						mergeStreamUsage(&record, chunk)
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			record.Error = err.Error()
		}
		record.DurationMs = time.Since(record.Time).Milliseconds()
		h.record(record)
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		record.Error = err.Error()
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		h.record(record)
		return
	}
	for k, values := range resp.Header {
		if _, excluded := excludedResponseHeaders[strings.ToLower(k)]; excluded {
			continue
		}
		for _, value := range values {
			w.Header().Add(k, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
	if len(body) > 0 {
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err == nil {
			if model := stringValue(payload["model"]); model != "" {
				record.Model = model
			}
			if usage, ok := payload["usage"].(map[string]any); ok {
				record.PromptTokens, record.CompletionTokens, record.TotalTokens = tokensFromUsage(usage)
			}
		}
	}
	record.DurationMs = time.Since(record.Time).Milliseconds()
	h.record(record)
}

func (h *Handler) authorized(r *http.Request) bool {
	expected := strings.TrimSpace(h.cfg.Security.APIKey)
	if expected == "" {
		return true
	}
	if r.URL.Query().Get("key") == expected {
		return true
	}
	if r.Header.Get("x-api-key") == expected || r.Header.Get("x-goog-api-key") == expected {
		return true
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") && strings.TrimSpace(auth[7:]) == expected {
		return true
	}
	return false
}

func ShouldExcludeFromStats(path string) bool {
	switch path {
	case "/favicon.ico", "/robots.txt":
		return true
	}
	return false
}

func (h *Handler) record(record RequestRecord) {
	if ShouldExcludeFromStats(record.Path) {
		return
	}
	if h.stats != nil {
		h.stats.Record(record)
	}
}

func modelFromJSONBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return stringValue(payload["model"])
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
