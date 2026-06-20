package operations

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/grokify/echartify/chartir"
)

// UsageSummaryInput is the request for getting usage summary.
type UsageSummaryInput struct {
	Since string `query:"since" doc:"Start time (RFC3339 or relative: 1h, 24h, 7d, 30d)"`
	Until string `query:"until" doc:"End time (RFC3339, defaults to now)"`
}

// UsageSummaryOutput is the response for usage summary.
type UsageSummaryOutput struct {
	Body UsageSummary
}

// UsageTimeseriesInput is the request for getting usage timeseries.
type UsageTimeseriesInput struct {
	Since    string `query:"since" doc:"Start time (RFC3339 or relative)"`
	Until    string `query:"until" doc:"End time (RFC3339)"`
	Interval string `query:"interval" default:"hour" doc:"Bucket interval (minute, hour, day)"`
}

// UsageTimeseriesOutput is the response for usage timeseries.
type UsageTimeseriesOutput struct {
	Body UsageTimeseries
}

// UsageRecordsInput is the request for getting usage records.
type UsageRecordsInput struct {
	Limit int `query:"limit" default:"100" doc:"Maximum number of records to return"`
}

// UsageRecordsOutput is the response for usage records.
type UsageRecordsOutput struct {
	Body struct {
		Object string        `json:"object" example:"list" doc:"Object type"`
		Data   []UsageRecord `json:"data" doc:"Usage records"`
	}
}

// ChartOutput is the response for chart data.
type ChartOutput struct {
	Body chartir.ChartIR
}

// RegisterUsageOperations registers usage analytics API operations.
func RegisterUsageOperations(api huma.API, store UsageStore) {
	if store == nil {
		return
	}

	// GET /v1/usage - Usage summary
	huma.Register(api, huma.Operation{
		OperationID:   "getUsageSummary",
		Method:        http.MethodGet,
		Path:          "/v1/usage",
		Summary:       "Get usage summary",
		Description:   "Returns aggregated usage statistics for a time range.",
		Tags:          []string{"Usage"},
		DefaultStatus: http.StatusOK,
	}, func(_ context.Context, input *UsageSummaryInput) (*UsageSummaryOutput, error) {
		since, until := parseTimeRange(input.Since, input.Until)
		summary := store.GetSummary(since, until)
		return &UsageSummaryOutput{Body: *summary}, nil
	})

	// GET /v1/usage/timeseries - Usage timeseries
	huma.Register(api, huma.Operation{
		OperationID:   "getUsageTimeseries",
		Method:        http.MethodGet,
		Path:          "/v1/usage/timeseries",
		Summary:       "Get usage timeseries",
		Description:   "Returns time-bucketed usage data.",
		Tags:          []string{"Usage"},
		DefaultStatus: http.StatusOK,
	}, func(_ context.Context, input *UsageTimeseriesInput) (*UsageTimeseriesOutput, error) {
		since, until := parseTimeRange(input.Since, input.Until)
		interval := input.Interval
		if interval == "" {
			interval = "hour"
		}
		ts := store.GetTimeseries(since, until, interval)
		return &UsageTimeseriesOutput{Body: *ts}, nil
	})

	// GET /v1/usage/records - Recent usage records
	huma.Register(api, huma.Operation{
		OperationID:   "getUsageRecords",
		Method:        http.MethodGet,
		Path:          "/v1/usage/records",
		Summary:       "Get usage records",
		Description:   "Returns recent usage records.",
		Tags:          []string{"Usage"},
		DefaultStatus: http.StatusOK,
	}, func(_ context.Context, input *UsageRecordsInput) (*UsageRecordsOutput, error) {
		limit := input.Limit
		if limit <= 0 {
			limit = 100
		}
		records := store.GetRecords(limit)
		out := &UsageRecordsOutput{}
		out.Body.Object = "list"
		out.Body.Data = records
		return out, nil
	})

	// GET /v1/usage/chart/tokens - Token usage chart
	huma.Register(api, huma.Operation{
		OperationID:   "getUsageChartTokens",
		Method:        http.MethodGet,
		Path:          "/v1/usage/chart/tokens",
		Summary:       "Get token usage chart",
		Description:   "Returns a ChartIR representation of token usage over time.",
		Tags:          []string{"Usage"},
		DefaultStatus: http.StatusOK,
	}, func(_ context.Context, input *UsageTimeseriesInput) (*ChartOutput, error) {
		since, until := parseTimeRange(input.Since, input.Until)
		interval := input.Interval
		if interval == "" {
			interval = "hour"
		}
		ts := store.GetTimeseries(since, until, interval)
		chart := generateTokensChart(ts)
		return &ChartOutput{Body: *chart}, nil
	})

	// GET /v1/usage/chart/cost - Cost chart
	huma.Register(api, huma.Operation{
		OperationID:   "getUsageChartCost",
		Method:        http.MethodGet,
		Path:          "/v1/usage/chart/cost",
		Summary:       "Get cost chart",
		Description:   "Returns a ChartIR representation of cost over time.",
		Tags:          []string{"Usage"},
		DefaultStatus: http.StatusOK,
	}, func(_ context.Context, input *UsageTimeseriesInput) (*ChartOutput, error) {
		since, until := parseTimeRange(input.Since, input.Until)
		interval := input.Interval
		if interval == "" {
			interval = "hour"
		}
		ts := store.GetTimeseries(since, until, interval)
		chart := generateCostChart(ts)
		return &ChartOutput{Body: *chart}, nil
	})

	// GET /v1/usage/chart/models - Model distribution chart
	huma.Register(api, huma.Operation{
		OperationID:   "getUsageChartModels",
		Method:        http.MethodGet,
		Path:          "/v1/usage/chart/models",
		Summary:       "Get model distribution chart",
		Description:   "Returns a ChartIR representation of usage by model.",
		Tags:          []string{"Usage"},
		DefaultStatus: http.StatusOK,
	}, func(_ context.Context, input *UsageSummaryInput) (*ChartOutput, error) {
		since, until := parseTimeRange(input.Since, input.Until)
		summary := store.GetSummary(since, until)
		chart := generateModelPieChart(summary)
		return &ChartOutput{Body: *chart}, nil
	})
}

