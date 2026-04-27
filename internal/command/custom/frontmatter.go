// Package custom loads custom slash commands from Markdown files.
// SPEC: SPEC-GOOSE-COMMAND-001
package custom

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/modu-ai/goose/internal/command"
	"gopkg.in/yaml.v3"
)

// FrontmatterSpec holds the parsed YAML frontmatter fields from a command .md file.
// Required fields: Name, Description. All others are optional.
type FrontmatterSpec struct {
	// Name is the command name (required, must pass [a-z0-9_-] validation).
	Name string `yaml:"name"`
	// Description is a one-line summary shown in /help (required).
	Description string `yaml:"description"`
	// ArgumentHint is shown alongside the description in /help output.
	ArgumentHint string `yaml:"argument-hint"`
	// AllowedTools is reserved for SKILLS-001 permission enforcement.
	AllowedTools []string `yaml:"allowed-tools"`
	// Mutates, when true, causes the command to be blocked in plan mode.
	Mutates bool `yaml:"mutates"`
}

const frontmatterDelimiter = "---"

// ParseFrontmatter extracts and parses the YAML frontmatter from a Markdown file.
// The frontmatter must be enclosed between the first and second "---\n" lines.
// Returns ErrFrontmatterInvalid if required fields are absent or YAML is malformed.
func ParseFrontmatter(data []byte) (spec FrontmatterSpec, body string, err error) {
	text := string(data)

	// The file must start with "---" (optionally with a leading newline).
	trimmed := strings.TrimLeft(text, "\r\n")
	if !strings.HasPrefix(trimmed, frontmatterDelimiter) {
		return FrontmatterSpec{}, "", fmt.Errorf("%w: missing opening ---", command.ErrFrontmatterInvalid)
	}

	// Find the closing "---".
	rest := trimmed[len(frontmatterDelimiter):]
	// Skip the newline immediately after the opening delimiter.
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 0 && rest[0] == '\r' && len(rest) > 1 && rest[1] == '\n' {
		rest = rest[2:]
	}

	closingIdx := strings.Index(rest, "\n"+frontmatterDelimiter)
	if closingIdx == -1 {
		return FrontmatterSpec{}, "", fmt.Errorf("%w: missing closing ---", command.ErrFrontmatterInvalid)
	}

	yamlContent := rest[:closingIdx]
	bodyStart := rest[closingIdx+1+len(frontmatterDelimiter):]
	// Trim the newline after the closing delimiter.
	bodyStart = strings.TrimLeft(bodyStart, "\r\n")

	if err := yaml.NewDecoder(bytes.NewReader([]byte(yamlContent))).Decode(&spec); err != nil {
		return FrontmatterSpec{}, "", fmt.Errorf("%w: yaml: %w", command.ErrFrontmatterInvalid, err)
	}

	return spec, bodyStart, nil
}
