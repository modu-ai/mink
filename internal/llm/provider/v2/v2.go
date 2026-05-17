// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package v2 provides the V2 LLM provider interface and factory for MINK.
//
// # V1 vs V2 Boundary
//
// V1 adapters live in internal/llm/provider/{anthropic,deepseek,glm,...} and
// implement the internal/llm/provider.Provider interface (Complete + Stream).
// They are retained unchanged.
//
// V2 adapters live in internal/llm/provider/v2/{anthropic,deepseek,...} and
// implement the [Provider] interface defined in this package (Chat + ChatStream
// + HealthCheck).  V2 is simpler, chat-centric, and stream-first.
//
// The two layers co-exist; routers choose which layer to invoke.
//
// # Factory
//
// Use [NewByName] to obtain a V2 Provider by canonical name string:
//
//	p, err := v2.NewByName("anthropic", v2.ClientConfig{APIKey: "sk-ant-..."})
//
// Supported names: "anthropic", "deepseek", "openai", "codex", "zai", "custom".
// Unknown names return [ErrModelNotFound].
package v2

import (
	"fmt"

	"github.com/modu-ai/mink/internal/llm/provider/v2/anthropic"
	"github.com/modu-ai/mink/internal/llm/provider/v2/codex"
	"github.com/modu-ai/mink/internal/llm/provider/v2/custom"
	"github.com/modu-ai/mink/internal/llm/provider/v2/deepseek"
	"github.com/modu-ai/mink/internal/llm/provider/v2/openai"
	"github.com/modu-ai/mink/internal/llm/provider/v2/zai"
)

// ClientConfig carries the common configuration used by all V2 adapters.
//
// Provider-specific options (e.g. BaseURL for custom endpoints) are set via
// the ClientOptions parameter of each adapter's New function.  For the
// convenience factory [NewByName], all common options are provided here and
// forwarded to the appropriate adapter.
type ClientConfig struct {
	// APIKey is the authentication credential for the provider.
	// For OAuth-based providers (Codex), pass the access token here; the OAuth
	// acquisition flow is handled separately in M2.
	APIKey string
	// BaseURL overrides the default API base URL.  Useful for custom endpoints
	// and for pointing at mock servers in tests.  Empty means use the
	// provider's production URL.
	BaseURL string
}

// NewByName constructs a V2 Provider by canonical provider name.
//
// Supported names:
//   - "anthropic"  → Anthropic Claude (Messages API)
//   - "deepseek"   → DeepSeek (OpenAI-compatible)
//   - "openai"     → OpenAI Chat Completions
//   - "codex"      → Codex / ChatGPT backend (OAuth-based, M1 stub)
//   - "zai"        → z.ai GLM (OpenAI-compatible)
//   - "custom"     → User-defined OpenAI-compatible endpoint (BaseURL required)
//
// Unknown names return [ErrModelNotFound].
//
// @MX:ANCHOR: [AUTO] NewByName factory — dispatches to all 6 V2 provider packages
// @MX:REASON: Single entry point called by router and tests; adding a new provider requires updating this switch
func NewByName(name string, cfg ClientConfig) (Provider, error) {
	switch name {
	case "anthropic":
		return anthropic.New(cfg.APIKey, anthropic.ClientOptions{BaseURL: cfg.BaseURL})
	case "deepseek":
		return deepseek.New(cfg.APIKey, deepseek.ClientOptions{BaseURL: cfg.BaseURL})
	case "openai":
		return openai.New(cfg.APIKey, openai.ClientOptions{BaseURL: cfg.BaseURL})
	case "codex":
		return codex.New(cfg.APIKey, codex.ClientOptions{BaseURL: cfg.BaseURL})
	case "zai":
		return zai.New(cfg.APIKey, zai.ClientOptions{BaseURL: cfg.BaseURL})
	case "custom":
		return custom.New(cfg.APIKey, custom.ClientOptions{BaseURL: cfg.BaseURL})
	default:
		return nil, fmt.Errorf("%w: %q", ErrModelNotFound, name)
	}
}
