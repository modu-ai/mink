// Package credential — vendor file source helpers.
//
// SPEC-GOOSE-CREDPOOL-001 OI-05.
//
// This file groups the file IO and metadata-mapping helpers shared by the
// three vendor-specific credential sources (Anthropic Claude, OpenAI Codex,
// Nous Hermes). The Zero-Knowledge invariant (REQ-CREDPOOL-014) is enforced
// here: parsed secret material lives only on local stack frames during the
// Load() call and is dropped before the function returns.
package credential

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

// loadVendorFile reads up to 1 MiB from path and returns its raw bytes.
//
// Returns:
//   - (nil, nil)  when the file does not exist (per OI-05 rule 3).
//   - (nil, err)  when ctx is cancelled before/after the read, or any other
//     filesystem error occurs.
//   - (data, nil) otherwise.
//
// Vendor credential files are always small (sub-kilobyte JSON), so reading
// the whole file in one shot keeps the implementation simple while still
// honoring REQ-CREDPOOL-017 (sub-500ms abort on context cancellation).
func loadVendorFile(ctx context.Context, path string) ([]byte, error) {
	// REQ-CREDPOOL-017: respect cancellation before any IO.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f, err := os.Open(path) // #nosec G304 -- path comes from validated config
	if err != nil {
		if os.IsNotExist(err) {
			// OI-05 rule 3: missing vendor file is a soft empty result.
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	// Cap at 1 MiB: vendor files are tiny; this guards against a pathological
	// symlink to /dev/zero or similar.
	const maxVendorBytes = 1 << 20
	data, err := io.ReadAll(io.LimitReader(f, maxVendorBytes))
	if err != nil {
		return nil, err
	}

	// Re-check context after IO so a deadline that elapsed during the read
	// still aborts the load (REQ-CREDPOOL-017).
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

// statusForExpiry classifies an expiry timestamp into the appropriate
// PooledCredential.Status. A zero expiresAt is treated as permanently valid
// per REQ-CREDPOOL-001 (b).
func statusForExpiry(expiresAt time.Time, now time.Time) CredStatus {
	if expiresAt.IsZero() {
		return CredOK
	}
	if !now.Before(expiresAt) {
		return CredExhausted
	}
	return CredOK
}

// wrapVendorParseError annotates a parse failure with the provider tag and
// the source file path. Callers must NOT include any parsed bytes in the
// error chain (Zero-Knowledge: a malformed file may still contain partial
// secrets).
func wrapVendorParseError(provider, path string, err error) error {
	return fmt.Errorf("%s source: parse %s: %w", provider, path, err)
}
