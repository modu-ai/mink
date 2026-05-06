package common

import (
	"context"
	"time"

	"github.com/modu-ai/goose/internal/audit"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/permission"
)

// RobotsCheckerIface abstracts common.RobotsChecker for testability.
type RobotsCheckerIface interface {
	IsAllowed(baseURL, path, userAgent string) (bool, error)
	IsAllowedExempt(baseURL, path, userAgent string) (bool, error)
}

// AuditWriter is satisfied by audit.DualWriter and audit.FileWriter.
// Using an interface makes the web package testable without real disk writes.
type AuditWriter interface {
	// Write records an audit event.
	Write(event audit.AuditEvent) error
}

// Deps is the dependency-injection container for all web tools.
// Construct one instance at bootstrap and share it across all web tool instances.
//
// @MX:ANCHOR: [AUTO] Dependency injection root for web tool package
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 — every web tool receives *Deps; fan_in >= 8
type Deps struct {
	// PermMgr is the permission manager used to check CapNet grants.
	// Must have Register("agent:goose", ...) called before any tool Call().
	PermMgr *permission.Manager

	// AuditWriter receives one audit.AuditEvent per web tool invocation.
	// nil means no audit logging (development/test only).
	AuditWriter AuditWriter

	// RateTracker tracks provider-specific rate limit state.
	// BraveParser must be registered on it before first web_search call.
	RateTracker *ratelimit.Tracker

	// Clock returns the current time. Inject a controllable clock in tests.
	// Production code uses time.Now.
	Clock func() time.Time

	// Cwd is the working directory used to derive the bbolt cache path.
	// Typically ~/.goose/cache/web/.
	Cwd string

	// SubjectIDProvider returns the permission subject ID for the calling context.
	// Defaults to "agent:goose" when nil.
	SubjectIDProvider func(ctx context.Context) string

	// Blocklist is the pre-permission host blocklist.
	// When nil, no hosts are blocked (only use nil in tests that don't need blocking).
	Blocklist *Blocklist

	// RobotsChecker performs robots.txt enforcement.
	// When nil, robots.txt checks are skipped (only use nil in unit tests).
	RobotsChecker RobotsCheckerIface
}

// SubjectID returns the subject ID for ctx, using the provider if set,
// or falling back to "agent:goose".
func (d *Deps) SubjectID(ctx context.Context) string {
	if d.SubjectIDProvider != nil {
		return d.SubjectIDProvider(ctx)
	}
	return "agent:goose"
}

// Now returns the current time using the injected Clock, or time.Now().
func (d *Deps) Now() time.Time {
	if d.Clock != nil {
		return d.Clock()
	}
	return time.Now()
}
