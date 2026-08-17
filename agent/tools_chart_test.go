package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestChartTool_Metadata(t *testing.T) {
	tool := NewChartTool()
	if tool.Name() != "render_chart" {
		t.Errorf("Name() = %q, want render_chart", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}
	params := tool.Parameters()
	if params["type"] != "object" {
		t.Errorf("Parameters()[type] = %v, want object", params["type"])
	}
}

func TestChartTool_Execute_ValidationErrors(t *testing.T) {
	tool := NewChartTool()
	ctx := context.Background()

	tests := []struct {
		name    string
		args    ChartArgs
		wantErr string
	}{
		{
			name:    "missing title",
			args:    ChartArgs{ChartType: "line", YColumns: []string{"y"}, Data: [][]interface{}{{"x", 1}}},
			wantErr: "title is required",
		},
		{
			name:    "missing chart type",
			args:    ChartArgs{Title: "t", YColumns: []string{"y"}, Data: [][]interface{}{{"x", 1}}},
			wantErr: "chart_type is required",
		},
		{
			name:    "missing y columns",
			args:    ChartArgs{Title: "t", ChartType: "line", Data: [][]interface{}{{"x", 1}}},
			wantErr: "y_column is required",
		},
		{
			name:    "missing data",
			args:    ChartArgs{Title: "t", ChartType: "line", YColumns: []string{"y"}},
			wantErr: "data is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.args)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			_, err = tool.Execute(ctx, raw)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Execute() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := tool.Execute(ctx, json.RawMessage(`not-json`))
		if err == nil || !strings.Contains(err.Error(), "parse arguments") {
			t.Errorf("Execute() error = %v, want parse-arguments error", err)
		}
	})
}

func TestChartTool_Execute_LineChart(t *testing.T) {
	tool := NewChartTool()
	args := ChartArgs{
		Title:     "Revenue",
		ChartType: "line",
		XColumn:   "date",
		YColumns:  []string{"revenue"},
		Data:      [][]interface{}{{"2026-01-01", 100}, {"2026-01-02", 150}},
		Smooth:    true,
	}
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	got, err := tool.Execute(context.Background(), raw)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"CHART_OUTPUT_START", "```chartir", "CHART_OUTPUT_END", `"title": "Revenue"`} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q, got %q", want, got)
		}
	}

	// Extract and parse the embedded ChartIR JSON to assert its structure,
	// not just substring presence.
	start := strings.Index(got, "```chartir\n") + len("```chartir\n")
	end := strings.Index(got[start:], "\n```")
	var ir ChartIR
	if err := json.Unmarshal([]byte(got[start:start+end]), &ir); err != nil {
		t.Fatalf("unmarshal embedded ChartIR: %v", err)
	}
	if len(ir.Datasets) != 1 || len(ir.Datasets[0].Rows) != 2 {
		t.Errorf("datasets = %+v, want 1 dataset with 2 rows", ir.Datasets)
	}
	if len(ir.Marks) != 1 || !ir.Marks[0].Smooth {
		t.Errorf("marks = %+v, want 1 smooth mark", ir.Marks)
	}
	if len(ir.Axes) != 2 {
		t.Errorf("axes = %+v, want x/y axes for a non-pie chart", ir.Axes)
	}
	// Single series: no legend unless explicitly requested.
	if ir.Legend != nil {
		t.Errorf("legend = %+v, want nil for a single-series chart without ShowLegend", ir.Legend)
	}
	if ir.Tooltip == nil || ir.Tooltip.Trigger != "axis" {
		t.Errorf("tooltip = %+v, want axis trigger for a non-pie chart", ir.Tooltip)
	}
}

func TestChartTool_Execute_PieChart(t *testing.T) {
	tool := NewChartTool()
	args := ChartArgs{
		Title:     "Share",
		ChartType: "pie",
		XColumn:   "segment",
		YColumns:  []string{"value"},
		Data:      [][]interface{}{{"A", 10}, {"B", 20}},
	}
	raw, _ := json.Marshal(args)

	got, err := tool.Execute(context.Background(), raw)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	start := strings.Index(got, "```chartir\n") + len("```chartir\n")
	end := strings.Index(got[start:], "\n```")
	var ir ChartIR
	if err := json.Unmarshal([]byte(got[start:start+end]), &ir); err != nil {
		t.Fatalf("unmarshal embedded ChartIR: %v", err)
	}
	if len(ir.Axes) != 0 {
		t.Errorf("axes = %+v, want none for a pie chart", ir.Axes)
	}
	if ir.Tooltip == nil || ir.Tooltip.Trigger != "item" {
		t.Errorf("tooltip = %+v, want item trigger for a pie chart", ir.Tooltip)
	}
	if len(ir.Marks) != 1 || ir.Marks[0].Encode["value"] != "value" || ir.Marks[0].Encode["name"] != "segment" {
		t.Errorf("marks = %+v, want pie-style value/name encoding", ir.Marks)
	}
}

func TestChartTool_Execute_StackedBarWithLegend(t *testing.T) {
	tool := NewChartTool()
	args := ChartArgs{
		Title:     "Stacked",
		ChartType: "bar",
		XColumn:   "month",
		YColumns:  []string{"a", "b"},
		Data:      [][]interface{}{{"Jan", 1, 2}},
		Stacked:   true,
	}
	raw, _ := json.Marshal(args)

	got, err := tool.Execute(context.Background(), raw)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	start := strings.Index(got, "```chartir\n") + len("```chartir\n")
	end := strings.Index(got[start:], "\n```")
	var ir ChartIR
	if err := json.Unmarshal([]byte(got[start:start+end]), &ir); err != nil {
		t.Fatalf("unmarshal embedded ChartIR: %v", err)
	}
	if len(ir.Marks) != 2 {
		t.Fatalf("marks = %+v, want one per y column", ir.Marks)
	}
	for _, m := range ir.Marks {
		if m.Stack != "stack" {
			t.Errorf("mark %+v not stacked", m)
		}
	}
	// Multi-series always gets a legend.
	if ir.Legend == nil || !ir.Legend.Show {
		t.Errorf("legend = %+v, want shown for multi-series", ir.Legend)
	}
}

func TestChartTool_ImplementsToolInterface(t *testing.T) {
	var _ Tool = NewChartTool()
}
