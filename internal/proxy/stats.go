package proxy

import (
	"sync"
	"time"
)

type RequestRecord struct {
	ID               int64     `json:"id"`
	Time             time.Time `json:"time"`
	Protocol         string    `json:"protocol"`
	Method           string    `json:"method"`
	Path             string    `json:"path"`
	Model            string    `json:"model"`
	Status           int       `json:"status"`
	Success          bool      `json:"success"`
	DurationMs       int64     `json:"duration_ms"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	TotalTokens      int64     `json:"total_tokens"`
	Error            string    `json:"error,omitempty"`
}

type ModelUsage struct {
	Requests         int64 `json:"requests"`
	Successes        int64 `json:"successes"`
	Failures         int64 `json:"failures"`
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

type StatsSnapshot struct {
	TotalRequests    int64                 `json:"total_requests"`
	Successful       int64                 `json:"successful"`
	Failed           int64                 `json:"failed"`
	PromptTokens     int64                 `json:"prompt_tokens"`
	CompletionTokens int64                 `json:"completion_tokens"`
	TotalTokens      int64                 `json:"total_tokens"`
	ByModel          map[string]ModelUsage `json:"by_model"`
	Recent           []RequestRecord       `json:"recent"`
}

type Stats struct {
	mu      sync.RWMutex
	nextID  int64
	max     int
	records []RequestRecord
}

func NewStats(max int) *Stats {
	if max <= 0 {
		max = 200
	}
	return &Stats{max: max}
}

func (s *Stats) Record(record RequestRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	record.ID = s.nextID
	if record.Time.IsZero() {
		record.Time = time.Now()
	}
	record.Success = record.Status >= 200 && record.Status < 400 && record.Error == ""
	s.records = append(s.records, record)
	if len(s.records) > s.max {
		s.records = s.records[len(s.records)-s.max:]
	}
}

func (s *Stats) Snapshot() StatsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := StatsSnapshot{ByModel: map[string]ModelUsage{}, Recent: []RequestRecord{}}
	snapshot.Recent = append(snapshot.Recent, s.records...)
	for i := 0; i < len(snapshot.Recent)/2; i++ {
		j := len(snapshot.Recent) - 1 - i
		snapshot.Recent[i], snapshot.Recent[j] = snapshot.Recent[j], snapshot.Recent[i]
	}
	for _, record := range s.records {
		snapshot.TotalRequests++
		if record.Success {
			snapshot.Successful++
		} else {
			snapshot.Failed++
		}
		snapshot.PromptTokens += record.PromptTokens
		snapshot.CompletionTokens += record.CompletionTokens
		snapshot.TotalTokens += record.TotalTokens
		model := record.Model
		if model == "" {
			model = "unknown"
		}
		usage := snapshot.ByModel[model]
		usage.Requests++
		if record.Success {
			usage.Successes++
		} else {
			usage.Failures++
		}
		usage.PromptTokens += record.PromptTokens
		usage.CompletionTokens += record.CompletionTokens
		usage.TotalTokens += record.TotalTokens
		snapshot.ByModel[model] = usage
	}
	return snapshot
}

func tokensFromUsage(usage map[string]any) (int64, int64, int64) {
	prompt := int64FromAny(usage["prompt_tokens"])
	if prompt == 0 {
		prompt = int64FromAny(usage["input_tokens"])
	}
	completion := int64FromAny(usage["completion_tokens"])
	if completion == 0 {
		completion = int64FromAny(usage["output_tokens"])
	}
	total := int64FromAny(usage["total_tokens"])
	if total == 0 {
		total = prompt + completion
	}
	return prompt, completion, total
}

func mergeStreamUsage(record *RequestRecord, chunk map[string]any) {
	usage, ok := chunk["usage"].(map[string]any)
	if !ok || len(usage) == 0 {
		return
	}
	prompt, completion, total := tokensFromUsage(usage)
	if prompt > 0 {
		record.PromptTokens = prompt
	}
	if completion > 0 {
		record.CompletionTokens = completion
	}
	if total > 0 {
		record.TotalTokens = total
	} else if record.PromptTokens > 0 || record.CompletionTokens > 0 {
		record.TotalTokens = record.PromptTokens + record.CompletionTokens
	}
}

func int64FromAny(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case jsonNumber:
		n, _ := v.Int64()
		return n
	default:
		return 0
	}
}

type jsonNumber interface {
	Int64() (int64, error)
}
