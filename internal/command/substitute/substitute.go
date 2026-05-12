// Package substitute implements the template substitution engine for custom commands.
// SPEC: SPEC-GOOSE-COMMAND-001
package substitute

import (
	"strings"
	"unicode"

	"github.com/modu-ai/mink/internal/command"
)

// Context carries the values used during template expansion.
type Context struct {
	// Args holds the parsed command arguments.
	Args command.Args
	// Env maps environment variable names (e.g. "CWD", "GOOSE_HOME") to their values.
	Env map[string]string
}

// Expand performs a single-pass substitution over template using the provided context.
//
// Substitution rules:
//   - "$$"              → literal "$"
//   - "$ARGUMENTS"      → ctx.Args.RawArgs (no further expansion of the result)
//   - "$1".."$9"        → ctx.Args.Positional[N-1], or "" if out of range
//   - "$CWD", "$GOOSE_HOME", etc. → value from ctx.Env; unknown keys remain literal
//   - Unknown "$NAME" where NAME is uppercase letters → literal (no error)
//
// The substitution is strictly single-pass: the result of any replacement is
// never scanned again. REQ-CMD-013.
//
// @MX:ANCHOR: [AUTO] Security boundary for template expansion; 100% branch coverage required.
// @MX:REASON: Processes untrusted user-supplied command bodies; single-pass is a security invariant.
// @MX:NOTE: [AUTO] 단일 패스(single-pass) 보장: 치환된 값은 다시 스캔되지 않음. REQ-CMD-013.
// $ARGUMENTS가 "$ARGUMENTS"를 반환해도 두 번째 치환 없음. 보안 침해(injection) 방지를 위한 핵심 불변식.
// @MX:SPEC: SPEC-GOOSE-COMMAND-001 REQ-CMD-006, REQ-CMD-013
func Expand(template string, ctx Context) (string, error) {
	if len(template) == 0 {
		return "", nil
	}

	runes := []rune(template)
	var out strings.Builder
	out.Grow(len(template))

	i := 0
	for i < len(runes) {
		r := runes[i]
		if r != '$' {
			out.WriteRune(r)
			i++
			continue
		}

		// Peek at next character.
		if i+1 >= len(runes) {
			// Trailing lone '$' — emit literally.
			out.WriteRune(r)
			i++
			continue
		}

		next := runes[i+1]

		// $$ → literal $
		if next == '$' {
			out.WriteRune('$')
			i += 2
			continue
		}

		// $1..$9 positional
		if next >= '1' && next <= '9' {
			idx := int(next - '1')
			if idx < len(ctx.Args.Positional) {
				out.WriteString(ctx.Args.Positional[idx])
			}
			// Missing positional → empty string (no panic, no error).
			i += 2
			continue
		}

		// Named variable: collect uppercase letters (and underscores for GOOSE_HOME).
		if isUpperOrUnderscore(next) {
			j := i + 1
			for j < len(runes) && isUpperOrUnderscore(runes[j]) {
				j++
			}
			varName := string(runes[i+1 : j])

			switch varName {
			case "ARGUMENTS":
				// Write the raw args string — do NOT scan the result again.
				out.WriteString(ctx.Args.RawArgs)
			default:
				if val, ok := ctx.Env[varName]; ok {
					out.WriteString(val)
				} else {
					// Unknown variable — emit the literal token unchanged.
					out.WriteRune('$')
					out.WriteString(varName)
				}
			}
			i = j
			continue
		}

		// '$' followed by something that is not a recognised pattern — emit literally.
		out.WriteRune('$')
		i++
	}

	return out.String(), nil
}

// isUpperOrUnderscore returns true for uppercase ASCII letters and underscore.
// This is used to collect variable names like ARGUMENTS, CWD, GOOSE_HOME.
func isUpperOrUnderscore(r rune) bool {
	return (r >= 'A' && r <= 'Z') || r == '_' || unicode.IsUpper(r)
}
