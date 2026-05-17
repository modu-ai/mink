// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package phase5 contains the M1 milestone scaffolding for
// SPEC-MINK-ONBOARDING-001-PHASE5-AMEND-001 — onboarding wizard Phase 5
// (LLM provider key collection + channel credential setup + brand theme).
//
// M1 establishes the Phase 5 step handler interface and stub implementation.
// Real Step 2 (provider keys) and Step 3 (channels) wiring on top of the
// existing internal/server/install handler.go SessionStore + CSRF
// infrastructure lands in M2~M4.
//
// SPEC: SPEC-MINK-ONBOARDING-001-PHASE5-AMEND-001 (REQ-ONB5-001~008,
// AC-OB5-001~005)
package phase5

import (
	"context"
	"errors"
)

// ErrNotImplemented marks a method whose real implementation lands in M2~M4.
var ErrNotImplemented = errors.New("onboarding phase5 m1: not implemented (real impl in M2~M4)")

// Step identifies an onboarding wizard step that Phase 5 extends.
type Step string

const (
	StepLLMProviders Step = "llm_providers" // Step 2 of Phase 5
	StepChannels     Step = "channels"      // Step 3 of Phase 5
	StepBrandTheme   Step = "brand_theme"   // Step 4 of Phase 5
	StepReview       Step = "review"        // Step 5 of Phase 5
)

// StepResult is the per-step result returned to the React wizard.
type StepResult struct {
	Step      Step   `json:"step"`
	Completed bool   `json:"completed"`
	NextStep  Step   `json:"next_step,omitempty"`
	ErrorMsg  string `json:"error,omitempty"`
}

// Handler is the M1 Phase 5 step handler interface.
type Handler interface {
	// SubmitStep accepts a step submission from the React wizard.
	// M2~M4 wires real credential storage + validation; M1 returns
	// ErrNotImplemented.
	SubmitStep(ctx context.Context, step Step, sessionID string, body []byte) (StepResult, error)

	// SkipStep marks a step as skipped (some steps are optional).
	SkipStep(ctx context.Context, step Step, sessionID string) (StepResult, error)
}

type stubHandler struct{}

// NewStubHandler returns the M1 stub handler.
func NewStubHandler() Handler { return &stubHandler{} }

func (*stubHandler) SubmitStep(_ context.Context, _ Step, _ string, _ []byte) (StepResult, error) {
	return StepResult{}, ErrNotImplemented
}

func (*stubHandler) SkipStep(_ context.Context, _ Step, _ string) (StepResult, error) {
	return StepResult{}, ErrNotImplemented
}
