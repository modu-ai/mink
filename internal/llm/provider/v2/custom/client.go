// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package custom implements the V2 Provider interface for user-defined
// OpenAI-compatible endpoints.
//
// The custom adapter allows users to point MINK at any OpenAI-compatible server
// (e.g. vllm, ollama, LM Studio, Together, OpenRouter) by specifying a BaseURL.
// Unlike the curated adapters, KnownModels is empty (any model string is
// accepted) and the API key may be empty for local servers that do not require
// authentication.
//
// M1 status: Name() and Capabilities() return real values.  Chat, ChatStream,
// and HealthCheck return ErrNotImplemented.
package custom

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/llm/provider/v2/iface"
)

// ClientOptions holds configuration for the custom endpoint adapter.
type ClientOptions struct {
	// BaseURL is the base URL of the custom OpenAI-compatible server.
	// Must be set; empty BaseURL is rejected by New.
	BaseURL string
}

// Client is the V2 custom OpenAI-compatible endpoint adapter.
//
// @MX:ANCHOR: [AUTO] Client — V2 custom endpoint adapter implementing iface.Provider
// @MX:REASON: Constructed by v2.NewByName("custom"); BaseURL must be user-provided — validation critical
type Client struct {
	// apiKey may be empty for servers that do not require authentication.
	apiKey  string
	baseURL string
}

// Compile-time assertion: Client must satisfy iface.Provider.
var _ iface.Provider = (*Client)(nil)

// New constructs a new custom endpoint Client.
//
// Returns an error if opts.BaseURL is empty.  apiKey may be empty for local
// servers.
func New(apiKey string, opts ClientOptions) (*Client, error) {
	if opts.BaseURL == "" {
		return nil, fmt.Errorf("%w: custom endpoint requires a non-empty BaseURL in ClientOptions", iface.ErrInvalidRequest)
	}
	return &Client{apiKey: apiKey, baseURL: opts.BaseURL}, nil
}

// Name returns the canonical provider name.
func (c *Client) Name() string { return "custom" }

// Capabilities returns the static capabilities of the custom adapter.
//
// KnownModels is empty because custom endpoints accept any model string.
// SupportsStream is true as all OpenAI-compatible servers must support SSE.
func (c *Client) Capabilities() iface.Capabilities {
	return iface.Capabilities{
		SupportsStream:   true,
		SupportsVision:   false,
		MaxContextTokens: 0, // Unknown; depends on the underlying server.
		KnownModels:      []string{},
	}
}

// Chat performs a non-streaming custom endpoint API call.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) Chat(_ context.Context, _ iface.ChatRequest) (iface.ChatResponse, error) {
	return iface.ChatResponse{}, fmt.Errorf("custom.Client.Chat: %w", iface.ErrNotImplemented)
}

// ChatStream begins a streaming custom endpoint API call.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) ChatStream(_ context.Context, _ iface.ChatRequest) (iface.ChatStream, error) {
	return nil, fmt.Errorf("custom.Client.ChatStream: %w", iface.ErrNotImplemented)
}

// HealthCheck probes the custom endpoint.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) HealthCheck(_ context.Context) error {
	return fmt.Errorf("custom.Client.HealthCheck: %w", iface.ErrNotImplemented)
}
