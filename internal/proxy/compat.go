package proxy

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
)

func firstChoice(payload map[string]any) map[string]any {
	choices, ok := payload["choices"].([]any)
	if !ok || len(choices) == 0 {
		return map[string]any{}
	}
	choice, _ := choices[0].(map[string]any)
	if choice == nil {
		return map[string]any{}
	}
	return choice
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func numberValue(value any) any {
	if value == nil {
		return 0
	}
	return value
}

func valueOrDefault(payload map[string]any, key string, fallback any) any {
	if value, ok := payload[key]; ok {
		return value
	}
	return fallback
}

func copyIfPresent(dst map[string]any, src map[string]any, srcKey string, dstKey string) {
	if value, ok := src[srcKey]; ok {
		dst[dstKey] = value
	}
}

func readErrorPreview(resp *http.Response) string {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
	return string(body)
}

func flush(flusher http.Flusher) {
	if flusher != nil {
		flusher.Flush()
	}
}

func randomHex(length int) string {
	if length <= 0 {
		return ""
	}
	raw := make([]byte, (length+1)/2)
	if _, err := rand.Read(raw); err != nil {
		return "000000000000000000000000"[:length]
	}
	return hex.EncodeToString(raw)[:length]
}
