// Package install implements the 7-step onboarding TUI built on charmbracelet/huh.
// SPEC: SPEC-MINK-ONBOARDING-001 §6 (Phase 2A — happy path; Phase 2B — Skip/Back/Resume/LOCALE)
//
// The package exposes RunWizard as its primary entry point. All TUI interaction
// uses huh forms; each step runs a separate huh.Form so that SubmitStep can be
// called incrementally after each step completes.
package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/modu-ai/mink/internal/onboarding"
	"golang.org/x/term"
)

// ErrWizardCancelled is returned by RunWizard when the user presses Ctrl+C.
// The cobra layer translates this to exit code 130.
var ErrWizardCancelled = errors.New("wizard cancelled by user")

// IsTTYFunc is the TTY detection predicate, indirected for test injection.
// By default it calls term.IsTerminal.
//
// @MX:NOTE: [AUTO] Package-level var enables unit-test substitution without OS-level TTY.
var IsTTYFunc = func(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

// WizardOptions configures RunWizard behaviour.
type WizardOptions struct {
	// DryRun passes DryRun:true to onboarding.CompletionOptions, suppressing file writes.
	DryRun bool

	// Resume loads an existing onboarding-draft.yaml and resumes from Draft.CurrentStep.
	// When no draft is found, RunWizard returns a clear error (no panic, no fresh start).
	Resume bool
}

// localeEntry is a predefined locale option shown in Step 1's Select form.
// Country/Language/Timezone/LegalFlags are used to construct onboarding.LocaleChoice;
// Display is the human-readable label shown in the TUI.
type localeEntry struct {
	Country    string   // ISO 3166-1 alpha-2, e.g., "KR"
	Language   string   // BCP 47 primary tag, e.g., "ko"
	Timezone   string   // IANA timezone ID, e.g., "Asia/Seoul"
	LegalFlags []string // active legal-regime flags, e.g., ["GDPR"]
	Display    string   // label shown in the huh Select widget
}

// localePresets is the ordered list of locale options offered in Step 1.
// Index 0 is the default (KR). Additional entries cover the most common
// GDPR jurisdictions (FR, DE) and the US as a no-flags baseline.
var localePresets = []localeEntry{
	{
		Country:    "KR",
		Language:   "ko",
		Timezone:   "Asia/Seoul",
		LegalFlags: []string{"PIPA"},
		Display:    "Korea (한국어, Asia/Seoul)",
	},
	{
		Country:    "US",
		Language:   "en",
		Timezone:   "America/New_York",
		LegalFlags: nil,
		Display:    "United States (English, America/New_York)",
	},
	{
		Country:    "FR",
		Language:   "fr",
		Timezone:   "Europe/Paris",
		LegalFlags: []string{"GDPR"},
		Display:    "France (français, Europe/Paris)",
	},
	{
		Country:    "DE",
		Language:   "de",
		Timezone:   "Europe/Berlin",
		LegalFlags: []string{"GDPR"},
		Display:    "Germany (Deutsch, Europe/Berlin)",
	},
}

// RunWizard executes the 7-step onboarding TUI.
//
// Returns nil on successful completion.
// Returns ErrWizardCancelled when the user presses Ctrl+C during any huh form.
// Returns a wrapped backend error on validation or persistence failure.
//
// @MX:ANCHOR: [AUTO] Primary public entry point for Phase 2A/2B TUI — called by init.go cobra command.
// @MX:REASON: Signature change breaks the cobra command layer and any future test harnesses.
func RunWizard(ctx context.Context, opts WizardOptions) error {
	// -----------------------------------------------------------------------
	// Pre-flight detection (best-effort; errors are non-fatal for happy path)
	// -----------------------------------------------------------------------
	fmt.Println("Detecting system configuration...")

	ollamaStatus, _ := onboarding.DetectOllama(ctx)
	ramBytes, _ := onboarding.DetectRAM(ctx)

	var detectedModel onboarding.DetectedModel
	if ollamaStatus.Installed && ollamaStatus.DaemonAlive {
		detectedModel, _ = onboarding.DetectMINKModel(ctx, ollamaStatus)
	}

	cliTools, _ := onboarding.DetectCLITools(ctx)

	fmt.Println(summarizeDetection(ollamaStatus, ramBytes, detectedModel, cliTools))

	// -----------------------------------------------------------------------
	// Start or resume onboarding flow
	// -----------------------------------------------------------------------
	var flow *onboarding.OnboardingFlow

	if opts.Resume {
		draft, err := onboarding.LoadDraft()
		if err != nil {
			if errors.Is(err, onboarding.ErrDraftNotFound) {
				return fmt.Errorf("no paused onboarding draft found — run `mink init` without --resume")
			}
			return fmt.Errorf("load draft: %w", err)
		}

		// Display a resume banner so the user knows they are continuing.
		fmt.Printf("Resuming onboarding from step %d/7 (started %s)\n",
			draft.CurrentStep, draft.StartedAt.UTC().Format("2006-01-02 15:04 UTC"))

		flow, err = onboarding.StartFlowFromDraft(ctx, draft,
			onboarding.WithKeyring(onboarding.SystemKeyring{}),
			onboarding.WithCompletionOptions(onboarding.CompletionOptions{DryRun: opts.DryRun}),
		)
		if err != nil {
			return fmt.Errorf("resume draft: %w", err)
		}
	} else {
		var err error
		flow, err = onboarding.StartFlow(ctx, nil,
			onboarding.WithKeyring(onboarding.SystemKeyring{}),
			onboarding.WithCompletionOptions(onboarding.CompletionOptions{DryRun: opts.DryRun}),
		)
		if err != nil {
			return fmt.Errorf("failed to start onboarding flow: %w", err)
		}
	}

	// -----------------------------------------------------------------------
	// Step dispatch loop — starts at flow.CurrentStep (supports resume)
	// -----------------------------------------------------------------------
	//
	// Each step runner returns one of three sentinels:
	//   nil            — step completed; advance normally
	//   errStepBack    — user chose Back; decrement and re-run previous step
	//   errStepSkipped — user chose Skip; SkipStep already called
	//   any other err  — fatal (bubble up)
	//
	// After every mutation (submit / skip / back) the draft is auto-saved,
	// unless DryRun is true.
	//
	// @MX:WARN: [AUTO] Loop modifies flow.CurrentStep via Back(), SkipStep(), SubmitStep().
	// @MX:REASON: Loop re-entrancy relies on flow state machine invariants; any
	// additional mutation inside a step runner can corrupt the step sequence.
	for flow.CurrentStep <= onboarding.TotalSteps() {
		step := flow.CurrentStep

		var stepErr error
		switch step {
		case 1:
			stepErr = runStep1Locale(ctx, flow)
		case 2:
			stepErr = runStep2Model(ctx, flow, ollamaStatus, detectedModel, ramBytes)
		case 3:
			stepErr = runStep3CLITools(ctx, flow, cliTools)
		case 4:
			stepErr = runStep4Persona(ctx, flow)
		case 5:
			stepErr = runStep5Provider(ctx, flow)
		case 6:
			stepErr = runStep6Messenger(ctx, flow)
		case 7:
			stepErr = runStep7Consent(ctx, flow)
		}

		// Handle special navigation sentinels before propagating real errors.
		if stepErr != nil {
			if errors.Is(stepErr, errStepBack) {
				// Back already decremented CurrentStep inside the step runner.
				autoSaveDraft(flow, opts.DryRun)
				continue
			}
			if errors.Is(stepErr, errStepSkipped) {
				// SkipStep already advanced CurrentStep inside the step runner.
				autoSaveDraft(flow, opts.DryRun)
				continue
			}
			return stepErr
		}

		// Successful submit — CurrentStep already advanced by SubmitStep.
		autoSaveDraft(flow, opts.DryRun)
	}

	// -----------------------------------------------------------------------
	// Finalize and persist
	// -----------------------------------------------------------------------
	data, err := flow.CompleteAndPersist()
	if err != nil {
		return fmt.Errorf("failed to persist onboarding config: %w", err)
	}

	// Remove the draft file on successful completion (best-effort).
	if !opts.DryRun {
		if deleteErr := onboarding.DeleteDraft(); deleteErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to delete onboarding draft: %v\n", deleteErr)
		}
	}

	// Display written paths.
	if globalPath, pathErr := onboarding.GlobalConfigPath(); pathErr == nil {
		fmt.Printf("Config written to: %s\n", globalPath)
	}
	if projectPath, pathErr := onboarding.ProjectConfigPath(); pathErr == nil {
		fmt.Printf("Project config:    %s\n", projectPath)
	}

	// Confirm persona name for user reassurance.
	fmt.Printf("\nWelcome, %s!\n", data.Persona.Name)

	return nil
}

