package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/auth"
	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/config"
)

func TestAnthropicToOpenAI_KnownModelNormalized(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		wantModel string
	}{
		{name: "canonical", model: "claude-sonnet-4.6", wantModel: "claude-sonnet-4.6"},
		{name: "dash alias", model: "claude-sonnet-4-6", wantModel: "claude-sonnet-4.6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]any{
				"model": tt.model,
				"messages": []any{
					map[string]any{"role": "user", "content": "hello"},
				},
			}

			got, err := anthropicToOpenAI(payload)
			if err != nil {
				t.Fatalf("anthropicToOpenAI() error = %v, want nil", err)
			}
			if got["model"] != tt.wantModel {
				t.Fatalf("model = %v, want %s", got["model"], tt.wantModel)
			}
		})
	}
}

func TestAnthropicToOpenAI_UnknownModelRejected(t *testing.T) {
	payload := map[string]any{
		"model": "not-a-claude-model",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	}

	_, err := anthropicToOpenAI(payload)
	if err == nil {
		t.Fatal("anthropicToOpenAI() error = nil, want unsupported model error")
	}
	if !strings.Contains(err.Error(), "unsupported Anthropic model: not-a-claude-model") {
		t.Fatalf("error = %q, want unsupported Anthropic model message", err.Error())
	}
}

func TestAnthropicToOpenAI_VendorPrefixedModelPassesThrough(t *testing.T) {
	payload := map[string]any{
		"model": "github-copilot/not-a-claude-model",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	}

	got, err := anthropicToOpenAI(payload)
	if err != nil {
		t.Fatalf("anthropicToOpenAI() error = %v, want nil", err)
	}
	if got["model"] != "github-copilot/not-a-claude-model" {
		t.Fatalf("model = %v, want vendor-prefixed model unchanged", got["model"])
	}
}

func TestAnthropicToOpenAI_ContentBlocks(t *testing.T) {
	tests := []struct {
		name        string
		payload     map[string]any
		wantContent string
		wantErr     string
	}{
		{
			name: "text content blocks are joined",
			payload: map[string]any{
				"model": "claude-sonnet-4.6",
				"messages": []any{
					map[string]any{"role": "user", "content": []any{
						map[string]any{"type": "text", "text": "hello"},
						map[string]any{"type": "text", "text": "world"},
					}},
				},
			},
			wantContent: "hello\nworld",
		},
		{
			name: "system string still works",
			payload: map[string]any{
				"model":  "claude-sonnet-4.6",
				"system": "be helpful",
				"messages": []any{
					map[string]any{"role": "user", "content": "hello"},
				},
			},
			wantContent: "be helpful",
		},
		{
			name: "message tool use block is rejected",
			payload: map[string]any{
				"model": "claude-sonnet-4.6",
				"messages": []any{
					map[string]any{"role": "assistant", "content": []any{
						map[string]any{"type": "tool_use", "id": "toolu_1", "name": "lookup"},
					}},
				},
			},
			wantErr: "unsupported Anthropic content type: tool_use",
		},
		{
			name: "system image block is rejected",
			payload: map[string]any{
				"model": "claude-sonnet-4.6",
				"system": []any{
					map[string]any{"type": "image", "source": map[string]any{"type": "base64"}},
				},
				"messages": []any{
					map[string]any{"role": "user", "content": "hello"},
				},
			},
			wantErr: "unsupported Anthropic content type: image",
		},
		{
			name: "string content block is rejected",
			payload: map[string]any{
				"model": "claude-sonnet-4.6",
				"messages": []any{
					map[string]any{"role": "user", "content": []any{"not an object"}},
				},
			},
			wantErr: "invalid Anthropic content block: expected object, got string",
		},
		{
			name: "content is a number (not string or array)",
			payload: map[string]any{
				"model": "claude-sonnet-4.6",
				"messages": []any{
					map[string]any{"role": "user", "content": 42},
				},
			},
			wantErr: "invalid Anthropic content",
		},
		{
			name: "text block with missing text field",
			payload: map[string]any{
				"model": "claude-sonnet-4.6",
				"messages": []any{
					map[string]any{"role": "user", "content": []any{
						map[string]any{"type": "text"},
					}},
				},
			},
			wantErr: "missing or not a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := anthropicToOpenAI(tt.payload)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("anthropicToOpenAI() error = nil, want content type error")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want to contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("anthropicToOpenAI() error = %v, want nil", err)
			}
			messages, ok := got["messages"].([]map[string]any)
			if !ok || len(messages) == 0 {
				t.Fatalf("messages = %#v, want non-empty []map[string]any", got["messages"])
			}
			if messages[0]["content"] != tt.wantContent {
				t.Fatalf("content = %v, want %q", messages[0]["content"], tt.wantContent)
			}
		})
	}
}

