// Package install implements the 7-step onboarding TUI built on charmbracelet/huh.
// SPEC: SPEC-MINK-ONBOARDING-001 §6 (Phase 2A — happy path)
//
// The package exposes RunWizard as its primary entry point. All TUI interaction
// uses huh forms; each step runs a separate huh.Form so that SubmitStep can be
// called incrementally after each step completes.
package install

import (
	"context"
	"errors"
	"fmt"
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
}

// RunWizard executes the 7-step onboarding TUI happy path.
//
// Returns nil on successful completion.
// Returns ErrWizardCancelled when the user presses Ctrl+C during any huh form.
// Returns a wrapped backend error on validation or persistence failure.
//
// @MX:ANCHOR: [AUTO] Primary public entry point for Phase 2A TUI — called by init.go cobra command.
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
	// Start onboarding flow
	// -----------------------------------------------------------------------
	flow, err := onboarding.StartFlow(ctx, nil,
		onboarding.WithKeyring(onboarding.SystemKeyring{}),
		onboarding.WithCompletionOptions(onboarding.CompletionOptions{DryRun: opts.DryRun}),
	)
	if err != nil {
		return fmt.Errorf("failed to start onboarding flow: %w", err)
	}

	// -----------------------------------------------------------------------
	// Step 1 — Locale
	// -----------------------------------------------------------------------
	if err := runStep1Locale(ctx, flow); err != nil {
		return err
	}

	// -----------------------------------------------------------------------
	// Step 2 — Model Setup
	// -----------------------------------------------------------------------
	if err := runStep2Model(ctx, flow, ollamaStatus, detectedModel, ramBytes); err != nil {
		return err
	}

	// -----------------------------------------------------------------------
	// Step 3 — CLI Tools
	// -----------------------------------------------------------------------
	if err := runStep3CLITools(ctx, flow, cliTools); err != nil {
		return err
	}

	// -----------------------------------------------------------------------
	// Step 4 — Persona
	// -----------------------------------------------------------------------
	if err := runStep4Persona(ctx, flow); err != nil {
		return err
	}

	// -----------------------------------------------------------------------
	// Step 5 — Provider
	// -----------------------------------------------------------------------
	if err := runStep5Provider(ctx, flow); err != nil {
		return err
	}

	// -----------------------------------------------------------------------
	// Step 6 — Messenger
	// -----------------------------------------------------------------------
	if err := runStep6Messenger(ctx, flow); err != nil {
		return err
	}

	// -----------------------------------------------------------------------
	// Step 7 — Consent
	// -----------------------------------------------------------------------
	if err := runStep7Consent(ctx, flow); err != nil {
		return err
	}

	// -----------------------------------------------------------------------
	// Finalize and persist
	// -----------------------------------------------------------------------
	data, err := flow.CompleteAndPersist()
	if err != nil {
		return fmt.Errorf("failed to persist onboarding config: %w", err)
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

// runStep1Locale presents the locale confirmation dialog and submits Step 1.
// Phase 2A hardcodes KR locale; Phase 2B will wire LOCALE-001 Detect().
func runStep1Locale(ctx context.Context, flow *onboarding.OnboardingFlow) error {
	// Phase 2A default: Korean / South Korea / Asia/Seoul / PIPA.
	locale := onboarding.LocaleChoice{
		Country:    "KR",
		Language:   "ko",
		Timezone:   "Asia/Seoul",
		LegalFlags: []string{"PIPA"},
	}

	confirmed := false
	localeDisplay := "Korean (KR / Asia/Seoul)"
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Step 1 / 7 — Locale\n\nDetected locale: %s\nUse this locale?", localeDisplay)).
				Affirmative("Yes, continue").
				Negative("No (Phase 2B: manual selection)").
				Value(&confirmed),
		),
	)

	if err := runForm(ctx, form); err != nil {
		return err
	}

	// Phase 2A only supports the default KR locale; if user declines, continue anyway
	// with defaults (manual locale selection is Phase 2B scope).
	return flow.SubmitStep(1, locale)
}

// runStep2Model handles Ollama and model setup for Step 2.
func runStep2Model(
	ctx context.Context,
	flow *onboarding.OnboardingFlow,
	status onboarding.OllamaStatus,
	detected onboarding.DetectedModel,
	ramBytes int64,
) error {
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

// runStep3CLITools displays detected tools as a MultiSelect and submits Step 3.
func runStep3CLITools(ctx context.Context, flow *onboarding.OnboardingFlow, detected []onboarding.CLITool) error {
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
func runStep4Persona(ctx context.Context, flow *onboarding.OnboardingFlow) error {
	var (
		name     string
		honorStr string
		pronouns string
		soul     string
	)

	name = "MINK"
	honorStr = string(onboarding.HonorificFormal)
	soul = soulMarkdownTemplate

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
func runStep5Provider(ctx context.Context, flow *onboarding.OnboardingFlow) error {
	var (
		providerStr string
		authStr     string
		apiKey      string
		customEP    string
		prefModel   string
	)

	providerStr = string(onboarding.ProviderAnthropic)
	authStr = string(onboarding.AuthMethodAPIKey)

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
func runStep6Messenger(ctx context.Context, flow *onboarding.OnboardingFlow) error {
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
func runStep7Consent(ctx context.Context, flow *onboarding.OnboardingFlow) error {
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

	// KR/PIPA locale does not require explicit GDPR consent; GDPRExplicitConsent = nil.
	return flow.SubmitStep(7, onboarding.ConsentFlags{
		ConversationStorageLocal: localOnly,
		TelemetryEnabled:         telemetry,
		CrashReportingEnabled:    crashReport,
		LoRATrainingAllowed:      loraTraining,
		GDPRExplicitConsent:      nil,
	})
}
