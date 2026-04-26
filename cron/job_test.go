package cron

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewJob(t *testing.T) {
	schedule := Schedule{Cron: "0 0 9 * * *"}
	action := Action{
		Type:      ActionTypeSendMessage,
		SessionID: "test-session",
		Message:   "Hello",
	}

	job := NewJob("job-1", "Test Job", schedule, action)

	if job.ID != "job-1" {
		t.Errorf("expected ID %q, got %q", "job-1", job.ID)
	}
	if job.Name != "Test Job" {
		t.Errorf("expected name %q, got %q", "Test Job", job.Name)
	}
	if job.Status != JobStatusEnabled {
		t.Errorf("expected status %q, got %q", JobStatusEnabled, job.Status)
	}
	if job.RunCount != 0 {
		t.Errorf("expected run count 0, got %d", job.RunCount)
	}
	if job.Metadata == nil {
		t.Error("expected metadata to be initialized")
	}
}

func TestJobValidate(t *testing.T) {
	tests := []struct {
		name    string
		job     *Job
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid cron job",
			job: &Job{
				ID:       "job-1",
				Name:     "Test Job",
				Schedule: Schedule{Cron: "0 0 9 * * *"},
				Action: Action{
					Type:      ActionTypeSendMessage,
					SessionID: "session-1",
					Message:   "Hello",
				},
			},
			wantErr: false,
		},
		{
			name: "valid interval job",
			job: &Job{
				ID:       "job-2",
				Name:     "Interval Job",
				Schedule: Schedule{Interval: Duration(time.Hour)},
				Action: Action{
					Type:       ActionTypeCallWebhook,
					WebhookURL: "https://example.com/hook",
				},
			},
			wantErr: false,
		},
		{
			name: "valid once job",
			job: func() *Job {
				t := time.Now().Add(time.Hour)
				return &Job{
					ID:       "job-3",
					Name:     "Once Job",
					Schedule: Schedule{Once: &t},
					Action: Action{
						Type:     ActionTypeCallTool,
						ToolName: "my_tool",
					},
				}
			}(),
			wantErr: false,
		},
		{
			name: "missing ID",
			job: &Job{
				Name:     "Test Job",
				Schedule: Schedule{Cron: "0 0 9 * * *"},
				Action: Action{
					Type:      ActionTypeSendMessage,
					SessionID: "session-1",
					Message:   "Hello",
				},
			},
			wantErr: true,
			errMsg:  "job ID is required",
		},
		{
			name: "missing name",
			job: &Job{
				ID:       "job-1",
				Schedule: Schedule{Cron: "0 0 9 * * *"},
				Action: Action{
					Type:      ActionTypeSendMessage,
					SessionID: "session-1",
					Message:   "Hello",
				},
			},
			wantErr: true,
			errMsg:  "job name is required",
		},
		{
			name: "missing schedule",
			job: &Job{
				ID:   "job-1",
				Name: "Test Job",
				Action: Action{
					Type:      ActionTypeSendMessage,
					SessionID: "session-1",
					Message:   "Hello",
				},
			},
			wantErr: true,
			errMsg:  "schedule is required",
		},
		{
			name: "multiple schedules",
			job: &Job{
				ID:       "job-1",
				Name:     "Test Job",
				Schedule: Schedule{Cron: "0 0 9 * * *", Interval: Duration(time.Hour)},
				Action: Action{
					Type:      ActionTypeSendMessage,
					SessionID: "session-1",
					Message:   "Hello",
				},
			},
			wantErr: true,
			errMsg:  "only one schedule type allowed",
		},
		{
			name: "send_message missing session_id",
			job: &Job{
				ID:       "job-1",
				Name:     "Test Job",
				Schedule: Schedule{Cron: "0 0 9 * * *"},
				Action: Action{
					Type:    ActionTypeSendMessage,
					Message: "Hello",
				},
			},
			wantErr: true,
			errMsg:  "session_id is required",
		},
		{
			name: "send_message missing message",
			job: &Job{
				ID:       "job-1",
				Name:     "Test Job",
				Schedule: Schedule{Cron: "0 0 9 * * *"},
				Action: Action{
					Type:      ActionTypeSendMessage,
					SessionID: "session-1",
				},
			},
			wantErr: true,
			errMsg:  "message is required",
		},
		{
			name: "call_webhook missing url",
			job: &Job{
				ID:       "job-1",
				Name:     "Test Job",
				Schedule: Schedule{Cron: "0 0 9 * * *"},
				Action: Action{
					Type: ActionTypeCallWebhook,
				},
			},
			wantErr: true,
			errMsg:  "webhook_url is required",
		},
		{
			name: "call_tool missing tool_name",
			job: &Job{
				ID:       "job-1",
				Name:     "Test Job",
				Schedule: Schedule{Cron: "0 0 9 * * *"},
				Action: Action{
					Type: ActionTypeCallTool,
				},
			},
			wantErr: true,
			errMsg:  "tool_name is required",
		},
		{
			name: "invalid action type",
			job: &Job{
				ID:       "job-1",
				Name:     "Test Job",
				Schedule: Schedule{Cron: "0 0 9 * * *"},
				Action: Action{
					Type: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid action type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if tt.errMsg != "" {
					// Simple substring check
					if !contains(err.Error(), tt.errMsg) {
						t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 1; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestJobIsRecurring(t *testing.T) {
	tests := []struct {
		name     string
		schedule Schedule
		want     bool
	}{
		{
			name:     "cron is recurring",
			schedule: Schedule{Cron: "0 0 9 * * *"},
			want:     true,
		},
		{
			name:     "interval is recurring",
			schedule: Schedule{Interval: Duration(time.Hour)},
			want:     true,
		},
		{
			name: "once is not recurring",
			schedule: func() Schedule {
				t := time.Now()
				return Schedule{Once: &t}
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{Schedule: tt.schedule}
			if got := job.IsRecurring(); got != tt.want {
				t.Errorf("IsRecurring() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDurationMarshalJSON(t *testing.T) {
	d := Duration(time.Hour + 30*time.Minute)

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `"1h30m0s"`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, string(data))
	}
}

func TestDurationUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Duration
		wantErr bool
	}{
		{
			name:  "string duration",
			input: `"1h30m"`,
			want:  Duration(time.Hour + 30*time.Minute),
		},
		{
			name:  "nanoseconds",
			input: `3600000000000`,
			want:  Duration(time.Hour),
		},
		{
			name:  "empty string",
			input: `""`,
			want:  0,
		},
		{
			name:    "invalid string",
			input:   `"invalid"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := json.Unmarshal([]byte(tt.input), &d)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				} else if d != tt.want {
					t.Errorf("expected %v, got %v", tt.want, d)
				}
			}
		})
	}
}

func TestJobJSONRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	job := &Job{
		ID:          "job-1",
		Name:        "Test Job",
		Description: "A test job",
		Schedule:    Schedule{Interval: Duration(time.Hour)},
		Action: Action{
			Type:      ActionTypeSendMessage,
			SessionID: "session-1",
			Message:   "Hello",
		},
		Status:    JobStatusEnabled,
		CreatedAt: now,
		UpdatedAt: now,
		RunCount:  5,
		Metadata: map[string]any{
			"key": "value",
		},
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded Job
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.ID != job.ID {
		t.Errorf("ID mismatch: %q != %q", decoded.ID, job.ID)
	}
	if decoded.Name != job.Name {
		t.Errorf("Name mismatch: %q != %q", decoded.Name, job.Name)
	}
	if decoded.Schedule.Interval != job.Schedule.Interval {
		t.Errorf("Interval mismatch: %v != %v", decoded.Schedule.Interval, job.Schedule.Interval)
	}
	if decoded.RunCount != job.RunCount {
		t.Errorf("RunCount mismatch: %d != %d", decoded.RunCount, job.RunCount)
	}
}
