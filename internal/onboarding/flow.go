// Package onboarding — flow.go implements the 7-step onboarding state machine.
// StartFlow, SubmitStep, SkipStep, Back, and Complete are the only entry points.
// No file I/O, no OS keyring, no CLI/Web layer code is present in Phase 1A.
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
)

// totalSteps is the canonical step count for SPEC-MINK-ONBOARDING-001 v0.3.
const totalSteps = 7

// completedSentinel is the value CurrentStep holds after the last step is submitted,
// indicating that Complete may be called.
const completedSentinel = totalSteps + 1

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
}

// StartFlow creates a fresh 7-step onboarding flow starting at Step 1.
// A cryptographically random 32-hex-character SessionID is generated.
// If locale is non-nil, Data.Locale is prefilled from the caller-supplied value;
// this is the forward reference to LOCALE-001's Detect() result.
// @MX:ANCHOR: [AUTO] Factory function — called by CLI init and Web UI HTTP handler.
// @MX:REASON: Single creation point for OnboardingFlow; signature change affects all callers.
// @MX:SPEC: SPEC-MINK-ONBOARDING-001 §6.2
func StartFlow(_ context.Context, locale *LocaleChoice) (*OnboardingFlow, error) {
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
// Phase 1A permits skipping any step. Validators that enforce GDPR no-skip (AC-OB-014)
// are added in Phase 1B (validators.go).
func (f *OnboardingFlow) SkipStep(step int) error {
	if err := f.validateStepNumber(step); err != nil {
		return err
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
// For Phase 1A this is the entire deliverable — no file write, no keyring, no
// UserProfile assembly (those are Phase 1C/1E).
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
		f.Data.Persona = v
	case 5:
		v, ok := data.(ProviderChoice)
		if !ok {
			return fmt.Errorf("%w: step 5 expects ProviderChoice, got %T", ErrStepDataMismatch, data)
		}
		f.Data.Provider = v
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