func TestAnthropicToOpenAI_InvalidMessagesShape(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
		wantErr string
	}{
		{
			name: "messages field string is rejected",
			payload: map[string]any{
				"model":    "claude-sonnet-4.6",
				"messages": "not an array",
			},
			wantErr: "invalid Anthropic messages: expected array, got string",
		},
		{
			name: "message string item is rejected",
			payload: map[string]any{
				"model": "claude-sonnet-4.6",
				"messages": []any{
					"not an object",
				},
			},
			wantErr: "invalid Anthropic message: expected object, got string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := anthropicToOpenAI(tt.payload)
			if err == nil {
				t.Fatal("anthropicToOpenAI() error = nil, want invalid messages error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestAnthropicStreamUpstreamErrorRecordsBodyAndDuration(t *testing.T) {
	const upstreamBody = `{"error":"unsupported model"}`
	handler := newAnthropicIntegrationHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("upstream path = %q, want /chat/completions", r.URL.Path)
		}
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(upstreamBody))
	})

	resp := serveAnthropicTestRequest(t, handler, map[string]any{
		"model":      "claude-sonnet-4.6",
		"max_tokens": 64,
		"stream":     true,
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	})

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body = %s", resp.Code, http.StatusUnprocessableEntity, resp.Body.String())
	}
	assertAnthropicErrorResponseBody(t, resp, upstreamBody)
	record := latestAnthropicRecord(t, handler)
	assertRecordedUpstreamError(t, record, http.StatusUnprocessableEntity, upstreamBody)
}

func TestAnthropicNonStreamUpstreamErrorRecordsBodyAndDuration(t *testing.T) {
	const upstreamBody = `{"error":"unsupported model"}`
	handler := newAnthropicIntegrationHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("upstream path = %q, want /chat/completions", r.URL.Path)
		}
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(upstreamBody))
	})

	resp := serveAnthropicTestRequest(t, handler, map[string]any{
		"model":      "claude-sonnet-4.6",
		"max_tokens": 64,
		"stream":     false,
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	})

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body = %s", resp.Code, http.StatusUnprocessableEntity, resp.Body.String())
	}
	assertAnthropicErrorResponseBody(t, resp, upstreamBody)
	record := latestAnthropicRecord(t, handler)
	assertRecordedUpstreamError(t, record, http.StatusUnprocessableEntity, upstreamBody)
}

