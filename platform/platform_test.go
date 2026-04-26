package platform

import "testing"

func TestHealthy(t *testing.T) {
	h := Healthy()
	if h.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", h.Status)
	}
	if h.Details != nil {
		t.Error("expected nil details for healthy status")
	}
}

func TestDegraded(t *testing.T) {
	details := map[string]any{"reason": "high load"}
	h := Degraded(details)

	if h.Status != "degraded" {
		t.Errorf("expected status 'degraded', got %q", h.Status)
	}
	if h.Details["reason"] != "high load" {
		t.Error("details not preserved")
	}
}

func TestUnhealthy(t *testing.T) {
	details := map[string]any{"error": "database down"}
	h := Unhealthy(details)

	if h.Status != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got %q", h.Status)
	}
	if h.Details["error"] != "database down" {
		t.Error("details not preserved")
	}
}
