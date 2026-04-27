// Package command implements the slash command system for AI.GOOSE.
// SPEC: SPEC-GOOSE-COMMAND-001
package command_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/modu-ai/goose/internal/command"
	// Import custom to trigger init() which wires SetCustomLoader.
	_ "github.com/modu-ai/goose/internal/command/custom"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// stubCommand is a minimal Command implementation for registry tests.
type stubCommand struct {
	name string
	src  command.Source
}

func (s *stubCommand) Name() string               { return s.name }
func (s *stubCommand) Metadata() command.Metadata { return command.Metadata{Source: s.src} }
func (s *stubCommand) Execute(_ context.Context, _ command.Args) (command.Result, error) {
	return command.Result{Kind: command.ResultLocalReply, Text: "stub"}, nil
}

// TestRegistry_Precedence_BuiltinWinsCustom verifies that a builtin command
// shadows a same-named custom command and that a WARN log is emitted for the
// shadowed entry. RED #3 — AC-CMD-011, REQ-CMD-002.
func TestRegistry_Precedence_BuiltinWinsCustom(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	reg, err := command.NewRegistry(command.WithLogger(logger))
	require.NoError(t, err)

	builtin := &stubCommand{name: "help", src: command.SourceBuiltin}
	custom := &stubCommand{name: "help", src: command.SourceCustomProject}

	require.NoError(t, reg.Register(builtin, command.SourceBuiltin))
	require.NoError(t, reg.Register(custom, command.SourceCustomProject))

	// Builtin must win resolution.
	resolved, ok := reg.Resolve("help")
	require.True(t, ok)
	require.Equal(t, command.SourceBuiltin, resolved.Metadata().Source)

	// A WARN log containing the shadowed name must have been emitted when the
	// lower-priority custom command was registered. AC-CMD-011.
	warnLogs := recorded.FilterMessage("command shadowed").All()
	require.NotEmpty(t, warnLogs, "expected a WARN log for cross-tier shadowing")
	// Verify that the 'name' field identifies the shadowed command.
	found := false
	for _, entry := range warnLogs {
		for _, f := range entry.Context {
			if f.Key == "name" && f.String == "help" {
				found = true
			}
		}
	}
	require.True(t, found, "WARN log must include the shadowed command name")
}

// TestRegistry_InvalidName verifies that a name failing [a-z0-9_-] is rejected.
// REQ-CMD-003.
func TestRegistry_InvalidName(t *testing.T) {
	t.Parallel()

	reg, err := command.NewRegistry()
	require.NoError(t, err)

	bad := &stubCommand{name: "has space", src: command.SourceBuiltin}
	err = reg.Register(bad, command.SourceBuiltin)
	require.ErrorIs(t, err, command.ErrInvalidCommandName)
}

// TestRegistry_Resolve_NotFound verifies that Resolve returns false for unknown names.
func TestRegistry_Resolve_NotFound(t *testing.T) {
	t.Parallel()

	reg, err := command.NewRegistry()
	require.NoError(t, err)

	_, ok := reg.Resolve("nonexistent")
	require.False(t, ok)
}

// TestRegistry_List_ReturnsMetadata verifies that List returns registered command metadata.
func TestRegistry_List_ReturnsMetadata(t *testing.T) {
	t.Parallel()

	reg, err := command.NewRegistry()
	require.NoError(t, err)

	cmd := &stubCommand{name: "foo", src: command.SourceBuiltin}
	require.NoError(t, reg.Register(cmd, command.SourceBuiltin))

	list := reg.List()
	require.Len(t, list, 1)
	require.Equal(t, command.SourceBuiltin, list[0].Source)
}

// TestRegistry_ProviderCommands verifies that RegisterProvider exposes provider commands.
func TestRegistry_ProviderCommands(t *testing.T) {
	t.Parallel()

	reg, err := command.NewRegistry()
	require.NoError(t, err)

	skillCmd := &stubCommand{name: "myskill", src: command.SourceSkill}
	provider := &stubProvider{cmds: []command.Command{skillCmd}}
	reg.RegisterProvider(provider)

	resolved, ok := reg.Resolve("myskill")
	require.True(t, ok)
	require.Equal(t, command.SourceSkill, resolved.Metadata().Source)
}

