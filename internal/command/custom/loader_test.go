// Package custom loads custom slash commands from Markdown files.
// SPEC: SPEC-GOOSE-COMMAND-001
package custom_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/modu-ai/goose/internal/command"
	"github.com/modu-ai/goose/internal/command/custom"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// Ensure runtime import is used — needed for TestCustomLoader_UnreadableFile (chmod test).
var _ = runtime.GOOS

// writeFile is a helper to write a file in a temp directory.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// TestCustomLoader_MalformedSkipped verifies that a malformed YAML frontmatter file
// is skipped but other files in the same directory are loaded, and that an ERROR log
// referencing the bad file path is emitted. RED #14 — AC-CMD-010.
func TestCustomLoader_MalformedSkipped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "good.md", "---\nname: good\ndescription: good command\n---\nHello $ARGUMENTS")
	badPath := writeFile(t, dir, "bad.md", "---\nname: [\ninvalid yaml\n---\nBody")

	core, recorded := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)

	cmds, err := custom.LoadDir(dir, command.SourceCustomProject, logger)
	// LoadDir must not return an error even when some files are malformed.
	require.NoError(t, err)

	// Only the valid command must be loaded.
	names := make([]string, 0, len(cmds))
	for _, c := range cmds {
		names = append(names, c.Name())
	}
	require.Contains(t, names, "good")

	found := false
	for _, n := range names {
		if n == "bad" {
			found = true
		}
	}
	require.False(t, found, "malformed command must not be loaded")

	// An ERROR log referencing the bad file path must have been emitted. AC-CMD-010.
	errorLogs := recorded.All()
	require.NotEmpty(t, errorLogs, "expected at least one ERROR log for malformed file")
	badFileLogged := false
	for _, entry := range errorLogs {
		// Check the message and all fields for the bad.md path.
		if strings.Contains(entry.Message, badPath) {
			badFileLogged = true
			break
		}
		for _, f := range entry.Context {
			if strings.Contains(f.String, badPath) {
				badFileLogged = true
				break
			}
		}
		if badFileLogged {
			break
		}
	}
	require.True(t, badFileLogged, "ERROR log must reference the bad file path: %s", badPath)
}

// TestCustomLoader_FrontmatterMissingName verifies that a file without a name field
// is skipped. REQ-CMD-009.
func TestCustomLoader_FrontmatterMissingName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "noname.md", "---\ndescription: no name here\n---\nBody")

	logger := zap.NewNop()
	cmds, err := custom.LoadDir(dir, command.SourceCustomProject, logger)
	require.NoError(t, err)
	require.Empty(t, cmds)
}

// TestCustomLoader_FrontmatterMissingDescription verifies that a file without a
// description field is skipped.
func TestCustomLoader_FrontmatterMissingDescription(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "nodesc.md", "---\nname: nodesc\n---\nBody")

	logger := zap.NewNop()
	cmds, err := custom.LoadDir(dir, command.SourceCustomProject, logger)
	require.NoError(t, err)
	require.Empty(t, cmds)
}

// TestCustomLoader_ExpandsArguments verifies that $ARGUMENTS is substituted on Execute.
// AC-CMD-004.
func TestCustomLoader_ExpandsArguments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "greet.md",
		"---\nname: greet\ndescription: greet user\nargument-hint: \"<name>\"\n---\nHello $ARGUMENTS, welcome to GOOSE.")

	logger := zap.NewNop()
	cmds, err := custom.LoadDir(dir, command.SourceCustomProject, logger)
	require.NoError(t, err)
	require.Len(t, cmds, 1)

	args := command.Args{
		RawArgs:    "Alice",
		Positional: []string{"Alice"},
	}
	result, err := cmds[0].Execute(context.Background(), args)
	require.NoError(t, err)
	require.Equal(t, command.ResultPromptExpansion, result.Kind)
	require.Equal(t, "Hello Alice, welcome to GOOSE.", result.Prompt)
}

// TestCustomLoader_EmptyDir verifies that an empty directory returns no commands.
func TestCustomLoader_EmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logger := zap.NewNop()
	cmds, err := custom.LoadDir(dir, command.SourceCustomProject, logger)
	require.NoError(t, err)
	require.Empty(t, cmds)
}

