package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAskClient is a mock implementation of AskClient for testing.
type mockAskClient struct {
	chatStreamFunc func(ctx context.Context, messages []Message) (<-chan StreamEvent, error)
}

func (m *mockAskClient) ChatStream(ctx context.Context, messages []Message) (<-chan StreamEvent, error) {
	if m.chatStreamFunc != nil {
		return m.chatStreamFunc(ctx, messages)
	}
	// Default mock implementation
	eventCh := make(chan StreamEvent, 2)
	eventCh <- StreamEvent{Type: "text", Content: "Hi!"}
	eventCh <- StreamEvent{Type: "done", Content: ""}
	close(eventCh)
	return eventCh, nil
}

func TestNewAskCommand(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		client       AskClient
		wantExitCode int
		wantOutput   string
		wantError    bool
	}{
		{
			name:         "no args no stdin - usage error",
			args:         []string{},
			client:       &mockAskClient{},
			wantExitCode: 2, // ExitUsage
			wantError:    true,
		},
		{
			name: "successful ask with message",
			args: []string{"Hello"},
			client: &mockAskClient{
				chatStreamFunc: func(ctx context.Context, messages []Message) (<-chan StreamEvent, error) {
					eventCh := make(chan StreamEvent, 2)
					eventCh <- StreamEvent{Type: "text", Content: "Hi!"}
					eventCh <- StreamEvent{Type: "done", Content: ""}
					close(eventCh)
					return eventCh, nil
				},
			},
			wantExitCode: 0,
			wantOutput:   "Hi!\n",
			wantError:    false,
		},
		{
			name: "streaming response",
			args: []string{"Test"},
			client: &mockAskClient{
				chatStreamFunc: func(ctx context.Context, messages []Message) (<-chan StreamEvent, error) {
					eventCh := make(chan StreamEvent, 4)
					eventCh <- StreamEvent{Type: "text", Content: "Hello"}
					eventCh <- StreamEvent{Type: "text", Content: " World"}
					eventCh <- StreamEvent{Type: "text", Content: "!"}
					eventCh <- StreamEvent{Type: "done", Content: ""}
					close(eventCh)
					return eventCh, nil
				},
			},
			wantExitCode: 0,
			wantOutput:   "Hello World!\n",
			wantError:    false,
		},
		{
			name: "error event from stream",
			args: []string{"Test"},
			client: &mockAskClient{
				chatStreamFunc: func(ctx context.Context, messages []Message) (<-chan StreamEvent, error) {
					eventCh := make(chan StreamEvent, 2)
					eventCh <- StreamEvent{Type: "error", Content: "LLM error"}
					close(eventCh)
					return eventCh, nil
				},
			},
			wantExitCode: 1,
			wantError:    true,
		},
		{
			name: "daemon unreachable",
			args: []string{"Test"},
			client: &mockAskClient{
				chatStreamFunc: func(ctx context.Context, messages []Message) (<-chan StreamEvent, error) {
					return nil, &mockUnreachableError{}
				},
			},
			wantExitCode: 69, // ExitUnavailable
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewAskCommand(tt.client, "127.0.0.1:17891")

			// Add persistent flag for daemon-addr (inherited from parent)
			cmd.PersistentFlags().String("daemon-addr", "127.0.0.1:17891", "Address of the goose daemon")

			// Capture output
			var outBuf, errBuf bytes.Buffer
			cmd.SetOut(&outBuf)
			cmd.SetErr(&errBuf)
			cmd.SetArgs(tt.args)

			// Execute command
			err := cmd.Execute()

			// Check error expectation
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Check output
			if tt.wantOutput != "" {
				assert.Equal(t, tt.wantOutput, outBuf.String())
			}
		})
	}
}

func TestAskCommand_Timeout(t *testing.T) {
	// Test timeout behavior
	client := &mockAskClient{
		chatStreamFunc: func(ctx context.Context, messages []Message) (<-chan StreamEvent, error) {
			eventCh := make(chan StreamEvent)
			// Simulate slow response - check if context is cancelled
			go func() {
				select {
				case <-time.After(2 * time.Second):
					close(eventCh)
				case <-ctx.Done():
					// Context cancelled - return error
					eventCh <- StreamEvent{Type: "error", Content: "context deadline exceeded"}
					close(eventCh)
				}
			}()
			return eventCh, nil
		},
	}

	cmd := NewAskCommand(client, "127.0.0.1:17891")
	cmd.PersistentFlags().String("daemon-addr", "127.0.0.1:17891", "")
	cmd.SetArgs([]string{"Test"})
	cmd.Flags().Set("timeout", "100ms") // Short timeout

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	start := time.Now()
	err := cmd.Execute()
	duration := time.Since(start)

	// Should timeout quickly
	assert.Error(t, err)
	assert.Less(t, duration, 500*time.Millisecond, "timeout should trigger quickly")
}

func TestAskCommand_Stdin(t *testing.T) {
	// Test --stdin flag
	client := &mockAskClient{
		chatStreamFunc: func(ctx context.Context, messages []Message) (<-chan StreamEvent, error) {
			eventCh := make(chan StreamEvent, 2)
			eventCh <- StreamEvent{Type: "text", Content: "Hello from stdin"}
			eventCh <- StreamEvent{Type: "done", Content: ""}
			close(eventCh)
			return eventCh, nil
		},
	}

	cmd := NewAskCommand(client, "127.0.0.1:17891")
	cmd.PersistentFlags().String("daemon-addr", "127.0.0.1:17891", "")

	// Simulate stdin input
	stdin := strings.NewReader("Hello from stdin\n")
	cmd.SetIn(stdin)

	// Set --stdin flag
	require.NoError(t, cmd.Flags().Set("stdin", "true"))

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, outBuf.String(), "Hello from stdin")
}

// mockUnreachableError simulates daemon unreachable error.
type mockUnreachableError struct{}

func (e *mockUnreachableError) Error() string {
	return "connection refused: daemon unreachable"
}
