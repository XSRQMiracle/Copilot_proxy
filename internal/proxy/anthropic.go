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

var anthropicModelMap = map[string]string{
	"claude-opus-4-7":            "claude-opus-4.7",
	"claude-opus-4.7":            "claude-opus-4.7",
	"claude-sonnet-4-6":          "claude-sonnet-4.6",
	"claude-sonnet-4.6":          "claude-sonnet-4.6",
	"claude-opus-4-6":            "claude-opus-4.6",
	"claude-opus-4.6":            "claude-opus-4.6",
	"claude-haiku-4-5":           "claude-haiku-4.5",
	"claude-haiku-4.5":           "claude-haiku-4.5",
	"claude-haiku-4-5-20251001":  "claude-haiku-4.5",
	"claude-sonnet-4-5":          "claude-sonnet-4.5",
	"claude-sonnet-4.5":          "claude-sonnet-4.5",
	"claude-sonnet-4-5-20250929": "claude-sonnet-4.5",
	"claude-opus-4-5":            "claude-opus-4.5",
	"claude-opus-4.5":            "claude-opus-4.5",
	"claude-opus-4-5-20251101":   "claude-opus-4.5",
	"claude-opus-4-1":            "claude-opus-4.1",
	"claude-opus-4.1":            "claude-opus-4.1",
	"claude-opus-4-1-20250805":   "claude-opus-4.1",
	"claude-sonnet-4-0":          "claude-sonnet-4.0",
	"claude-sonnet-4.0":          "claude-sonnet-4.0",
	"claude-sonnet-4-20250514":   "claude-sonnet-4.0",
	"claude-opus-4-0":            "claude-opus-4.0",
	"claude-opus-4.0":            "claude-opus-4.0",
	"claude-opus-4-20250514":     "claude-opus-4.0",
	"claude-3-7-sonnet-latest":   "claude-3.7-sonnet",
	"claude-3-7-sonnet-20250219": "claude-3.7-sonnet",
	"claude-3.7-sonnet":          "claude-3.7-sonnet",
	"claude-3-5-sonnet-latest":   "claude-3.5-sonnet",
	"claude-3-5-sonnet-20241022": "claude-3.5-sonnet",
	"claude-3-5-sonnet-20240620": "claude-3.5-sonnet",
	"claude-3.5-sonnet":          "claude-3.5-sonnet",
	"claude-3-5-haiku-latest":    "claude-3.5-haiku",
	"claude-3-5-haiku-20241022":  "claude-3.5-haiku",
	"claude-3-5-haiku":           "claude-3.5-haiku",
	"claude-3.5-haiku":           "claude-3.5-haiku",
	"claude-3-opus-latest":       "claude-3-opus",
	"claude-3-opus-20240229":     "claude-3-opus",
	"claude-3-opus":              "claude-3-opus",
	"claude-3-sonnet-20240229":   "claude-3-sonnet",
	"claude-3-sonnet":            "claude-3-sonnet",
	"claude-3-haiku-20240307":    "claude-3-haiku",
	"claude-3-haiku":             "claude-3-haiku",
}

func (h *Handler) ServeAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Status: http.StatusMethodNotAllowed, DurationMs: time.Since(start).Milliseconds(), Error: "method not allowed"})
		return
	}
	if h.proxyDisabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "proxy service is disabled"})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Status: http.StatusServiceUnavailable, DurationMs: time.Since(start).Milliseconds(), Error: "proxy disabled"})
		return
	}
	if !h.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid api key"})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Status: http.StatusUnauthorized, DurationMs: time.Since(start).Milliseconds(), Error: "invalid api key"})
		return
	}

	token := h.auth.CopilotToken()
	if token == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Copilot token 未就绪，请检查授权状态"})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Status: http.StatusServiceUnavailable, DurationMs: time.Since(start).Milliseconds(), Error: "missing copilot token"})
		return
	}

	var payload map[string]any
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的 JSON 请求体"})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Status: http.StatusBadRequest, DurationMs: time.Since(start).Milliseconds(), Error: "invalid json"})
		return
	}
	if _, ok := payload["messages"]; !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少必填字段: messages"})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(payload["model"]), Status: http.StatusBadRequest, DurationMs: time.Since(start).Milliseconds(), Error: "missing messages"})
		return
	}

	model := stringValue(payload["model"])
	if !h.checkModelAllowed(model) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": ModelNotAllowedError(model)})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: model, Status: http.StatusForbidden, DurationMs: time.Since(start).Milliseconds(), Error: ModelNotAllowedError(model)})
		return
	}

	oaiPayload, err := anthropicToOpenAI(payload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: model, Status: http.StatusBadRequest, DurationMs: time.Since(start).Milliseconds(), Error: err.Error()})
		return
	}
	if stream, _ := payload["stream"].(bool); stream {
		oaiPayload["stream"] = true
		ensureStreamUsage(oaiPayload)
		h.serveAnthropicStream(w, r, token, oaiPayload, start)
		return
	}

	body, err := json.Marshal(oaiPayload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadRequest, DurationMs: time.Since(start).Milliseconds(), Error: err.Error()})
		return
	}
	copilotURL := h.chatCompletionsURL()
	resp, err := h.forward(r.Context(), http.MethodPost, copilotURL, token, "application/json", body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadGateway, DurationMs: time.Since(start).Milliseconds(), Error: err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readErrorPreview(resp)
		anthropicError(w, resp.StatusCode, body)
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: resp.StatusCode, DurationMs: time.Since(start).Milliseconds(), Error: body})
		return
	}

	var oaiData map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&oaiData); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadGateway, DurationMs: time.Since(start).Milliseconds(), Error: err.Error()})
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

