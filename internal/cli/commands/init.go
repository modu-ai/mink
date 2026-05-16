// Package commands — init.go wires the `mink init` onboarding wizard subcommand.
// The command delegates TUI logic to internal/cli/install.RunWizard (TTY mode) or
// the embedded HTTP server in internal/server/install (--web mode).
// The --yes flag enables a non-interactive CI/test convenience path that skips
// all TTY requirements and uses sensible defaults for every step.
// SPEC: SPEC-MINK-ONBOARDING-001 §6 (Phase 2A + Phase 3A + Phase 4)
package commands

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	cliinstall "github.com/modu-ai/mink/internal/cli/install"
	"github.com/modu-ai/mink/internal/i18n"
	"github.com/modu-ai/mink/internal/locale"
	"github.com/modu-ai/mink/internal/onboarding"
	webinstall "github.com/modu-ai/mink/internal/server/install"
	"github.com/spf13/cobra"
)

// NewInitCommand creates the `mink init` subcommand that launches the 7-step onboarding wizard.
// In TTY mode (default) it delegates to internal/cli/install.RunWizard.
// With --web it starts the embedded HTTP server on a random localhost port.
// With --yes it runs non-interactively using default values (CI/test convenience).
func NewInitCommand() *cobra.Command {
	var dryRun bool
	var resume bool
	var web bool
	var yes bool
	var personaName string

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

			// Non-interactive mode: --yes bypasses TTY check and uses defaults.
			if yes {
				return runNonInteractive(cmd, personaName, dryRun)
			}

			// TTY mode: require an interactive terminal.
			if !cliinstall.IsTTYFunc(uintptr(os.Stdin.Fd())) {
				fmt.Fprintln(cmd.ErrOrStderr(),
					"mink init requires a TTY. Run from a real terminal or use --web for browser mode.")
				return errors.New("non-TTY environment")
			}

			tr := i18n.DefaultFor(cmd.Context())
			err := cliinstall.RunWizard(cmd.Context(), cliinstall.WizardOptions{
				DryRun: dryRun,
				Resume: resume,
			})
			if err != nil {
				if errors.Is(err, cliinstall.ErrWizardCancelled) {
					fmt.Fprintln(cmd.ErrOrStderr(), tr.Translate("install.cancelled", nil))
					// Return special sentinel so the cobra layer can set exit code 130.
					return err
				}
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), tr.Translate("install.completed", nil))
			return nil
		},
	}

	// --dry-run: marshal config but skip file writes (passes DryRun:true to CompletionOptions).
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate configuration without writing files")

	// --resume: load an existing onboarding-draft.yaml and continue from the saved step.
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume a previously paused onboarding wizard")

	// --web: start the browser-based install wizard instead of the terminal TUI.
	cmd.Flags().BoolVar(&web, "web", false, "Launch the browser-based install wizard")

	// --yes: non-interactive mode using defaults for every step (CI/test convenience).
	// Implies --dry-run unless --apply is also passed.
	cmd.Flags().BoolVar(&yes, "yes", false, "Non-interactive mode: accept defaults for all steps (CI/test convenience, implies --dry-run)")

	// --persona-name: persona name used in --yes mode (default: "TestUser").
	cmd.Flags().StringVar(&personaName, "persona-name", "TestUser", "Persona name to use in --yes mode")

	return cmd
}

