// Package commands — HandleReAuthRequired helper for Codex re-authentication.
//
// When a credential operation surfaces ErrReAuthRequired for the Codex
// provider, callers should call HandleReAuthRequired to print a user-facing
// hint directing the user to run `mink login codex` again.  The hint is
// printed to stderr so that it does not pollute stdout pipeline output.
//
// Korean output per language.yaml (conversation_language: ko); English
// comments per language.yaml (code_comments: en).
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (ED-5, T-012, AC-CR-017)
package commands

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// HandleReAuthRequired checks whether err wraps ErrReAuthRequired and, if so,
// writes a Korean-language hint to stderr directing the user to re-run
// `mink login codex`.
//
// Returns true when the error was recognised as ErrReAuthRequired (regardless
// of whether the hint was successfully written to stderr).  Returns false when
// the error is unrelated to re-authentication.
//
// Callers in M4 (CLI wiring) invoke this after every Codex Load / Refresh
// call so that the user always receives a clear next-step instruction.
func HandleReAuthRequired(err error) bool {
	return handleReAuthRequiredTo(err, os.Stderr)
}

// handleReAuthRequiredTo is the testable core of HandleReAuthRequired.
// It accepts an io.Writer so tests can capture the output.
func handleReAuthRequiredTo(err error, w io.Writer) bool {
	if !errors.Is(err, credential.ErrReAuthRequired) {
		return false
	}
	fmt.Fprintln(w, "Codex 인증이 만료되었습니다. `mink login codex` 를 다시 실행해 주세요.")
	return true
}