func anthropicToOpenAI(payload map[string]any) (map[string]any, error) {
	model, err := normalizeAnthropicModel(stringValue(payload["model"]))
	if err != nil {
		return nil, err
	}

	messages := make([]map[string]any, 0)
	if system, ok := payload["system"]; ok && system != nil {
		text, err := anthropicContentToText(system)
		if err != nil {
			return nil, err
		}
		if text != "" {
			messages = append(messages, map[string]any{"role": "system", "content": text})
		}
	}
	rawMessages, ok := payload["messages"].([]any)
	if !ok {
		return nil, fmt.Errorf("invalid Anthropic messages: expected array, got %T", payload["messages"])
	}
	for _, item := range rawMessages {
		msg, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Anthropic message: expected object, got %T", item)
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
		content, err := anthropicContentToText(msg["content"])
		if err != nil {
			return nil, err
		}
		messages = append(messages, map[string]any{
			"role":    role,
			"content": content,
		})
	}

	result := map[string]any{
		"model":      model,
		"messages":   messages,
		"max_tokens": valueOrDefault(payload, "max_tokens", float64(4096)),
		"stream":     valueOrDefault(payload, "stream", false),
	}
	copyIfPresent(result, payload, "temperature", "temperature")
	copyIfPresent(result, payload, "top_p", "top_p")
	copyIfPresent(result, payload, "stop_sequences", "stop")
	copyIfPresent(result, payload, "metadata", "metadata")
	return result, nil
}

func normalizeAnthropicModel(model string) (string, error) {
	if extractVendor(model) != "" {
		return model, nil
	}
	canonical, ok := anthropicModelMap[strings.ToLower(model)]
	if !ok {
		return "", fmt.Errorf("unsupported Anthropic model: %s", model)
	}
	return canonical, nil
}

func anthropicContentToText(content any) (string, error) {
	switch v := content.(type) {
	case string:
		return v, nil
	case []any:
		texts := make([]string, 0, len(v))
		for _, item := range v {
			block, ok := item.(map[string]any)
			if !ok {
				return "", fmt.Errorf("invalid Anthropic content block: expected object, got %T", item)
			}
			blockType, _ := block["type"].(string)
			if blockType != "text" {
				return "", fmt.Errorf("unsupported Anthropic content type: %s", blockType)
			}
			textVal, ok := block["text"].(string)
			if !ok {
				return "", fmt.Errorf("invalid Anthropic text block: 'text' field is missing or not a string")
			}
			texts = append(texts, textVal)
		}
		return strings.Join(texts, "\n"), nil
	default:
		return "", fmt.Errorf("invalid Anthropic content: expected string or array, got %T", v)
	}
}

func anthropicUsageFromOpenAIUsage(usage map[string]any) map[string]any {
	return map[string]any{
		"input_tokens":  orZero(usage["prompt_tokens"]),
		"output_tokens": orZero(usage["completion_tokens"]),
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
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadRequest, DurationMs: time.Since(start).Milliseconds(), Error: err.Error()})
		return
	}
	copilotURL := h.chatCompletionsURL()
	resp, err := h.forward(r.Context(), http.MethodPost, copilotURL, token, "application/json", body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadGateway, DurationMs: time.Since(start).Milliseconds(), Error: err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readErrorPreview(resp)
		anthropicError(w, resp.StatusCode, body)
		h.record(RequestRecord{Time: start, Protocol: "anthropic", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: resp.StatusCode, DurationMs: time.Since(start).Milliseconds(), Error: body})
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
