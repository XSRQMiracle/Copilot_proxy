package proxy

import (
	"bufio"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	geminiModelPathPattern  = regexp.MustCompile(`(?:^|/)models/([^:]+)$`)
	geminiFinishReasonMap   = map[string]string{"stop": "STOP", "length": "MAX_TOKENS", "content_filter": "SAFETY"}
	validGeminiActionValues = map[string]struct{}{"generateContent": {}, "streamGenerateContent": {}}
)

func (h *Handler) ServeGeminiModels(w http.ResponseWriter, r *http.Request, geminiPath string) {
	start := time.Now()
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		geminiError(w, http.StatusMethodNotAllowed, "method not allowed")
		h.record(RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Status: http.StatusMethodNotAllowed, Error: "method not allowed"})
		return
	}
	if h.proxyDisabled() {
		geminiError(w, http.StatusServiceUnavailable, "proxy service is disabled")
		h.record(RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Status: http.StatusServiceUnavailable, Error: "proxy disabled"})
		return
	}
	if !h.authorized(r) {
		geminiError(w, http.StatusUnauthorized, "invalid api key")
		h.record(RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Status: http.StatusUnauthorized, Error: "invalid api key"})
		return
	}

	token := h.auth.CopilotToken()
	if token == "" {
		geminiError(w, http.StatusServiceUnavailable, "Copilot token 未就绪，请检查授权状态")
		h.record(RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Status: http.StatusServiceUnavailable, Error: "missing copilot token"})
		return
	}

	model, action, ok := parseGeminiPath(geminiPath)
	if !ok {
		geminiError(w, http.StatusBadRequest, "Invalid request path")
		h.record(RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Status: http.StatusBadRequest, Error: "invalid path"})
		return
	}

	var payload map[string]any
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		geminiError(w, http.StatusBadRequest, "Invalid JSON body")
		h.record(RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Model: model, Status: http.StatusBadRequest, Error: "invalid json"})
		return
	}
	if _, ok := payload["contents"]; !ok {
		geminiError(w, http.StatusBadRequest, "Missing required field: contents")
		h.record(RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Model: model, Status: http.StatusBadRequest, Error: "missing contents"})
		return
	}

	hasSafety := payload["safetySettings"] != nil
	oaiPayload := geminiToOpenAI(model, payload, action == "streamGenerateContent")
	if action == "streamGenerateContent" {
		h.serveGeminiStream(w, r, token, oaiPayload, hasSafety, start)
		return
	}
	h.serveGeminiGenerate(w, r, token, oaiPayload, hasSafety, start)
}

func parseGeminiPath(geminiPath string) (string, string, bool) {
	idx := strings.LastIndex(geminiPath, ":")
	if idx == -1 {
		return "", "", false
	}
	action := geminiPath[idx+1:]
	if _, ok := validGeminiActionValues[action]; !ok {
		return "", "", false
	}
	modelPart := geminiPath[:idx]
	model := modelPart
	if matches := geminiModelPathPattern.FindStringSubmatch(modelPart); len(matches) == 2 {
		model = matches[1]
	}
	if model == "" {
		return "", "", false
	}
	return model, action, true
}

func geminiToOpenAI(model string, payload map[string]any, stream bool) map[string]any {
	messages := make([]map[string]any, 0)
	if system, ok := payload["systemInstruction"].(map[string]any); ok {
		if text := geminiPartsToText(system["parts"]); text != "" {
			messages = append(messages, map[string]any{"role": "system", "content": text})
		}
	}
	if contents, ok := payload["contents"].([]any); ok {
		for _, item := range contents {
			content, ok := item.(map[string]any)
			if !ok {
				continue
			}
			role := stringValue(content["role"])
			if role == "model" {
				role = "assistant"
			}
			if role == "" {
				role = "user"
			}
			messages = append(messages, map[string]any{
				"role":    role,
				"content": geminiPartsToText(content["parts"]),
			})
		}
	}

	result := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   stream,
	}
	ensureStreamUsage(result)
	if gc, ok := payload["generationConfig"].(map[string]any); ok {
		copyIfPresent(result, gc, "maxOutputTokens", "max_tokens")
		copyIfPresent(result, gc, "temperature", "temperature")
		copyIfPresent(result, gc, "topP", "top_p")
		copyIfPresent(result, gc, "stopSequences", "stop")
	}
	return result
}

