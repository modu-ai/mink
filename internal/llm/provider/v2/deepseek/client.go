// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package deepseek implements the V2 Provider interface for DeepSeek.
//
// DeepSeek exposes an OpenAI-compatible Chat Completions API.
// M1 status: Name() and Capabilities() return real values.  Chat, ChatStream,
// and HealthCheck return ErrNotImplemented; full OpenAI-compat SSE translation
// lands in M2.
package deepseek

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/llm/provider/v2/iface"
)

const defaultBaseURL = "https://api.deepseek.com"

var knownModels = []string{
	"deepseek-chat",
	"deepseek-reasoner",
}

// ClientOptions holds optional configuration for the DeepSeek adapter.
type ClientOptions struct {
	// BaseURL overrides the DeepSeek API base URL.
	BaseURL string
}

// Client is the V2 DeepSeek adapter.
//
// @MX:ANCHOR: [AUTO] Client — V2 DeepSeek adapter implementing iface.Provider
// @MX:REASON: Constructed by v2.NewByName("deepseek") and used by the V2 router
type Client struct {
	apiKey  string
	baseURL string
}

// Compile-time assertion: Client must satisfy iface.Provider.
var _ iface.Provider = (*Client)(nil)

// New constructs a new DeepSeek Client.
//
// Returns iface.ErrAPIKey if apiKey is empty.
func New(apiKey string, opts ClientOptions) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("%w: deepseek requires a non-empty API key", iface.ErrAPIKey)
	}
	base := opts.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{apiKey: apiKey, baseURL: base}, nil
}

// Name returns the canonical provider name.
func (c *Client) Name() string { return "deepseek" }

// Capabilities returns the static capabilities of the DeepSeek adapter.
func (c *Client) Capabilities() iface.Capabilities {
	return iface.Capabilities{
		SupportsStream:   true,
		SupportsVision:   false,
		MaxContextTokens: 64_000,
		KnownModels:      knownModels,
	}
}

// Chat performs a non-streaming DeepSeek API call.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) Chat(_ context.Context, _ iface.ChatRequest) (iface.ChatResponse, error) {
	return iface.ChatResponse{}, fmt.Errorf("deepseek.Client.Chat: %w", iface.ErrNotImplemented)
}

// ChatStream begins a streaming DeepSeek API call.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) ChatStream(_ context.Context, _ iface.ChatRequest) (iface.ChatStream, error) {
	return nil, fmt.Errorf("deepseek.Client.ChatStream: %w", iface.ErrNotImplemented)
}

// HealthCheck probes the DeepSeek API.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) HealthCheck(_ context.Context) error {
	return fmt.Errorf("deepseek.Client.HealthCheck: %w", iface.ErrNotImplemented)
}