// TestRegistry_Reload_AtomicSwap verifies Reload does not leave the registry empty.
func TestRegistry_Reload_AtomicSwap(t *testing.T) {
	t.Parallel()

	reg, err := command.NewRegistry()
	require.NoError(t, err)

	cmd := &stubCommand{name: "persistent", src: command.SourceBuiltin}
	require.NoError(t, reg.Register(cmd, command.SourceBuiltin))

	// Reload without custom roots should preserve builtins and clear custom commands.
	require.NoError(t, reg.Reload(context.Background()))

	_, ok := reg.Resolve("persistent")
	require.True(t, ok, "builtin commands must survive Reload")
}

// TestRegistry_WithCustomRoots_Option verifies the option wires correctly.
func TestRegistry_WithCustomRoots_Option(t *testing.T) {
	t.Parallel()
	reg, err := command.NewRegistry(command.WithCustomRoots("/tmp/nowhere"))
	require.NoError(t, err)
	require.NotNil(t, reg)
}

// TestRegistry_WithProvider_Option verifies that WithProvider pre-registers a provider.
func TestRegistry_WithProvider_Option(t *testing.T) {
	t.Parallel()
	skillCmd := &stubCommand{name: "skill-preloaded", src: command.SourceSkill}
	provider := &stubProvider{cmds: []command.Command{skillCmd}}

	reg, err := command.NewRegistry(command.WithProvider(provider))
	require.NoError(t, err)

	_, ok := reg.Resolve("skill-preloaded")
	require.True(t, ok)
}

// TestRegistry_List_SortedAlphabetically verifies List returns metadata in order.
func TestRegistry_List_SortedAlphabetically(t *testing.T) {
	t.Parallel()
	reg, err := command.NewRegistry()
	require.NoError(t, err)

	require.NoError(t, reg.Register(&stubCommand{name: "zebra", src: command.SourceBuiltin}, command.SourceBuiltin))
	require.NoError(t, reg.Register(&stubCommand{name: "alpha", src: command.SourceBuiltin}, command.SourceBuiltin))

	list := reg.List()
	require.Len(t, list, 2)
}

// TestRegistry_ListNamed_IncludesCustom verifies ListNamed includes custom commands.
func TestRegistry_ListNamed_IncludesCustom(t *testing.T) {
	t.Parallel()
	reg, err := command.NewRegistry()
	require.NoError(t, err)

	require.NoError(t, reg.Register(&stubCommand{name: "mycmd", src: command.SourceCustomProject}, command.SourceCustomProject))

	named := reg.ListNamed()
	require.Len(t, named, 1)
	require.Equal(t, "mycmd", named[0].Name)
}

// TestRegistry_RegisterAlias verifies alias resolution works.
func TestRegistry_RegisterAlias(t *testing.T) {
	t.Parallel()
	reg, err := command.NewRegistry()
	require.NoError(t, err)

	exitCmd := &stubCommand{name: "exit", src: command.SourceBuiltin}
	require.NoError(t, reg.Register(exitCmd, command.SourceBuiltin))
	reg.RegisterAlias("quit", "exit")

	resolved, ok := reg.Resolve("quit")
	require.True(t, ok)
	require.Equal(t, "exit", resolved.Name())
}

// TestRegistry_Precedence_ProjectBeatsUser verifies custom-project beats custom-user.
func TestRegistry_Precedence_ProjectBeatsUser(t *testing.T) {
	t.Parallel()

	reg, err := command.NewRegistry()
	require.NoError(t, err)

	projectCmd := &stubCommand{name: "overlap", src: command.SourceCustomProject}
	userCmd := &stubCommand{name: "overlap", src: command.SourceCustomUser}

	require.NoError(t, reg.Register(projectCmd, command.SourceCustomProject))
	require.NoError(t, reg.Register(userCmd, command.SourceCustomUser))

	resolved, ok := reg.Resolve("overlap")
	require.True(t, ok)
	require.Equal(t, command.SourceCustomProject, resolved.Metadata().Source)
}

// TestRegistry_List_IncludesProvider verifies List includes skill-backed commands.
func TestRegistry_List_IncludesProvider(t *testing.T) {
	t.Parallel()
	reg, err := command.NewRegistry()
	require.NoError(t, err)

	skillCmd := &stubCommand{name: "skill-list", src: command.SourceSkill}
	reg.RegisterProvider(&stubProvider{cmds: []command.Command{skillCmd}})

	list := reg.List()
	require.Len(t, list, 1)
}

