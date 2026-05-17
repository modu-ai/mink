// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package iface defines the shared types, interfaces, and sentinel errors used
// by the V2 LLM provider system.
//
// Adapters (anthropic, deepseek, openai, codex, zai, custom) import this
// package to implement the Provider interface.  The parent v2 package imports
// both iface and the adapter packages to provide the factory function, avoiding
// the import cycle that would occur if adapters imported v2 directly.
//
// Consumers should import the parent v2 package, which re-exports all types
// from this package via type aliases.
package iface

import (
	"context"
	"errors"
)

// Sentinel errors returned by V2 provider adapters.
//
// All errors are intended to be matched with errors.Is so that callers can
// react to specific failure modes without inspecting error text.

// ErrNotImplemented is returned by stub methods that will be filled in M2+.
// M1 adapters return this for Chat, ChatStream, and HealthCheck.
var ErrNotImplemented = errors.New("v2: not implemented")

// ErrInvalidRequest is returned when the caller supplies a malformed
// ChatRequest (e.g., empty Messages, unsupported Role value, or a model not
// in KnownModels).
var ErrInvalidRequest = errors.New("v2: invalid request")

// ErrAPIKey is returned when the API key is missing, empty, or structurally
// invalid (e.g., wrong prefix for the provider).
var ErrAPIKey = errors.New("v2: invalid or missing API key")

// ErrRateLimited is returned when the upstream provider signals HTTP 429 or an
// equivalent rate-limit / quota-exceeded response.
var ErrRateLimited = errors.New("v2: rate limited")

// ErrModelNotFound is returned when the requested model identifier is not
// recognised by the provider, or when NewByName receives an unknown provider
// name.
var ErrModelNotFound = errors.New("v2: model or provider not found")

// ErrStreamClosed is returned when the caller calls ChatStream.Next after the
// stream has already been closed or exhausted.
var ErrStreamClosed = errors.New("v2: stream already closed")

// Role represents the sender role of a conversation message.
type Role string

const (
	// RoleSystem identifies a system-level instruction message.
	RoleSystem Role = "system"
	// RoleUser identifies a message from the human user.
	RoleUser Role = "user"
	// RoleAssistant identifies a message from the AI assistant.
	RoleAssistant Role = "assistant"
)

// Message is a single turn in a conversation.
type Message struct {
	// Role identifies who sent this message.
	Role Role
	// Content is the text body of the message.
	Content string
}

// ChatRequest is the unified input to Provider.Chat and Provider.ChatStream.
// All adapters translate this into their native request formats.
type ChatRequest struct {
	// Model is the model identifier (e.g. "claude-opus-4-7", "gpt-4o").
	// Adapters may validate this against Capabilities.KnownModels.
	Model string
	// Messages is the ordered conversation history, including the latest user
	// turn.  Must not be empty.
	Messages []Message
	// MaxTokens limits the number of output tokens.  Zero means use the
	// provider default.
	MaxTokens int
	// Temperature controls output randomness (0.0–2.0).  Zero means use the
	// provider default.
	Temperature float64
	// Stream, when true, signals that the caller intends to use ChatStream
	// rather than Chat.  Adapters may use this flag to set streaming mode in
	// the upstream request body.
	Stream bool
}

// ChatResponse is the complete, non-streaming response from Provider.Chat.
type ChatResponse struct {
	// Content is the full assistant reply text.
	Content string
	// ModelUsed is the exact model identifier returned by the provider (may
	// differ from the requested model if the provider aliased it).
	ModelUsed string
	// InputTokens is the number of prompt tokens consumed.
	InputTokens int
	// OutputTokens is the number of completion tokens generated.
	OutputTokens int
}

// ChatChunk is a single delta emitted by Provider.ChatStream.
type ChatChunk struct {
	// Delta is the incremental text for this chunk.
	Delta string
	// Done signals that the stream has ended.  When true, Delta may be empty.
	Done bool
	// ModelUsed carries the model identifier on the final chunk (Done == true).
	// It is empty on intermediate chunks.
	ModelUsed string
}

// ChatStream is an iterator over streaming response chunks.
//
// Callers must call Close when they are done consuming the stream, whether or
// not all chunks have been read.
//
// @MX:ANCHOR: [AUTO] ChatStream — V2 streaming iterator contract
// @MX:REASON: All 6 provider adapters implement this interface; changes break all callers
type ChatStream interface {
	// Next returns the next ChatChunk.  It returns (chunk, nil) for each
	// incremental piece, and (chunk{Done:true}, nil) when the stream ends
	// normally.  After the final Done chunk, subsequent calls return
	// ErrStreamClosed.
	Next() (ChatChunk, error)
	// Close releases any resources held by the stream (e.g. HTTP response
	// body).  Safe to call multiple times.
	Close() error
}

// Capabilities describes the static feature set of a provider.
type Capabilities struct {
	// SupportsStream indicates whether the provider can return a ChatStream.
	SupportsStream bool
	// SupportsVision indicates whether the provider accepts image content in
	// Message.Content (format TBD in M2+).
	SupportsVision bool
	// MaxContextTokens is the maximum number of tokens (input + output)
	// supported in a single request.  Zero means unknown.
	MaxContextTokens int
	// KnownModels lists the model identifiers that this provider recognises.
	// Empty slice means the provider accepts any model string (e.g. custom).
	KnownModels []string
}

// Provider is the single interface that all V2 LLM adapters must implement.
//
// M1 stubs return ErrNotImplemented for Chat, ChatStream, and HealthCheck.
// Full implementations land in M2+.
//
// @MX:ANCHOR: [AUTO] Provider V2 interface — single contract for 5 curated + custom adapters
// @MX:REASON: anthropic/deepseek/openai/codex/zai/custom all implement this; interface change ripples to all 6 packages
type Provider interface {
	// Name returns the canonical provider name (e.g. "anthropic", "deepseek").
	Name() string
	// Capabilities returns the static feature set of this provider.
	Capabilities() Capabilities
	// Chat performs a blocking, non-streaming LLM call and returns the full
	// response.
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	// ChatStream begins a streaming LLM call and returns an iterator over
	// response chunks.  The caller must call ChatStream.Close when done.
	ChatStream(ctx context.Context, req ChatRequest) (ChatStream, error)
	// HealthCheck performs a lightweight probe to verify that the provider is
	// reachable and the credentials are valid.
	HealthCheck(ctx context.Context) error
}
