package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ChartTool provides chart rendering capabilities using ChartIR format.
type ChartTool struct{}

// ChartArgs are the arguments for the chart tool.
type ChartArgs struct {
	Title     string          `json:"title"`
	ChartType string          `json:"chart_type"` // "line", "bar", "pie", "area", "scatter"
	XColumn   string          `json:"x_column"`   // Name of x-axis column
	YColumns  []string        `json:"y_columns"`  // Names of y-axis columns (for multi-series)
	Data      [][]interface{} `json:"data"`       // Array of rows, each row matches columns order
	Smooth    bool            `json:"smooth,omitempty"`
	Stacked   bool            `json:"stacked,omitempty"`
	ShowLegend bool           `json:"show_legend,omitempty"`
}

// ChartIR represents the intermediate representation for charts.
type ChartIR struct {
	Title    string       `json:"title,omitempty"`
	Datasets []Dataset    `json:"datasets"`
	Marks    []Mark       `json:"marks"`
	Axes     []Axis       `json:"axes,omitempty"`
	Legend   *Legend      `json:"legend,omitempty"`
	Tooltip  *Tooltip     `json:"tooltip,omitempty"`
}

type Dataset struct {
	ID      string   `json:"id"`
	Columns []Column `json:"columns"`
	Rows    [][]string `json:"rows"`
}

type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Mark struct {
	ID        string            `json:"id"`
	DatasetID string            `json:"datasetId"`
	Geometry  string            `json:"geometry"`
	Encode    map[string]string `json:"encode"`
	Name      string            `json:"name,omitempty"`
	Smooth    bool              `json:"smooth,omitempty"`
	Stack     string            `json:"stack,omitempty"`
	Style     *MarkStyle        `json:"style,omitempty"`
}

type MarkStyle struct {
	Color   string  `json:"color,omitempty"`
	Opacity float64 `json:"opacity,omitempty"`
}

type Axis struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Position string `json:"position"`
	Name     string `json:"name,omitempty"`
}

type Legend struct {
	Show     bool   `json:"show"`
	Position string `json:"position,omitempty"`
}

type Tooltip struct {
	Show    bool   `json:"show"`
	Trigger string `json:"trigger,omitempty"`
}

// NewChartTool creates a new chart tool.
func NewChartTool() *ChartTool {
	return &ChartTool{}
}

func (t *ChartTool) Name() string {
	return "render_chart"
}

func (t *ChartTool) Description() string {
	return `Render data as an interactive chart. Use this tool when you need to visualize data such as stock prices, trends, comparisons, or distributions.

IMPORTANT: This tool returns a special chartir code block. You MUST include the returned code block EXACTLY as-is in your response (including the triple backticks). Do NOT convert it to an image link or markdown image syntax. The UI will automatically render it as an interactive chart.

Supported chart types:
- line: Time series, trends (use smooth:true for curved lines)
- bar: Comparisons, categories
- area: Cumulative values, filled line charts
- pie: Proportions, percentages (only needs one y_column)
- scatter: Correlations, distributions

For multi-series charts (multiple lines/bars), provide multiple y_columns.
For stacked bar charts, set stacked:true.`
}

func (t *ChartTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Chart title",
			},
			"chart_type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"line", "bar", "pie", "area", "scatter"},
				"description": "Type of chart to render",
			},
			"x_column": map[string]interface{}{
				"type":        "string",
				"description": "Name of the x-axis column (e.g., 'date', 'category')",
			},
			"y_columns": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Names of y-axis columns to plot (e.g., ['price', 'volume'])",
			},
			"data": map[string]interface{}{
				"type":        "array",
				"description": "Array of data rows. Each row is an array with values matching column order [x_value, y1_value, y2_value, ...]",
				"items": map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{},
				},
			},
			"smooth": map[string]interface{}{
				"type":        "boolean",
				"description": "Use smooth curves for line/area charts (default: false)",
			},
			"stacked": map[string]interface{}{
				"type":        "boolean",
				"description": "Stack bars/areas on top of each other (default: false)",
			},
			"show_legend": map[string]interface{}{
				"type":        "boolean",
				"description": "Show legend for multi-series charts (default: true for multi-series)",
			},
		},
		"required": []string{"title", "chart_type", "x_column", "y_columns", "data"},
	}
}

