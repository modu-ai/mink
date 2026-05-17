// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package custom_test

import (
	"context"
	"errors"
	"testing"

	"github.com/modu-ai/mink/internal/llm/provider/v2/custom"
	"github.com/modu-ai/mink/internal/llm/provider/v2/iface"
)

var _ iface.Provider = (*custom.Client)(nil)

func TestNew_HappyPath(t *testing.T) {
	t.Parallel()
	c, err := custom.New("", custom.ClientOptions{BaseURL: "http://localhost:11434"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_WithAPIKey(t *testing.T) {
	t.Parallel()
	c, err := custom.New("some-key", custom.ClientOptions{BaseURL: "https://openrouter.ai/api/v1"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_EmptyBaseURL(t *testing.T) {
	t.Parallel()
	_, err := custom.New("", custom.ClientOptions{BaseURL: ""})
	if !errors.Is(err, iface.ErrInvalidRequest) {
		t.Fatalf("New(empty BaseURL) error = %v, want ErrInvalidRequest", err)
	}
}

func TestClient_Name(t *testing.T) {
	t.Parallel()
	c, _ := custom.New("", custom.ClientOptions{BaseURL: "http://localhost:8080"})
	if c.Name() != "custom" {
		t.Errorf("Name() = %q, want custom", c.Name())
	}
}

func TestClient_Capabilities(t *testing.T) {
	t.Parallel()
	c, _ := custom.New("", custom.ClientOptions{BaseURL: "http://localhost:8080"})
	cap := c.Capabilities()
	if !cap.SupportsStream {
		t.Error("SupportsStream should be true")
	}
	// Custom adapter must have empty KnownModels.
	if len(cap.KnownModels) != 0 {
		t.Errorf("KnownModels should be empty, got %v", cap.KnownModels)
	}
	// MaxContextTokens should be 0 (unknown).
	if cap.MaxContextTokens != 0 {
		t.Errorf("MaxContextTokens = %d, want 0 (unknown)", cap.MaxContextTokens)
	}
}

func TestClient_Chat_NotImplemented(t *testing.T) {
	t.Parallel()
	c, _ := custom.New("", custom.ClientOptions{BaseURL: "http://localhost:8080"})
	_, err := c.Chat(context.Background(), iface.ChatRequest{})
	if !errors.Is(err, iface.ErrNotImplemented) {
		t.Errorf("Chat() error = %v, want ErrNotImplemented", err)
	}
}

func TestClient_ChatStream_NotImplemented(t *testing.T) {
	t.Parallel()
	c, _ := custom.New("", custom.ClientOptions{BaseURL: "http://localhost:8080"})
	_, err := c.ChatStream(context.Background(), iface.ChatRequest{})
	if !errors.Is(err, iface.ErrNotImplemented) {
		t.Errorf("ChatStream() error = %v, want ErrNotImplemented", err)
	}
}

func TestClient_HealthCheck_NotImplemented(t *testing.T) {
	t.Parallel()
	c, _ := custom.New("", custom.ClientOptions{BaseURL: "http://localhost:8080"})
	err := c.HealthCheck(context.Background())
	if !errors.Is(err, iface.ErrNotImplemented) {
		t.Errorf("HealthCheck() error = %v, want ErrNotImplemented", err)
	}
}
