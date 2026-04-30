// Package commands provides unit tests for the daemon command.
package commands

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// mockPingClient is a mock implementation of PingClient for testing.
type mockPingClient struct {
	latency    time.Duration
	err        error
	pingCalled bool
}

func (m *mockPingClient) Ping(ctx context.Context, addr string, writer io.Writer) error {
	m.pingCalled = true
	return m.err
}

// TestNewDaemonCommand verifies the daemon command is properly initialized.
func TestNewDaemonCommand(t *testing.T) {
	client := &mockPingClient{latency: 10 * time.Millisecond}
	cmd := NewDaemonCommand(client, "127.0.0.1:9005")

	if cmd == nil {
		t.Fatal("NewDaemonCommand returned nil")
	}

	if cmd.Use != "daemon" {
		t.Errorf("expected Use 'daemon', got '%s'", cmd.Use)
	}

	// Verify subcommands exist
	hasStatus := false
	hasShutdown := false

	for _, subcmd := range cmd.Commands() {
		switch subcmd.Use {
		case "status":
			hasStatus = true
		case "shutdown":
			hasShutdown = true
		}
	}

	if !hasStatus {
		t.Error("daemon command should have 'status' subcommand")
	}
	if !hasShutdown {
		t.Error("daemon command should have 'shutdown' subcommand")
	}
}

// TestDaemonStatusCommand verifies the status subcommand behavior.
func TestDaemonStatusCommand(t *testing.T) {
	tests := []struct {
		name      string
		client    *mockPingClient
		args      []string
		exitCode  int
		verifyOut func(*testing.T, string)
	}{
		{
			name:     "daemon is running",
			client:   &mockPingClient{latency: 15 * time.Millisecond},
			args:     []string{"status"},
			exitCode: 0,
			verifyOut: func(t *testing.T, out string) {
				if !strings.Contains(out, "Daemon is running") {
					t.Errorf("expected 'Daemon is running' in output, got: %s", out)
				}
				if !strings.Contains(out, "Latency:") {
					t.Errorf("expected latency in output, got: %s", out)
				}
			},
		},
		{
			name:     "daemon is not running",
			client:   &mockPingClient{err: context.DeadlineExceeded},
			args:     []string{"status"},
			exitCode: ExitError,
			verifyOut: func(t *testing.T, out string) {
				// Cobra shows usage on error, just check it failed
				if len(out) == 0 {
					t.Error("expected some output")
				}
			},
		},
		{
			name:     "connection refused",
			client:   &mockPingClient{err: &stubError{msg: "connection refused"}},
			args:     []string{"status"},
			exitCode: ExitError,
			verifyOut: func(t *testing.T, out string) {
				// Cobra shows usage on error, just check it failed
				if len(out) == 0 {
					t.Error("expected some output")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewDaemonCommand(tt.client, "127.0.0.1:9005")
			cmd.SetArgs(tt.args)

			out := &bytes.Buffer{}
			errOut := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(errOut)

			exitCode := ExecuteWithCommand(cmd)

			if exitCode != tt.exitCode {
				t.Errorf("expected exit code %d, got %d", tt.exitCode, exitCode)
			}

			// Verify ping was called
			if !tt.client.pingCalled {
				t.Error("expected Ping to be called")
			}

			if tt.verifyOut != nil {
				output := out.String()
				if output == "" {
					output = errOut.String()
				}
				tt.verifyOut(t, output)
			}
		})
	}
}

// TestDaemonShutdownCommand verifies the shutdown subcommand behavior.
func TestDaemonShutdownCommand(t *testing.T) {
	cmd := NewDaemonCommand(&mockPingClient{}, "127.0.0.1:9005")
	cmd.SetArgs([]string{"shutdown"})

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)

	exitCode := ExecuteWithCommand(cmd)

	// Shutdown should fail (not implemented)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	if !strings.Contains(errOut.String(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' in error output, got: %s", errOut.String())
	}
}

// stubError is a simple error implementation for testing.
type stubError struct {
	msg string
}

func (s *stubError) Error() string {
	return s.msg
}
