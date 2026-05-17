// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package m1 contains the M1 milestone scaffolding for SPEC-MINK-MSG-DISCORD-001.
//
// M1 establishes the DiscordAdapter interface and stub implementation. Real
// Discord Interactions API integration, Ed25519 signature verification,
// LLM-ROUTING-V2 delegation, and Bot invite OAuth flow land in M2~M4.
//
// SPEC: SPEC-MINK-MSG-DISCORD-001 (REQ-DIS-001~006, AC-DIS-001~004)
package m1

import (
	"context"
	"errors"
)

// ErrNotImplemented marks a method whose real implementation lands in M2~M4.
var ErrNotImplemented = errors.New("discord m1: not implemented (real impl in M2~M4)")

// CanonicalInteraction is the normalized Discord interaction surface consumed
// by the downstream router/LLM/credential pipeline.
type CanonicalInteraction struct {
	UserID    string
	ChannelID string
	GuildID   string
	Type      int // 1=PING, 2=APPLICATION_COMMAND, 3=MESSAGE_COMPONENT
	Content   string
	Token     string
	AppID     string
}

// Adapter is the M1 Discord adapter interface.
type Adapter interface {
	// VerifyEd25519 validates the X-Signature-Ed25519 + X-Signature-Timestamp
	// headers against the public key stored under credential service "discord".
	VerifyEd25519(ctx context.Context, publicKey, timestamp, signature string, body []byte) error

	// NormalizeInteraction translates a raw Discord Interactions API payload
	// into the CanonicalInteraction shape consumed by the router.
	NormalizeInteraction(ctx context.Context, raw []byte) (CanonicalInteraction, error)

	// HandleSlashCommand is invoked when an APPLICATION_COMMAND interaction
	// arrives.  M3 wires LLM-ROUTING-V2 delegation.
	HandleSlashCommand(ctx context.Context, intr CanonicalInteraction) error
}

type stubAdapter struct{}

// NewStubAdapter returns the M1 stub adapter.
func NewStubAdapter() Adapter { return &stubAdapter{} }

func (*stubAdapter) VerifyEd25519(_ context.Context, _, _, _ string, _ []byte) error {
	return ErrNotImplemented
}

func (*stubAdapter) NormalizeInteraction(_ context.Context, _ []byte) (CanonicalInteraction, error) {
	return CanonicalInteraction{}, ErrNotImplemented
}

func (*stubAdapter) HandleSlashCommand(_ context.Context, _ CanonicalInteraction) error {
	return ErrNotImplemented
}
