package openai

import (
	"testing"
	"time"
)

func TestToolUsageStore_RecordAndSummary(t *testing.T) {
	store := NewToolUsageStore(100)
	now := time.Now()

	store.Record(ToolUsageRecord{ToolName: "search", Timestamp: now, Latency: 100, Success: true})
	store.Record(ToolUsageRecord{ToolName: "search", Timestamp: now.Add(time.Second), Latency: 200, Success: false})
	store.Record(ToolUsageRecord{ToolName: "read_file", Timestamp: now, Latency: 50, Success: true})

	summary := store.GetSummary(time.Time{}, time.Time{})
	if summary.TotalCalls != 3 {
		t.Errorf("TotalCalls = %d, want 3", summary.TotalCalls)
	}

	search, ok := summary.ByTool["search"]
	if !ok {
		t.Fatal("expected 'search' entry in ByTool")
	}
	if search.CallCount != 2 {
		t.Errorf("search.CallCount = %d, want 2", search.CallCount)
	}
	if search.AvgLatency != 150 {
		t.Errorf("search.AvgLatency = %v, want 150", search.AvgLatency)
	}
	if search.SuccessRate != 0.5 {
		t.Errorf("search.SuccessRate = %v, want 0.5", search.SuccessRate)
	}

	readFile, ok := summary.ByTool["read_file"]
	if !ok {
		t.Fatal("expected 'read_file' entry in ByTool")
	}
	if readFile.SuccessRate != 1.0 {
		t.Errorf("read_file.SuccessRate = %v, want 1.0", readFile.SuccessRate)
	}

	if len(summary.TopTools) != 2 {
		t.Fatalf("TopTools len = %d, want 2", len(summary.TopTools))
	}
	// search has more calls, should be first.
	if summary.TopTools[0].ToolName != "search" {
		t.Errorf("TopTools[0] = %q, want search", summary.TopTools[0].ToolName)
	}
}

func TestToolUsageStore_GetSummary_TimeFiltered(t *testing.T) {
	store := NewToolUsageStore(100)
	base := time.Now()

	store.Record(ToolUsageRecord{ToolName: "old_tool", Timestamp: base.Add(-time.Hour), Success: true})
	store.Record(ToolUsageRecord{ToolName: "recent_tool", Timestamp: base, Success: true})

	summary := store.GetSummary(base.Add(-time.Minute), base.Add(time.Minute))
	if summary.TotalCalls != 1 {
		t.Fatalf("TotalCalls = %d, want 1 (only recent_tool in range)", summary.TotalCalls)
	}
	if _, ok := summary.ByTool["recent_tool"]; !ok {
		t.Error("expected recent_tool in filtered summary")
	}
	if _, ok := summary.ByTool["old_tool"]; ok {
		t.Error("did not expect old_tool in filtered summary")
	}
}

func TestToolUsageStore_GetToolStats(t *testing.T) {
	store := NewToolUsageStore(100)
	now := time.Now()

	store.Record(ToolUsageRecord{ToolName: "search", Timestamp: now, Latency: 100, Success: true})
	store.Record(ToolUsageRecord{ToolName: "search", Timestamp: now.Add(time.Second), Latency: 300, Success: true})
	store.Record(ToolUsageRecord{ToolName: "other", Timestamp: now, Latency: 999, Success: false})

	stats := store.GetToolStats("search", time.Time{}, time.Time{})
	if stats.CallCount != 2 {
		t.Errorf("CallCount = %d, want 2", stats.CallCount)
	}
	if stats.AvgLatency != 200 {
		t.Errorf("AvgLatency = %v, want 200", stats.AvgLatency)
	}
	if stats.SuccessRate != 1.0 {
		t.Errorf("SuccessRate = %v, want 1.0", stats.SuccessRate)
	}
}

func TestToolUsageStore_GetToolStats_UnknownTool(t *testing.T) {
	store := NewToolUsageStore(100)
	stats := store.GetToolStats("nonexistent", time.Time{}, time.Time{})
	if stats.CallCount != 0 {
		t.Errorf("CallCount = %d, want 0 for unknown tool", stats.CallCount)
	}
	if stats.SuccessRate != 0 {
		t.Errorf("SuccessRate = %v, want 0 for unknown tool", stats.SuccessRate)
	}
}

func TestToolUsageStore_EvictsOldestAtCapacity(t *testing.T) {
	store := NewToolUsageStore(10)
	for i := 0; i < 15; i++ {
		store.Record(ToolUsageRecord{ToolName: "tool", Timestamp: time.Now(), Success: true})
	}
	summary := store.GetSummary(time.Time{}, time.Time{})
	if summary.TotalCalls > 10 {
		t.Errorf("TotalCalls = %d, want <= 10 after eviction", summary.TotalCalls)
	}
}
