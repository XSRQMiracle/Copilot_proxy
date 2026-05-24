package proxy

import (
	"fmt"
	"strings"

	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/config"
)

// extractVendor 从 model 字符串中提取 vendor prefix（'/' 之前的部分）。
// 例如 "github-copilot/gpt-5-mini" → "github-copilot"，"gpt-4.1" → ""。
func extractVendor(model string) string {
	before, _, found := strings.Cut(model, "/")
	if !found {
		return ""
	}
	return strings.ToLower(before)
}

// IsModelAllowed 检查 model 是否被当前配置允许。
// 无 vendor 前缀的 model 视为允许（无法判断供应商）。
// cfg 为 nil 时视为允许（防御性编程）。
func IsModelAllowed(model string, cfg *config.Config) bool {
	if cfg == nil {
		return true
	}
	vendor := extractVendor(model)
	if vendor == "" {
		return true
	}
	return !cfg.IsVendorDenied(vendor)
}

// ModelNotAllowedError 返回禁止访问的错误消息。
func ModelNotAllowedError(model string) string {
	return fmt.Sprintf("model not allowed: %s", model)
}

// checkModelAllowed 是 Handler 辅助方法，返回 true 表示允许。
func (h *Handler) checkModelAllowed(model string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return IsModelAllowed(model, &h.cfg)
}
