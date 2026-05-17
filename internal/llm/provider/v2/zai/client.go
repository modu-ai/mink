// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package zai implements the V2 Provider interface for z.ai GLM.
//
// z.ai exposes an OpenAI-compatible Chat Completions API.
// M1 status: Name() and Capabilities() return real values.  Chat, ChatStream,
// and HealthCheck return ErrNotImplemented.
package zai

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/llm/provider/v2/iface"
)

const defaultBaseURL = "https://open.bigmodel.cn"

var knownModels = []string{
	"glm-5-turbo",
	"glm-4-plus",
}

// ClientOptions holds optional configuration for the z.ai GLM adapter.
type ClientOptions struct {
	// BaseURL overrides the z.ai API base URL.
	BaseURL string
}

// Client is the V2 z.ai GLM adapter.
//
// @MX:ANCHOR: [AUTO] Client — V2 z.ai GLM adapter implementing iface.Provider
// @MX:REASON: Constructed by v2.NewByName("zai") and used by the V2 router
type Client struct {
	apiKey  string
	baseURL string
}

// Compile-time assertion: Client must satisfy iface.Provider.
var _ iface.Provider = (*Client)(nil)

// New constructs a new z.ai GLM Client.
//
// Returns iface.ErrAPIKey if apiKey is empty.
func New(apiKey string, opts ClientOptions) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("%w: zai requires a non-empty API key", iface.ErrAPIKey)
	}
	base := opts.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{apiKey: apiKey, baseURL: base}, nil
}

// Name returns the canonical provider name.
func (c *Client) Name() string { return "zai" }

// Capabilities returns the static capabilities of the z.ai GLM adapter.
func (c *Client) Capabilities() iface.Capabilities {
	return iface.Capabilities{
		SupportsStream:   true,
		SupportsVision:   false,
		MaxContextTokens: 128_000,
		KnownModels:      knownModels,
	}
}

// Chat performs a non-streaming z.ai GLM API call.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) Chat(_ context.Context, _ iface.ChatRequest) (iface.ChatResponse, error) {
	return iface.ChatResponse{}, fmt.Errorf("zai.Client.Chat: %w", iface.ErrNotImplemented)
}

// ChatStream begins a streaming z.ai GLM API call.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) ChatStream(_ context.Context, _ iface.ChatRequest) (iface.ChatStream, error) {
	return nil, fmt.Errorf("zai.Client.ChatStream: %w", iface.ErrNotImplemented)
}

// HealthCheck probes the z.ai GLM API.
//
// M1 stub: returns ErrNotImplemented.
func (c *Client) HealthCheck(_ context.Context) error {
	return fmt.Errorf("zai.Client.HealthCheck: %w", iface.ErrNotImplemented)
}
