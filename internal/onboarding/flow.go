// Package onboarding — flow.go implements the 7-step onboarding state machine.
// StartFlow, SubmitStep, SkipStep, Back, Complete, and CompleteAndPersist are the
// public entry points. Phase 1F wires validators, keyring persistence, and
// completion writers into the state machine.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2
// REQ: REQ-OB-001, REQ-OB-024
// AC: AC-OB-001 (partial), AC-OB-006 (partial), AC-OB-025 (partial)
package onboarding

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Sentinel errors returned by the state machine methods.
// Callers use errors.Is to distinguish error categories.
var (
	// ErrStepOutOfOrder is returned when the supplied step does not equal CurrentStep.
	ErrStepOutOfOrder = errors.New("onboarding: step out of order")

	// ErrStepRange is returned when the supplied step is not in [1, 7].
	ErrStepRange = errors.New("onboarding: step out of valid range [1, 7]")

	// ErrStepDataMismatch is returned when data does not match the type expected for step.
	ErrStepDataMismatch = errors.New("onboarding: data type does not match step")

	// ErrCannotGoBack is returned by Back when CurrentStep is already 1.
	ErrCannotGoBack = errors.New("onboarding: cannot go back from step 1")

	// ErrNotReadyToComplete is returned by Complete when CurrentStep != 8.
	ErrNotReadyToComplete = errors.New("onboarding: all 7 steps must be submitted before completing")

	// ErrPersistFailed is returned by CompleteAndPersist when WriteCompletionConfig fails.
	ErrPersistFailed = errors.New("onboarding: completion config write failed")

	// ErrMarkerFailed is returned by CompleteAndPersist when WriteOnboardingCompleted fails.
	ErrMarkerFailed = errors.New("onboarding: completion marker write failed")
)

// totalSteps is the canonical step count for SPEC-MINK-ONBOARDING-001 v0.3.
const totalSteps = 7

// completedSentinel is the value CurrentStep holds after the last step is submitted,
// indicating that Complete may be called.
const completedSentinel = totalSteps + 1

// TotalSteps returns the total number of onboarding wizard steps.
// Exported for use by the TUI layer to bound its dispatch loop.
func TotalSteps() int { return totalSteps }

// FlowOption configures an OnboardingFlow during StartFlow.
type FlowOption func(*OnboardingFlow)

// WithKeyring injects an OS keyring client used by step 5 API key persistence.
// When nil or unset, step 5 still validates the API key but does NOT persist it,
// and ProviderChoice.APIKeyStored remains false.
func WithKeyring(kr KeyringClient) FlowOption {
	return func(f *OnboardingFlow) { f.keyring = kr }
}

// WithCompletionOptions configures the persistence options used by CompleteAndPersist.
// Defaults: DryRun=false, real paths from GlobalConfigPath / ProjectConfigPath /
// OnboardingCompletedPath, time.Now.
func WithCompletionOptions(opts CompletionOptions) FlowOption {
	return func(f *OnboardingFlow) { f.completionOpts = opts }
}

// ProviderStepInput is the step-5 SubmitStep payload combining the user's
// ProviderChoice selection with the raw API key string. The key is validated,
// stored in the keyring if a KeyringClient is configured, and immediately
// zeroed from this struct after persistence.
type ProviderStepInput struct {
	Choice ProviderChoice
	APIKey string // raw secret; consumed by SubmitStep and not retained in Data
}

