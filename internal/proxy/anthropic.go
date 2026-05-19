package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var anthropicFinishReasonMap = map[string]string{
	"stop":           "end_turn",
	"length":         "max_tokens",
	"content_filter": "content_filter",
}

func (h *Handler) ServeAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Status: http.StatusMethodNotAllowed, Error: "method not allowed"})
		return
	}
	if h.proxyDisabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "proxy service is disabled"})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Status: http.StatusServiceUnavailable, Error: "proxy disabled"})
		return
	}
	if !h.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid api key"})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Status: http.StatusUnauthorized, Error: "invalid api key"})
		return
	}

	token := h.auth.CopilotToken()
	if token == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Copilot token 未就绪，请检查授权状态"})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Status: http.StatusServiceUnavailable, Error: "missing copilot token"})
		return
	}

	var payload map[string]any
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的 JSON 请求体"})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Status: http.StatusBadRequest, Error: "invalid json"})
		return
	}
	if _, ok := payload["messages"]; !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少必填字段: messages"})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(payload["model"]), Status: http.StatusBadRequest, Error: "missing messages"})
		return
	}

	oaiPayload := anthropicToOpenAI(payload)
	if stream, _ := payload["stream"].(bool); stream {
		oaiPayload["stream"] = true
		ensureStreamUsage(oaiPayload)
		h.serveAnthropicStream(w, r, token, oaiPayload, start)
		return
	}

	body, err := json.Marshal(oaiPayload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadRequest, Error: err.Error()})
		return
	}
	copilotURL := h.chatCompletionsURL()
	resp, err := h.forward(r.Context(), http.MethodPost, copilotURL, token, "application/json", body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadGateway, Error: err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		if retry, retryErr := h.tryFallback(r.Context(), http.MethodPost, copilotURL, token, "application/json", body, resp); retryErr == nil && retry != nil {
			resp.Body.Close()
			resp = retry
			defer resp.Body.Close()
		}
	}
	if resp.StatusCode != http.StatusOK {
		body := readErrorPreview(resp)
		anthropicError(w, resp.StatusCode, body)
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: resp.StatusCode, Error: body})
		return
	}

	var oaiData map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&oaiData); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadGateway, Error: err.Error()})
		return
	}
	record := RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusOK, DurationMs: time.Since(start).Milliseconds()}
	if model := stringValue(oaiData["model"]); model != "" {
		record.Model = model
	}
	if usage, ok := oaiData["usage"].(map[string]any); ok {
		record.PromptTokens, record.CompletionTokens, record.TotalTokens = tokensFromUsage(usage)
	}
	h.record(record)
	writeJSON(w, http.StatusOK, openAIToAnthropicResponse(oaiData))
}

func anthropicToOpenAI(payload map[string]any) map[string]any {
	messages := make([]map[string]any, 0)
	if system, ok := payload["system"]; ok && system != nil {
		if text := anthropicContentToText(system); text != "" {
			messages = append(messages, map[string]any{"role": "system", "content": text})
		}
	}
	if rawMessages, ok := payload["messages"].([]any); ok {
		for _, item := range rawMessages {
			msg, ok := item.(map[string]any)
			if !ok {
				continue
			}
			role, _ := msg["role"].(string)
			if role == "" {
				role = "user"
			}
			// 角色映射：Anthropic "assistant" → OpenAI "assistant"，其余映射为 "user"
			// 之前 Bug: 所有非 assistant 角色（包括正确的 "user"）都被强制映射成了 "user"，没有走这个分支
			if role != "user" && role != "assistant" {
				role = "user"
			}
			messages = append(messages, map[string]any{
				"role":    role,
				"content": anthropicContentToText(msg["content"]),
			})
		}
	}

	result := map[string]any{
		"model":      stringValue(payload["model"]),
		"messages":   messages,
		"max_tokens": valueOrDefault(payload, "max_tokens", float64(4096)),
		"stream":     valueOrDefault(payload, "stream", false),
	}
	copyIfPresent(result, payload, "temperature", "temperature")
	copyIfPresent(result, payload, "top_p", "top_p")
	copyIfPresent(result, payload, "stop_sequences", "stop")
	copyIfPresent(result, payload, "metadata", "metadata")
	return result
}

func anthropicContentToText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		texts := make([]string, 0, len(v))
		for _, item := range v {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if blockType, _ := block["type"].(string); blockType == "text" {
				texts = append(texts, stringValue(block["text"]))
			}
		}
		return strings.Join(texts, "\n")
	default:
		return ""
	}
}

func anthropicUsageFromOpenAIUsage(usage map[string]any) map[string]any {
	return map[string]any{
		"input_tokens":  numberValue(usage["prompt_tokens"]),
		"output_tokens": numberValue(usage["completion_tokens"]),
	}
}

