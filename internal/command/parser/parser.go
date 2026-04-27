// Package parser implements the slash command line parser.
// SPEC: SPEC-GOOSE-COMMAND-001
package parser

import (
	"strings"
	"unicode"
)

// ParsedArgs holds the split arguments returned by SplitArgs.
// The caller (Dispatcher) converts this to command.Args.
type ParsedArgs struct {
	// RawArgs is the untouched argument string passed to SplitArgs.
	RawArgs string
	// Positional contains whitespace-separated tokens (respecting quotes).
	Positional []string
	// Flags holds key-value pairs from --key=value or --key value tokens.
	Flags map[string]string
}

// Parse examines a single input line and determines whether it is a slash command.
// If the first non-whitespace character is '/' and the immediately following character
// is an ASCII letter, the function returns the lowercase command name, the trimmed
// argument string, and ok=true.
// In all other cases ok=false is returned and the caller should treat the line as a
// plain LLM prompt.
//
// Parse performs no IO. REQ-CMD-001, REQ-CMD-015.
//
// @MX:ANCHOR: [AUTO] Entry point for all slash command detection; called by Dispatcher on every user input.
// @MX:REASON: Fan-in >= 3: Dispatcher.ProcessUserInput, integration tests, builtin tests.
// @MX:NOTE: [AUTO] '/' 직후 ASCII 문자(a-zA-Z)가 없으면 slash command로 인식하지 않음.
// 이 규칙은 경로(//), 숫자(/1), 공백(/ foo)으로 시작하는 입력이 LLM으로 전달되도록 보장한다. REQ-CMD-001.
// @MX:SPEC: SPEC-GOOSE-COMMAND-001 REQ-CMD-001
func Parse(line string) (name, rawArgs string, ok bool) {
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	if len(trimmed) == 0 || trimmed[0] != '/' {
		return "", "", false
	}
	rest := trimmed[1:]
	if len(rest) == 0 || !isASCIILetter(rest[0]) {
		return "", "", false
	}

	// Split on first whitespace to extract name and rawArgs.
	idx := strings.IndexFunc(rest, unicode.IsSpace)
	var cmdName, args string
	if idx == -1 {
		cmdName = rest
		args = ""
	} else {
		cmdName = rest[:idx]
		args = strings.TrimSpace(rest[idx+1:])
	}

	return strings.ToLower(cmdName), args, true
}

// SplitArgs parses rawArgs into positional tokens and --flag key-value pairs.
// It supports shell-like quoting (double-quote, single-quote) and backslash escaping.
//
// REQ-CMD-015: no IO is performed.
func SplitArgs(rawArgs string) (ParsedArgs, error) {
	tokens, err := tokenize(rawArgs)
	if err != nil {
		return ParsedArgs{}, err
	}

	result := ParsedArgs{
		RawArgs: rawArgs,
		Flags:   make(map[string]string),
	}

	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if strings.HasPrefix(tok, "--") {
			key := tok[2:]
			if eqIdx := strings.IndexByte(key, '='); eqIdx != -1 {
				// --key=value form
				result.Flags[key[:eqIdx]] = key[eqIdx+1:]
			} else if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "--") {
				// --key value form
				result.Flags[key] = tokens[i+1]
				i++
			} else {
				// --flag without value — store empty string
				result.Flags[key] = ""
			}
		} else {
			result.Positional = append(result.Positional, tok)
		}
		i++
	}

	return result, nil
}

// tokenize splits rawArgs into tokens with shell-like quoting and backslash escape.
func tokenize(s string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inDouble := false
	inSingle := false
	i := 0
	runes := []rune(s)

	for i < len(runes) {
		r := runes[i]
		switch {
		case r == '\\' && !inSingle:
			// Backslash escape: consume next character literally.
			if i+1 < len(runes) {
				i++
				current.WriteRune(runes[i])
			}
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case unicode.IsSpace(r) && !inDouble && !inSingle:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
		i++
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens, nil
}

// isASCIILetter returns true for [a-zA-Z].
func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
