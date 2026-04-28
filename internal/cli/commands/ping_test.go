package commands_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/modu-ai/goose/internal/cli"
	"github.com/modu-ai/goose/internal/cli/commands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPingClient is a mock implementation of the ping client interface.
type mockPingClient struct {
	pingFunc func(ctx context.Context, addr string, writer io.Writer) error
}

func (m *mockPingClient) Ping(ctx context.Context, addr string, writer io.Writer) error {
	if m.pingFunc != nil {
		return m.pingFunc(ctx, addr, writer)
	}
	return nil
}

func TestPingCommandSuccess(t *testing.T) {
	t.Parallel()

	mockClient := &mockPingClient{
		pingFunc: func(ctx context.Context, addr string, writer io.Writer) error {
			// Print ping response to stdout
			fmt.Fprintln(writer, "pong (version=dev, state=serving, uptime=0s)")
			return nil
		},
	}

	cmd := commands.NewPingCommand(mockClient, "127.0.0.1:9005")
	// Set the daemon-addr flag on the command
	cmd.PersistentFlags().String("daemon-addr", "127.0.0.1:9005", "")

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	require.NoError(t, err)

	output := strings.TrimSpace(buf.String())
	assert.Contains(t, output, "pong")
	assert.Contains(t, output, "version=")
	assert.Contains(t, output, "state=serving")
	assert.Contains(t, output, "uptime=")
}

func TestPingCommandDaemonUnreachable(t *testing.T) {
	t.Parallel()

	mockClient := &mockPingClient{
		pingFunc: func(ctx context.Context, addr string, writer io.Writer) error {
			return errors.New("connection refused")
		},
	}

	cmd := commands.NewPingCommand(mockClient, "127.0.0.1:9005")
	// Set the daemon-addr flag on the command
	cmd.PersistentFlags().String("daemon-addr", "127.0.0.1:9005", "")

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	require.Error(t, err)

	errOutput := buf.String()
	assert.True(t, strings.HasPrefix(errOutput, "goose:"), "Error should start with 'goose:' prefix")
	assert.Contains(t, errOutput, "daemon unreachable")
	assert.Contains(t, errOutput, "127.0.0.1:9005")

	// Verify exit code constant
	exitCode := cli.ExitUnavailable
	assert.Equal(t, 69, exitCode)
}

func TestPingCommandCustomAddress(t *testing.T) {
	t.Parallel()

	customAddr := "192.168.1.100:9999"
	mockClient := &mockPingClient{
		pingFunc: func(ctx context.Context, addr string, writer io.Writer) error {
			assert.Equal(t, customAddr, addr)
			fmt.Fprintln(writer, "pong (version=dev, state=serving, uptime=0s)")
			return nil
		},
	}

	cmd := commands.NewPingCommand(mockClient, customAddr)
	// Set the daemon-addr flag to the custom address
	cmd.PersistentFlags().String("daemon-addr", customAddr, "")

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.Execute()
	require.NoError(t, err)
}