// errStepBack is an internal sentinel returned by step runners when the user
// picks the "Back" navigation action. It is NOT surfaced to callers of RunWizard.
var errStepBack = errors.New("step: navigate back")

// errStepSkipped is an internal sentinel returned by step runners when the user
// picks the "Skip" navigation action. It is NOT surfaced to callers of RunWizard.
var errStepSkipped = errors.New("step: skipped")

// autoSaveDraft saves the current flow state as a draft file.
// Best-effort: a save failure logs a warning but does not abort the wizard.
// When DryRun is true, no disk write is performed.
func autoSaveDraft(flow *onboarding.OnboardingFlow, dryRun bool) {
	if dryRun {
		return
	}
	if saveErr := onboarding.SaveDraft(onboarding.DraftFromFlow(flow)); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save onboarding draft: %v\n", saveErr)
	}
}

// summarizeDetection builds the pre-flight detection summary line shown to the user.
// Exported for unit testing in tui_test.go.
func summarizeDetection(
	status onboarding.OllamaStatus,
	ramBytes int64,
	model onboarding.DetectedModel,
	tools []onboarding.CLITool,
) string {
	// Ollama field
	ollamaField := "not installed"
	if status.Installed && status.DaemonAlive {
		ollamaField = "installed+running"
	} else if status.Installed {
		ollamaField = "installed (daemon down)"
	}

	// RAM field
	ramField := "<unknown>"
	if ramBytes > 0 {
		ramGB := ramBytes / (1024 * 1024 * 1024)
		ramField = fmt.Sprintf("%d GB", ramGB)
	}

	// Model field
	modelField := "<none>"
	if model.Name != "" {
		modelField = model.Name
	}

	// CLI tools field
	toolsField := "none"
	if len(tools) > 0 {
		names := make([]string, len(tools))
		for i, t := range tools {
			names[i] = t.Name
		}
		toolsField = strings.Join(names, "+")
	}

	return fmt.Sprintf("Detected: Ollama=%s, RAM=%s, model=%s, CLI tools=%s",
		ollamaField, ramField, modelField, toolsField)
}

