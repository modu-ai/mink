package cli_test

import (
	"bytes"
	"testing"

	"github.com/modu-ai/goose/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommandCreation(t *testing.T) {
	t.Parallel()

	rootCmd := cli.NewRootCommand("v0.1.0", "abc123", "2026-04-28")
	assert.NotNil(t, rootCmd)
	assert.Equal(t, "goose", rootCmd.Name())
	assert.True(t, rootCmd.SilenceUsage)
	assert.True(t, rootCmd.SilenceErrors)
}

func TestRootCommandGlobalFlags(t *testing.T) {
	t.Parallel()

	rootCmd := cli.NewRootCommand("v0.1.0", "abc123", "2026-04-28")

	// Test --config flag (PersistentFlags)
	flag := rootCmd.PersistentFlags().Lookup("config")
	assert.NotNil(t, flag)
	assert.Equal(t, "string", flag.Value.Type())

	// Test --daemon-addr flag (PersistentFlags)
	flag = rootCmd.PersistentFlags().Lookup("daemon-addr")
	assert.NotNil(t, flag)
	assert.Equal(t, "string", flag.Value.Type())

	// Test --format flag (PersistentFlags)
	flag = rootCmd.PersistentFlags().Lookup("format")
	assert.NotNil(t, flag)
	assert.Equal(t, "string", flag.Value.Type())

	// Test --log-level flag (PersistentFlags)
	flag = rootCmd.PersistentFlags().Lookup("log-level")
	assert.NotNil(t, flag)
	assert.Equal(t, "string", flag.Value.Type())

	// Test --no-color flag (PersistentFlags)
	flag = rootCmd.PersistentFlags().Lookup("no-color")
	assert.NotNil(t, flag)
	assert.Equal(t, "bool", flag.Value.Type())
}

func TestRootCommandDefaultBehavior(t *testing.T) {
	t.Parallel()

	rootCmd := cli.NewRootCommand("v0.1.0", "abc123", "2026-04-28")
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{})

	// The TUI requires a TTY, which isn't available in tests
	// We expect an error about TTY not being available
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to start TUI")
}

func TestRootCommandWithInvalidFlag(t *testing.T) {
	t.Parallel()

	rootCmd := cli.NewRootCommand("v0.1.0", "abc123", "2026-04-28")
	rootCmd.SetArgs([]string{"--invalid-flag"})

	err := rootCmd.Execute()
	assert.Error(t, err)

	// With SilenceErrors: true, the error is returned but not printed
	// The error message itself should contain the flag information
	assert.ErrorContains(t, err, "unknown flag")
}

func TestRootCommandDaemonAddrDefault(t *testing.T) {
	t.Parallel()

	rootCmd := cli.NewRootCommand("v0.1.0", "abc123", "2026-04-28")
	flag := rootCmd.PersistentFlags().Lookup("daemon-addr")
	require.NotNil(t, flag)

	// Test default value
	assert.Equal(t, "127.0.0.1:9005", flag.DefValue)
}