// TestCustomLoader_Metadata verifies that the loaded command's Metadata is populated.
func TestCustomLoader_Metadata(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "meta.md",
		"---\nname: meta\ndescription: meta test\nargument-hint: \"<arg>\"\n---\nBody")

	logger := zap.NewNop()
	cmds, err := custom.LoadDir(dir, command.SourceCustomProject, logger)
	require.NoError(t, err)
	require.Len(t, cmds, 1)

	meta := cmds[0].Metadata()
	require.Equal(t, "meta test", meta.Description)
	require.Equal(t, "<arg>", meta.ArgumentHint)
	require.Equal(t, command.SourceCustomProject, meta.Source)
}

// TestCustomLoader_NonExistentDir verifies that a non-existent directory returns no error.
func TestCustomLoader_NonExistentDir(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	cmds, err := custom.LoadDir("/nonexistent/path/12345", command.SourceCustomProject, logger)
	require.NoError(t, err)
	require.Empty(t, cmds)
}

// TestParseFrontmatter_MissingClosingDelimiter verifies error on missing closing ---.
func TestParseFrontmatter_MissingClosingDelimiter(t *testing.T) {
	t.Parallel()

	data := []byte("---\nname: foo\ndescription: bar\n")
	_, _, err := custom.ParseFrontmatter(data)
	require.Error(t, err)
}

// TestParseFrontmatter_MissingOpeningDelimiter verifies error on missing opening ---.
func TestParseFrontmatter_MissingOpeningDelimiter(t *testing.T) {
	t.Parallel()

	data := []byte("name: foo\n---\nBody")
	_, _, err := custom.ParseFrontmatter(data)
	require.Error(t, err)
}

// TestCustomLoader_IgnoresNonMarkdown verifies that non-.md files are ignored.
func TestCustomLoader_IgnoresNonMarkdown(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "tool.sh", "#!/bin/bash\necho hello")
	writeFile(t, dir, "config.yaml", "name: tool")

	logger := zap.NewNop()
	cmds, err := custom.LoadDir(dir, command.SourceCustomProject, logger)
	require.NoError(t, err)
	require.Empty(t, cmds)
}

// TestCustomLoader_SymlinkEscapeRejected verifies that a symlink pointing outside
// the root directory is rejected. REQ-CMD-016.
func TestCustomLoader_SymlinkEscapeRejected(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()

	// Write a real file outside the root.
	outsideFile := filepath.Join(outside, "escape.md")
	require.NoError(t, os.WriteFile(outsideFile, []byte("---\nname: escape\ndescription: escaped\n---\nbody"), 0o600))

	// Create a symlink inside root pointing to the outside file.
	symlinkPath := filepath.Join(root, "escape.md")
	require.NoError(t, os.Symlink(outsideFile, symlinkPath))

	logger := zap.NewNop()
	cmds, err := custom.LoadDir(root, command.SourceCustomProject, logger)
	require.NoError(t, err)

	// The symlink-escaped file must not be loaded.
	for _, c := range cmds {
		require.NotEqual(t, "escape", c.Name(), "symlink escaping root must be rejected")
	}
}

// TestCustomLoader_UnreadableFile verifies that an unreadable .md file is skipped
// with an ERROR log but does not abort the load of other files.
// Skipped on Windows since chmod 000 semantics differ. Defect 5 coverage.
func TestCustomLoader_UnreadableFile(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 not supported on Windows")
	}

	dir := t.TempDir()
	writeFile(t, dir, "readable.md", "---\nname: readable\ndescription: readable command\n---\nBody")
	unreadablePath := writeFile(t, dir, "unreadable.md", "---\nname: unreadable\ndescription: should not load\n---\nBody")

	// Remove read permission so os.ReadFile fails.
	require.NoError(t, os.Chmod(unreadablePath, 0o000))
	t.Cleanup(func() {
		// Restore so TempDir cleanup can delete the file.
		_ = os.Chmod(unreadablePath, 0o600)
	})

	core, recorded := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)

	cmds, err := custom.LoadDir(dir, command.SourceCustomProject, logger)
	require.NoError(t, err, "unreadable file must not cause LoadDir to fail")

	// Only the readable command should be loaded.
	names := make([]string, 0, len(cmds))
	for _, c := range cmds {
		names = append(names, c.Name())
	}
	require.Contains(t, names, "readable")
	for _, n := range names {
		require.NotEqual(t, "unreadable", n, "unreadable file must be skipped")
	}

	// An ERROR log must mention the unreadable file.
	errorLogs := recorded.All()
	require.NotEmpty(t, errorLogs, "expected ERROR log for unreadable file")
	logged := false
	for _, entry := range errorLogs {
		if strings.Contains(entry.Message, unreadablePath) {
			logged = true
			break
		}
		for _, f := range entry.Context {
			if strings.Contains(f.String, unreadablePath) {
				logged = true
				break
			}
		}
		if logged {
			break
		}
	}
	require.True(t, logged, "ERROR log must reference the unreadable file path")
}