// TestRegistry_ListNamed_IncludesProvider verifies ListNamed includes skill providers.
func TestRegistry_ListNamed_IncludesProvider(t *testing.T) {
	t.Parallel()
	reg, err := command.NewRegistry()
	require.NoError(t, err)

	skillCmd := &stubCommand{name: "skill-named", src: command.SourceSkill}
	reg.RegisterProvider(&stubProvider{cmds: []command.Command{skillCmd}})

	named := reg.ListNamed()
	require.Len(t, named, 1)
	require.Equal(t, "skill-named", named[0].Name)
}

// TestDispatcher_SctxContextKey verifies the exported key is non-nil.
func TestDispatcher_SctxContextKey(t *testing.T) {
	t.Parallel()
	key := command.SctxContextKey()
	require.NotNil(t, key)
}

// TestRegistry_Reload_RescansCustomRoots verifies that Reload re-scans configured
// custom roots and exposes newly added commands. REQ-CMD-010.
func TestRegistry_Reload_RescansCustomRoots(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Step 1: Write an initial command file.
	initialPath := filepath.Join(dir, "initial.md")
	require.NoError(t, os.WriteFile(initialPath,
		[]byte("---\nname: initial\ndescription: initial command\n---\nBody"), 0o600))

	logger := zap.NewNop()
	reg, err := command.NewRegistry(
		command.WithLogger(logger),
		command.WithCustomRoots(dir),
	)
	require.NoError(t, err)

	// Step 2: Reload — initial.md should now be resolvable.
	require.NoError(t, reg.Reload(context.Background()))
	_, ok := reg.Resolve("initial")
	require.True(t, ok, "Reload must expose initial command")

	// Step 3: Add another.md to the directory.
	anotherPath := filepath.Join(dir, "another.md")
	require.NoError(t, os.WriteFile(anotherPath,
		[]byte("---\nname: another\ndescription: another command\n---\nBody"), 0o600))

	// Step 4: Reload again — another.md must now be resolvable AND initial still works.
	require.NoError(t, reg.Reload(context.Background()))
	_, ok = reg.Resolve("another")
	require.True(t, ok, "Reload must expose newly added command")
	_, ok = reg.Resolve("initial")
	require.True(t, ok, "Reload must preserve previously loaded commands")
}

// TestRegistry_Register_UserSource verifies user-source commands are registered and resolvable.
func TestRegistry_Register_UserSource(t *testing.T) {
	t.Parallel()
	reg, err := command.NewRegistry()
	require.NoError(t, err)

	userCmd := &stubCommand{name: "usercmd", src: command.SourceCustomUser}
	require.NoError(t, reg.Register(userCmd, command.SourceCustomUser))

	resolved, ok := reg.Resolve("usercmd")
	require.True(t, ok)
	require.Equal(t, command.SourceCustomUser, resolved.Metadata().Source)
}

// TestRegistry_ShadowedBuiltin_LogsSourceString verifies that when a builtin command
// is registered and then shadowed (duplicate builtin registration), the WARN log that
// calls sourceString() is emitted. Task A-3: exercises sourceString code path.
func TestRegistry_ShadowedBuiltin_LogsSourceString(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	reg, err := command.NewRegistry(command.WithLogger(logger))
	require.NoError(t, err)

	// Register the same builtin name twice; the second registration causes
	// the "builtin command shadowed" log which calls sourceString(existing.Metadata().Source).
	first := &stubCommand{name: "shadowed", src: command.SourceBuiltin}
	second := &stubCommand{name: "shadowed", src: command.SourceBuiltin}

	require.NoError(t, reg.Register(first, command.SourceBuiltin))
	require.NoError(t, reg.Register(second, command.SourceBuiltin))

	// A WARN log mentioning "builtin command shadowed" must be emitted.
	warnLogs := recorded.FilterMessage("builtin command shadowed").All()
	require.NotEmpty(t, warnLogs, "expected WARN log for duplicate builtin registration (exercises sourceString)")
}

