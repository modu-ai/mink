// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package openai_test

import (
	"context"
	"errors"
	"testing"

	"github.com/modu-ai/mink/internal/llm/provider/v2/iface"
	"github.com/modu-ai/mink/internal/llm/provider/v2/openai"
)

var _ iface.Provider = (*openai.Client)(nil)

func TestNew_HappyPath(t *testing.T) {
	t.Parallel()
	c, err := openai.New("sk-openai-test", openai.ClientOptions{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_EmptyKey(t *testing.T) {
	t.Parallel()
	_, err := openai.New("", openai.ClientOptions{})
	if !errors.Is(err, iface.ErrAPIKey) {
		t.Fatalf("New(empty) error = %v, want ErrAPIKey", err)
	}
}

func TestClient_Name(t *testing.T) {
	t.Parallel()
	c, _ := openai.New("sk-openai-test", openai.ClientOptions{})
	if c.Name() != "openai" {
		t.Errorf("Name() = %q, want openai", c.Name())
	}
}

func TestClient_Capabilities(t *testing.T) {
	t.Parallel()
	c, _ := openai.New("sk-openai-test", openai.ClientOptions{})
	cap := c.Capabilities()
	if !cap.SupportsStream {
		t.Error("SupportsStream should be true")
	}
	if !cap.SupportsVision {
		t.Error("SupportsVision should be true for OpenAI")
	}
	want := []string{"gpt-4o", "gpt-4o-mini", "gpt-5"}
	if len(cap.KnownModels) != len(want) {
		t.Fatalf("KnownModels len = %d, want %d", len(cap.KnownModels), len(want))
	}
	for i, m := range want {
		if cap.KnownModels[i] != m {
			t.Errorf("KnownModels[%d] = %q, want %q", i, cap.KnownModels[i], m)
		}
	}
}

func TestClient_Chat_NotImplemented(t *testing.T) {
	t.Parallel()
	c, _ := openai.New("sk-openai-test", openai.ClientOptions{})
	_, err := c.Chat(context.Background(), iface.ChatRequest{})
	if !errors.Is(err, iface.ErrNotImplemented) {
		t.Errorf("Chat() error = %v, want ErrNotImplemented", err)
	}
}

func TestClient_ChatStream_NotImplemented(t *testing.T) {
	t.Parallel()
	c, _ := openai.New("sk-openai-test", openai.ClientOptions{})
	_, err := c.ChatStream(context.Background(), iface.ChatRequest{})
	if !errors.Is(err, iface.ErrNotImplemented) {
		t.Errorf("ChatStream() error = %v, want ErrNotImplemented", err)
	}
}

func TestClient_HealthCheck_NotImplemented(t *testing.T) {
	t.Parallel()
	c, _ := openai.New("sk-openai-test", openai.ClientOptions{})
	err := c.HealthCheck(context.Background())
	if !errors.Is(err, iface.ErrNotImplemented) {
		t.Errorf("HealthCheck() error = %v, want ErrNotImplemented", err)
	}
}
