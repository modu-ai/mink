package commands_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/cli"
	"github.com/modu-ai/mink/internal/cli/commands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		version  string
		commit   string
		builtAt  string
		expected string
	}{
		{
			name:     "production version",
			version:  "v0.1.0",
			commit:   "abc123def",
			builtAt:  "2026-04-28T10:00:00Z",
			expected: "goose version v0.1.0 (commit abc123def, built 2026-04-28T10:00:00Z)",
		},
		{
			name:     "development version",
			version:  "dev",
			commit:   "none",
			builtAt:  "unknown",
			expected: "goose version dev (commit none, built unknown)",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := commands.NewVersionCommand(tt.version, tt.commit, tt.builtAt)
			var buf bytes.Buffer
			cmd.SetOut(&buf)

			err := cmd.Execute()
			require.NoError(t, err)

			output := strings.TrimSpace(buf.String())
			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestVersionCommandExitCode(t *testing.T) {
	t.Parallel()

	cmd := commands.NewVersionCommand("v0.1.0", "abc123", "2026-04-28")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.Execute()
	assert.NoError(t, err)

	// Verify exit code through Execute wrapper
	exitCode := cli.ExecuteWithCommand(cmd)
	assert.Equal(t, cli.ExitOK, exitCode)
}