// runForm executes a huh.Form with context cancellation support.
// Returns ErrWizardCancelled when the user presses Ctrl+C.
func runForm(ctx context.Context, form *huh.Form) error {
	err := form.RunWithContext(ctx)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return ErrWizardCancelled
		}
		return err
	}
	return nil
}

// -----------------------------------------------------------------------
// Step implementations
// -----------------------------------------------------------------------

// stepAction represents the navigation choice a user makes at each step.
type stepAction int

const (
	stepActionSubmit stepAction = iota // default: complete the step
	stepActionSkip                     // skip this step
	stepActionBack                     // go back to previous step
)

// runStep1Locale presents a locale Select with 4 presets and submits Step 1.
// Step 1 has no Back or Skip options (it is the first step).
func runStep1Locale(ctx context.Context, flow *onboarding.OnboardingFlow) error {
	selectedIndex := 0

	options := make([]huh.Option[int], len(localePresets))
	for i, p := range localePresets {
		options[i] = huh.NewOption(p.Display, i)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Step 1 / 7 — Locale\n\nSelect your region and language:").
				Options(options...).
				Value(&selectedIndex),
		),
	)

	if err := runForm(ctx, form); err != nil {
		return err
	}

	preset := localePresets[selectedIndex]
	locale := onboarding.LocaleChoice{
		Country:    preset.Country,
		Language:   preset.Language,
		Timezone:   preset.Timezone,
		LegalFlags: preset.LegalFlags,
	}
	return flow.SubmitStep(1, locale)
}