// OnboardingFlow is the backend state machine for the 7-step install wizard.
// It is shared between the CLI (charmbracelet/huh) and Web UI (React + Go HTTP) layers.
// Thread-safety is the caller's responsibility; the struct is not safe for concurrent use.
// @MX:ANCHOR: [AUTO] Entry point for both CLI and Web UI onboarding paths — fan_in >= 3.
// @MX:REASON: StartFlow, CLI TUI, and Web UI handler all create or pass *OnboardingFlow.
// @MX:SPEC: SPEC-MINK-ONBOARDING-001 §6.2
type OnboardingFlow struct {
	// SessionID is a random 32-character hex string assigned by StartFlow.
	SessionID string

	// CurrentStep tracks the step the user must complete next.
	// Range: 1..7 during active wizard; 8 (completedSentinel) after step 7 is submitted.
	CurrentStep int

	// Data accumulates the user's answers across all 7 steps.
	Data OnboardingData

	// StartedAt records when StartFlow was called (UTC).
	StartedAt time.Time

	// CompletedAt is set to a non-nil pointer when Complete succeeds.
	CompletedAt *time.Time

	// visitedSteps tracks steps that have been submitted at least once,
	// enabling Back → re-submit overwrites without loss of previous answers.
	visitedSteps map[int]bool

	// keyring is the optional OS keyring client injected via WithKeyring.
	// When nil, step 5 validates the API key but skips keyring persistence.
	keyring KeyringClient

	// completionOpts holds the persistence options used by CompleteAndPersist.
	completionOpts CompletionOptions
}

// StartFlow creates a fresh 7-step onboarding flow starting at Step 1.
// A cryptographically random 32-hex-character SessionID is generated.
// If locale is non-nil, Data.Locale is prefilled from the caller-supplied value;
// this is the forward reference to LOCALE-001's Detect() result.
// Optional FlowOption values configure keyring and completion options.
// @MX:ANCHOR: [AUTO] Factory function — called by CLI init and Web UI HTTP handler.
// @MX:REASON: Single creation point for OnboardingFlow; signature change affects all callers.
// @MX:SPEC: SPEC-MINK-ONBOARDING-001 §6.2
func StartFlow(_ context.Context, locale *LocaleChoice, opts ...FlowOption) (*OnboardingFlow, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("onboarding: failed to generate session ID: %w", err)
	}

	f := &OnboardingFlow{
		SessionID:    id,
		CurrentStep:  1,
		StartedAt:    time.Now().UTC(),
		visitedSteps: make(map[int]bool),
	}

	if locale != nil {
		f.Data.Locale = *locale
	}

	for _, opt := range opts {
		opt(f)
	}

	return f, nil
}

// SubmitStep stores the supplied data for the given step and advances CurrentStep.
// Rules:
//   - step must equal CurrentStep (ErrStepOutOfOrder otherwise)
//   - step must be in [1, totalSteps] (ErrStepRange otherwise)
//   - data must be assignable to the field type for the step (ErrStepDataMismatch otherwise)
//   - On success with step == totalSteps, CurrentStep advances to completedSentinel (8).
func (f *OnboardingFlow) SubmitStep(step int, data any) error {
	if err := f.validateStepNumber(step); err != nil {
		return err
	}

	if err := f.assignStepData(step, data); err != nil {
		return err
	}

	f.visitedSteps[step] = true
	f.CurrentStep = step + 1
	return nil
}

// SkipStep advances CurrentStep without storing data; the field retains its zero value
// (or whatever was previously written by an earlier SubmitStep on the same step).
// When step == 7 and the locale LegalFlags contains "GDPR" (case-insensitive),
// SkipStep returns ErrGDPRConsentRequired — consent cannot be skipped in GDPR regions.
func (f *OnboardingFlow) SkipStep(step int) error {
	if err := f.validateStepNumber(step); err != nil {
		return err
	}

	// GDPR enforcement: step 7 cannot be skipped in GDPR jurisdictions.
	if step == 7 {
		for _, flag := range f.Data.Locale.LegalFlags {
			if strings.EqualFold(flag, "GDPR") {
				return fmt.Errorf("%w: consent step cannot be skipped in GDPR regions", ErrGDPRConsentRequired)
			}
		}
	}

	f.visitedSteps[step] = true
	f.CurrentStep = step + 1
	return nil
}

// Back decrements CurrentStep by 1, returning ErrCannotGoBack when already at step 1.
// Data accumulated in the vacated step is preserved; a subsequent SubmitStep will overwrite.
func (f *OnboardingFlow) Back() error {
	if f.CurrentStep <= 1 {
		return ErrCannotGoBack
	}

	f.CurrentStep--
	return nil
}

