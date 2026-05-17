package commands

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/auth/credential"
)

func TestHandleReAuthRequired_ErrReAuthRequired(t *testing.T) {
	var buf bytes.Buffer
	err := fmt.Errorf("codex: %w", credential.ErrReAuthRequired)
	recognised := handleReAuthRequiredTo(err, &buf)
	if !recognised {
		t.Error("expected recognised=true for ErrReAuthRequired, got false")
	}
	output := buf.String()
	if !strings.Contains(output, "mink login codex") {
		t.Errorf("output should contain 'mink login codex', got: %q", output)
	}
}

func TestHandleReAuthRequired_OtherError(t *testing.T) {
	var buf bytes.Buffer
	err := errors.New("some unrelated error")
	recognised := handleReAuthRequiredTo(err, &buf)
	if recognised {
		t.Error("expected recognised=false for unrelated error, got true")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for unrelated error, got: %q", buf.String())
	}
}

func TestHandleReAuthRequired_Nil(t *testing.T) {
	var buf bytes.Buffer
	recognised := handleReAuthRequiredTo(nil, &buf)
	if recognised {
		t.Error("expected recognised=false for nil error, got true")
	}
}

func TestHandleReAuthRequired_WrappedError(t *testing.T) {
	var buf bytes.Buffer
	// Deeply wrapped ErrReAuthRequired should still be detected.
	err := fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", credential.ErrReAuthRequired))
	recognised := handleReAuthRequiredTo(err, &buf)
	if !recognised {
		t.Error("expected recognised=true for deeply wrapped ErrReAuthRequired")
	}
}
