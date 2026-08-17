package openai

import (
	"testing"
	"time"
)

func TestCalculateCost_KnownModel(t *testing.T) {
	cost := CalculateCost("gpt-4", 1000, 1000)
	want := 0.03 + 0.06
	if cost != want {
		t.Errorf("cost = %v, want %v", cost, want)
	}
}

func TestCalculateCost_UnknownModel(t *testing.T) {
	cost := CalculateCost("some-unknown-model", 1000, 1000)
	want := 0.001 + 0.002
	if cost != want {
		t.Errorf("cost = %v, want %v", cost, want)
	}
}

func TestUsageStore_GetSummary_ByModelAndAgent(t *testing.T) {
	store := NewUsageStore(100)
	now := time.Now()

	store.Record(UsageRecord{Model: "gpt-4", AgentID: "agent-1", Timestamp: now, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, Cost: 1.0, Latency: 100})
	store.Record(UsageRecord{Model: "gpt-4", AgentID: "agent-1", Timestamp: now, PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30, Cost: 2.0, Latency: 200})
	store.Record(UsageRecord{Model: "gpt-3.5-turbo", AgentID: "agent-2", Timestamp: now, PromptTokens: 5, CompletionTokens: 5, TotalTokens: 10, Cost: 0.5, Latency: 50})

	summary := store.GetSummary(time.Time{}, time.Time{})
	if summary.TotalRequests != 3 {
		t.Errorf("TotalRequests = %d, want 3", summary.TotalRequests)
	}
	if summary.TotalTokens != 55 {
		t.Errorf("TotalTokens = %d, want 55", summary.TotalTokens)
	}
	if summary.TotalCost != 3.5 {
		t.Errorf("TotalCost = %v, want 3.5", summary.TotalCost)
	}
	if summary.AvgLatency != (100.0+200.0+50.0)/3.0 {
		t.Errorf("AvgLatency = %v, want %v", summary.AvgLatency, (100.0+200.0+50.0)/3.0)
	}

	gpt4, ok := summary.ByModel["gpt-4"]
	if !ok {
		t.Fatal("expected gpt-4 in ByModel")
	}
	if gpt4.Requests != 2 || gpt4.TotalTokens != 45 {
		t.Errorf("gpt-4 usage = %+v, unexpected", gpt4)
	}

	agent1, ok := summary.ByAgent["agent-1"]
	if !ok {
		t.Fatal("expected agent-1 in ByAgent")
	}
	if agent1.Requests != 2 || agent1.TotalTokens != 45 {
		t.Errorf("agent-1 usage = %+v, unexpected", agent1)
	}
}

func TestUsageStore_GetSummary_TimeFiltered(t *testing.T) {
	store := NewUsageStore(100)
	base := time.Now()

	store.Record(UsageRecord{Model: "m", Timestamp: base.Add(-2 * time.Hour), TotalTokens: 100})
	store.Record(UsageRecord{Model: "m", Timestamp: base, TotalTokens: 50})

	summary := store.GetSummary(base.Add(-time.Minute), base.Add(time.Minute))
	if summary.TotalRequests != 1 {
		t.Fatalf("TotalRequests = %d, want 1 (only the in-range record)", summary.TotalRequests)
	}
	if summary.TotalTokens != 50 {
		t.Errorf("TotalTokens = %d, want 50", summary.TotalTokens)
	}
}

func TestUsageStore_GetTimeseries_HourBuckets(t *testing.T) {
	store := NewUsageStore(100)
	base := time.Date(2024, 1, 1, 10, 15, 0, 0, time.UTC)

	store.Record(UsageRecord{Model: "m", Timestamp: base, TotalTokens: 10, Cost: 1})
	store.Record(UsageRecord{Model: "m", Timestamp: base.Add(20 * time.Minute), TotalTokens: 20, Cost: 2})
	store.Record(UsageRecord{Model: "m", Timestamp: base.Add(2 * time.Hour), TotalTokens: 30, Cost: 3})

	ts := store.GetTimeseries(time.Time{}, time.Time{}, "hour")
	if ts.Interval != "hour" {
		t.Errorf("Interval = %q, want hour", ts.Interval)
	}
	if len(ts.Buckets) != 2 {
		t.Fatalf("Buckets len = %d, want 2 (two distinct hours)", len(ts.Buckets))
	}

	var totalRequests int64
	for _, b := range ts.Buckets {
		totalRequests += b.Requests
	}
	if totalRequests != 3 {
		t.Errorf("total requests across buckets = %d, want 3", totalRequests)
	}
}

func TestUsageStore_GetTimeseries_DefaultsToHourOnUnknownInterval(t *testing.T) {
	store := NewUsageStore(100)
	store.Record(UsageRecord{Model: "m", Timestamp: time.Now(), TotalTokens: 1})
	ts := store.GetTimeseries(time.Time{}, time.Time{}, "fortnight")
	if ts.Interval != "hour" {
		t.Errorf("Interval = %q, want hour (fallback)", ts.Interval)
	}
}

