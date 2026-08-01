// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gateway

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics("")
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}

	if m.ActiveConnections == nil {
		t.Error("ActiveConnections gauge not initialized")
	}

	if m.TotalConnections == nil {
		t.Error("TotalConnections counter not initialized")
	}

	if m.MessagesReceived == nil {
		t.Error("MessagesReceived counter not initialized")
	}
}

func TestNewMetrics_CustomNamespace(t *testing.T) {
	m := NewMetrics("custom")
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
}

func TestMetrics_Handler(t *testing.T) {
	m := NewMetrics("test_handler")

	handler := m.Handler()
	if handler == nil {
		t.Fatal("Handler returned nil")
	}

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "test_handler") {
		t.Error("metrics output should contain namespace")
	}
}

func TestMetrics_RecordConnection(t *testing.T) {
	m := NewMetrics("test_conn")

	m.RecordConnection()
	m.RecordConnection()
	m.RecordDisconnection()
}

func TestMetrics_RecordMessage(t *testing.T) {
	m := NewMetrics("test_msg")

	m.RecordMessage("chat", "received", 100*time.Millisecond)
	m.RecordMessage("ping", "received", 1*time.Millisecond)
	m.RecordMessage("pong", "sent", 1*time.Millisecond)
	m.RecordMessage("response", "sent", 500*time.Millisecond)
}

func TestMetrics_RecordRateLimited(t *testing.T) {
	m := NewMetrics("test_rl")

	m.RecordRateLimited()
	m.RecordRateLimited()
}

func TestMetrics_RecordAgentRequest(t *testing.T) {
	m := NewMetrics("test_agent")

	m.RecordAgentRequest(100*time.Millisecond, nil)
	m.RecordAgentRequest(200*time.Millisecond, errors.New("test error"))
}

func TestMetrics_RecordToolInvocation(t *testing.T) {
	m := NewMetrics("test_tool")

	m.RecordToolInvocation("web_search", 50*time.Millisecond, nil)
	m.RecordToolInvocation("calculator", 10*time.Millisecond, nil)
	m.RecordToolInvocation("broken_tool", 5*time.Millisecond, errors.New("failed"))
}
