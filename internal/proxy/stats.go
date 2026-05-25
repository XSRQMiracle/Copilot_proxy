package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	defaultStatsRetentionDays = 90
	defaultStatsMaxRecords    = 100000
	statsCompactEveryWrites   = 100
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
	ReasoningTokens  int64     `json:"reasoning_tokens"`
	TotalTokens      int64     `json:"total_tokens"`
	Error            string    `json:"error,omitempty"`
}

type ModelUsage struct {
	Requests         int64 `json:"requests"`
	Successes        int64 `json:"successes"`
	Failures         int64 `json:"failures"`
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	ReasoningTokens  int64 `json:"reasoning_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

type StatsSnapshot struct {
	TotalRequests    int64                 `json:"total_requests"`
	Successful       int64                 `json:"successful"`
	Failed           int64                 `json:"failed"`
	PromptTokens     int64                 `json:"prompt_tokens"`
	CompletionTokens int64                 `json:"completion_tokens"`
	ReasoningTokens  int64                 `json:"reasoning_tokens"`
	TotalTokens      int64                 `json:"total_tokens"`
	ByModel          map[string]ModelUsage `json:"by_model"`
	Recent           []RequestRecord       `json:"recent"`
}

type Stats struct {
	mu      sync.RWMutex
	nextID  int64
	max     int
	records []RequestRecord

	path               string
	logger             func(format string, args ...any)
	retention          time.Duration
	maxRecords         int
	writesSinceCompact int
}

func NewStats(max int) *Stats {
	if max <= 0 {
		max = 200
	}
	return &Stats{max: max}
}

func NewPersistentStats(maxRecent int, path string, logger func(format string, args ...any)) *Stats {
	stats := NewStats(maxRecent)
	stats.path = path
	stats.logger = logger
	stats.retention = defaultStatsRetentionDays * 24 * time.Hour
	stats.maxRecords = defaultStatsMaxRecords
	stats.load()
	return stats
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
	s.pruneLocked()
	s.persistLocked(record)
	s.writesSinceCompact++
	if s.path != "" && s.writesSinceCompact >= statsCompactEveryWrites {
		if err := s.rewriteLocked(); err != nil {
			s.logf("[!] stats compact failed: %v", err)
		} else {
			s.writesSinceCompact = 0
		}
	}
}

func (s *Stats) Snapshot() StatsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := StatsSnapshot{ByModel: map[string]ModelUsage{}, Recent: []RequestRecord{}}
	start := 0
	if s.max > 0 && len(s.records) > s.max {
		start = len(s.records) - s.max
	}
	snapshot.Recent = append(snapshot.Recent, s.records[start:]...)
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
		snapshot.ReasoningTokens += record.ReasoningTokens
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
		usage.ReasoningTokens += record.ReasoningTokens
		usage.TotalTokens += record.TotalTokens
		snapshot.ByModel[model] = usage
	}
	return snapshot
}

func tokensFromUsage(usage map[string]any) (int64, int64, int64, int64) {
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
	reasoning := int64FromAny(usage["reasoning_tokens"])
	if reasoning == 0 {
		reasoning = nestedInt64(usage, "completion_tokens_details", "reasoning_tokens")
	}
	if reasoning == 0 {
		reasoning = nestedInt64(usage, "output_tokens_details", "reasoning_tokens")
	}
	return prompt, completion, total, reasoning
}

func mergeStreamUsage(record *RequestRecord, chunk map[string]any) {
	usage, ok := chunk["usage"].(map[string]any)
	if !ok || len(usage) == 0 {
		return
	}
	applyUsage(record, usage)
}

func applyUsage(record *RequestRecord, usage map[string]any) {
	prompt, completion, total, reasoning := tokensFromUsage(usage)
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
	if reasoning > 0 {
		record.ReasoningTokens = reasoning
	}
}

func nestedInt64(usage map[string]any, objectKey, valueKey string) int64 {
	obj, ok := usage[objectKey].(map[string]any)
	if !ok {
		return 0
	}
	return int64FromAny(obj[valueKey])
}

func (s *Stats) load() {
	if s.path == "" {
		return
	}
	file, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		s.logf("[!] stats load failed: %v", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 10*1024*1024)
	now := time.Now()
	for scanner.Scan() {
		var record RequestRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			s.logf("[!] stats skipped malformed line: %v", err)
			continue
		}
		if record.Time.IsZero() {
			continue
		}
		record.Success = record.Status >= 200 && record.Status < 400 && record.Error == ""
		if s.retention > 0 && record.Time.Before(now.Add(-s.retention)) {
			continue
		}
		s.records = append(s.records, record)
		if record.ID > s.nextID {
			s.nextID = record.ID
		}
	}
	if err := scanner.Err(); err != nil {
		s.logf("[!] stats scan failed: %v", err)
	}
	s.pruneLocked()
	if err := s.rewriteLocked(); err != nil {
		s.logf("[!] stats startup compact failed: %v", err)
	}
}

func (s *Stats) pruneLocked() {
	if s.retention > 0 {
		cutoff := time.Now().Add(-s.retention)
		kept := s.records[:0]
		for _, record := range s.records {
			if !record.Time.Before(cutoff) {
				kept = append(kept, record)
			}
		}
		s.records = kept
	}
	if s.maxRecords > 0 && len(s.records) > s.maxRecords {
		s.records = append([]RequestRecord{}, s.records[len(s.records)-s.maxRecords:]...)
	}
}

func (s *Stats) persistLocked(record RequestRecord) {
	if s.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		s.logf("[!] stats mkdir failed: %v", err)
		return
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		s.logf("[!] stats append open failed: %v", err)
		return
	}
	defer file.Close()
	raw, err := json.Marshal(record)
	if err != nil {
		s.logf("[!] stats marshal failed: %v", err)
		return
	}
	if _, err := fmt.Fprintln(file, string(raw)); err != nil {
		s.logf("[!] stats append failed: %v", err)
	}
}

func (s *Stats) rewriteLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	for _, record := range s.records {
		if err := encoder.Encode(record); err != nil {
			_ = file.Close()
			_ = os.Remove(tmp)
			return err
		}
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Stats) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger(format, args...)
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
