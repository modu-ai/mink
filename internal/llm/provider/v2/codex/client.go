// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package codex implements the V2 Provider interface for the Codex / ChatGPT
// backend.
//
// Codex uses OAuth 2.1 PKCE for authentication rather than a static API key;
// the OAuth acquisition flow is handled by the auth package (M2).  In M1,
// apiKey carries the already-obtained access token (passed by the caller).
//
// M1 status: Name() and Capabilities() return real values.  Chat, ChatStream,
// and HealthCheck return ErrNotImplemented.
package codex

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/llm/provider/v2/iface"
)

const defaultBaseURL = "https://api.openai.com"

var knownModels = []string{
	"codex-1",
}

// ClientOptions holds optional configuration for the Codex adapter.
type ClientOptions struct {
	// BaseURL overrides the Codex API base URL.
	BaseURL string
}

// Client is the V2 Codex adapter.
//
// @MX:ANCHOR: [AUTO] Client — V2 Codex adapter implementing iface.Provider
// @MX:REASON: Constructed by v2.NewByName("codex"); OAuth flow integration (M2) adds token refresh complexity
type Client struct {
	// accessToken holds the OAuth 2.1 access token obtained externally.
	accessToken string
	baseURL     string
}

// Compile-time assertion: Client must satisfy iface.Provider.
var _ iface.Provider = (*Client)(nil)

// New constructs a new Codex Client.
//
// accessToken should be the OAuth 2.1 access token obtained via the auth
// package.  Returns iface.ErrAPIKey if the token is empty.
func New(accessToken string, opts ClientOptions) (*Client, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("%w: codex requires a non-empty access token", iface.ErrAPIKey)
	}
	base := opts.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{accessToken: accessToken, baseURL: base}, nil
}

// Name returns the canonical provider name.
func (c *Client) Name() string { return "codex" }

// Capabilities returns the static capabilities of the Codex adapter.
func (c *Client) Capabilities() iface.Capabilities {
	return iface.Capabilities{
		SupportsStream:   true,
		SupportsVision:   false,
		MaxContextTokens: 64_000,
		KnownModels:      knownModels,
	}
}

// Chat performs a non-streaming Codex API call.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) Chat(_ context.Context, _ iface.ChatRequest) (iface.ChatResponse, error) {
	return iface.ChatResponse{}, fmt.Errorf("codex.Client.Chat: %w", iface.ErrNotImplemented)
}

// ChatStream begins a streaming Codex API call.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) ChatStream(_ context.Context, _ iface.ChatRequest) (iface.ChatStream, error) {
	return nil, fmt.Errorf("codex.Client.ChatStream: %w", iface.ErrNotImplemented)
}

// HealthCheck probes the Codex API.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) HealthCheck(_ context.Context) error {
	return fmt.Errorf("codex.Client.HealthCheck: %w", iface.ErrNotImplemented)
}