func openAIToAnthropicResponse(oaiData map[string]any) map[string]any {
	choice := firstChoice(oaiData)
	message, _ := choice["message"].(map[string]any)
	content := stringValue(message["content"])
	finish := anthropicFinishReason(stringValue(choice["finish_reason"]))
	usage, _ := oaiData["usage"].(map[string]any)

	id := stringValue(oaiData["id"])
	if id == "" {
		id = randomHex(24)
	}

	return map[string]any{
		"id":            "msg_" + strings.TrimPrefix(id, "msg_"),
		"type":          "message",
		"role":          "assistant",
		"content":       []map[string]any{{"type": "text", "text": content}},
		"model":         stringValue(oaiData["model"]),
		"stop_reason":   finish,
		"stop_sequence": nil,
		"usage":         anthropicUsageFromOpenAIUsage(usage),
	}
}

func (h *Handler) serveAnthropicStream(w http.ResponseWriter, r *http.Request, token string, oaiPayload map[string]any, start time.Time) {
	body, err := json.Marshal(oaiPayload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadRequest, Error: err.Error()})
		return
	}
	copilotURL := h.chatCompletionsURL()
	resp, err := h.forward(r.Context(), http.MethodPost, copilotURL, token, "application/json", body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadGateway, Error: err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		if retry, retryErr := h.tryFallback(r.Context(), http.MethodPost, copilotURL, token, "application/json", body, resp); retryErr == nil && retry != nil {
			resp.Body.Close()
			resp = retry
			defer resp.Body.Close()
		}
	}
	if resp.StatusCode != http.StatusOK {
		anthropicError(w, resp.StatusCode, readErrorPreview(resp))
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: resp.StatusCode, Error: "upstream error"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, _ := w.(http.Flusher)

	messageID := "msg_" + randomHex(24)
	model := stringValue(oaiPayload["model"])
	started := false
	streamRecord := RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: model}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		mergeStreamUsage(&streamRecord, chunk)
		if model := stringValue(chunk["model"]); model != "" {
			streamRecord.Model = model
		}
		if !started {
			started = true
			writeSSE(w, "message_start", map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":            messageID,
					"type":          "message",
					"role":          "assistant",
					"content":       []any{},
					"model":         model,
					"stop_reason":   nil,
					"stop_sequence": nil,
					"usage":         map[string]any{"input_tokens": 0, "output_tokens": 0},
				},
			})
			writeSSE(w, "content_block_start", map[string]any{
				"type":          "content_block_start",
				"index":         0,
				"content_block": map[string]any{"type": "text", "text": ""},
			})
			flush(flusher)
		}
		choice := firstChoice(chunk)
		delta, _ := choice["delta"].(map[string]any)
		if text := stringValue(delta["content"]); text != "" {
			writeSSE(w, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]any{"type": "text_delta", "text": text},
			})
			flush(flusher)
		}
		if finish := stringValue(choice["finish_reason"]); finish != "" {
			writeSSE(w, "content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
			// 从上游 OpenAI chunk 中提取 usage 而非硬编码为 0
			usage := map[string]any{"prompt_tokens": streamRecord.PromptTokens, "completion_tokens": streamRecord.CompletionTokens}
			if streamRecord.PromptTokens == 0 && streamRecord.CompletionTokens == 0 {
				// 如果流中没有 usage 数据，检查最后一个 chunk 是否有 usage
				if chunkUsage, ok := chunk["usage"].(map[string]any); ok {
					if pt, ct, _ := tokensFromUsage(chunkUsage); pt > 0 || ct > 0 {
						usage = chunkUsage
					}
				}
			}
			writeSSE(w, "message_delta", map[string]any{
				"type":  "message_delta",
				"delta": map[string]any{"stop_reason": anthropicFinishReason(finish), "stop_sequence": nil},
				"usage": anthropicUsageFromOpenAIUsage(usage),
			})
			writeSSE(w, "message_stop", map[string]any{"type": "message_stop"})
			flush(flusher)
		}
	}
	if !started {
		writeSSE(w, "error", map[string]any{"type": "error", "error": map[string]any{"type": "api_error", "message": "上游无响应"}})
	}
	status := http.StatusOK
	errMsg := ""
	if !started {
		status = http.StatusServiceUnavailable
		errMsg = "empty upstream stream"
	}
	streamRecord.Status = status
	streamRecord.DurationMs = time.Since(start).Milliseconds()
	streamRecord.Error = errMsg
	h.record(streamRecord)
}

func anthropicFinishReason(reason string) string {
	if mapped, ok := anthropicFinishReasonMap[reason]; ok {
		return mapped
	}
	return "end_turn"
}

func anthropicError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    "api_error",
			"message": message,
		},
	})
}

func writeSSE(w http.ResponseWriter, event string, payload any) {
	raw, _ := json.Marshal(payload)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw)
}