// runStep2Model handles Ollama and model setup for Step 2.
// Supports Skip (leaves Data.Model zero-valued) and Back (returns to step 1).
func runStep2Model(
	ctx context.Context,
	flow *onboarding.OnboardingFlow,
	status onboarding.OllamaStatus,
	detected onboarding.DetectedModel,
	ramBytes int64,
) error {
	// Navigation choice before the main form.
	nav, err := runNavChoice(ctx, "Step 2 / 7 — Model Setup", true, true)
	if err != nil {
		return err
	}
	if nav == stepActionBack {
		if backErr := flow.Back(); backErr != nil {
			return backErr
		}
		return errStepBack
	}
	if nav == stepActionSkip {
		if skipErr := flow.SkipStep(2); skipErr != nil {
			return skipErr
		}
		return errStepSkipped
	}

	setup := onboarding.ModelSetup{
		OllamaInstalled: status.Installed,
		RAMBytes:        ramBytes,
	}

	recommendedName := onboarding.RecommendModel(ramBytes)

	switch {
	case status.Installed && status.DaemonAlive && detected.Name != "":
		// Model already installed — offer to use it.
		useDetected := true
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf(
						"Step 2 / 7 — Model Setup\n\nDetected MINK model: %s (%.1f GB)\nUse this model?",
						detected.Name, float64(detected.SizeBytes)/(1024*1024*1024),
					)).
					Affirmative("Yes, use detected model").
					Negative("No, choose different").
					Value(&useDetected),
			),
		)
		if err := runForm(ctx, form); err != nil {
			return err
		}

		if useDetected {
			setup.DetectedModel = detected.Name
			setup.SelectedModel = detected.Name
			setup.ModelSizeBytes = detected.SizeBytes
		} else {
			// Let user pick from recommended or enter custom.
			selected, err := pickModel(ctx, recommendedName)
			if err != nil {
				return err
			}
			setup.DetectedModel = detected.Name
			setup.SelectedModel = selected
		}

	case status.Installed && status.DaemonAlive && detected.Name == "":
		// Ollama running but no MINK model — offer to download recommended.
		download := true
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf(
						"Step 2 / 7 — Model Setup\n\nNo MINK model installed.\nRecommended for your RAM (%d GB): %s\n\nDownload now?",
						ramBytes/(1024*1024*1024), recommendedName,
					)).
					Affirmative("Yes, download").
					Negative("Skip for now").
					Value(&download),
			),
		)
		if err := runForm(ctx, form); err != nil {
			return err
		}

		if download {
			if pullErr := pullModelWithSpinner(ctx, recommendedName); pullErr != nil {
				return pullErr
			}
			setup.SelectedModel = recommendedName
		}

	default:
		// Ollama not installed or daemon not running.
		continueWithout := true
		var instructions string
		switch {
		case !status.Installed:
			instructions = "Ollama is not installed.\n\n" +
				"  macOS:   brew install ollama\n" +
				"  Linux:   curl -fsSL https://ollama.ai/install.sh | sh\n" +
				"  Windows: https://ollama.ai/download/OllamaSetup.exe"
		default:
			instructions = "Ollama is installed but the daemon is not running.\n\n" +
				"Start it with: ollama serve"
		}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf(
						"Step 2 / 7 — Model Setup\n\n%s\n\nContinue without Ollama?",
						instructions,
					)).
					Affirmative("Yes, continue without Ollama").
					Negative("No, I'll install it first").
					Value(&continueWithout),
			),
		)
		if err := runForm(ctx, form); err != nil {
			return err
		}

		if !continueWithout {
			return errors.New("onboarding aborted: Ollama setup required before continuing")
		}
	}

	return flow.SubmitStep(2, setup)
}

