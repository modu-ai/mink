package cli_test

import (
	"testing"

	"github.com/modu-ai/goose/internal/cli"
	"github.com/stretchr/testify/assert"
)

func TestExecuteWithValidVersion(t *testing.T) {
	t.Parallel()

	// Test with version subcommand to avoid TUI startup
	cmd := cli.NewRootCommand("v0.1.0", "abc123", "2026-04-28")
	cmd.SetArgs([]string{"version"})
	exitCode := cli.ExecuteWithCommand(cmd)
	assert.Equal(t, cli.ExitOK, exitCode)
}

func TestExecuteWithDevVersion(t *testing.T) {
	t.Parallel()

	// Test with version subcommand to avoid TUI startup
	cmd := cli.NewRootCommand("dev", "none", "unknown")
	cmd.SetArgs([]string{"version"})
	exitCode := cli.ExecuteWithCommand(cmd)
	assert.Equal(t, cli.ExitOK, exitCode)
}

func TestExitCodeConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, cli.ExitOK)
	assert.Equal(t, 1, cli.ExitError)
	assert.Equal(t, 2, cli.ExitUsage)
	assert.Equal(t, 69, cli.ExitUnavailable)
	assert.Equal(t, 78, cli.ExitConfig)
}
