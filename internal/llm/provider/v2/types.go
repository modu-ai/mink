// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package v2

import "github.com/modu-ai/mink/internal/llm/provider/v2/iface"

// Type aliases re-export the canonical types from the iface package so that
// consumers can use "v2.Provider", "v2.ChatRequest" etc. without importing
// the internal iface package directly.

// Provider is the single interface that all V2 LLM adapters must implement.
type Provider = iface.Provider

// Capabilities describes the static feature set of a provider.
type Capabilities = iface.Capabilities

// ChatRequest is the unified input to Provider.Chat and Provider.ChatStream.
type ChatRequest = iface.ChatRequest

// ChatResponse is the complete, non-streaming response from Provider.Chat.
type ChatResponse = iface.ChatResponse

// ChatChunk is a single delta emitted by Provider.ChatStream.
type ChatChunk = iface.ChatChunk

// ChatStream is an iterator over streaming response chunks.
type ChatStream = iface.ChatStream

// Message is a single turn in a conversation.
type Message = iface.Message

// Role represents the sender role of a conversation message.
type Role = iface.Role

// Role constants.
const (
	RoleSystem    = iface.RoleSystem
	RoleUser      = iface.RoleUser
	RoleAssistant = iface.RoleAssistant
)
