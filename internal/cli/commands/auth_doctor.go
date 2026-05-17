// Package commands — auth doctor subcommand.
//
// "mink doctor auth-keyring" displays the active credential backend and the
// presence/masked status of all 8 known providers.  No plaintext is ever
// printed (UN-1).
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

// runAuthKeyring executes the auth-keyring health check and writes a
// human-readable table to w.  It is extracted from the cobra RunE so it can
// be called directly by tests.
func runAuthKeyring(w io.Writer) error {
	b := keyring.NewBackend()

	// Probe detects actual OS keyring availability; on real hardware the
	// result may differ from the mock used in unit tests.
	_, probeReason := keyring.Probe()
	backendName := "keyring"
	if probeReason != "" {
		backendName = "keyring (unavailable: " + probeReason + ")"
	}

	fmt.Fprintf(w, "Backend: %s\n\n", backendName)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PROVIDER\tSTATUS")
	fmt.Fprintln(tw, "--------\t------")

	for _, provider := range knownProviders {
		status, err := b.Health(provider)
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
