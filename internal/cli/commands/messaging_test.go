package commands_test

import (
	"bytes"
	"testing"

	"github.com/modu-ai/goose/internal/cli/commands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMessagingCommand verifies that the parent messaging command registers
// expected subcommands and is routable without error.
func TestNewMessagingCommand(t *testing.T) {
	cmd := commands.NewMessagingCommand()
	require.NotNil(t, cmd)
	assert.Equal(t, "messaging", cmd.Use)

	// Verify that "telegram" subcommand is registered.
	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Use == "telegram" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'telegram' subcommand under 'messaging'")
}

// TestMessagingCommand_HelpExits0 verifies that --help exits cleanly.
func TestMessagingCommand_HelpExits0(t *testing.T) {
	cmd := commands.NewMessagingCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	// Cobra returns nil on --help unless DisableFlagParsing is set
	assert.NoError(t, err)
}
