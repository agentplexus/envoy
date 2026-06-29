package openai

import (
	"sort"
	"sync"
	"time"
)

// ToolUsageRecord represents a single tool invocation.
type ToolUsageRecord struct {
	ToolName  string    `json:"tool_name"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id,omitempty"`
	Latency   int64     `json:"latency_ms,omitempty"`
	Success   bool      `json:"success"`
}

// ToolUsageStats represents aggregated statistics for a tool.
type ToolUsageStats struct {
	ToolName    string    `json:"tool_name"`
	CallCount   int64     `json:"call_count"`
	LastUsed    time.Time `json:"last_used"`
	AvgLatency  float64   `json:"avg_latency_ms,omitempty"`
	SuccessRate float64   `json:"success_rate"`
}

// ToolUsageSummary provides overall tool usage statistics.
type ToolUsageSummary struct {
	TotalCalls int64                      `json:"total_calls"`
	ByTool     map[string]*ToolUsageStats `json:"by_tool"`
	TopTools   []*ToolUsageStats          `json:"top_tools"`
}

// ToolUsageStore stores tool usage records in memory.
type ToolUsageStore struct {
	records []ToolUsageRecord
	maxSize int
	mu      sync.RWMutex
}

// NewToolUsageStore creates a new tool usage store.
func NewToolUsageStore(maxSize int) *ToolUsageStore {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &ToolUsageStore{
		records: make([]ToolUsageRecord, 0, maxSize),
		maxSize: maxSize,
	}
}

// Record adds a tool usage record.
func (s *ToolUsageStore) Record(r ToolUsageRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove oldest records if at capacity
	if len(s.records) >= s.maxSize {
		removeCount := s.maxSize / 10
		s.records = s.records[removeCount:]
	}

	s.records = append(s.records, r)
}

// GetSummary returns aggregated tool usage statistics.
func (s *ToolUsageStore) GetSummary(since, until time.Time) *ToolUsageSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := &ToolUsageSummary{
		ByTool: make(map[string]*ToolUsageStats),
	}

	// Track latency sums and counts for averaging
	latencySums := make(map[string]int64)
	latencyCounts := make(map[string]int64)
	successCounts := make(map[string]int64)

	for _, r := range s.records {
		// Filter by time range
		if !since.IsZero() && r.Timestamp.Before(since) {
			continue
		}
		if !until.IsZero() && r.Timestamp.After(until) {
			continue
		}

		summary.TotalCalls++

		stats, ok := summary.ByTool[r.ToolName]
		if !ok {
			stats = &ToolUsageStats{ToolName: r.ToolName}
			summary.ByTool[r.ToolName] = stats
		}

		stats.CallCount++
		if r.Timestamp.After(stats.LastUsed) {
			stats.LastUsed = r.Timestamp
		}
		if r.Latency > 0 {
			latencySums[r.ToolName] += r.Latency
			latencyCounts[r.ToolName]++
		}
		if r.Success {
			successCounts[r.ToolName]++
		}
	}

	// Calculate averages and rates
	for name, stats := range summary.ByTool {
		if count := latencyCounts[name]; count > 0 {
			stats.AvgLatency = float64(latencySums[name]) / float64(count)
		}
		if stats.CallCount > 0 {
			stats.SuccessRate = float64(successCounts[name]) / float64(stats.CallCount)
		}
	}

	// Build top tools list sorted by call count
	for _, stats := range summary.ByTool {
		summary.TopTools = append(summary.TopTools, stats)
	}
	sort.Slice(summary.TopTools, func(i, j int) bool {
		return summary.TopTools[i].CallCount > summary.TopTools[j].CallCount
	})

	// Limit to top 10
	if len(summary.TopTools) > 10 {
		summary.TopTools = summary.TopTools[:10]
	}

	return summary
}

// GetToolStats returns statistics for a specific tool.
func (s *ToolUsageStore) GetToolStats(toolName string, since, until time.Time) *ToolUsageStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &ToolUsageStats{ToolName: toolName}
	var latencySum int64
	var latencyCount int64
	var successCount int64

	for _, r := range s.records {
		if r.ToolName != toolName {
			continue
		}
		if !since.IsZero() && r.Timestamp.Before(since) {
			continue
		}
		if !until.IsZero() && r.Timestamp.After(until) {
			continue
		}

		stats.CallCount++
		if r.Timestamp.After(stats.LastUsed) {
			stats.LastUsed = r.Timestamp
		}
		if r.Latency > 0 {
			latencySum += r.Latency
			latencyCount++
		}
		if r.Success {
			successCount++
		}
	}

	if latencyCount > 0 {
		stats.AvgLatency = float64(latencySum) / float64(latencyCount)
	}
	if stats.CallCount > 0 {
		stats.SuccessRate = float64(successCount) / float64(stats.CallCount)
	}

	return stats
}