func geminiPartsToText(raw any) string {
	parts, ok := raw.([]any)
	if !ok {
		return ""
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		item, ok := part.(map[string]any)
		if !ok {
			continue
		}
		if text := stringValue(item["text"]); text != "" {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, "\n")
}

func (h *Handler) serveGeminiGenerate(w http.ResponseWriter, r *http.Request, token string, oaiPayload map[string]any, hasSafety bool, start time.Time) {
	body, err := json.Marshal(oaiPayload)
	if err != nil {
		geminiError(w, http.StatusBadRequest, err.Error())
		h.record(RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadRequest, Error: err.Error()})
		return
	}
	copilotURL := h.chatCompletionsURL()
	resp, err := h.forward(r.Context(), http.MethodPost, copilotURL, token, "application/json", body)
	if err != nil {
		geminiError(w, http.StatusBadGateway, err.Error())
		h.record(RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadGateway, Error: err.Error()})
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
		geminiError(w, resp.StatusCode, readErrorPreview(resp))
		h.record(RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: resp.StatusCode, Error: "upstream error"})
		return
	}

	var oaiData map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&oaiData); err != nil {
		geminiError(w, http.StatusBadGateway, err.Error())
		h.record(RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadGateway, Error: err.Error()})
		return
	}
	record := RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusOK, DurationMs: time.Since(start).Milliseconds()}
	if model := stringValue(oaiData["model"]); model != "" {
		record.Model = model
	}
	if usage, ok := oaiData["usage"].(map[string]any); ok {
		record.PromptTokens, record.CompletionTokens, record.TotalTokens = tokensFromUsage(usage)
	}
	h.record(record)
	if hasSafety {
		w.Header().Set("X-Gemini-Warning", "safety_settings_ignored")
	}
	writeJSON(w, http.StatusOK, openAIToGeminiResponse(oaiData))
}

func openAIToGeminiResponse(oaiData map[string]any) map[string]any {
	choice := firstChoice(oaiData)
	message, _ := choice["message"].(map[string]any)
	usage, _ := oaiData["usage"].(map[string]any)
	return map[string]any{
		"candidates": []map[string]any{
			{
				"content": map[string]any{
					"role":  "model",
					"parts": []map[string]any{{"text": stringValue(message["content"])}},
				},
				"finishReason":  geminiFinishReason(stringValue(choice["finish_reason"])),
				"safetyRatings": []any{},
				"index":         0,
			},
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     numberValue(usage["prompt_tokens"]),
			"candidatesTokenCount": numberValue(usage["completion_tokens"]),
			"totalTokenCount":      numberValue(usage["total_tokens"]),
		},
		"modelVersion": stringValue(oaiData["model"]),
	}
}

func (h *Handler) serveGeminiStream(w http.ResponseWriter, r *http.Request, token string, oaiPayload map[string]any, hasSafety bool, start time.Time) {
	body, err := json.Marshal(oaiPayload)
	if err != nil {
		geminiError(w, http.StatusBadRequest, err.Error())
		h.record(RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadRequest, Error: err.Error()})
		return
	}
	copilotURL := h.chatCompletionsURL()
	resp, err := h.forward(r.Context(), http.MethodPost, copilotURL, token, "application/json", body)
	if err != nil {
		geminiError(w, http.StatusBadGateway, err.Error())
		h.record(RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: http.StatusBadGateway, Error: err.Error()})
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
		geminiError(w, resp.StatusCode, readErrorPreview(resp))
		h.record(RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"]), Status: resp.StatusCode, Error: "upstream error"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	if hasSafety {
		w.Header().Set("X-Gemini-Warning", "safety_settings_ignored")
	}
	flusher, _ := w.(http.Flusher)
	streamRecord := RequestRecord{Time: start, Protocol: "gemini", Method: r.Method, Path: r.URL.Path, Model: stringValue(oaiPayload["model"])}
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
		choice := firstChoice(chunk)
		delta, _ := choice["delta"].(map[string]any)
		candidate := map[string]any{
			"index": 0,
		}
		if text := stringValue(delta["content"]); text != "" {
			candidate["content"] = map[string]any{"role": "model", "parts": []map[string]any{{"text": text}}}
		}
		if finish := stringValue(choice["finish_reason"]); finish != "" {
			candidate["finishReason"] = geminiFinishReason(finish)
		}
		raw, _ := json.Marshal(map[string]any{"candidates": []map[string]any{candidate}})
		w.Write([]byte("data: "))
		w.Write(raw)
		w.Write([]byte("\n\n"))
		flush(flusher)
	}
	streamRecord.Status = http.StatusOK
	streamRecord.DurationMs = time.Since(start).Milliseconds()
	h.record(streamRecord)
}

func geminiFinishReason(reason string) string {
	if mapped, ok := geminiFinishReasonMap[reason]; ok {
		return mapped
	}
	return "STOP"
}

func geminiError(w http.ResponseWriter, status int, message string) {
	geminiStatus := "INVALID_ARGUMENT"
	if status >= 500 {
		geminiStatus = "UNAVAILABLE"
	}
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    status,
			"message": message,
			"status":  geminiStatus,
		},
	})
}
