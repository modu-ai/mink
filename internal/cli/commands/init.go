// Package commands — init.go wires the `mink init` onboarding wizard subcommand.
// The command delegates all TUI logic to internal/cli/install.RunWizard.
// SPEC: SPEC-MINK-ONBOARDING-001 §6 (Phase 2A)
package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/modu-ai/mink/internal/cli/install"
	"github.com/spf13/cobra"
)

// NewInitCommand creates the `mink init` subcommand that launches the 7-step onboarding wizard.
// The command requires a real TTY on stdin; non-TTY environments exit 1 with a clear message.
func NewInitCommand() *cobra.Command {
	var dryRun bool
	var resume bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Run the 7-step onboarding wizard",
		Long: `Launch the interactive MINK setup wizard.

The wizard guides you through 7 steps:
  1. Locale selection
  2. Local AI model (Ollama) setup
  3. CLI delegation tool detection
  4. MINK persona configuration
  5. LLM provider and API key
  6. Messenger channel selection
  7. Privacy and consent settings

Requires a real terminal (TTY). Use --dry-run to validate configuration without writing files.

Use --resume to continue a previously paused onboarding session. MINK saves your progress
automatically; if you close the wizard mid-way, run 'mink init --resume' to pick up where
you left off.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TTY check: mink init requires interactive terminal input.
			if !install.IsTTYFunc(uintptr(os.Stdin.Fd())) {
				fmt.Fprintln(cmd.ErrOrStderr(),
					"mink init requires a TTY. Run from a real terminal or pipe stdin/stdout manually.")
				return errors.New("non-TTY environment")
			}

			err := install.RunWizard(cmd.Context(), install.WizardOptions{
				DryRun: dryRun,
				Resume: resume,
			})
			if err != nil {
				if errors.Is(err, install.ErrWizardCancelled) {
					fmt.Fprintln(cmd.ErrOrStderr(), "Cancelled.")
					// Return special sentinel so the cobra layer can set exit code 130.
					return err
				}
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Onboarding complete. Run mink to start.")
			return nil
		},
	}

	// --dry-run: marshal config but skip file writes (passes DryRun:true to CompletionOptions).
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate configuration without writing files")

	// --resume: load an existing onboarding-draft.yaml and continue from the saved step.
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume a previously paused onboarding wizard")

	return cmd
}