// runStep3CLITools displays detected tools as a MultiSelect and submits Step 3.
// Supports Skip and Back.
func runStep3CLITools(ctx context.Context, flow *onboarding.OnboardingFlow, detected []onboarding.CLITool) error {
	nav, err := runNavChoice(ctx, "Step 3 / 7 — CLI Delegation Tools", true, true)
	if err != nil {
		return err
	}
	if nav == stepActionBack {
		if backErr := flow.Back(); backErr != nil {
			return backErr
		}
		return errStepBack
	}
	if nav == stepActionSkip {
		if skipErr := flow.SkipStep(3); skipErr != nil {
			return skipErr
		}
		return errStepSkipped
	}

	if len(detected) == 0 {
		// No tools found — submit empty detection and continue.
		fmt.Println("Step 3 / 7 — CLI Tools\n\nNo CLI delegation tools detected (claude / gemini / codex).")
		return flow.SubmitStep(3, onboarding.CLIToolsDetection{DetectedTools: nil})
	}

	options := make([]huh.Option[string], len(detected))
	for i, t := range detected {
		label := t.Name
		if t.Version != "" {
			label = fmt.Sprintf("%s v%s", t.Name, t.Version)
		}
		options[i] = huh.NewOption(label, t.Name).Selected(true)
	}

	var selectedNames []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Step 3 / 7 — CLI Delegation Tools\n\nSelect which tools MINK may delegate tasks to:").
				Options(options...).
				Value(&selectedNames),
		),
	)
	if err := runForm(ctx, form); err != nil {
		return err
	}

	// Filter detected tools to only the user-selected names.
	selected := make([]onboarding.CLITool, 0, len(selectedNames))
	nameSet := make(map[string]bool, len(selectedNames))
	for _, n := range selectedNames {
		nameSet[n] = true
	}
	for _, t := range detected {
		if nameSet[t.Name] {
			selected = append(selected, t)
		}
	}

	return flow.SubmitStep(3, onboarding.CLIToolsDetection{DetectedTools: selected})
}

// soulMarkdownTemplate is the default persona description template shown to the user.
const soulMarkdownTemplate = `# MINK Persona

## Identity
MINK is my AI daily companion — thoughtful, direct, and genuinely helpful.

## Communication Style
- Concise and clear
- Honest about uncertainty
- Proactively flags potential issues

## Values
- Respect for privacy
- Transparency in reasoning
- Quality over speed
`

// runStep4Persona collects persona settings and submits Step 4.
// Supports Skip and Back. Pre-populates from existing flow.Data.Persona on re-entry.
func runStep4Persona(ctx context.Context, flow *onboarding.OnboardingFlow) error {
	nav, err := runNavChoice(ctx, "Step 4 / 7 — Persona", true, true)
	if err != nil {
		return err
	}
	if nav == stepActionBack {
		if backErr := flow.Back(); backErr != nil {
			return backErr
		}
		return errStepBack
	}
	if nav == stepActionSkip {
		if skipErr := flow.SkipStep(4); skipErr != nil {
			return skipErr
		}
		return errStepSkipped
	}

	// Pre-populate from previously entered data (Back → re-enter scenario).
	name := flow.Data.Persona.Name
	if name == "" {
		name = "MINK"
	}
	honorStr := string(flow.Data.Persona.HonorificLevel)
	if honorStr == "" {
		honorStr = string(onboarding.HonorificFormal)
	}
	pronouns := flow.Data.Persona.Pronouns
	soul := flow.Data.Persona.SoulMarkdown
	if soul == "" {
		soul = soulMarkdownTemplate
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Step 4 / 7 — Persona\n\nPersona name (required):").
				Placeholder("MINK").
				Value(&name).
				Validate(func(s string) error {
					return onboarding.ValidatePersonaName(s)
				}),
			huh.NewSelect[string]().
				Title("Address style:").
				Options(
					huh.NewOption("Formal (존댓말)", string(onboarding.HonorificFormal)),
					huh.NewOption("Casual (반말)", string(onboarding.HonorificCasual)),
					huh.NewOption("Intimate (친근체)", string(onboarding.HonorificIntimate)),
				).
				Value(&honorStr),
			huh.NewInput().
				Title("Pronouns (optional):").
				Placeholder("e.g., they/them, she/her").
				Value(&pronouns),
		),
		huh.NewGroup(
			huh.NewText().
				Title("Soul description (Markdown — who is MINK to you?):").
				Value(&soul).
				Lines(12),
		),
	)

	if err := runForm(ctx, form); err != nil {
		return err
	}

	return flow.SubmitStep(4, onboarding.PersonaProfile{
		Name:           name,
		HonorificLevel: onboarding.HonorificLevel(honorStr),
		Pronouns:       pronouns,
		SoulMarkdown:   soul,
	})
}

