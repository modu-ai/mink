// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package m1 contains the M1 milestone scaffolding for SPEC-MINK-MSG-SLACK-001.
//
// M1 establishes the SlackAdapter interface and stub implementation. Real
// Slack Events API integration, signing-secret verification, LLM-ROUTING-V2
// delegation, and OAuth installation flow land in M2~M4.
//
// SPEC: SPEC-MINK-MSG-SLACK-001 (REQ-SLK-001~006, AC-SLK-001~004)
package m1

import (
	"context"
	"errors"
)

// ErrNotImplemented marks a method whose real implementation lands in M2~M4.
var ErrNotImplemented = errors.New("slack m1: not implemented (real impl in M2~M4)")

// CanonicalEvent is the normalized Slack event surface consumed by the
// downstream router/LLM/credential pipeline.  M1 fixes the shape so future
// milestones can extend without breaking the contract.
type CanonicalEvent struct {
	UserID    string
	ChannelID string
	TeamID    string
	Text      string
	Timestamp string
	ThreadTS  string
}

// Adapter is the M1 Slack adapter interface.  Methods return ErrNotImplemented
// until M2~M4 wires real Slack Events API + signing-secret + OAuth.
type Adapter interface {
	// VerifySigningSecret validates the X-Slack-Signature header against the
	// signing secret stored under credential service "slack" (M3 wiring).
	VerifySigningSecret(ctx context.Context, body []byte, header string) error

	// NormalizeEvent translates a raw Slack Events API payload into the
	// CanonicalEvent shape consumed by the router (M2 wiring).
	NormalizeEvent(ctx context.Context, raw []byte) (CanonicalEvent, error)

	// HandleAppMention is invoked when an app_mention event arrives.
	// M3 wires LLM-ROUTING-V2 delegation; M1 returns ErrNotImplemented.
	HandleAppMention(ctx context.Context, evt CanonicalEvent) error
}

// stubAdapter is the M1 placeholder.  Replaced in M2 by an httpAdapter
// implementation against api.slack.com.
type stubAdapter struct{}

// NewStubAdapter returns the M1 stub adapter.
func NewStubAdapter() Adapter { return &stubAdapter{} }

func (*stubAdapter) VerifySigningSecret(_ context.Context, _ []byte, _ string) error {
	return ErrNotImplemented
}

func (*stubAdapter) NormalizeEvent(_ context.Context, _ []byte) (CanonicalEvent, error) {
	return CanonicalEvent{}, ErrNotImplemented
}

func (*stubAdapter) HandleAppMention(_ context.Context, _ CanonicalEvent) error {
	return ErrNotImplemented
}
