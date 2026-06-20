package openai

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/grokify/echartify/chartir"
)

// UsageRecord represents a single usage event.
type UsageRecord struct {
	ID               string    `json:"id"`
	Timestamp        time.Time `json:"timestamp"`
	Model            string    `json:"model"`
	AgentID          string    `json:"agent_id,omitempty"`
	SessionID        string    `json:"session_id,omitempty"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	Cost             float64   `json:"cost"`
	Latency          int64     `json:"latency_ms"`
}

// UsageSummary provides aggregated usage statistics.
type UsageSummary struct {
	TotalRequests     int64                  `json:"total_requests"`
	TotalPromptTokens int64                  `json:"total_prompt_tokens"`
	TotalCompTokens   int64                  `json:"total_completion_tokens"`
	TotalTokens       int64                  `json:"total_tokens"`
	TotalCost         float64                `json:"total_cost"`
	AvgLatency        float64                `json:"avg_latency_ms"`
	ByModel           map[string]*ModelUsage `json:"by_model"`
	ByAgent           map[string]*AgentUsage `json:"by_agent"`
	PeriodStart       time.Time              `json:"period_start"`
	PeriodEnd         time.Time              `json:"period_end"`
}

// ModelUsage tracks usage per model.
type ModelUsage struct {
	Model            string  `json:"model"`
	Requests         int64   `json:"requests"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	Cost             float64 `json:"cost"`
}

// AgentUsage tracks usage per agent.
type AgentUsage struct {
	AgentID          string  `json:"agent_id"`
	Requests         int64   `json:"requests"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	Cost             float64 `json:"cost"`
}

// UsageTimeseries represents time-bucketed usage data.
type UsageTimeseries struct {
	Interval string        `json:"interval"` // "hour", "day"
	Buckets  []UsageBucket `json:"buckets"`
}

// UsageBucket represents usage for a single time bucket.
type UsageBucket struct {
	Timestamp        time.Time `json:"timestamp"`
	Requests         int64     `json:"requests"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	TotalTokens      int64     `json:"total_tokens"`
	Cost             float64   `json:"cost"`
}

// ModelPricing defines token costs per model.
type ModelPricing struct {
	PromptPer1K     float64
	CompletionPer1K float64
}

// Default pricing for common models (per 1K tokens).
var defaultPricing = map[string]ModelPricing{
	"claude-sonnet-4-20250514":   {PromptPer1K: 0.003, CompletionPer1K: 0.015},
	"claude-3-5-sonnet-20241022": {PromptPer1K: 0.003, CompletionPer1K: 0.015},
	"claude-3-opus-20240229":     {PromptPer1K: 0.015, CompletionPer1K: 0.075},
	"claude-3-haiku-20240307":    {PromptPer1K: 0.00025, CompletionPer1K: 0.00125},
	"gpt-4":                      {PromptPer1K: 0.03, CompletionPer1K: 0.06},
	"gpt-4-turbo":                {PromptPer1K: 0.01, CompletionPer1K: 0.03},
	"gpt-3.5-turbo":              {PromptPer1K: 0.0005, CompletionPer1K: 0.0015},
}

// CalculateCost estimates the cost based on model and tokens.
func CalculateCost(model string, promptTokens, completionTokens int) float64 {
	pricing, ok := defaultPricing[model]
	if !ok {
		// Default pricing for unknown models
		pricing = ModelPricing{PromptPer1K: 0.001, CompletionPer1K: 0.002}
	}

	promptCost := float64(promptTokens) / 1000.0 * pricing.PromptPer1K
	completionCost := float64(completionTokens) / 1000.0 * pricing.CompletionPer1K

	return promptCost + completionCost
}

// UsageStore stores usage records in memory.
type UsageStore struct {
	records []UsageRecord
	maxSize int
	mu      sync.RWMutex
}

// NewUsageStore creates a new usage store.
func NewUsageStore(maxSize int) *UsageStore {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &UsageStore{
		records: make([]UsageRecord, 0, maxSize),
		maxSize: maxSize,
	}
}

// Record adds a usage record.
func (s *UsageStore) Record(r UsageRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove oldest records if at capacity
	if len(s.records) >= s.maxSize {
		// Remove oldest 10%
		removeCount := s.maxSize / 10
		s.records = s.records[removeCount:]
	}

	s.records = append(s.records, r)
}