func TestUsageStore_GetRecords_LimitAndOrder(t *testing.T) {
	store := NewUsageStore(100)
	for i := 0; i < 5; i++ {
		store.Record(UsageRecord{ID: string(rune('a' + i)), Timestamp: time.Now()})
	}

	all := store.GetRecords(0)
	if len(all) != 5 {
		t.Fatalf("GetRecords(0) len = %d, want 5 (0 means unlimited)", len(all))
	}

	limited := store.GetRecords(2)
	if len(limited) != 2 {
		t.Fatalf("GetRecords(2) len = %d, want 2", len(limited))
	}
	// Should be the two most recently inserted records ("d", "e").
	if limited[0].ID != "d" || limited[1].ID != "e" {
		t.Errorf("GetRecords(2) = %+v, want [d e] (most recent)", limited)
	}
}

func TestUsageStore_Clear(t *testing.T) {
	store := NewUsageStore(100)
	store.Record(UsageRecord{Model: "m", Timestamp: time.Now()})
	if store.Count() != 1 {
		t.Fatalf("Count = %d, want 1 before Clear", store.Count())
	}
	store.Clear()
	if store.Count() != 0 {
		t.Errorf("Count = %d, want 0 after Clear", store.Count())
	}
}

func TestUsageStore_EvictsOldestAtCapacity(t *testing.T) {
	store := NewUsageStore(10)
	for i := 0; i < 15; i++ {
		store.Record(UsageRecord{Model: "m", Timestamp: time.Now()})
	}
	if store.Count() > 10 {
		t.Errorf("Count = %d, want <= 10 after eviction", store.Count())
	}
}

func TestGenerateTokensChart(t *testing.T) {
	ts := &UsageTimeseries{
		Interval: "hour",
		Buckets: []UsageBucket{
			{Timestamp: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), PromptTokens: 100, CompletionTokens: 50},
		},
	}

	chart := GenerateTokensChart(ts)
	if chart.Title != "Token Usage" {
		t.Errorf("Title = %q, want 'Token Usage'", chart.Title)
	}
	if len(chart.Datasets) != 1 || len(chart.Datasets[0].Rows) != 1 {
		t.Fatalf("unexpected dataset shape: %+v", chart.Datasets)
	}
	row := chart.Datasets[0].Rows[0]
	if row[0] != "10:00" || row[1] != "100" || row[2] != "50" {
		t.Errorf("row = %v, want [10:00 100 50]", row)
	}
	if len(chart.Marks) != 2 {
		t.Errorf("Marks len = %d, want 2 (prompt + completion bars)", len(chart.Marks))
	}
}

func TestGenerateTokensChart_DayInterval(t *testing.T) {
	ts := &UsageTimeseries{
		Interval: "day",
		Buckets: []UsageBucket{
			{Timestamp: time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC), PromptTokens: 1, CompletionTokens: 2},
		},
	}
	chart := GenerateTokensChart(ts)
	if chart.Datasets[0].Rows[0][0] != "Mar 5" {
		t.Errorf("time label = %q, want 'Mar 5'", chart.Datasets[0].Rows[0][0])
	}
}

func TestGenerateCostChart(t *testing.T) {
	ts := &UsageTimeseries{
		Interval: "hour",
		Buckets: []UsageBucket{
			{Timestamp: time.Date(2024, 1, 1, 9, 30, 0, 0, time.UTC), Cost: 1.2345},
		},
	}
	chart := GenerateCostChart(ts)
	if chart.Title != "Cost Over Time" {
		t.Errorf("Title = %q, want 'Cost Over Time'", chart.Title)
	}
	row := chart.Datasets[0].Rows[0]
	if row[0] != "09:30" || row[1] != "1.2345" {
		t.Errorf("row = %v, want [09:30 1.2345]", row)
	}
	if len(chart.Marks) != 2 {
		t.Errorf("Marks len = %d, want 2 (line + area)", len(chart.Marks))
	}
}

func TestGenerateModelPieChart(t *testing.T) {
	summary := &UsageSummary{
		ByModel: map[string]*ModelUsage{
			"gpt-4": {Model: "gpt-4", TotalTokens: 1000},
		},
	}
	chart := GenerateModelPieChart(summary)
	if chart.Title != "Usage by Model" {
		t.Errorf("Title = %q, want 'Usage by Model'", chart.Title)
	}
	if len(chart.Datasets[0].Rows) != 1 {
		t.Fatalf("Rows len = %d, want 1", len(chart.Datasets[0].Rows))
	}
	row := chart.Datasets[0].Rows[0]
	if row[0] != "gpt-4" || row[1] != "1000" {
		t.Errorf("row = %v, want [gpt-4 1000]", row)
	}
}

func TestFormatHelpers(t *testing.T) {
	if got := formatInt64(12345); got != "12345" {
		t.Errorf("formatInt64 = %q, want 12345", got)
	}
	if got := formatFloat64(1.5); got != "1.5000" {
		t.Errorf("formatFloat64 = %q, want 1.5000", got)
	}
	p := floatPtr(0.25)
	if p == nil || *p != 0.25 {
		t.Errorf("floatPtr = %v, want pointer to 0.25", p)
	}
}
