// Package commands — auth doctor subcommand.
//
// "mink doctor auth-keyring" displays the active credential backend and the
// presence/masked status of all 8 known providers.  No plaintext is ever
// printed (UN-1).
//
// The backend is accepted as a credential.Service so the doctor works with any
// backend (keyring, file, or Dispatcher) without modification (M2 requirement).
// When no explicit backend is provided via RunAuthKeyringWithBackend, the
// command defaults to a keyring.Backend for backward compatibility.
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (UB-8, UB-9, AC-CR-008, AC-CR-031)
package commands

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/modu-ai/mink/internal/auth/credential"
	"github.com/modu-ai/mink/internal/auth/keyring"
	"github.com/spf13/cobra"
)

// knownProviders is the canonical list of 8 credential provider IDs defined
// in research.md §4.2 and tasks.md T-004.
var knownProviders = []string{
	"anthropic",
	"deepseek",
	"openai_gpt",
	"codex",
	"zai_glm",
	"telegram_bot",
	"slack",
	"discord",
}

// NewDoctorCommand returns the "doctor" parent cobra.Command.
// Sub-commands are added by RegisterDoctorSubcommands.
func NewDoctorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose MINK subsystem health",
	}
	cmd.AddCommand(newAuthKeyringCommand())
	return cmd
}

// newAuthKeyringCommand returns the "doctor auth-keyring" subcommand.
func newAuthKeyringCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "auth-keyring",
		Short: "Show active auth backend and credential status for all providers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAuthKeyring(cmd.OutOrStdout())
		},
	}
}

// runAuthKeyring executes the auth-keyring health check using the default
// keyring backend and writes a human-readable table to w.
//
// Callers that want to use a different backend (e.g. the Dispatcher in
// integration tests) should call RunAuthKeyringWithBackend instead.
func runAuthKeyring(w io.Writer) error {
	b := keyring.NewBackend()

	// Probe detects actual OS keyring availability; on real hardware the
	// result may differ from the mock used in unit tests.
	_, probeReason := keyring.Probe()
	backendLabel := "keyring"
	if probeReason != "" {
		backendLabel = "keyring (unavailable: " + probeReason + ")"
	}

	return runAuthKeyringWithBackend(w, b, backendLabel)
}

// RunAuthKeyringWithBackend executes the auth-keyring health check against
// the supplied credential.Service and writes a human-readable table to w.
//
// This function accepts any credential.Service (keyring.Backend, file.Backend,
// or Dispatcher) so that the doctor command remains backend-agnostic (M2
// requirement).
func RunAuthKeyringWithBackend(w io.Writer, svc credential.Service, backendLabel string) error {
	return runAuthKeyringWithBackend(w, svc, backendLabel)
}

// runAuthKeyringWithBackend is the internal implementation shared by
// runAuthKeyring and RunAuthKeyringWithBackend.
func runAuthKeyringWithBackend(w io.Writer, svc credential.Service, backendLabel string) error {
	fmt.Fprintf(w, "Backend: %s\n\n", backendLabel)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PROVIDER\tSTATUS")
	fmt.Fprintln(tw, "--------\t------")

	for _, provider := range knownProviders {
		status, err := svc.Health(provider)
		row := formatHealthRow(status, err)
		fmt.Fprintf(tw, "%s\t%s\n", provider, row)
	}
	return tw.Flush()
}

// formatHealthRow builds the STATUS column text for a single provider.
// It never includes plaintext values (UN-1).
func formatHealthRow(status credential.HealthStatus, err error) string {
	if err != nil {
		if credential.IsKeyringUnavailable(err) {
			return "error: keyring unavailable"
		}
		return fmt.Sprintf("error: %s", err.Error())
	}
	if !status.Present {
		return "missing"
	}
	return fmt.Sprintf("present (%s)", status.MaskedLast4)
}

// RunAuthKeyringToStdout is a convenience wrapper used by integration tests
// that want to capture CLI output without constructing a cobra.Command.
func RunAuthKeyringToStdout() error {
	return runAuthKeyring(os.Stdout)
}
