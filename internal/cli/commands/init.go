// Package commands — init.go wires the `mink init` onboarding wizard subcommand.
// The command delegates TUI logic to internal/cli/install.RunWizard (TTY mode) or
// the embedded HTTP server in internal/server/install (--web mode).
// SPEC: SPEC-MINK-ONBOARDING-001 §6 (Phase 2A + Phase 3A)
package commands

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	cliinstall "github.com/modu-ai/mink/internal/cli/install"
	webinstall "github.com/modu-ai/mink/internal/server/install"
	"github.com/spf13/cobra"
)

// NewInitCommand creates the `mink init` subcommand that launches the 7-step onboarding wizard.
// In TTY mode (default) it delegates to internal/cli/install.RunWizard.
// With --web it starts the embedded HTTP server on a random localhost port.
func NewInitCommand() *cobra.Command {
	var dryRun bool
	var resume bool
	var web bool

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
you left off.

Use --web to launch the browser-based wizard instead of the terminal TUI. The command
starts a local HTTP server and opens the install wizard in your default browser.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if web {
				// Web mode: start embedded HTTP server and open the browser.
				ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
				defer stop()

				listening := func(url string) {
					fmt.Fprintln(cmd.OutOrStdout(),
						"MINK install wizard ready at "+url+"\nPress Ctrl+C to exit.")
				}

				return webinstall.RunServer(ctx, true /* openBrowser */, listening)
			}

			// TTY mode: require an interactive terminal.
			if !cliinstall.IsTTYFunc(uintptr(os.Stdin.Fd())) {
				fmt.Fprintln(cmd.ErrOrStderr(),
					"mink init requires a TTY. Run from a real terminal or use --web for browser mode.")
				return errors.New("non-TTY environment")
			}

			err := cliinstall.RunWizard(cmd.Context(), cliinstall.WizardOptions{
				DryRun: dryRun,
				Resume: resume,
			})
			if err != nil {
				if errors.Is(err, cliinstall.ErrWizardCancelled) {
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

	// --web: start the browser-based install wizard instead of the terminal TUI.
	cmd.Flags().BoolVar(&web, "web", false, "Launch the browser-based install wizard")

	return cmd
}
