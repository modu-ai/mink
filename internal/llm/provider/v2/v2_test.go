// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package v2_test

import (
	"errors"
	"testing"

	v2 "github.com/modu-ai/mink/internal/llm/provider/v2"
)

func TestNewByName_AllCuratedProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      v2.ClientConfig
		wantName string
	}{
		{
			name:     "anthropic",
			cfg:      v2.ClientConfig{APIKey: "sk-ant-test"},
			wantName: "anthropic",
		},
		{
			name:     "deepseek",
			cfg:      v2.ClientConfig{APIKey: "sk-deepseek-test"},
			wantName: "deepseek",
		},
		{
			name:     "openai",
			cfg:      v2.ClientConfig{APIKey: "sk-openai-test"},
			wantName: "openai",
		},
		{
			name:     "codex",
			cfg:      v2.ClientConfig{APIKey: "access-token-test"},
			wantName: "codex",
		},
		{
			name:     "zai",
			cfg:      v2.ClientConfig{APIKey: "glm-key-test"},
			wantName: "zai",
		},
		{
			name:     "custom",
			cfg:      v2.ClientConfig{APIKey: "", BaseURL: "http://localhost:11434"},
			wantName: "custom",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, err := v2.NewByName(tc.name, tc.cfg)
			if err != nil {
				t.Fatalf("NewByName(%q) error = %v, want nil", tc.name, err)
			}
			if p == nil {
				t.Fatalf("NewByName(%q) returned nil provider", tc.name)
			}
			if p.Name() != tc.wantName {
				t.Errorf("Provider.Name() = %q, want %q", p.Name(), tc.wantName)
			}
		})
	}
}

func TestNewByName_UnknownProvider(t *testing.T) {
	t.Parallel()

	unknowns := []string{"groq", "gemini", "ollama", "mistral", "", "ANTHROPIC"}
	for _, name := range unknowns {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := v2.NewByName(name, v2.ClientConfig{APIKey: "any"})
			if !errors.Is(err, v2.ErrModelNotFound) {
				t.Errorf("NewByName(%q) error = %v, want ErrModelNotFound", name, err)
			}
		})
	}
}

func TestNewByName_CustomWithBaseURL(t *testing.T) {
	t.Parallel()
	p, err := v2.NewByName("custom", v2.ClientConfig{
		APIKey:  "sk-openrouter",
		BaseURL: "https://openrouter.ai/api/v1",
	})
	if err != nil {
		t.Fatalf("NewByName(custom) error = %v", err)
	}
	if p.Name() != "custom" {
		t.Errorf("Name() = %q, want custom", p.Name())
	}
}

func TestNewByName_CustomWithoutBaseURL(t *testing.T) {
	t.Parallel()
	// custom without BaseURL must fail with ErrInvalidRequest (not ErrModelNotFound).
	_, err := v2.NewByName("custom", v2.ClientConfig{APIKey: "key", BaseURL: ""})
	if err == nil {
		t.Fatal("NewByName(custom, empty BaseURL) should return error, got nil")
	}
	if errors.Is(err, v2.ErrModelNotFound) {
		t.Error("NewByName(custom, empty BaseURL) should not return ErrModelNotFound")
	}
}
