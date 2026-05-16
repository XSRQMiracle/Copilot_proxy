package proxy

import (
	"encoding/json"
	"testing"
)

func TestCleanBodyRemovesClientSecrets(t *testing.T) {
	raw := []byte(`{"model":"gpt-4.1","api_key":"secret","api_base":"http://example.test","messages":[]}`)
	cleaned := cleanBody(raw)

	var got map[string]any
	if err := json.Unmarshal(cleaned, &got); err != nil {
		t.Fatalf("cleaned body is not JSON: %v", err)
	}
	if _, ok := got["api_key"]; ok {
		t.Fatal("api_key was not removed")
	}
	if _, ok := got["api_base"]; ok {
		t.Fatal("api_base was not removed")
	}
	if got["model"] != "gpt-4.1" {
		t.Fatalf("model = %v", got["model"])
	}
}

func TestIsModelNotSupported(t *testing.T) {
	body := []byte(`{"error":{"code":"model_not_supported","message":"model unavailable"}}`)
	if !isModelNotSupported(400, body) {
		t.Fatal("expected model_not_supported body to be detected")
	}
	if isModelNotSupported(500, body) {
		t.Fatal("did not expect non-400 response to be detected")
	}
}

func TestExtractModelItemsSupportsDataAndModelsShapes(t *testing.T) {
	dataPayload := map[string]any{"data": []any{map[string]any{"id": "gpt-4.1"}}}
	if got := extractModelItems(dataPayload); len(got) != 1 || got[0]["id"] != "gpt-4.1" {
		t.Fatalf("extractModelItems(data) = %#v", got)
	}

	modelsPayload := map[string]any{"models": []any{map[string]any{"id": "gpt-4o"}}}
	if got := extractModelItems(modelsPayload); len(got) != 1 || got[0]["id"] != "gpt-4o" {
		t.Fatalf("extractModelItems(models) = %#v", got)
	}
}