// Color palette for chart series
var chartColors = []string{
	"#5470c6", "#91cc75", "#fac858", "#ee6666", "#73c0de",
	"#3ba272", "#fc8452", "#9a60b4", "#ea7ccc",
}

func (t *ChartTool) Execute(ctx context.Context, argsJSON json.RawMessage) (string, error) {
	var args ChartArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	if args.Title == "" {
		return "", fmt.Errorf("title is required")
	}
	if args.ChartType == "" {
		return "", fmt.Errorf("chart_type is required")
	}
	if len(args.YColumns) == 0 {
		return "", fmt.Errorf("at least one y_column is required")
	}
	if len(args.Data) == 0 {
		return "", fmt.Errorf("data is required")
	}

	// Build columns
	columns := []Column{{Name: args.XColumn, Type: "string"}}
	for _, yCol := range args.YColumns {
		columns = append(columns, Column{Name: yCol, Type: "number"})
	}

	// Build rows (convert all values to strings for ChartIR)
	rows := make([][]string, len(args.Data))
	for i, row := range args.Data {
		strRow := make([]string, len(row))
		for j, val := range row {
			strRow[j] = fmt.Sprintf("%v", val)
		}
		rows[i] = strRow
	}

	// Build marks (one per y column)
	marks := make([]Mark, len(args.YColumns))
	for i, yCol := range args.YColumns {
		geometry := args.ChartType
		mark := Mark{
			ID:        fmt.Sprintf("series-%d", i),
			DatasetID: "main",
			Geometry:  geometry,
			Name:      yCol,
			Encode:    map[string]string{"x": args.XColumn, "y": yCol},
			Smooth:    args.Smooth,
			Style:     &MarkStyle{Color: chartColors[i%len(chartColors)]},
		}

		// Handle pie charts differently
		if args.ChartType == "pie" {
			mark.Encode = map[string]string{"value": yCol, "name": args.XColumn}
		}

		// Handle stacked charts
		if args.Stacked && (args.ChartType == "bar" || args.ChartType == "area") {
			mark.Stack = "stack"
		}

		marks[i] = mark
	}

	// Build axes (not for pie charts)
	var axes []Axis
	if args.ChartType != "pie" {
		axes = []Axis{
			{ID: "x", Type: "category", Position: "bottom"},
			{ID: "y", Type: "value", Position: "left"},
		}
	}

	// Build legend
	var legend *Legend
	if len(args.YColumns) > 1 || args.ShowLegend {
		legend = &Legend{Show: true, Position: "top"}
	}

	// Build ChartIR
	ir := ChartIR{
		Title: args.Title,
		Datasets: []Dataset{{
			ID:      "main",
			Columns: columns,
			Rows:    rows,
		}},
		Marks:   marks,
		Axes:    axes,
		Legend:  legend,
		Tooltip: &Tooltip{Show: true, Trigger: "axis"},
	}

	// For pie charts, use item trigger
	if args.ChartType == "pie" {
		ir.Tooltip.Trigger = "item"
	}

	// Serialize to JSON
	irJSON, err := json.MarshalIndent(ir, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serialize chart: %w", err)
	}

	// Wrap in chartir code block with instructions
	var sb strings.Builder
	sb.WriteString("CHART_OUTPUT_START\n")
	sb.WriteString("```chartir\n")
	sb.Write(irJSON)
	sb.WriteString("\n```\n")
	sb.WriteString("CHART_OUTPUT_END\n")
	sb.WriteString("Include the above chartir code block EXACTLY as-is in your response. Do not convert it to an image link or modify it.")

	return sb.String(), nil
}

// Ensure ChartTool implements Tool interface.
var _ Tool = (*ChartTool)(nil)
