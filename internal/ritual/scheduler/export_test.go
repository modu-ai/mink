// Package scheduler — test-only exports for white-box testing.
// This file is compiled only during `go test`.
package scheduler

// WithCronSpecOverride is the test-exported version of withCronSpecOverride.
// It replaces all ritual cron specs with spec, enabling fast-firing tests
// without waiting for wall-clock HH:MM triggers.
var WithCronSpecOverride = withCronSpecOverride
