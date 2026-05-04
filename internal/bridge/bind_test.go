// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-005
// AC: AC-BR-005
// M0-T5 — verifyLoopbackBind covers spec.md §6.4 item 1.

package bridge

import (
	"errors"
	"strings"
	"testing"
)

func TestVerifyLoopbackBind_AcceptsLoopbackHosts(t *testing.T) {
	t.Parallel()

	cases := []string{
		"127.0.0.1:8091",
		"[::1]:8091",
		"localhost:8091",
		"127.0.0.1:1",
		"localhost:65535",
	}
	for _, addr := range cases {
		t.Run(addr, func(t *testing.T) {
			t.Parallel()
			if err := verifyLoopbackBind(addr); err != nil {
				t.Fatalf("verifyLoopbackBind(%q) = %v, want nil", addr, err)
			}
		})
	}
}

func TestVerifyLoopbackBind_RejectsNonLoopback(t *testing.T) {
	t.Parallel()

	cases := []string{
		"0.0.0.0:8091",
		"192.168.1.10:8091",
		"10.0.0.1:8091",
		"203.0.113.5:8091",
		"[2001:db8::1]:8091",
	}
	for _, addr := range cases {
		t.Run(addr, func(t *testing.T) {
			t.Parallel()
			err := verifyLoopbackBind(addr)
			if !errors.Is(err, ErrNonLoopbackBind) {
				t.Fatalf("verifyLoopbackBind(%q) = %v, want ErrNonLoopbackBind", addr, err)
			}
		})
	}
}

func TestVerifyLoopbackBind_InvalidAddressForm(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		addr string
	}{
		{"missing_port", "127.0.0.1"},
		{"empty_string", ""},
		{"port_only", ":8091"}, // host empty after split → not a loopback alias
		{"non_numeric_port", "127.0.0.1:abc"},
		{"port_zero", "127.0.0.1:0"},
		{"port_too_high", "127.0.0.1:70000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := verifyLoopbackBind(tc.addr)
			if err == nil {
				t.Fatalf("verifyLoopbackBind(%q) = nil, want error", tc.addr)
			}
		})
	}
}

func TestVerifyLoopbackBind_ErrorMessageMentionsAddress(t *testing.T) {
	t.Parallel()

	addr := "0.0.0.0:8091"
	err := verifyLoopbackBind(addr)
	if err == nil {
		t.Fatalf("verifyLoopbackBind(%q) = nil, want error", addr)
	}
	if !strings.Contains(err.Error(), "0.0.0.0") {
		t.Fatalf("error %q should mention bind address %q", err, addr)
	}
}
