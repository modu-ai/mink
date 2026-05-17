// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package openai implements the V2 Provider interface for OpenAI Chat
// Completions.
//
// M1 status: Name() and Capabilities() return real values.  Chat, ChatStream,
// and HealthCheck return ErrNotImplemented; full Chat Completions + function
// calling + SSE translation lands in M2.
package openai

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/llm/provider/v2/iface"
)

const defaultBaseURL = "https://api.openai.com"

var knownModels = []string{
	"gpt-4o",
	"gpt-4o-mini",
	"gpt-5",
}

// ClientOptions holds optional configuration for the OpenAI adapter.
type ClientOptions struct {
	// BaseURL overrides the OpenAI API base URL.
	BaseURL string
}

// Client is the V2 OpenAI adapter.
//
// @MX:ANCHOR: [AUTO] Client — V2 OpenAI adapter implementing iface.Provider
// @MX:REASON: Constructed by v2.NewByName("openai") and used by the V2 router
type Client struct {
	apiKey  string
	baseURL string
}

// Compile-time assertion: Client must satisfy iface.Provider.
var _ iface.Provider = (*Client)(nil)

// New constructs a new OpenAI Client.
//
// Returns iface.ErrAPIKey if apiKey is empty.
func New(apiKey string, opts ClientOptions) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("%w: openai requires a non-empty API key", iface.ErrAPIKey)
	}
	base := opts.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{apiKey: apiKey, baseURL: base}, nil
}

// Name returns the canonical provider name.
func (c *Client) Name() string { return "openai" }

// Capabilities returns the static capabilities of the OpenAI adapter.
func (c *Client) Capabilities() iface.Capabilities {
	return iface.Capabilities{
		SupportsStream:   true,
		SupportsVision:   true,
		MaxContextTokens: 128_000,
		KnownModels:      knownModels,
	}
}

// Chat performs a non-streaming OpenAI Chat Completions call.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) Chat(_ context.Context, _ iface.ChatRequest) (iface.ChatResponse, error) {
	return iface.ChatResponse{}, fmt.Errorf("openai.Client.Chat: %w", iface.ErrNotImplemented)
}

// ChatStream begins a streaming OpenAI Chat Completions call.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) ChatStream(_ context.Context, _ iface.ChatRequest) (iface.ChatStream, error) {
	return nil, fmt.Errorf("openai.Client.ChatStream: %w", iface.ErrNotImplemented)
}

// HealthCheck probes the OpenAI API.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) HealthCheck(_ context.Context) error {
	return fmt.Errorf("openai.Client.HealthCheck: %w", iface.ErrNotImplemented)
}