// Complete finalizes the flow if CurrentStep == completedSentinel (8).
// It stamps CompletedAt and returns a snapshot of the collected OnboardingData.
// Returns ErrNotReadyToComplete when not all steps have been submitted or skipped.
func (f *OnboardingFlow) Complete() (*OnboardingData, error) {
	if f.CurrentStep != completedSentinel {
		return nil, ErrNotReadyToComplete
	}

	now := time.Now().UTC()
	f.CompletedAt = &now

	snapshot := f.Data // value copy
	return &snapshot, nil
}

// CompleteAndPersist finalizes the flow and persists the result to disk.
// Equivalent to Complete() + WriteCompletionConfig(data, keyring, opts) +
// WriteOnboardingCompleted(opts), using the keyring injected via WithKeyring and
// the CompletionOptions from WithCompletionOptions.
//
// Returns wrapped errors:
//
//	ErrNotReadyToComplete -- when CurrentStep != completedSentinel
//	ErrPersistFailed      -- when WriteCompletionConfig fails
//	ErrMarkerFailed       -- when WriteOnboardingCompleted fails
//
// On success, returns the OnboardingData snapshot identical to Complete().
func (f *OnboardingFlow) CompleteAndPersist() (*OnboardingData, error) {
	data, err := f.Complete()
	if err != nil {
		return nil, err
	}

	if err := WriteCompletionConfig(data, f.keyring, f.completionOpts); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPersistFailed, err)
	}

	if err := WriteOnboardingCompleted(f.completionOpts); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMarkerFailed, err)
	}

	return data, nil
}

// ErrInvalidDraft is returned by StartFlowFromDraft when the provided Draft fails
// basic sanity checks (nil, or CurrentStep out of range [1, completedSentinel]).
var ErrInvalidDraft = errors.New("onboarding: invalid draft for resume")

// StartFlowFromDraft reconstructs an OnboardingFlow from a previously saved Draft.
// CurrentStep must be in [1, completedSentinel] (1..8); steps 1..CurrentStep-1
// are marked as visited so subsequent Back() works correctly.
// Data, SessionID, StartedAt are copied verbatim from the draft.
// CompletedAt is left nil (Resume always re-runs at least one step before completion).
// FlowOption(s) are applied identically to StartFlow.
//
// Returns ErrInvalidDraft when d is nil or d.CurrentStep is outside [1, completedSentinel].
//
// @MX:ANCHOR: [AUTO] Entry point for resume path — called by CLI init --resume and tests.
// @MX:REASON: Reconstructs unexported fields (visitedSteps, keyring, completionOpts);
// any change to OnboardingFlow internal layout must be reflected here.
// @MX:SPEC: SPEC-MINK-ONBOARDING-001 §6 (Phase 2B)
func StartFlowFromDraft(_ context.Context, d *Draft, opts ...FlowOption) (*OnboardingFlow, error) {
	if d == nil {
		return nil, fmt.Errorf("%w: draft is nil", ErrInvalidDraft)
	}
	if d.CurrentStep < 1 || d.CurrentStep > completedSentinel {
		return nil, fmt.Errorf("%w: current_step out of range [1, %d]: %d",
			ErrInvalidDraft, completedSentinel, d.CurrentStep)
	}

	f := &OnboardingFlow{
		SessionID:    d.SessionID,
		CurrentStep:  d.CurrentStep,
		Data:         d.Data,
		StartedAt:    d.StartedAt,
		visitedSteps: make(map[int]bool, d.CurrentStep),
	}

	// Mark steps 1..CurrentStep-1 as visited so Back() navigates them correctly.
	for s := 1; s < d.CurrentStep; s++ {
		f.visitedSteps[s] = true
	}

	for _, opt := range opts {
		opt(f)
	}

	return f, nil
}

// validateStepNumber checks that step equals CurrentStep and is within [1, totalSteps].
func (f *OnboardingFlow) validateStepNumber(step int) error {
	if step < 1 || step > totalSteps {
		return ErrStepRange
	}
	if step != f.CurrentStep {
		return ErrStepOutOfOrder
	}
	return nil
}

