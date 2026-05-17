// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package v2

import "github.com/modu-ai/mink/internal/llm/provider/v2/iface"

// Sentinel error variables re-exported from the iface package.
//
// All errors are intended to be matched with errors.Is so that callers can
// react to specific failure modes without inspecting error text.

// ErrNotImplemented is returned by stub methods that will be filled in M2+.
var ErrNotImplemented = iface.ErrNotImplemented

// ErrInvalidRequest is returned when the caller supplies a malformed request.
var ErrInvalidRequest = iface.ErrInvalidRequest

// ErrAPIKey is returned when the API key is missing or invalid.
var ErrAPIKey = iface.ErrAPIKey

// ErrRateLimited is returned when the upstream provider rate-limits the request.
var ErrRateLimited = iface.ErrRateLimited

// ErrModelNotFound is returned when the model or provider name is not found.
var ErrModelNotFound = iface.ErrModelNotFound

// ErrStreamClosed is returned when ChatStream.Next is called after the stream
// is closed.
var ErrStreamClosed = iface.ErrStreamClosed