// parseTimeRange parses since/until strings into time.Time values.
func parseTimeRange(sinceStr, untilStr string) (since, until time.Time) {
	now := time.Now()

	// Parse relative time strings
	parseDuration := func(s string) time.Duration {
		if d, err := time.ParseDuration(s); err == nil {
			return d
		}
		// Handle day format (e.g., "7d", "30d")
		if len(s) > 1 && s[len(s)-1] == 'd' {
			if days := parsePositiveInt(s[:len(s)-1]); days > 0 {
				return time.Duration(days) * 24 * time.Hour
			}
		}
		return 0
	}

	// Parse since
	if sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = t
		} else if d := parseDuration(sinceStr); d > 0 {
			since = now.Add(-d)
		}
	}

	// Parse until
	if untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			until = t
		}
	}

	// Default to last 24 hours if no range specified
	if since.IsZero() && until.IsZero() {
		until = now
		since = now.Add(-24 * time.Hour)
	} else if until.IsZero() {
		until = now
	}

	return since, until
}

func parsePositiveInt(s string) int {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// Chart generation helpers

func formatInt64Simple(n int64) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

func formatFloat64(f float64) string {
	// Simple formatting - this is used for cost values
	intPart := int64(f)
	fracPart := int64((f - float64(intPart)) * 10000)
	if fracPart < 0 {
		fracPart = -fracPart
	}
	return formatInt64Simple(intPart) + "." + padLeft(formatInt64Simple(fracPart), 4)
}

func padLeft(s string, n int) string {
	for len(s) < n {
		s = "0" + s
	}
	return s
}

func floatPtr(f float64) *float64 {
	return &f
}

func generateTokensChart(ts *UsageTimeseries) *chartir.ChartIR {
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
			formatInt64Simple(b.PromptTokens),
			formatInt64Simple(b.CompletionTokens),
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

func generateCostChart(ts *UsageTimeseries) *chartir.ChartIR {
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

func generateModelPieChart(summary *UsageSummary) *chartir.ChartIR {
	rows := make([][]string, 0, len(summary.ByModel))
	for _, m := range summary.ByModel {
		rows = append(rows, []string{
			m.Model,
			formatInt64Simple(m.TotalTokens),
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