// runStep5Provider collects LLM provider settings and submits Step 5.
// Supports Skip and Back.
func runStep5Provider(ctx context.Context, flow *onboarding.OnboardingFlow) error {
	nav, err := runNavChoice(ctx, "Step 5 / 7 — LLM Provider", true, true)
	if err != nil {
		return err
	}
	if nav == stepActionBack {
		if backErr := flow.Back(); backErr != nil {
			return backErr
		}
		return errStepBack
	}
	if nav == stepActionSkip {
		if skipErr := flow.SkipStep(5); skipErr != nil {
			return skipErr
		}
		return errStepSkipped
	}

	var (
		providerStr string
		authStr     string
		apiKey      string
		customEP    string
		prefModel   string
	)

	// Pre-populate from existing data when re-entering after Back.
	if flow.Data.Provider.Provider != "" {
		providerStr = string(flow.Data.Provider.Provider)
		authStr = string(flow.Data.Provider.AuthMethod)
		customEP = flow.Data.Provider.CustomEndpoint
		prefModel = flow.Data.Provider.PreferredModel
	} else {
		providerStr = string(onboarding.ProviderAnthropic)
		authStr = string(onboarding.AuthMethodAPIKey)
	}

	// Provider selection.
	providerForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Step 5 / 7 — LLM Provider\n\nSelect your primary provider:").
				Options(
					huh.NewOption("Anthropic (Claude)", string(onboarding.ProviderAnthropic)),
					huh.NewOption("OpenAI (GPT)", string(onboarding.ProviderOpenAI)),
					huh.NewOption("Google (Gemini)", string(onboarding.ProviderGoogle)),
					huh.NewOption("Ollama (local)", string(onboarding.ProviderOllama)),
					huh.NewOption("DeepSeek", string(onboarding.ProviderDeepSeek)),
					huh.NewOption("Custom endpoint", string(onboarding.ProviderCustom)),
					huh.NewOption("Skip for now", string(onboarding.ProviderUnset)),
				).
				Value(&providerStr),
		),
	)
	if err := runForm(ctx, providerForm); err != nil {
		return err
	}

	provider := onboarding.Provider(providerStr)

	// Auth method and API key — only for non-local providers.
	if provider != onboarding.ProviderOllama && provider != onboarding.ProviderUnset {
		authStr = string(onboarding.AuthMethodAPIKey)

		authForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Authentication method:").
					Options(
						huh.NewOption("API key (stored in OS keyring)", string(onboarding.AuthMethodAPIKey)),
						huh.NewOption("Environment variable (read at runtime)", string(onboarding.AuthMethodEnv)),
					).
					Value(&authStr),
			),
		)
		if err := runForm(ctx, authForm); err != nil {
			return err
		}

		if onboarding.AuthMethod(authStr) == onboarding.AuthMethodAPIKey {
			apiKeyForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title(fmt.Sprintf("API key for %s:", providerStr)).
						Placeholder("paste your key here").
						EchoMode(huh.EchoModePassword).
						Value(&apiKey).
						Validate(func(s string) error {
							return onboarding.ValidateProviderAPIKey(providerStr, s)
						}),
				),
			)
			if err := runForm(ctx, apiKeyForm); err != nil {
				return err
			}
		}
	}

	// Custom endpoint and preferred model (optional).
	if provider == onboarding.ProviderCustom {
		customForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Custom endpoint URL:").
					Placeholder("https://api.example.com/v1").
					Value(&customEP),
				huh.NewInput().
					Title("Preferred model (optional):").
					Placeholder("e.g., gpt-4o").
					Value(&prefModel),
			),
		)
		if err := runForm(ctx, customForm); err != nil {
			return err
		}
	}

	return flow.SubmitStep(5, onboarding.ProviderStepInput{
		Choice: onboarding.ProviderChoice{
			Provider:       provider,
			AuthMethod:     onboarding.AuthMethod(authStr),
			CustomEndpoint: customEP,
			PreferredModel: prefModel,
		},
		APIKey: apiKey,
	})
}

