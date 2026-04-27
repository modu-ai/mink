// Package custom loads custom slash commands from Markdown files.
// SPEC: SPEC-GOOSE-COMMAND-001
package custom

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/modu-ai/goose/internal/command"
	"github.com/modu-ai/goose/internal/command/substitute"
	"go.uber.org/zap"
)

// validNameRe mirrors the registry's validation — command names must be [a-z0-9_-].
var validNameRe = regexp.MustCompile(`^[a-z0-9_-]+$`)

func init() {
	// Wire LoadDir into the command package's customLoader function variable.
	// This breaks the import cycle: command → custom would be circular, but
	// custom → command is fine, and command.SetCustomLoader accepts the function.
	command.SetCustomLoader(LoadDir)
}

// LoadDir walks root, loads all *.md files, parses their YAML frontmatter, and
// returns a slice of Commands. Files with malformed frontmatter are skipped with an
// ERROR log entry; the overall error return is nil unless a filesystem error occurs.
//
// Symlinks that resolve to paths outside root are silently skipped. REQ-CMD-016.
//
// @MX:ANCHOR: [AUTO] Custom command ingestion boundary; called by Registry.Reload and application startup.
// @MX:REASON: Fan-in >= 3: Registry.Reload, integration tests, builtin test setup.
// @MX:SPEC: SPEC-GOOSE-COMMAND-001 REQ-CMD-009, REQ-CMD-016
func LoadDir(root string, src command.Source, logger *zap.Logger) ([]command.Command, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	// Resolve symlinks in the root itself so the prefix comparison works correctly
	// on macOS (where /var/folders is a symlink to /private/var/folders).
	cleanRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		// Root does not exist yet — return empty without error.
		return nil, nil
	}
	// Ensure cleanRoot ends with separator for prefix comparison.
	rootPrefix := cleanRoot + string(os.PathSeparator)

	var cmds []command.Command

	walkErr := filepath.WalkDir(cleanRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip unreadable entries.
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}

		// Symlink escape check. REQ-CMD-016.
		// @MX:NOTE: [AUTO] 심링크가 root 디렉토리 외부를 가리키는 경우를 차단하는 보안 경계.
		// macOS의 /var/folders → /private/var/folders 심링크를 고려하여 cleanRoot를 EvalSymlinks로 정규화한 후 비교.
		// rootPrefix 비교는 정확한 prefix 매칭을 보장한다(하위 디렉토리에도 안전).
		// @MX:SPEC: SPEC-GOOSE-COMMAND-001 REQ-CMD-016
		resolved, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr != nil {
			logger.Error("failed to resolve symlink", zap.String("path", path), zap.Error(resolveErr))
			return nil
		}
		absResolved, absErr := filepath.Abs(resolved)
		if absErr != nil {
			return nil
		}
		// Allow the path itself (exact match with cleanRoot is a file inside — fine)
		// but reject any path that does not share the rootPrefix.
		if absResolved != cleanRoot && !strings.HasPrefix(absResolved, rootPrefix) {
			logger.Warn("symlink escape rejected", zap.String("path", path), zap.String("resolved", absResolved))
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			logger.Error("failed to read command file", zap.String("path", path), zap.Error(readErr))
			return nil
		}

		spec, body, parseErr := ParseFrontmatter(data)
		if parseErr != nil {
			logger.Error("malformed frontmatter", zap.String("path", path), zap.Error(parseErr))
			return nil
		}

		if spec.Name == "" || spec.Description == "" {
			logger.Error("missing required frontmatter fields",
				zap.String("path", path),
				zap.Bool("has_name", spec.Name != ""),
				zap.Bool("has_description", spec.Description != ""),
			)
			return nil
		}

		// Validate the command name against [a-z0-9_-]; reject names with uppercase or
		// special characters early so Registry.Register does not receive an invalid name.
		lowerName := strings.ToLower(spec.Name)
		if !validNameRe.MatchString(lowerName) {
			logger.Error("invalid command name",
				zap.String("path", path),
				zap.String("name", spec.Name),
				zap.Error(command.ErrInvalidCommandName),
			)
			return nil
		}

		cmd := &customCommand{
			name: spec.Name,
			meta: command.Metadata{
				Description:  spec.Description,
				ArgumentHint: spec.ArgumentHint,
				AllowedTools: spec.AllowedTools,
				Mutates:      spec.Mutates,
				Source:       src,
				FilePath:     path,
			},
			body: body,
		}
		cmds = append(cmds, cmd)
		return nil
	})

	if walkErr != nil && !errors.Is(walkErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("walk %s: %w", root, walkErr)
	}

	return cmds, nil
}

// customCommand is an unexported Command implementation for Markdown-backed commands.
type customCommand struct {
	name string
	meta command.Metadata
	body string
}

func (c *customCommand) Name() string               { return c.name }
func (c *customCommand) Metadata() command.Metadata { return c.meta }

// Execute expands the body template using the provided args and returns a ResultPromptExpansion.
func (c *customCommand) Execute(_ context.Context, args command.Args) (command.Result, error) {
	ctx := substitute.Context{
		Args: args,
		Env:  map[string]string{},
	}
	expanded, err := substitute.Expand(c.body, ctx)
	if err != nil {
		return command.Result{}, fmt.Errorf("expand template: %w", err)
	}
	return command.Result{
		Kind:   command.ResultPromptExpansion,
		Prompt: expanded,
	}, nil
}
