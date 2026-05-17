// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package m1 contains the M1 milestone scaffolding for SPEC-MINK-WEB-CONFIG-001.
//
// M1 establishes the ConfigHandler interface and stub implementation for the
// post-install web-based config UI (`mink config --web`).  Real CRUD endpoints
// for auth.store, LLM provider keys, channel credentials, and brand theme land
// in M2~M4 reusing the install server (CSRF + SessionStore) infrastructure.
//
// SPEC: SPEC-MINK-WEB-CONFIG-001 (REQ-WCFG-001~008, AC-WCFG-001~005)
package m1

import (
	"context"
	"errors"
)

// ErrNotImplemented marks a method whose real implementation lands in M2~M4.
var ErrNotImplemented = errors.New("web config m1: not implemented (real impl in M2~M4)")

// ConfigSection identifies a settable group of configuration values.
type ConfigSection string

const (
	SectionAuthStore    ConfigSection = "auth_store"
	SectionLLMProviders ConfigSection = "llm_providers"
	SectionChannels     ConfigSection = "channels"
	SectionBrandTheme   ConfigSection = "brand_theme"
	SectionLocale       ConfigSection = "locale"
)

// ConfigSnapshot is the response shape for GET /api/config.
type ConfigSnapshot struct {
	AuthStore     string            `json:"auth_store"`
	LLMProviders  map[string]string `json:"llm_providers"` // provider id → masked LAST4
	Channels      map[string]string `json:"channels"`      // channel id → "configured" | "missing"
	BrandTheme    string            `json:"brand_theme"`
	LocaleCountry string            `json:"locale_country"`
}

// Handler is the M1 web config handler interface.
type Handler interface {
	// Snapshot returns the current configuration with secrets masked.
	Snapshot(ctx context.Context) (ConfigSnapshot, error)

	// UpdateSection applies a section update.  Body shape is section-specific
	// and validated against the corresponding schema in M2~M4.
	UpdateSection(ctx context.Context, section ConfigSection, body []byte) error
}

type stubHandler struct{}

// NewStubHandler returns the M1 stub handler.
func NewStubHandler() Handler { return &stubHandler{} }

func (*stubHandler) Snapshot(_ context.Context) (ConfigSnapshot, error) {
	return ConfigSnapshot{}, ErrNotImplemented
}

func (*stubHandler) UpdateSection(_ context.Context, _ ConfigSection, _ []byte) error {
	return ErrNotImplemented
}
