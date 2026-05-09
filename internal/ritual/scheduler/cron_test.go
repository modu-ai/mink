// Package scheduler — internal tests for cron helpers.
package scheduler

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestParseClock_Valid exercises the happy path.
func TestParseClock_Valid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input        string
		wantH, wantM int
	}{
		{"07:00", 7, 0},
		{"12:30", 12, 30},
		{"23:59", 23, 59},
		{"00:00", 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			h, m, err := parseClock(tc.input)
			if err != nil {
				t.Fatalf("parseClock(%q) unexpected error: %v", tc.input, err)
			}
			if h != tc.wantH || m != tc.wantM {
				t.Errorf("parseClock(%q) = %d:%d, want %d:%d", tc.input, h, m, tc.wantH, tc.wantM)
			}
		})
	}
}

// TestParseClock_Invalid exercises error branches.
func TestParseClock_Invalid(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",        // empty string
		"7:00",    // missing leading zero is actually valid, but let's verify it parses
		"25:00",   // hour out of range
		"07:60",   // minute out of range
		"abc",     // no colon
		"07",      // no colon with single component
		"not:int", // non-integer parts
	}
	// All of these except "7:00" should error. "7:00" parses as 7 hours 0 minutes.
	errExpected := map[string]bool{
		"":        true,
		"25:00":   true,
		"07:60":   true,
		"abc":     true,
		"07":      true,
		"not:int": true,
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			t.Parallel()
			_, _, err := parseClock(c)
			if errExpected[c] && err == nil {
				t.Errorf("parseClock(%q) expected error, got nil", c)
			}
			if !errExpected[c] && err != nil {
				t.Errorf("parseClock(%q) unexpected error: %v", c, err)
			}
		})
	}
}

// TestClockToCronExpr verifies the cron expression format.
func TestClockToCronExpr(t *testing.T) {
	t.Parallel()
	expr := clockToCronExpr(7, 30)
	if expr != "30 7 * * *" {
		t.Errorf("clockToCronExpr(7,30) = %q, want %q", expr, "30 7 * * *")
	}
}

// TestCronZapLogger_Info exercises the Info adapter branch.
func TestCronZapLogger_Info(t *testing.T) {
	t.Parallel()
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)
	cl := cronZapLogger{log: logger}
	cl.Info("test info message", "key1", "val1", "key2", 42)
	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	if logs.All()[0].Message != "test info message" {
		t.Errorf("unexpected message: %q", logs.All()[0].Message)
	}
}

// TestCronZapLogger_Error exercises the Error adapter branch.
func TestCronZapLogger_Error(t *testing.T) {
	t.Parallel()
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)
	cl := cronZapLogger{log: logger}
	cl.Error(nil, "test error message", "k", "v")
	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	if logs.All()[0].Message != "test error message" {
		t.Errorf("unexpected message: %q", logs.All()[0].Message)
	}
}

// TestKvToZapFields_OddLength verifies that an odd-length kv slice is handled gracefully.
func TestKvToZapFields_OddLength(t *testing.T) {
	t.Parallel()
	// Odd-length: the last element has no value pair and should be skipped.
	fields := kvToZapFields([]any{"k1", "v1", "orphan"})
	if len(fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(fields))
	}
}