// runStep6Messenger collects the first messenger channel and submits Step 6.
// Supports Skip and Back.
func runStep6Messenger(ctx context.Context, flow *onboarding.OnboardingFlow) error {
	nav, err := runNavChoice(ctx, "Step 6 / 7 — Messenger Channel", true, true)
	if err != nil {
		return err
	}
	if nav == stepActionBack {
		if backErr := flow.Back(); backErr != nil {
			return backErr
		}
		return errStepBack
	}
	if nav == stepActionSkip {
		if skipErr := flow.SkipStep(6); skipErr != nil {
			return skipErr
		}
		return errStepSkipped
	}

	messengerStr := string(onboarding.MessengerLocalTerminal)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Step 6 / 7 — Messenger Channel\n\nHow will you chat with MINK?").
				Options(
					huh.NewOption("Local terminal (default)", string(onboarding.MessengerLocalTerminal)),
					huh.NewOption("Telegram bot", string(onboarding.MessengerTelegram)),
					huh.NewOption("Slack", string(onboarding.MessengerSlack)),
					huh.NewOption("Discord", string(onboarding.MessengerDiscord)),
					huh.NewOption("Custom webhook", string(onboarding.MessengerCustom)),
				).
				Value(&messengerStr),
		),
	)
	if err := runForm(ctx, form); err != nil {
		return err
	}

	return flow.SubmitStep(6, onboarding.MessengerChannel{
		Type:        onboarding.MessengerType(messengerStr),
		BotTokenKey: "", // Phase 2B will collect bot tokens for non-terminal channels.
	})
}

// runStep7Consent collects privacy and consent choices and submits Step 7.
// Supports Back. Skip is blocked when the locale carries a GDPR legal flag —
// in that case an explanatory message is printed and the user is re-prompted.
//
// @MX:WARN: [AUTO] GDPR locale blocks the Skip path; any change to skip-blocking logic
// must account for both SkipStep(7) guard in flow.go and this TUI enforcement layer.
// @MX:REASON: Two-layer enforcement (backend + TUI) ensures that even a buggy TUI cannot
// accidentally bypass GDPR consent for EU/UK users.
func runStep7Consent(ctx context.Context, flow *onboarding.OnboardingFlow) error {
	// Determine whether Skip is allowed for step 7 given the locale.
	skipAllowed := !localeHasGDPR(flow)

	nav, err := runNavChoice(ctx, "Step 7 / 7 — Privacy & Consent", skipAllowed, true)
	if err != nil {
		return err
	}
	if nav == stepActionBack {
		if backErr := flow.Back(); backErr != nil {
			return backErr
		}
		return errStepBack
	}
	if nav == stepActionSkip {
		skipErr := flow.SkipStep(7)
		if skipErr != nil {
			// GDPR enforcement: explain and fall through to the consent form.
			fmt.Println("GDPR jurisdiction requires explicit consent. Skip is not permitted.")
			// Re-run navigation without the skip option.
			nav2, navErr := runNavChoice(ctx, "Step 7 / 7 — Privacy & Consent (GDPR required)", false, true)
			if navErr != nil {
				return navErr
			}
			if nav2 == stepActionBack {
				if backErr := flow.Back(); backErr != nil {
					return backErr
				}
				return errStepBack
			}
			// Fall through to the consent form below.
		} else {
			return errStepSkipped
		}
	}

	var (
		localOnly    bool
		telemetry    bool
		crashReport  bool
		loraTraining bool
	)

	// Secure defaults.
	localOnly = true
	telemetry = false
	crashReport = false
	loraTraining = false

	// Determine whether explicit GDPR consent is required.
	gdprRequired := localeHasGDPR(flow)

	if gdprRequired {
		// GDPR users: explicit consent checkbox must be shown and must be confirmed.
		gdprAccepted := false
		gdprForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Step 7 / 7 — GDPR Consent (required)\n\nI consent to the processing of my personal data as described in the Privacy Policy.").
					Affirmative("I accept").
					Negative("I do not accept").
					Value(&gdprAccepted),
			),
		)
		if err := runForm(ctx, gdprForm); err != nil {
			return err
		}
		if !gdprAccepted {
			return errors.New("onboarding: GDPR consent is required to continue in EU/UK regions")
		}
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Step 7 / 7 — Privacy & Consent\n\nStore conversations locally only?\n(Recommended: Yes — your data stays on your machine)").
				Affirmative("Yes (local only)").
				Negative("No (allow cloud sync)").
				Value(&localOnly),
			huh.NewConfirm().
				Title("Allow anonymous telemetry to improve MINK?").
				Affirmative("Yes").
				Negative("No").
				Value(&telemetry),
			huh.NewConfirm().
				Title("Send crash reports to help fix bugs?").
				Affirmative("Yes").
				Negative("No").
				Value(&crashReport),
			huh.NewConfirm().
				Title("Opt-in to LoRA fine-tuning using your conversations?").
				Affirmative("Yes").
				Negative("No (default)").
				Value(&loraTraining),
		),
	)
	if err := runForm(ctx, form); err != nil {
		return err
	}

	var gdprPtr *bool
	if gdprRequired {
		trueVal := true
		gdprPtr = &trueVal
	}

	return flow.SubmitStep(7, onboarding.ConsentFlags{
		ConversationStorageLocal: localOnly,
		TelemetryEnabled:         telemetry,
		CrashReportingEnabled:    crashReport,
		LoRATrainingAllowed:      loraTraining,
		GDPRExplicitConsent:      gdprPtr,
	})
}

