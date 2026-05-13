package cli_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/modu-ai/mink/internal/cli"
	"github.com/spf13/cobra"
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

// TestRootCommand_PhaseB_SubcommandsRegistered verifies that the Phase B
// wiring (ping / ask / config / tool / daemon) is reachable via cobra's
// command tree. This guards against regressions where a wiring change
// silently drops a subcommand from the root.
func TestRootCommand_PhaseB_SubcommandsRegistered(t *testing.T) {
	t.Parallel()

	rootCmd := cli.NewRootCommand("v0.1.0", "abc123", "2026-04-28")

	have := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		have[c.Name()] = true
	}

	for _, name := range []string{"ping", "ask", "config", "tool", "daemon"} {
		assert.True(t, have[name], "subcommand %q must be registered", name)
	}
}

// TestRootCommand_MigrateOnce_CalledOnStartup verifies that PersistentPreRunE
// invokes userpath.MigrateOnce without error when no legacy ~/.goose directory
// exists (migration is a no-op).
// T-015: SPEC-MINK-USERDATA-MIGRATE-001 CLI entrypoint wiring.
func TestRootCommand_MigrateOnce_CalledOnStartup(t *testing.T) {
	// 테스트 격리: HOME → tmpDir (레거시 .goose 없음), MINK_HOME 해제
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("MINK_HOME", "")
	// MINK_HOME 빈 문자열 → ErrMinkHomeEmpty 방지: 키 자체를 제거
	os.Unsetenv("MINK_HOME") //nolint:errcheck
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") }) //nolint:errcheck

	rootCmd := cli.NewRootCommand("v0.1.0", "abc123", "2026-05-13")

	// dummy 서브커맨드로 PersistentPreRunE 를 실행시킨다
	ran := false
	dummyCmd := &cobra.Command{
		Use: "dummy",
		RunE: func(cmd *cobra.Command, args []string) error {
			ran = true
			return nil
		},
	}
	rootCmd.AddCommand(dummyCmd)
	rootCmd.SetArgs([]string{"dummy"})

	err := rootCmd.Execute()
	require.NoError(t, err, "MigrateOnce no-op 시 CLI 시작은 성공해야 함")
	assert.True(t, ran, "dummy 커맨드가 실행되어야 함")
}

// TestRootCommand_InitAppWiring verifies that PersistentPreRunE initializes App.
func TestRootCommand_InitAppWiring(t *testing.T) {
	t.Parallel()

	rootCmd := cli.NewRootCommand("v0.1.0", "abc123", "2026-04-28")

	// Set up a dummy subcommand that can retrieve App from context
	var retrievedApp *cli.App
	dummyCmd := &cobra.Command{
		Use: "dummy",
		RunE: func(cmd *cobra.Command, args []string) error {
			retrievedApp = cli.AppFromContext(cmd.Context())
			return nil
		},
	}
	rootCmd.AddCommand(dummyCmd)

	// Execute the dummy command
	rootCmd.SetArgs([]string{"dummy"})
	err := rootCmd.Execute()

	// Should not error
	require.NoError(t, err)

	// App should be initialized and retrievable from context
	assert.NotNil(t, retrievedApp, "App should be initialized by PersistentPreRunE")
	assert.NotNil(t, retrievedApp.Dispatcher, "App.Dispatcher should be initialized")
	assert.NotNil(t, retrievedApp.Adapter, "App.Adapter should be initialized")
}