// TestRegistry_WarnCrossTierShadow_ProjectShadowsBuiltin verifies that registering a
// custom-project command whose name already exists in the builtin tier emits a WARN.
// Task A-4: exercises warnCrossTierShadow builtin-tier check path.
func TestRegistry_WarnCrossTierShadow_ProjectShadowsBuiltin(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	reg, err := command.NewRegistry(command.WithLogger(logger))
	require.NoError(t, err)

	// Register a builtin command first.
	builtinCmd := &stubCommand{name: "overlap-b", src: command.SourceBuiltin}
	require.NoError(t, reg.Register(builtinCmd, command.SourceBuiltin))

	// Register a custom-project command with the same name.
	// warnCrossTierShadow must detect the builtin and emit a "command shadowed" WARN.
	projectCmd := &stubCommand{name: "overlap-b", src: command.SourceCustomProject}
	require.NoError(t, reg.Register(projectCmd, command.SourceCustomProject))

	warnLogs := recorded.FilterMessage("command shadowed").All()
	require.NotEmpty(t, warnLogs, "WARN must be emitted for custom-project shadowing builtin")
}

// TestRegistry_WarnCrossTierShadow_UserShadowsProject verifies that registering a
// custom-user command whose name already exists in the custom-project tier emits a WARN.
// Task A-4: exercises warnCrossTierShadow project-tier check (src > SourceCustomProject).
func TestRegistry_WarnCrossTierShadow_UserShadowsProject(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	reg, err := command.NewRegistry(command.WithLogger(logger))
	require.NoError(t, err)

	// Register a custom-project command first.
	projectCmd := &stubCommand{name: "overlap-p", src: command.SourceCustomProject}
	require.NoError(t, reg.Register(projectCmd, command.SourceCustomProject))

	// Register a custom-user command with the same name.
	// warnCrossTierShadow must detect the project entry and emit a WARN.
	userCmd := &stubCommand{name: "overlap-p", src: command.SourceCustomUser}
	require.NoError(t, reg.Register(userCmd, command.SourceCustomUser))

	warnLogs := recorded.FilterMessage("command shadowed").All()
	require.NotEmpty(t, warnLogs, "WARN must be emitted for custom-user shadowing custom-project")
}

// TestRegistry_ShadowedCustomProject_LogsSourceString verifies that a shadowed
// custom-project command causes sourceString("custom-project") to be called via the
// "custom-project command shadowed" WARN log. Task A-3: covers sourceString case.
func TestRegistry_ShadowedCustomProject_LogsSourceString(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	reg, err := command.NewRegistry(command.WithLogger(logger))
	require.NoError(t, err)

	// Register the same custom-project name twice — second triggers "custom-project command shadowed"
	// log which reads existing.Metadata().Source (SourceCustomProject → sourceString coverage).
	first := &stubCommand{name: "dup-project", src: command.SourceCustomProject}
	second := &stubCommand{name: "dup-project", src: command.SourceCustomProject}

	require.NoError(t, reg.Register(first, command.SourceCustomProject))
	require.NoError(t, reg.Register(second, command.SourceCustomProject))

	warnLogs := recorded.FilterMessage("custom-project command shadowed").All()
	require.NotEmpty(t, warnLogs, "expected WARN for duplicate custom-project registration (exercises sourceString custom-project case)")
}

// TestRegistry_ShadowedCustomUser_LogsSourceString verifies that a shadowed
// custom-user command causes sourceString("custom-user") to be called via the
// "custom-user command shadowed" WARN log. Task A-3: covers sourceString custom-user case.
func TestRegistry_ShadowedCustomUser_LogsSourceString(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	reg, err := command.NewRegistry(command.WithLogger(logger))
	require.NoError(t, err)

	// Register the same custom-user name twice — second triggers "custom-user command shadowed"
	// log which reads existing.Metadata().Source (SourceCustomUser → sourceString coverage).
	first := &stubCommand{name: "dup-user", src: command.SourceCustomUser}
	second := &stubCommand{name: "dup-user", src: command.SourceCustomUser}

	require.NoError(t, reg.Register(first, command.SourceCustomUser))
	require.NoError(t, reg.Register(second, command.SourceCustomUser))

	warnLogs := recorded.FilterMessage("custom-user command shadowed").All()
	require.NotEmpty(t, warnLogs, "expected WARN for duplicate custom-user registration (exercises sourceString custom-user case)")
}

// stubProvider is a minimal Provider implementation.
type stubProvider struct {
	cmds []command.Command
}

func (p *stubProvider) Commands() []command.Command { return p.cmds }
