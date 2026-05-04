// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-003
// AC: AC-BR-003
// M2-T1 — single HTTP listener with path-based dispatching.

package bridge

import (
	"net/http"
)

// MuxConfig collects every dependency BuildMux needs to wire the bridge
// endpoints together.
type MuxConfig struct {
	Auth       *Authenticator
	Registry   *Registry
	Revocation *RevocationStore
	Adapter    QueryEngineAdapter // optional; nil falls back to noopAdapter

	// PermissionRequester routes InboundPermissionResponse messages to
	// blocked Bridge.RequestPermission callers. Optional: when nil,
	// permission_response inbounds are forwarded to Adapter like any
	// other type.
	permRequester *permissionRequester

	// RateLimiter throttles per-cookie reconnect attempts (M5, REQ-BR-018).
	// Optional: nil disables rate-limit enforcement.
	RateLimiter *rateLimiter

	// Metrics records inbound/outbound/reconnect counters and exposes the
	// observable session/stall gauges (M5, REQ-BR-004). Optional: nil
	// skips metric emission.
	Metrics *bridgeMetrics

	// WSAcceptOrigins controls coder/websocket's Origin header check.
	// Empty defaults to {"127.0.0.1:*", "localhost:*", "[::1]:*"}.
	WSAcceptOrigins []string
}

// BuildMux returns the http.Handler that routes every Bridge endpoint:
//
//	POST /bridge/login    → LoginHandler
//	POST /bridge/logout   → LogoutHandler
//	GET  /bridge/ws       → WebSocket upgrade
//	GET  /bridge/stream   → SSE handler
//	POST /bridge/inbound  → SSE-fallback inbound
//
// All handlers reuse AuthRequest from M1; transport-specific rejection
// (close 4401 vs HTTP 401) is the per-handler responsibility.
//
// @MX:ANCHOR
// @MX:REASON Single entry point for every browser-facing route.
func BuildMux(cfg MuxConfig) http.Handler {
	if cfg.Adapter == nil {
		cfg.Adapter = noopAdapter{}
	}
	if len(cfg.WSAcceptOrigins) == 0 {
		cfg.WSAcceptOrigins = defaultLoopbackOriginPatterns()
	}

	mux := http.NewServeMux()
	mux.Handle("POST /bridge/login", NewLoginHandler(cfg.Auth))
	mux.Handle("POST /bridge/logout", NewLogoutHandler(cfg.Auth, cfg.Registry, cfg.Revocation))
	mux.Handle("GET /bridge/ws", NewWebSocketHandler(cfg))
	mux.Handle("GET /bridge/stream", NewSSEHandler(cfg))
	mux.Handle("POST /bridge/inbound", NewInboundPostHandler(cfg))
	return mux
}

// defaultLoopbackOriginPatterns mirrors hostIsLoopback: only loopback aliases
// are permitted as WebSocket Origin values.
func defaultLoopbackOriginPatterns() []string {
	return []string{
		"127.0.0.1:*",
		"localhost:*",
		"[::1]:*",
	}
}
