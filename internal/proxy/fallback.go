package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/config"
)

type FallbackSelector struct {
	cfg        config.Config
	httpClient *http.Client
	selected   string
}

func NewFallbackSelector(cfg config.Config, client *http.Client) *FallbackSelector {
	if client == nil {
		client = http.DefaultClient
	}
	return &FallbackSelector{cfg: cfg, httpClient: client}
}

func (s *FallbackSelector) Selected() string {
	return s.selected
}

func (s *FallbackSelector) Set(model string) {
	s.selected = model
}

func (s *FallbackSelector) UpdateConfig(cfg config.Config) {
	s.cfg = cfg
}

func (s *FallbackSelector) Choose(ctx context.Context, modelsURL string, headers map[string]string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("models endpoint returned %d", resp.StatusCode)
	}
	var payload any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	items := extractModelItems(payload)
	usable := func(item map[string]any) bool {
		return isEnabled(item) && isPickerEnabled(item) && supportsEndpoint(item, s.cfg.Fallback.RequiredEndpoint)
	}
	for _, pref := range s.cfg.Fallback.PreferredPrefixes {
		for _, item := range items {
			if id, _ := item["id"].(string); id == pref && usable(item) {
				s.selected = id
				return id, nil
			}
		}
		for _, item := range items {
			if matchesPrefix(item, pref) && usable(item) {
				id, _ := item["id"].(string)
				s.selected = id
				return id, nil
			}
		}
	}
	for _, item := range items {
		if usable(item) {
			id, _ := item["id"].(string)
			s.selected = id
			return id, nil
		}
	}
	return "", nil
}

func extractModelItems(payload any) []map[string]any {
	if root, ok := payload.(map[string]any); ok {
		if data, ok := root["data"].([]any); ok {
			return mapsFromAny(data)
		}
		if models, ok := root["models"].([]any); ok {
			return mapsFromAny(models)
		}
	}
	if items, ok := payload.([]any); ok {
		return mapsFromAny(items)
	}
	return nil
}

func mapsFromAny(items []any) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

func isEnabled(model map[string]any) bool {
	if id, _ := model["id"].(string); strings.HasPrefix(id, "claude-opus-") {
		return false
	}
	policy, _ := model["policy"].(map[string]any)
	state, _ := policy["state"].(string)
	return state == "" || state == "enabled"
}

func isPickerEnabled(model map[string]any) bool {
	enabled, ok := model["model_picker_enabled"].(bool)
	return !ok || enabled
}

func supportsEndpoint(model map[string]any, required string) bool {
	if required == "" {
		return true
	}
	raw, ok := model["supported_endpoints"].([]any)
	if !ok || len(raw) == 0 {
		return true
	}
	for _, item := range raw {
		if endpoint, ok := item.(string); ok && endpoint == required {
			return true
		}
	}
	return false
}

func matchesPrefix(model map[string]any, pref string) bool {
	pref = strings.ToLower(pref)
	for _, key := range []string{"id", "version", "name", "family"} {
		if value, ok := model[key].(string); ok && strings.HasPrefix(strings.ToLower(value), pref) {
			return true
		}
	}
	return false
}