// GetSummary returns aggregated usage statistics.
func (s *UsageStore) GetSummary(since, until time.Time) *UsageSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := &UsageSummary{
		ByModel:     make(map[string]*ModelUsage),
		ByAgent:     make(map[string]*AgentUsage),
		PeriodStart: since,
		PeriodEnd:   until,
	}

	var totalLatency int64

	for _, r := range s.records {
		// Filter by time range
		if !since.IsZero() && r.Timestamp.Before(since) {
			continue
		}
		if !until.IsZero() && r.Timestamp.After(until) {
			continue
		}

		summary.TotalRequests++
		summary.TotalPromptTokens += int64(r.PromptTokens)
		summary.TotalCompTokens += int64(r.CompletionTokens)
		summary.TotalTokens += int64(r.TotalTokens)
		summary.TotalCost += r.Cost
		totalLatency += r.Latency

		// By model
		if _, ok := summary.ByModel[r.Model]; !ok {
			summary.ByModel[r.Model] = &ModelUsage{Model: r.Model}
		}
		m := summary.ByModel[r.Model]
		m.Requests++
		m.PromptTokens += int64(r.PromptTokens)
		m.CompletionTokens += int64(r.CompletionTokens)
		m.TotalTokens += int64(r.TotalTokens)
		m.Cost += r.Cost

		// By agent
		if r.AgentID != "" {
			if _, ok := summary.ByAgent[r.AgentID]; !ok {
				summary.ByAgent[r.AgentID] = &AgentUsage{AgentID: r.AgentID}
			}
			a := summary.ByAgent[r.AgentID]
			a.Requests++
			a.PromptTokens += int64(r.PromptTokens)
			a.CompletionTokens += int64(r.CompletionTokens)
			a.TotalTokens += int64(r.TotalTokens)
			a.Cost += r.Cost
		}
	}

	if summary.TotalRequests > 0 {
		summary.AvgLatency = float64(totalLatency) / float64(summary.TotalRequests)
	}

	return summary
}

// GetTimeseries returns time-bucketed usage data.
func (s *UsageStore) GetTimeseries(since, until time.Time, interval string) *UsageTimeseries {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Determine bucket duration
	var bucketDuration time.Duration
	switch interval {
	case "hour":
		bucketDuration = time.Hour
	case "day":
		bucketDuration = 24 * time.Hour
	default:
		interval = "hour"
		bucketDuration = time.Hour
	}

	// Create buckets map
	buckets := make(map[int64]*UsageBucket)

	for _, r := range s.records {
		// Filter by time range
		if !since.IsZero() && r.Timestamp.Before(since) {
			continue
		}
		if !until.IsZero() && r.Timestamp.After(until) {
			continue
		}

		// Calculate bucket key (truncate to bucket boundary)
		bucketKey := r.Timestamp.Truncate(bucketDuration).Unix()

		if _, ok := buckets[bucketKey]; !ok {
			buckets[bucketKey] = &UsageBucket{
				Timestamp: time.Unix(bucketKey, 0),
			}
		}

		b := buckets[bucketKey]
		b.Requests++
		b.PromptTokens += int64(r.PromptTokens)
		b.CompletionTokens += int64(r.CompletionTokens)
		b.TotalTokens += int64(r.TotalTokens)
		b.Cost += r.Cost
	}

	// Convert to sorted slice
	result := &UsageTimeseries{
		Interval: interval,
		Buckets:  make([]UsageBucket, 0, len(buckets)),
	}

	for _, b := range buckets {
		result.Buckets = append(result.Buckets, *b)
	}

	// Sort by timestamp
	sort.Slice(result.Buckets, func(i, j int) bool {
		return result.Buckets[i].Timestamp.Before(result.Buckets[j].Timestamp)
	})

	return result
}

// GetRecords returns recent usage records.
func (s *UsageStore) GetRecords(limit int) []UsageRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.records) {
		limit = len(s.records)
	}

	// Return most recent records
	start := len(s.records) - limit
	if start < 0 {
		start = 0
	}

	result := make([]UsageRecord, limit)
	copy(result, s.records[start:])

	return result
}

// Clear removes all records.
func (s *UsageStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = s.records[:0]
}