// assignStepData performs a type-switch on data and writes it into the appropriate
// OnboardingData field. Returns ErrStepDataMismatch when the type does not match.
// @MX:WARN: [AUTO] Step 5 handles two accepted types (ProviderStepInput and ProviderChoice).
// @MX:REASON: Legacy ProviderChoice path exists for Phase 1A backward compatibility;
// adding a third type here will require careful ordering to avoid silent type coercion.
func (f *OnboardingFlow) assignStepData(step int, data any) error {
	switch step {
	case 1:
		v, ok := data.(LocaleChoice)
		if !ok {
			return fmt.Errorf("%w: step 1 expects LocaleChoice, got %T", ErrStepDataMismatch, data)
		}
		f.Data.Locale = v
	case 2:
		v, ok := data.(ModelSetup)
		if !ok {
			return fmt.Errorf("%w: step 2 expects ModelSetup, got %T", ErrStepDataMismatch, data)
		}
		f.Data.Model = v
	case 3:
		v, ok := data.(CLIToolsDetection)
		if !ok {
			return fmt.Errorf("%w: step 3 expects CLIToolsDetection, got %T", ErrStepDataMismatch, data)
		}
		f.Data.CLITools = v
	case 4:
		v, ok := data.(PersonaProfile)
		if !ok {
			return fmt.Errorf("%w: step 4 expects PersonaProfile, got %T", ErrStepDataMismatch, data)
		}
		// Validate persona name (required, length, injection chars).
		if err := ValidatePersonaName(v.Name); err != nil {
			return fmt.Errorf("step 4 persona validation: %w", err)
		}
		// HonorificLevel "" is allowed; completion.go fills "formal" default.
		// Only validate when non-empty.
		if v.HonorificLevel != "" {
			if err := ValidateHonorificLevel(string(v.HonorificLevel)); err != nil {
				return fmt.Errorf("step 4 honorific validation: %w", err)
			}
		}
		f.Data.Persona = v
	case 5:
		// ProviderStepInput is the preferred step-5 payload (Phase 1F).
		// ProviderChoice is accepted for backward compatibility (Phase 1A callers).
		switch v := data.(type) {
		case ProviderStepInput:
			choice := v.Choice
			apiKey := v.APIKey

			// Always validate the API key regardless of keyring availability.
			if err := ValidateProviderAPIKey(string(choice.Provider), apiKey); err != nil {
				return fmt.Errorf("step 5 API key validation: %w", err)
			}

			// Store in keyring only when all three conditions are true:
			//   1. AuthMethod is api_key
			//   2. The key is non-empty
			//   3. A keyring client is injected
			if choice.AuthMethod == AuthMethodAPIKey && apiKey != "" && f.keyring != nil {
				if err := SetProviderAPIKey(f.keyring, string(choice.Provider), apiKey); err != nil {
					return fmt.Errorf("step 5 keyring write: %w", err)
				}
				choice.APIKeyStored = true
			}

			// Zero the key string before assigning to Data (security invariant).
			v.APIKey = ""
			f.Data.Provider = choice

		case ProviderChoice:
			// Legacy path: assign directly, no validation, no keyring write.
			f.Data.Provider = v

		default:
			return fmt.Errorf("%w: step 5 expects ProviderStepInput or ProviderChoice, got %T", ErrStepDataMismatch, data)
		}
	case 6:
		v, ok := data.(MessengerChannel)
		if !ok {
			return fmt.Errorf("%w: step 6 expects MessengerChannel, got %T", ErrStepDataMismatch, data)
		}
		f.Data.Messenger = v
	case 7:
		v, ok := data.(ConsentFlags)
		if !ok {
			return fmt.Errorf("%w: step 7 expects ConsentFlags, got %T", ErrStepDataMismatch, data)
		}
		// Validate GDPR consent using the locale captured in step 1.
		if err := ValidateGDPRConsent(v, f.Data.Locale); err != nil {
			return fmt.Errorf("step 7 consent validation: %w", err)
		}
		f.Data.Consent = v
	}
	return nil
}

// generateSessionID returns a 32-character lowercase hex string from 16 random bytes.
func generateSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
