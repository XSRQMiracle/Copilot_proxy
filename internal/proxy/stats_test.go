package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/auth"
	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/config"
)

func TestTokensFromUsageReasoningDetails(t *testing.T) {
	usage := map[string]any{
		"prompt_tokens":     11,
		"completion_tokens": 7,
		"total_tokens":      18,
		"completion_tokens_details": map[string]any{
			"reasoning_tokens": 5,
		},
	}

	prompt, completion, total, reasoning := tokensFromUsage(usage)
	if prompt != 11 || completion != 7 || total != 18 || reasoning != 5 {
		t.Fatalf("tokens = (%d, %d, %d, %d), want (11, 7, 18, 5)", prompt, completion, total, reasoning)
	}
}

func TestTokensFromUsageTotalFallbackAndOutputReasoning(t *testing.T) {
	usage := map[string]any{
		"input_tokens":  13,
		"output_tokens": 9,
		"output_tokens_details": map[string]any{
			"reasoning_tokens": 4,
		},
	}

	prompt, completion, total, reasoning := tokensFromUsage(usage)
	if prompt != 13 || completion != 9 || total != 22 || reasoning != 4 {
		t.Fatalf("tokens = (%d, %d, %d, %d), want (13, 9, 22, 4)", prompt, completion, total, reasoning)
	}
}

func TestPersistentStatsAppendRecoverSkipBadLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats", "requests.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	existing := RequestRecord{
		ID:              7,
		Time:            time.Now().Add(-time.Minute),
		Protocol:        "openai",
		Method:          http.MethodPost,
		Path:            "/v1/chat/completions",
		Model:           "gpt-4.1",
		Status:          http.StatusOK,
		PromptTokens:    3,
		ReasoningTokens: 2,
		TotalTokens:     5,
	}
	raw, _ := json.Marshal(existing)
	if err := os.WriteFile(path, []byte(string(raw)+"\nnot-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stats := NewPersistentStats(10, path, nil)
	stats.Record(RequestRecord{
		Time:             time.Now(),
		Protocol:         "gemini",
		Method:           http.MethodPost,
		Path:             "/v1beta/models/gpt-4.1:generateContent",
		Model:            "gpt-4.1",
		Status:           http.StatusOK,
		CompletionTokens: 4,
		ReasoningTokens:  1,
		TotalTokens:      4,
	})

	recovered := NewPersistentStats(10, path, nil).Snapshot()
	if recovered.TotalRequests != 2 {
		t.Fatalf("total requests = %d, want 2", recovered.TotalRequests)
	}
	if recovered.ReasoningTokens != 3 {
		t.Fatalf("reasoning tokens = %d, want 3", recovered.ReasoningTokens)
	}
	if len(recovered.Recent) != 2 || recovered.Recent[0].Protocol != "gemini" {
		t.Fatalf("recent = %#v, want newest gemini first", recovered.Recent)
	}
}

func TestPersistentStatsRetentionAndMaxRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "requests.jsonl")
	now := time.Now()
	records := []RequestRecord{
		{ID: 1, Time: now.Add(-48 * time.Hour), Protocol: "openai", Status: http.StatusOK},
		{ID: 2, Time: now.Add(-3 * time.Hour), Protocol: "openai", Status: http.StatusOK},
		{ID: 3, Time: now.Add(-2 * time.Hour), Protocol: "openai", Status: http.StatusOK},
		{ID: 4, Time: now.Add(-1 * time.Hour), Protocol: "openai", Status: http.StatusOK},
	}
	var lines []string
	for _, record := range records {
		raw, _ := json.Marshal(record)
		lines = append(lines, string(raw))
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stats := NewStats(10)
	stats.path = path
	stats.retention = 24 * time.Hour
	stats.maxRecords = 2
	stats.load()

	snapshot := stats.Snapshot()
	if snapshot.TotalRequests != 2 {
		t.Fatalf("total requests = %d, want 2", snapshot.TotalRequests)
	}
	if snapshot.Recent[0].ID != 4 || snapshot.Recent[1].ID != 3 {
		t.Fatalf("recent ids = %d, %d; want 4, 3", snapshot.Recent[0].ID, snapshot.Recent[1].ID)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(raw), "\n"); got != 2 {
		t.Fatalf("persisted lines = %d, want 2; raw = %s", got, raw)
	}
}

func TestOpenAIRecordsReasoningTokensNonStreamAndStream(t *testing.T) {
	nonStream := newProxyIntegrationHandler(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"id":      "chatcmpl-test",
			"model":   "gpt-4.1",
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
			"usage":   usageWithReasoning(10, 6, 16, 5),
		})
	})
	serveOpenAIRequest(t, nonStream, false)
	if record := latestRecord(t, nonStream, "openai"); record.ReasoningTokens != 5 {
		t.Fatalf("non-stream reasoning = %d, want 5", record.ReasoningTokens)
	}

	stream := newProxyIntegrationHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"model":"gpt-4.1","choices":[{"delta":{"content":"ok"}}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"model":"gpt-4.1","choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":6,"total_tokens":16,"completion_tokens_details":{"reasoning_tokens":5}}}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})
	serveOpenAIRequest(t, stream, true)
	if record := latestRecord(t, stream, "openai"); record.ReasoningTokens != 5 {
		t.Fatalf("stream reasoning = %d, want 5", record.ReasoningTokens)
	}
}