// localeHasGDPR reports whether the flow's current locale carries a "GDPR" legal flag.
func localeHasGDPR(flow *onboarding.OnboardingFlow) bool {
	for _, flag := range flow.Data.Locale.LegalFlags {
		if strings.EqualFold(flag, "GDPR") {
			return true
		}
	}
	return false
}

// runNavChoice presents a navigation selector before the main step form.
// It returns the user's navigation intent without modifying flow state.
//
//   - showSkip: when false, the Skip option is hidden (e.g., step 1, GDPR step 7)
//   - showBack: when false, the Back option is hidden (e.g., step 1)
//
// When neither Skip nor Back is available, runNavChoice returns stepActionSubmit
// immediately without showing any form.
func runNavChoice(ctx context.Context, title string, showSkip, showBack bool) (stepAction, error) {
	if !showSkip && !showBack {
		return stepActionSubmit, nil
	}

	options := []huh.Option[stepAction]{
		huh.NewOption("Continue with this step", stepActionSubmit),
	}
	if showSkip {
		options = append(options, huh.NewOption("Skip this step", stepActionSkip))
	}
	if showBack {
		options = append(options, huh.NewOption("Go back to previous step", stepActionBack))
	}

	var action stepAction
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[stepAction]().
				Title(fmt.Sprintf("%s\n\nWhat would you like to do?", title)).
				Options(options...).
				Value(&action),
		),
	)

	if err := runForm(ctx, form); err != nil {
		return stepActionSubmit, err
	}
	return action, nil
}

// pickModel presents a select for recommended vs custom model name.
func pickModel(ctx context.Context, recommended string) (string, error) {
	const customSentinel = "__custom__"
	selected := recommended

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose a model").
				Options(
					huh.NewOption(fmt.Sprintf("Recommended: %s", recommended), recommended),
					huh.NewOption("Enter custom model name", customSentinel),
				).
				Value(&selected),
		),
	)
	if err := runForm(ctx, form); err != nil {
		return "", err
	}

	if selected == customSentinel {
		var customName string
		customForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Custom model name (e.g., llama3:8b)").
					Placeholder("model:tag").
					Value(&customName),
			),
		)
		if err := runForm(ctx, customForm); err != nil {
			return "", err
		}
		return customName, nil
	}

	return selected, nil
}

// pullModelWithSpinner calls PullModel synchronously, draining progress into a spinner-like output.
// PullModel requires a non-nil channel; we allocate a buffered channel and drain it in a goroutine.
//
// @MX:WARN: [AUTO] Goroutine launched to drain PullModel's progress channel.
// @MX:REASON: PullModel panics on nil channel and blocks until the channel is consumed;
// the drain goroutine must outlive the PullModel call to avoid deadlock.
func pullModelWithSpinner(ctx context.Context, modelName string) error {
	fmt.Printf("Downloading %s... (this may take a while)\n", modelName)

	progress := make(chan onboarding.ProgressUpdate, 32)

	// Drain the progress channel and print dots to indicate activity.
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for range progress {
			// Phase 2A: consume silently; Phase 2C will wire a progress bar.
		}
	}()

	err := onboarding.PullModel(ctx, modelName, progress)
	<-drainDone // wait for drain goroutine before returning

	if err != nil {
		return fmt.Errorf("model download failed: %w", err)
	}

	fmt.Println("Download complete.")
	return nil
}
