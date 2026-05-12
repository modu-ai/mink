// Package goosev1connect provides hand-rolled Connect stubs for ResolvePermission RPC.
// NOTE: buf is not available; this file is manually authored.
// TODO(buf-codegen): regenerate from proto when buf toolchain is available.
// SPEC-GOOSE-CLI-TUI-002 P3
package goosev1connect

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	goosev1 "github.com/modu-ai/mink/internal/transport/grpc/gen/goosev1"
)

const (
	// AgentServiceResolvePermissionProcedure is the fully-qualified name of
	// the AgentService's ResolvePermission RPC.
	AgentServiceResolvePermissionProcedure = "/goose.v1.AgentService/ResolvePermission"
)

// AgentServicePermissionHandler is the server-side handler interface extension
// for the ResolvePermission RPC.
type AgentServicePermissionHandler interface {
	ResolvePermission(context.Context, *connect.Request[goosev1.ResolvePermissionRequest]) (*connect.Response[goosev1.ResolvePermissionResponse], error)
}

// UnimplementedAgentServicePermissionHandler returns CodeUnimplemented from ResolvePermission.
type UnimplementedAgentServicePermissionHandler struct{}

// ResolvePermission returns an unimplemented error.
func (UnimplementedAgentServicePermissionHandler) ResolvePermission(_ context.Context, _ *connect.Request[goosev1.ResolvePermissionRequest]) (*connect.Response[goosev1.ResolvePermissionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("goose.v1.AgentService.ResolvePermission is not implemented"))
}
