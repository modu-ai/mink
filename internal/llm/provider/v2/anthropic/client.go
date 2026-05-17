// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package anthropic implements the V2 Provider interface for Anthropic Claude.
//
// M1 status: Name() and Capabilities() return real values.  Chat, ChatStream,
// and HealthCheck return ErrNotImplemented; full Anthropic Messages API
// integration (SSE event translation) lands in M2.
package anthropic

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/llm/provider/v2/iface"
)

// defaultBaseURL is the Anthropic Messages API base URL.
const defaultBaseURL = "https://api.anthropic.com"

// knownModels lists the Anthropic model identifiers supported by this adapter.
var knownModels = []string{
	"claude-opus-4-7",
	"claude-sonnet-4-6",
	"claude-haiku-4-5",
}

// ClientOptions holds optional configuration for the Anthropic adapter.
// The zero value is valid (uses production defaults).
type ClientOptions struct {
	// BaseURL overrides the Anthropic API base URL.  Empty means use the
	// production URL.
	BaseURL string
}

// Client is the V2 Anthropic adapter.
//
// @MX:ANCHOR: [AUTO] Client — V2 Anthropic adapter implementing iface.Provider
// @MX:REASON: Constructed by v2.NewByName("anthropic") and used by the V2 router
type Client struct {
	apiKey  string
	baseURL string
}

// Compile-time assertion: Client must satisfy the iface.Provider interface.
var _ iface.Provider = (*Client)(nil)

// New constructs a new Anthropic Client.
//
// Returns iface.ErrAPIKey if apiKey is empty.
func New(apiKey string, opts ClientOptions) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("%w: anthropic requires a non-empty API key", iface.ErrAPIKey)
	}
	base := opts.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{apiKey: apiKey, baseURL: base}, nil
}

// Name returns the canonical provider name.
func (c *Client) Name() string { return "anthropic" }

// Capabilities returns the static capabilities of the Anthropic adapter.
func (c *Client) Capabilities() iface.Capabilities {
	return iface.Capabilities{
		SupportsStream:   true,
		SupportsVision:   true,
		MaxContextTokens: 1_000_000,
		KnownModels:      knownModels,
	}
}

// Chat performs a non-streaming Anthropic Messages API call.
//
// M1 stub: returns ErrNotImplemented.  Full SSE implementation lands in M2.
func (c *Client) Chat(_ context.Context, _ iface.ChatRequest) (iface.ChatResponse, error) {
	return iface.ChatResponse{}, fmt.Errorf("anthropic.Client.Chat: %w", iface.ErrNotImplemented)
}

// ChatStream begins a streaming Anthropic Messages API call.
//
// M1 stub: returns ErrNotImplemented.  SSE event translation lands in M2.
func (c *Client) ChatStream(_ context.Context, _ iface.ChatRequest) (iface.ChatStream, error) {
	return nil, fmt.Errorf("anthropic.Client.ChatStream: %w", iface.ErrNotImplemented)
}

// HealthCheck probes the Anthropic API to verify connectivity and key validity.
//
// M1 stub: returns ErrNotImplemented.  Live probe lands in M2.
func (c *Client) HealthCheck(_ context.Context) error {
	return fmt.Errorf("anthropic.Client.HealthCheck: %w", iface.ErrNotImplemented)
}
