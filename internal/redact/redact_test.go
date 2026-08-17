package redact

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func newBufLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(NewHandler(slog.NewTextHandler(buf, nil)))
}

func TestString_MasksRegisteredValue(t *testing.T) {
	t.Cleanup(Reset)
	Register("super-secret-token-value")

	got := String("auth header: Bearer super-secret-token-value")

	if strings.Contains(got, "super-secret-token-value") {
		t.Errorf("String() = %q, still contains the raw value", got)
	}
	if !strings.Contains(got, mask) {
		t.Errorf("String() = %q, want it to contain %q", got, mask)
	}
}

func TestString_UnregisteredValuePassesThrough(t *testing.T) {
	t.Cleanup(Reset)

	got := String("nothing secret here")

	if got != "nothing secret here" {
		t.Errorf("String() = %q, want unchanged", got)
	}
}

func TestRegister_IgnoresShortValues(t *testing.T) {
	t.Cleanup(Reset)
	Register("ok", "1", "")

	got := String("status: ok, code: 1")

	if got != "status: ok, code: 1" {
		t.Errorf("String() = %q, short values should not be registered", got)
	}
}

func TestHandler_MasksMessageAndStringAttr(t *testing.T) {
	t.Cleanup(Reset)
	Register("sk-live-abc123xyz")

	var buf bytes.Buffer
	logger := newBufLogger(&buf)
	logger.Info("token is sk-live-abc123xyz", "detail", "value=sk-live-abc123xyz")

	out := buf.String()
	if strings.Contains(out, "sk-live-abc123xyz") {
		t.Errorf("log output leaked the secret: %s", out)
	}
	if strings.Count(out, mask) != 2 {
		t.Errorf("log output = %q, want 2 masks (message + attr)", out)
	}
}

func TestHandler_MasksNestedGroupAttr(t *testing.T) {
	t.Cleanup(Reset)
	Register("nested-secret-value")

	var buf bytes.Buffer
	logger := newBufLogger(&buf)
	logger.Info("dump", slog.Group("config", slog.String("token", "nested-secret-value")))

	out := buf.String()
	if strings.Contains(out, "nested-secret-value") {
		t.Errorf("log output leaked the secret from a group attr: %s", out)
	}
}

func TestHandler_MasksErrorAttr(t *testing.T) {
	t.Cleanup(Reset)
	Register("err-embedded-secret")

	var buf bytes.Buffer
	logger := newBufLogger(&buf)
	logger.Error("call failed", "error", errors.New("auth failed for err-embedded-secret"))

	out := buf.String()
	if strings.Contains(out, "err-embedded-secret") {
		t.Errorf("log output leaked the secret from an error attr: %s", out)
	}
}

func TestHandler_WithAttrsMasksAtBindTime(t *testing.T) {
	t.Cleanup(Reset)
	Register("bound-secret-value")

	var buf bytes.Buffer
	logger := newBufLogger(&buf).With("token", "bound-secret-value")
	logger.Info("ready")

	out := buf.String()
	if strings.Contains(out, "bound-secret-value") {
		t.Errorf("log output leaked a value bound via With: %s", out)
	}
}

func TestHandler_WithGroupPreservesRedaction(t *testing.T) {
	t.Cleanup(Reset)
	Register("grouped-secret-value")

	var buf bytes.Buffer
	logger := newBufLogger(&buf).WithGroup("req").With("token", "grouped-secret-value")
	logger.Info("ready")

	out := buf.String()
	if strings.Contains(out, "grouped-secret-value") {
		t.Errorf("log output leaked a value bound under WithGroup: %s", out)
	}
}

func TestReset_ClearsRegistry(t *testing.T) {
	Register("will-be-reset-value")
	Reset()

	got := String("will-be-reset-value")

	if got != "will-be-reset-value" {
		t.Errorf("String() after Reset = %q, want unmasked", got)
	}
}