// TestCustomLoader_Execute_NoArgs verifies that Execute with no arguments
// returns the body with $ARGUMENTS substituted as empty string.
// Exercises the Execute happy path with no-arg input. Defect 5 coverage.
func TestCustomLoader_Execute_NoArgs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "greet2.md",
		"---\nname: greet2\ndescription: greet no args\n---\nHello $ARGUMENTS!")

	logger := zap.NewNop()
	cmds, err := custom.LoadDir(dir, command.SourceCustomProject, logger)
	require.NoError(t, err)
	require.Len(t, cmds, 1)

	result, execErr := cmds[0].Execute(context.Background(), command.Args{})
	require.NoError(t, execErr)
	require.Equal(t, command.ResultPromptExpansion, result.Kind)
	// With no RawArgs, $ARGUMENTS is substituted with "".
	require.Equal(t, "Hello !", result.Prompt)
}

// TestCustomLoader_Execute_WithPositionalArgs verifies that $1 etc. are substituted.
// Exercises Execute with positional argument substitution. Defect 5 coverage.
func TestCustomLoader_Execute_WithPositionalArgs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "pos.md",
		"---\nname: pos\ndescription: positional args test\n---\nFirst: $1, Second: $2")

	logger := zap.NewNop()
	cmds, err := custom.LoadDir(dir, command.SourceCustomProject, logger)
	require.NoError(t, err)
	require.Len(t, cmds, 1)

	args := command.Args{
		RawArgs:    "foo bar",
		Positional: []string{"foo", "bar"},
	}
	result, execErr := cmds[0].Execute(context.Background(), args)
	require.NoError(t, execErr)
	require.Equal(t, "First: foo, Second: bar", result.Prompt)
}

// TestParseFrontmatter_CRLF verifies that CR+LF line endings are handled correctly.
// This covers the Windows-style line ending branch in ParseFrontmatter. Defect 5 coverage.
func TestParseFrontmatter_CRLF(t *testing.T) {
	t.Parallel()

	// Build a frontmatter block with \r\n line endings.
	data := []byte("---\r\nname: crlf\r\ndescription: crlf test\r\n---\r\nBody text")
	spec, body, err := custom.ParseFrontmatter(data)
	require.NoError(t, err)
	require.Equal(t, "crlf", spec.Name)
	require.Equal(t, "crlf test", spec.Description)
	require.Equal(t, "Body text", strings.TrimSpace(body))
}

// TestCustomLoader_InvalidCommandName verifies that files whose frontmatter name
// fails [a-z0-9_-] validation are skipped. Defect 5 coverage.
func TestCustomLoader_InvalidCommandName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// "BadName!" contains uppercase and special characters — must be rejected.
	writeFile(t, dir, "invalid.md", "---\nname: \"BadName!\"\ndescription: invalid name\n---\nBody")

	logger := zap.NewNop()
	cmds, err := custom.LoadDir(dir, command.SourceCustomProject, logger)
	require.NoError(t, err)
	// The command with an invalid name must not be loaded.
	require.Empty(t, cmds, "command with invalid name must be rejected")
}

// TestParseFrontmatter_WrapsCommandErrFrontmatterInvalid verifies that errors returned
// by ParseFrontmatter wrap command.ErrFrontmatterInvalid so callers can use errors.Is.
// This covers the Task A-1 fix: frontmatter.go now uses command.ErrFrontmatterInvalid
// directly instead of the removed internal sentinel.
func TestParseFrontmatter_WrapsCommandErrFrontmatterInvalid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		data []byte
	}{
		{"missing opening delimiter", []byte("name: foo\ndescription: bar\n")},
		{"missing closing delimiter", []byte("---\nname: foo\ndescription: bar\n")},
		{"malformed yaml", []byte("---\nname: [\ninvalid\n---\nBody")},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := custom.ParseFrontmatter(tc.data)
			require.Error(t, err)
			require.ErrorIs(t, err, command.ErrFrontmatterInvalid,
				"ParseFrontmatter must wrap command.ErrFrontmatterInvalid so callers can use errors.Is")
		})
	}
}
