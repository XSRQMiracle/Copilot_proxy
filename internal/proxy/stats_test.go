package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/config"
)

func TestTokensFromUsageAliases(t *testing.T) {
	prompt, completion, total := tokensFromUsage(map[string]any{
		"input_tokens":  float64(11),
		"output_tokens": float64(7),
	})
	if prompt != 11 || completion != 7 || total != 18 {
		t.Fatalf("tokens = (%d,%d,%d), want (11,7,18)", prompt, completion, total)
	}
}

func TestMergeStreamUsage(t *testing.T) {
	record := RequestRecord{}
	mergeStreamUsage(&record, map[string]any{
		"usage": map[string]any{"prompt_tokens": float64(3), "completion_tokens": float64(5)},
	})
	if record.PromptTokens != 3 || record.CompletionTokens != 5 || record.TotalTokens != 8 {
		t.Fatalf("record tokens = %+v", record)
	}
	mergeStreamUsage(&record, map[string]any{
		"usage": map[string]any{"prompt_tokens": float64(10), "completion_tokens": float64(20), "total_tokens": float64(30)},
	})
	if record.PromptTokens != 10 || record.CompletionTokens != 20 || record.TotalTokens != 30 {
		t.Fatalf("updated record tokens = %+v", record)
	}
}

func TestCleanBodyAddsStreamUsage(t *testing.T) {
	body := cleanBody([]byte(`{"model":"gpt-4.1","stream":true,"stream_options":{"foo":"bar"},"api_key":"secret"}`))
	if string(body) == "" {
		t.Fatal("expected non-empty body")
	}
	if string(body) == `{"model":"gpt-4.1","stream":true}` {
		t.Fatal("stream options were not preserved")
	}
	if !jsonContains(body, `"include_usage":true`) {
		t.Fatalf("expected include_usage in %s", body)
	}
	if jsonContains(body, "secret") {
		t.Fatalf("api key leaked in %s", body)
	}
}

func jsonContains(body []byte, text string) bool {
	return strings.Contains(string(body), text)
}

func testConfig() config.Config {
	return config.Default()
}

func TestCopyAndRecordResponseUsesActualResponseModel(t *testing.T) {
	stats := NewStats(10)
	handler := NewHandler(testConfig(), nil, nil, nil, nil, stats)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"model":"gpt-4o","usage":{"prompt_tokens":2,"completion_tokens":3}}`)),
	}
	recorder := httptest.NewRecorder()

	handler.copyAndRecordResponse(recorder, resp, RequestRecord{
		Model:    "unavailable-model",
		Protocol: "openai",
		Method:   http.MethodPost,
		Path:     "/v1/chat/completions",
	})

	snapshot := stats.Snapshot()
	if got := snapshot.Recent[0].Model; got != "gpt-4o" {
		t.Fatalf("recorded model = %q, want actual response model", got)
	}
	if _, ok := snapshot.ByModel["unavailable-model"]; ok {
		t.Fatal("stats should not be grouped under requested model after fallback")
	}
	if snapshot.ByModel["gpt-4o"].TotalTokens != 5 {
		t.Fatalf("gpt-4o usage = %+v", snapshot.ByModel["gpt-4o"])
	}
}

func TestCopyAndRecordStreamResponseUsesActualResponseModel(t *testing.T) {
	stats := NewStats(10)
	handler := NewHandler(testConfig(), nil, nil, nil, nil, stats)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader("data: {\"model\":\"gpt-4o\",\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":6}}\n\n" +
			"data: [DONE]\n\n")),
	}
	recorder := httptest.NewRecorder()

	handler.copyAndRecordResponse(recorder, resp, RequestRecord{
		Model:    "unavailable-model",
		Protocol: "openai",
		Method:   http.MethodPost,
		Path:     "/v1/chat/completions",
	})

	snapshot := stats.Snapshot()
	if got := snapshot.Recent[0].Model; got != "gpt-4o" {
		t.Fatalf("recorded stream model = %q, want actual response model", got)
	}
	if snapshot.ByModel["gpt-4o"].TotalTokens != 10 {
		t.Fatalf("gpt-4o stream usage = %+v", snapshot.ByModel["gpt-4o"])
	}
}