func TestGeminiRecordsReasoningTokensNonStreamAndStream(t *testing.T) {
	nonStream := newProxyIntegrationHandler(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"id":      "chatcmpl-test",
			"model":   "gpt-4.1",
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
			"usage":   usageWithReasoning(8, 4, 12, 3),
		})
	})
	serveGeminiRequest(t, nonStream, "generateContent")
	if record := latestRecord(t, nonStream, "gemini"); record.ReasoningTokens != 3 {
		t.Fatalf("non-stream reasoning = %d, want 3", record.ReasoningTokens)
	}

	stream := newProxyIntegrationHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"model":"gpt-4.1","choices":[{"delta":{"content":"ok"}}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"model":"gpt-4.1","choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":4,"total_tokens":12,"completion_tokens_details":{"reasoning_tokens":3}}}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})
	serveGeminiRequest(t, stream, "streamGenerateContent")
	if record := latestRecord(t, stream, "gemini"); record.ReasoningTokens != 3 {
		t.Fatalf("stream reasoning = %d, want 3", record.ReasoningTokens)
	}
}

func TestAnthropicRecordsReasoningTokensStream(t *testing.T) {
	handler := newAnthropicIntegrationHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"model":"claude-sonnet-4.6","choices":[{"delta":{"content":"ok"}}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"model":"claude-sonnet-4.6","choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10,"completion_tokens_details":{"reasoning_tokens":2}}}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})

	resp := serveAnthropicTestRequest(t, handler, map[string]any{
		"model":      "claude-sonnet-4.6",
		"max_tokens": 64,
		"stream":     true,
		"messages":   []any{map[string]any{"role": "user", "content": "hello"}},
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.Code, resp.Body.String())
	}
	if record := latestAnthropicRecord(t, handler); record.ReasoningTokens != 2 {
		t.Fatalf("stream reasoning = %d, want 2", record.ReasoningTokens)
	}
}

func newProxyIntegrationHandler(t *testing.T, chatHandler http.HandlerFunc) *Handler {
	t.Helper()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/copilot-token":
			writeJSON(w, http.StatusOK, map[string]any{"token": "test-copilot-token", "expires_at": time.Now().Add(time.Hour).Unix()})
		case "/chat/completions":
			chatHandler(w, r)
		default:
			t.Fatalf("unexpected upstream path %q", r.URL.Path)
		}
	}))
	t.Cleanup(upstream.Close)

	cfg := config.Default()
	cfg.Copilot.APIBase = upstream.URL
	cfg.GitHub.CopilotTokenURL = upstream.URL + "/copilot-token"
	cfg.Auth.Accounts = []config.AccountConfig{{ID: "test", Name: "test", GitHubToken: "github-token"}}
	cfg.Auth.ActiveAccountID = "test"

	authManager := auth.NewManager(&cfg, filepath.Join(t.TempDir(), "config.json"), upstream.Client())
	if err := authManager.RefreshCopilotToken(context.Background()); err != nil {
		t.Fatalf("RefreshCopilotToken() error = %v", err)
	}

	return NewHandler(cfg, authManager, upstream.Client(), nil, NewStats(100))
}

func serveOpenAIRequest(t *testing.T, handler *Handler, stream bool) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"model":    "gpt-4.1",
		"stream":   stream,
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer dummy")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.Code, resp.Body.String())
	}
}

func serveGeminiRequest(t *testing.T, handler *Handler, action string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"contents": []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": "hello"}}}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/gpt-4.1:"+action, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", "dummy")
	resp := httptest.NewRecorder()
	handler.ServeGeminiModels(resp, req, "gpt-4.1:"+action)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.Code, resp.Body.String())
	}
}

func latestRecord(t *testing.T, handler *Handler, protocol string) RequestRecord {
	t.Helper()
	snapshot := handler.Stats()
	if len(snapshot.Recent) == 0 {
		t.Fatal("stats recent records are empty")
	}
	record := snapshot.Recent[0]
	if record.Protocol != protocol {
		t.Fatalf("record protocol = %q, want %s", record.Protocol, protocol)
	}
	return record
}

func usageWithReasoning(prompt, completion, total, reasoning int64) map[string]any {
	return map[string]any{
		"prompt_tokens":     prompt,
		"completion_tokens": completion,
		"total_tokens":      total,
		"completion_tokens_details": map[string]any{
			"reasoning_tokens": reasoning,
		},
	}
}