func TestAnthropicNonStreamHappyPathRecordsDuration(t *testing.T) {
	handler := newAnthropicIntegrationHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("upstream path = %q, want /chat/completions", r.URL.Path)
		}
		time.Sleep(10 * time.Millisecond)
		writeJSON(w, http.StatusOK, map[string]any{
			"id":    "chatcmpl-test",
			"model": "claude-sonnet-4.6",
			"choices": []map[string]any{
				{
					"message":       map[string]any{"role": "assistant", "content": "hello from upstream"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{"prompt_tokens": 7, "completion_tokens": 3, "total_tokens": 10},
		})
	})

	resp := serveAnthropicTestRequest(t, handler, map[string]any{
		"model":      "claude-sonnet-4.6",
		"max_tokens": 64,
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", resp.Code, http.StatusOK, resp.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode Anthropic response: %v", err)
	}
	if body["type"] != "message" {
		t.Fatalf("type = %v, want message", body["type"])
	}
	content, ok := body["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("content = %#v, want one Anthropic text block", body["content"])
	}
	block, ok := content[0].(map[string]any)
	if !ok || block["type"] != "text" || block["text"] != "hello from upstream" {
		t.Fatalf("content block = %#v, want Anthropic text block", content[0])
	}

	record := latestAnthropicRecord(t, handler)
	if record.Status != http.StatusOK {
		t.Fatalf("record status = %d, want %d", record.Status, http.StatusOK)
	}
	if record.Error != "" {
		t.Fatalf("record error = %q, want empty", record.Error)
	}
	if record.DurationMs <= 0 {
		t.Fatalf("record DurationMs = %d, want > 0", record.DurationMs)
	}
	if record.PromptTokens != 7 || record.CompletionTokens != 3 || record.TotalTokens != 10 {
		t.Fatalf("record tokens = (%d, %d, %d), want (7, 3, 10)", record.PromptTokens, record.CompletionTokens, record.TotalTokens)
	}
}

func newAnthropicIntegrationHandler(t *testing.T, chatHandler http.HandlerFunc) *Handler {
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

	authManager := auth.NewManager(&cfg, t.TempDir()+"/config.json", upstream.Client())
	if err := authManager.RefreshCopilotToken(context.Background()); err != nil {
		t.Fatalf("RefreshCopilotToken() error = %v", err)
	}

	return NewHandler(cfg, authManager, upstream.Client(), nil, NewStats(100))
}

func serveAnthropicTestRequest(t *testing.T, handler *Handler, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "dummy")

	resp := httptest.NewRecorder()
	handler.ServeAnthropicMessages(resp, req)
	return resp
}

func latestAnthropicRecord(t *testing.T, handler *Handler) RequestRecord {
	t.Helper()

	snapshot := handler.Stats()
	if len(snapshot.Recent) == 0 {
		t.Fatal("stats recent records are empty")
	}
	record := snapshot.Recent[0]
	if record.Protocol != "anthropic" {
		t.Fatalf("record protocol = %q, want anthropic", record.Protocol)
	}
	return record
}

func assertAnthropicErrorResponseBody(t *testing.T, resp *httptest.ResponseRecorder, upstreamBody string) {
	t.Helper()

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode Anthropic error response: %v", err)
	}
	if body["type"] != "error" {
		t.Fatalf("error response type = %v, want error", body["type"])
	}
	errorSection, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("error response error section = %#v, want object", body["error"])
	}
	message, ok := errorSection["message"].(string)
	if !ok {
		t.Fatalf("error response message = %#v, want string", errorSection["message"])
	}
	if !strings.Contains(message, upstreamBody) {
		t.Fatalf("error response message = %q, want to contain upstream body %q", message, upstreamBody)
	}
	if !strings.Contains(message, "unsupported model") {
		t.Fatalf("error response message = %q, want to contain unsupported model", message)
	}
}

func assertRecordedUpstreamError(t *testing.T, record RequestRecord, status int, upstreamBody string) {
	t.Helper()

	if record.Status != status {
		t.Fatalf("record status = %d, want %d", record.Status, status)
	}
	if record.Error == "upstream error" {
		t.Fatal("record error used generic upstream error, want real upstream body")
	}
	if !strings.Contains(record.Error, upstreamBody) {
		t.Fatalf("record error = %q, want to contain upstream body %q", record.Error, upstreamBody)
	}
	if record.DurationMs <= 0 {
		t.Fatalf("record DurationMs = %d, want > 0", record.DurationMs)
	}
}
