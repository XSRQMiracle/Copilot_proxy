package proxy

import "testing"

func TestAnthropicToOpenAI(t *testing.T) {
	payload := map[string]any{
		"model":      "claude-sonnet-4.6",
		"system":     "You are concise.",
		"max_tokens": float64(128),
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "hello"},
					map[string]any{"type": "image", "source": "ignored"},
					map[string]any{"type": "text", "text": "world"},
				},
			},
		},
	}

	got := anthropicToOpenAI(payload)
	messages := got["messages"].([]map[string]any)
	if len(messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(messages))
	}
	if messages[0]["role"] != "system" || messages[0]["content"] != "You are concise." {
		t.Fatalf("system message = %#v", messages[0])
	}
	if messages[1]["role"] != "user" || messages[1]["content"] != "hello\nworld" {
		t.Fatalf("user message = %#v", messages[1])
	}
	if got["model"] != "claude-sonnet-4.6" {
		t.Fatalf("model = %v", got["model"])
	}
}

func TestOpenAIToAnthropicResponse(t *testing.T) {
	got := openAIToAnthropicResponse(map[string]any{
		"id":    "chatcmpl_123",
		"model": "gpt-4.1",
		"choices": []any{
			map[string]any{
				"message":       map[string]any{"content": "done"},
				"finish_reason": "length",
			},
		},
		"usage": map[string]any{"prompt_tokens": float64(3), "completion_tokens": float64(5)},
	})

	if got["type"] != "message" || got["role"] != "assistant" {
		t.Fatalf("bad anthropic envelope: %#v", got)
	}
	if got["stop_reason"] != "max_tokens" {
		t.Fatalf("stop_reason = %v", got["stop_reason"])
	}
}

func TestParseGeminiPath(t *testing.T) {
	model, action, ok := parseGeminiPath("publishers/google/models/gemini-pro:streamGenerateContent")
	if !ok {
		t.Fatal("expected path to parse")
	}
	if model != "gemini-pro" || action != "streamGenerateContent" {
		t.Fatalf("got model=%q action=%q", model, action)
	}

	if _, _, ok := parseGeminiPath("gemini-pro:badAction"); ok {
		t.Fatal("expected invalid action to fail")
	}
}

func TestGeminiToOpenAI(t *testing.T) {
	got := geminiToOpenAI("gemini-pro", map[string]any{
		"systemInstruction": map[string]any{"parts": []any{map[string]any{"text": "system"}}},
		"contents": []any{
			map[string]any{"role": "user", "parts": []any{map[string]any{"text": "hello"}}},
			map[string]any{"role": "model", "parts": []any{map[string]any{"text": "hi"}}},
		},
		"generationConfig": map[string]any{"maxOutputTokens": float64(100), "topP": float64(0.9)},
	}, true)

	messages := got["messages"].([]map[string]any)
	if len(messages) != 3 {
		t.Fatalf("messages len = %d, want 3", len(messages))
	}
	if messages[2]["role"] != "assistant" {
		t.Fatalf("model role was not converted: %#v", messages[2])
	}
	if got["model"] != "gemini-pro" || got["stream"] != true {
		t.Fatalf("bad model/stream: %#v", got)
	}
	if got["max_tokens"] != float64(100) || got["top_p"] != float64(0.9) {
		t.Fatalf("generation config not mapped: %#v", got)
	}
}

func TestOpenAIToGeminiResponse(t *testing.T) {
	got := openAIToGeminiResponse(map[string]any{
		"model": "gpt-4.1",
		"choices": []any{
			map[string]any{
				"message":       map[string]any{"content": "hello"},
				"finish_reason": "content_filter",
			},
		},
		"usage": map[string]any{"prompt_tokens": float64(1), "completion_tokens": float64(2), "total_tokens": float64(3)},
	})

	candidates := got["candidates"].([]map[string]any)
	if candidates[0]["finishReason"] != "SAFETY" {
		t.Fatalf("finishReason = %v", candidates[0]["finishReason"])
	}
	if got["modelVersion"] != "gpt-4.1" {
		t.Fatalf("modelVersion = %v", got["modelVersion"])
	}
}
