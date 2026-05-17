// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package v2_test

import (
	"context"
	"testing"

	v2 "github.com/modu-ai/mink/internal/llm/provider/v2"
)

// stubProvider is a minimal in-package stub used only for compile-time
// interface verification in this test file.
type stubProvider struct{}

func (s *stubProvider) Name() string                  { return "stub" }
func (s *stubProvider) Capabilities() v2.Capabilities { return v2.Capabilities{} }
func (s *stubProvider) Chat(_ context.Context, _ v2.ChatRequest) (v2.ChatResponse, error) {
	return v2.ChatResponse{}, v2.ErrNotImplemented
}
func (s *stubProvider) ChatStream(_ context.Context, _ v2.ChatRequest) (v2.ChatStream, error) {
	return nil, v2.ErrNotImplemented
}
func (s *stubProvider) HealthCheck(_ context.Context) error { return v2.ErrNotImplemented }

// Compile-time assertion: stubProvider must satisfy the Provider interface.
var _ v2.Provider = (*stubProvider)(nil)

func TestProvider_InterfaceAssert(t *testing.T) {
	t.Parallel()
	// If stubProvider does not satisfy Provider, this file will not compile.
	var p v2.Provider = &stubProvider{}
	if p.Name() != "stub" {
		t.Fatalf("Name() = %q, want %q", p.Name(), "stub")
	}
}

func TestChatRequest_ZeroValue(t *testing.T) {
	t.Parallel()
	var req v2.ChatRequest
	if req.Model != "" {
		t.Error("zero-value ChatRequest.Model should be empty")
	}
	if req.Messages != nil {
		t.Error("zero-value ChatRequest.Messages should be nil")
	}
}

func TestMessage_Roles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		role v2.Role
		want string
	}{
		{v2.RoleSystem, "system"},
		{v2.RoleUser, "user"},
		{v2.RoleAssistant, "assistant"},
	}

	for _, tc := range tests {
		if string(tc.role) != tc.want {
			t.Errorf("Role %v string = %q, want %q", tc.role, string(tc.role), tc.want)
		}
	}
}

func TestCapabilities_ZeroValue(t *testing.T) {
	t.Parallel()
	var cap v2.Capabilities
	if cap.SupportsStream {
		t.Error("zero-value SupportsStream should be false")
	}
	if cap.MaxContextTokens != 0 {
		t.Error("zero-value MaxContextTokens should be 0")
	}
	if len(cap.KnownModels) != 0 {
		t.Error("zero-value KnownModels should be empty")
	}
}