// Count returns the number of records.
func (s *UsageStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// GenerateTokensChart generates a ChartIR for token usage over time.
func GenerateTokensChart(ts *UsageTimeseries) *chartir.ChartIR {
	// Build dataset rows
	rows := make([][]string, len(ts.Buckets))
	for i, b := range ts.Buckets {
		var timeLabel string
		if ts.Interval == "hour" {
			timeLabel = b.Timestamp.Format("15:04")
		} else {
			timeLabel = b.Timestamp.Format("Jan 2")
		}
		rows[i] = []string{
			timeLabel,
			formatInt64(b.PromptTokens),
			formatInt64(b.CompletionTokens),
		}
	}

	return &chartir.ChartIR{
		Title: "Token Usage",
		Datasets: []chartir.Dataset{
			{
				ID: "tokens",
				Columns: []chartir.Column{
					{Name: "time", Type: chartir.ColumnTypeString},
					{Name: "prompt", Type: chartir.ColumnTypeNumber},
					{Name: "completion", Type: chartir.ColumnTypeNumber},
				},
				Rows: rows,
			},
		},
		Marks: []chartir.Mark{
			{
				ID:        "prompt-bar",
				DatasetID: "tokens",
				Geometry:  chartir.GeometryBar,
				Encode: chartir.Encode{
					X: "time",
					Y: "prompt",
				},
				Stack: "tokens",
				Name:  "Prompt Tokens",
				Style: &chartir.Style{
					Color: "#5470c6",
				},
			},
			{
				ID:        "completion-bar",
				DatasetID: "tokens",
				Geometry:  chartir.GeometryBar,
				Encode: chartir.Encode{
					X: "time",
					Y: "completion",
				},
				Stack: "tokens",
				Name:  "Completion Tokens",
				Style: &chartir.Style{
					Color: "#91cc75",
				},
			},
		},
		Axes: []chartir.Axis{
			{
				ID:       "x",
				Type:     chartir.AxisTypeCategory,
				Position: chartir.AxisPositionBottom,
			},
			{
				ID:       "y",
				Type:     chartir.AxisTypeValue,
				Position: chartir.AxisPositionLeft,
				Name:     "Tokens",
			},
		},
		Legend: &chartir.Legend{
			Show:     true,
			Position: chartir.LegendPositionTop,
		},
		Tooltip: &chartir.Tooltip{
			Show:    true,
			Trigger: chartir.TooltipTriggerAxis,
		},
		Grid: &chartir.Grid{
			Left:         "10%",
			Right:        "10%",
			Bottom:       "15%",
			ContainLabel: true,
		},
	}
}

// GenerateCostChart generates a ChartIR for cost over time.
func GenerateCostChart(ts *UsageTimeseries) *chartir.ChartIR {
	// Build dataset rows
	rows := make([][]string, len(ts.Buckets))
	for i, b := range ts.Buckets {
		var timeLabel string
		if ts.Interval == "hour" {
			timeLabel = b.Timestamp.Format("15:04")
		} else {
			timeLabel = b.Timestamp.Format("Jan 2")
		}
		rows[i] = []string{
			timeLabel,
			formatFloat64(b.Cost),
		}
	}

	return &chartir.ChartIR{
		Title: "Cost Over Time",
		Datasets: []chartir.Dataset{
			{
				ID: "cost",
				Columns: []chartir.Column{
					{Name: "time", Type: chartir.ColumnTypeString},
					{Name: "cost", Type: chartir.ColumnTypeNumber},
				},
				Rows: rows,
			},
		},
		Marks: []chartir.Mark{
			{
				ID:        "cost-line",
				DatasetID: "cost",
				Geometry:  chartir.GeometryLine,
				Encode: chartir.Encode{
					X: "time",
					Y: "cost",
				},
				Smooth: true,
				Name:   "Cost ($)",
				Style: &chartir.Style{
					Color: "#ee6666",
				},
			},
			{
				ID:        "cost-area",
				DatasetID: "cost",
				Geometry:  chartir.GeometryArea,
				Encode: chartir.Encode{
					X: "time",
					Y: "cost",
				},
				Name: "Cost ($)",
				Style: &chartir.Style{
					Color:   "#ee6666",
					Opacity: floatPtr(0.3),
				},
			},
		},
		Axes: []chartir.Axis{
			{
				ID:       "x",
				Type:     chartir.AxisTypeCategory,
				Position: chartir.AxisPositionBottom,
			},
			{
				ID:       "y",
				Type:     chartir.AxisTypeValue,
				Position: chartir.AxisPositionLeft,
				Name:     "Cost ($)",
			},
		},
		Tooltip: &chartir.Tooltip{
			Show:    true,
			Trigger: chartir.TooltipTriggerAxis,
		},
		Grid: &chartir.Grid{
			Left:         "10%",
			Right:        "10%",
			Bottom:       "15%",
			ContainLabel: true,
		},
	}
}

// GenerateModelPieChart generates a ChartIR for usage by model.
func GenerateModelPieChart(summary *UsageSummary) *chartir.ChartIR {
	// Build dataset rows
	rows := make([][]string, 0, len(summary.ByModel))
	for _, m := range summary.ByModel {
		rows = append(rows, []string{
			m.Model,
			formatInt64(m.TotalTokens),
		})
	}

	return &chartir.ChartIR{
		Title: "Usage by Model",
		Datasets: []chartir.Dataset{
			{
				ID: "models",
				Columns: []chartir.Column{
					{Name: "model", Type: chartir.ColumnTypeString},
					{Name: "tokens", Type: chartir.ColumnTypeNumber},
				},
				Rows: rows,
			},
		},
		Marks: []chartir.Mark{
			{
				ID:               "model-pie",
				DatasetID:        "models",
				Geometry:         chartir.GeometryPie,
				CoordinateSystem: chartir.CoordinateRadial,
				Encode: chartir.Encode{
					Value: "tokens",
					Name:  "model",
				},
				Name: "Tokens",
			},
		},
		Legend: &chartir.Legend{
			Show:     true,
			Position: chartir.LegendPositionRight,
		},
		Tooltip: &chartir.Tooltip{
			Show:    true,
			Trigger: chartir.TooltipTriggerItem,
		},
	}
}

// Helper functions

func formatInt64(n int64) string {
	return fmt.Sprintf("%d", n)
}

func formatFloat64(f float64) string {
	return fmt.Sprintf("%.4f", f)
}

func floatPtr(f float64) *float64 {
	return &f
}
