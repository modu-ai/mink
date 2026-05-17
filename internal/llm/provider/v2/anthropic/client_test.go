// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package anthropic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/modu-ai/mink/internal/llm/provider/v2/anthropic"
	"github.com/modu-ai/mink/internal/llm/provider/v2/iface"
)

// Compile-time assertion: Client must satisfy iface.Provider.
var _ iface.Provider = (*anthropic.Client)(nil)

func TestNew_HappyPath(t *testing.T) {
	t.Parallel()
	c, err := anthropic.New("sk-ant-test-key", anthropic.ClientOptions{})
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if c == nil {
		t.Fatal("New() returned nil client")
	}
}

func TestNew_EmptyKey(t *testing.T) {
	t.Parallel()
	_, err := anthropic.New("", anthropic.ClientOptions{})
	if !errors.Is(err, iface.ErrAPIKey) {
		t.Fatalf("New(empty key) error = %v, want ErrAPIKey", err)
	}
}

func TestNew_CustomBaseURL(t *testing.T) {
	t.Parallel()
	c, err := anthropic.New("sk-ant-test-key", anthropic.ClientOptions{BaseURL: "http://localhost:8080"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

func TestClient_Name(t *testing.T) {
	t.Parallel()
	c, _ := anthropic.New("sk-ant-test-key", anthropic.ClientOptions{})
	if c.Name() != "anthropic" {
		t.Errorf("Name() = %q, want %q", c.Name(), "anthropic")
	}
}

func TestClient_Capabilities(t *testing.T) {
	t.Parallel()
	c, _ := anthropic.New("sk-ant-test-key", anthropic.ClientOptions{})
	cap := c.Capabilities()
	if !cap.SupportsStream {
		t.Error("SupportsStream should be true")
	}
	if !cap.SupportsVision {
		t.Error("SupportsVision should be true")
	}
	if cap.MaxContextTokens != 1_000_000 {
		t.Errorf("MaxContextTokens = %d, want 1000000", cap.MaxContextTokens)
	}

	wantModels := []string{"claude-opus-4-7", "claude-sonnet-4-6", "claude-haiku-4-5"}
	if len(cap.KnownModels) != len(wantModels) {
		t.Fatalf("KnownModels len = %d, want %d", len(cap.KnownModels), len(wantModels))
	}
	for i, m := range wantModels {
		if cap.KnownModels[i] != m {
			t.Errorf("KnownModels[%d] = %q, want %q", i, cap.KnownModels[i], m)
		}
	}
}

func TestClient_Chat_NotImplemented(t *testing.T) {
	t.Parallel()
	c, _ := anthropic.New("sk-ant-test-key", anthropic.ClientOptions{})
	_, err := c.Chat(context.Background(), iface.ChatRequest{
		Model:    "claude-opus-4-7",
		Messages: []iface.Message{{Role: iface.RoleUser, Content: "hello"}},
	})
	if !errors.Is(err, iface.ErrNotImplemented) {
		t.Errorf("Chat() error = %v, want ErrNotImplemented", err)
	}
}

func TestClient_ChatStream_NotImplemented(t *testing.T) {
	t.Parallel()
	c, _ := anthropic.New("sk-ant-test-key", anthropic.ClientOptions{})
	_, err := c.ChatStream(context.Background(), iface.ChatRequest{
		Model:    "claude-sonnet-4-6",
		Messages: []iface.Message{{Role: iface.RoleUser, Content: "hi"}},
	})
	if !errors.Is(err, iface.ErrNotImplemented) {
		t.Errorf("ChatStream() error = %v, want ErrNotImplemented", err)
	}
}

func TestClient_HealthCheck_NotImplemented(t *testing.T) {
	t.Parallel()
	c, _ := anthropic.New("sk-ant-test-key", anthropic.ClientOptions{})
	err := c.HealthCheck(context.Background())
	if !errors.Is(err, iface.ErrNotImplemented) {
		t.Errorf("HealthCheck() error = %v, want ErrNotImplemented", err)
	}
}
