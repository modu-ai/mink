// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-005
// AC: AC-BR-005
// M0-T5 — verifyLoopbackBind enforces loopback-only HTTP listener bind
// per spec.md §6.4 item 1.

package bridge

import (
	"errors"
	"fmt"
	"net"
	"strconv"
)

// ErrNonLoopbackBind is returned when the requested bind host is not one of
// the allowed loopback aliases (127.0.0.1, ::1, localhost).
//
// @MX:ANCHOR
// @MX:REASON Security invariant — Bridge MUST refuse to listen on any
// non-loopback interface to satisfy REQ-BR-005 (§3.1 item 2).
var ErrNonLoopbackBind = errors.New("bridge: non-loopback bind rejected")

// loopbackHosts is the closed set of acceptable bind hosts.
// IPv6 loopback "::1" is matched after net.SplitHostPort strips the surrounding brackets.
var loopbackHosts = map[string]struct{}{
	"127.0.0.1": {},
	"::1":       {},
	"localhost": {},
}

// verifyLoopbackBind validates that addr resolves to a loopback host with a
// usable TCP port (1..65535). Returns ErrNonLoopbackBind for non-loopback hosts.
// Returns a wrapped formatting error for malformed addresses or invalid ports.
func verifyLoopbackBind(addr string) error {
	if addr == "" {
		return fmt.Errorf("bridge: empty bind address")
	}

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("bridge: invalid bind address %q: %w", addr, err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("bridge: invalid bind port %q: %w", portStr, err)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("bridge: bind port %d out of range (1..65535)", port)
	}

	if _, ok := loopbackHosts[host]; !ok {
		return fmt.Errorf("%w: host %q in %q (allowed: 127.0.0.1, ::1, localhost)",
			ErrNonLoopbackBind, host, addr)
	}

	return nil
}