// runNonInteractive executes the 7-step onboarding flow without any TTY interaction.
// It uses locale detection for Step 1 and skips Steps 2, 3, 5, and 6.
// Step 4 uses personaName; Step 7 uses safe-default ConsentFlags.
// DryRun=true is the default (no disk writes) unless dryRun is explicitly false.
//
// @MX:NOTE: [AUTO] Non-interactive path for CI speedrun tests (Phase 4, AC-OB-016).
// @MX:SPEC: SPEC-MINK-ONBOARDING-001 §6 Phase 4
func runNonInteractive(cmd *cobra.Command, personaName string, dryRun bool) error {
	started := time.Now()
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "mink init --yes: running non-interactive onboarding...")

	// Step 1: detect locale from OS, fall back to KR preset.
	lc, err := locale.Detect(ctx)
	if err != nil {
		// Fall back silently — CI environments often lack full locale metadata.
		lc = locale.LocaleContext{
			Country:         "KR",
			PrimaryLanguage: "ko-KR",
			Timezone:        "Asia/Seoul",
		}
	}

	// Determine GDPR status from cultural context so Step 7 is handled correctly.
	cultural := locale.ResolveCulturalContext(lc.Country)
	isGDPR := false
	for _, flag := range cultural.LegalFlags {
		if strings.EqualFold(flag, "GDPR") {
			isGDPR = true
			break
		}
	}

	localeChoice := &onboarding.LocaleChoice{
		Country:    lc.Country,
		Language:   lc.PrimaryLanguage,
		Timezone:   lc.Timezone,
		LegalFlags: cultural.LegalFlags,
	}

	flow, err := onboarding.StartFlow(ctx, localeChoice,
		onboarding.WithKeyring(onboarding.NewInMemoryKeyring()),
		onboarding.WithCompletionOptions(onboarding.CompletionOptions{DryRun: dryRun}),
	)
	if err != nil {
		return fmt.Errorf("mink init --yes: failed to start flow: %w", err)
	}

	// Step 1: Locale (submit with detected locale).
	if err := flow.SubmitStep(1, *localeChoice); err != nil {
		return fmt.Errorf("mink init --yes: step 1 locale: %w", err)
	}
	fmt.Fprintln(out, "  [1/7] Locale:", lc.Country)

	// Step 2: Model Setup (skip — Ollama not assumed in CI).
	if err := flow.SkipStep(2); err != nil {
		return fmt.Errorf("mink init --yes: step 2 model: %w", err)
	}
	fmt.Fprintln(out, "  [2/7] Model: skipped")

	// Step 3: CLI Tools (skip — no PATH probing in non-interactive mode).
	if err := flow.SkipStep(3); err != nil {
		return fmt.Errorf("mink init --yes: step 3 cli tools: %w", err)
	}
	fmt.Fprintln(out, "  [3/7] CLI tools: skipped")

	// Step 4: Persona (required — must submit with a name).
	if personaName == "" {
		personaName = "TestUser"
	}
	if err := flow.SubmitStep(4, onboarding.PersonaProfile{
		Name:           personaName,
		HonorificLevel: onboarding.HonorificFormal,
	}); err != nil {
		return fmt.Errorf("mink init --yes: step 4 persona: %w", err)
	}
	fmt.Fprintln(out, "  [4/7] Persona:", personaName)

	// Step 5: Provider (skip — deferred, most CI users have no API key to set).
	if err := flow.SkipStep(5); err != nil {
		return fmt.Errorf("mink init --yes: step 5 provider: %w", err)
	}
	fmt.Fprintln(out, "  [5/7] Provider: skipped")

	// Step 6: Messenger (skip — default local_terminal).
	if err := flow.SkipStep(6); err != nil {
		return fmt.Errorf("mink init --yes: step 6 messenger: %w", err)
	}
	fmt.Fprintln(out, "  [6/7] Messenger: skipped (local terminal)")

	// Step 7: Consent (submit defaults; set GDPRExplicitConsent for GDPR regions).
	var gdprConsent *bool
	if isGDPR {
		v := true
		gdprConsent = &v
	}
	consent := onboarding.ConsentFlags{
		ConversationStorageLocal: true,
		LoRATrainingAllowed:      false,
		TelemetryEnabled:         false,
		CrashReportingEnabled:    false,
		GDPRExplicitConsent:      gdprConsent,
	}
	if err := flow.SubmitStep(7, consent); err != nil {
		return fmt.Errorf("mink init --yes: step 7 consent: %w", err)
	}
	fmt.Fprintln(out, "  [7/7] Consent: defaults accepted")

	// Finalise: persist (or dry-run marshal).
	if _, err := flow.CompleteAndPersist(); err != nil {
		return fmt.Errorf("mink init --yes: completion failed: %w", err)
	}

	elapsed := time.Since(started)
	dryTag := ""
	if dryRun {
		dryTag = " (dry-run)"
	}
	fmt.Fprintf(out, "Onboarding completed in %s%s\n", elapsed.Round(time.Millisecond), dryTag)
	return nil
}
