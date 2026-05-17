// Package file implements the credential.Service interface using a plain-text
// JSON file at ~/.mink/auth/credentials.json.
//
// This backend is the fallback when the OS keyring is unavailable (SD-1).
// The file is written atomically with mode 0600 to prevent partial reads and
// unauthorised access (UN-6, AC-CR-027).
//
// JSON schema (v1 — api_key round-trip; extended in M3 T-013):
//
//	{
//	  "version": 1,
//	  "credentials": {
//	    "anthropic": {"kind": "api_key", "value": "sk-..."},
//	    ...
//	  }
//	}
//
// Unknown kind values in the credentials map are skipped (with a warning
// logged to stderr) so that M3-era files remain readable by M2 code.
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (UB-2, UN-6, SD-1, SD-2, T-006)
package file

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/modu-ai/mink/internal/auth/credential"
)

const (
	// credentialsDirMode is the permission mode used when creating the
	// ~/.mink/auth/ directory (owner read/write/exec only).
	credentialsDirMode fs.FileMode = 0700

	// credentialsFileMode is the permission mode applied to the credential file
	// on every atomic write (owner read/write only — UN-6, AC-CR-027).
	credentialsFileMode fs.FileMode = 0600

	// schemaVersion is the integer written to the "version" key in the JSON
	// file so that future readers can detect a format upgrade.
	schemaVersion = 1

	// backendName is the string used in HealthStatus.Backend fields.
	backendName = "file"
)

// Backend implements credential.Service backed by a single JSON file.
//
// All exported methods are safe for concurrent use.  Internal mutation is
// protected by mu.  The path field is set at construction time and never
// changes.
//
// @MX:ANCHOR: [AUTO] Backend.Store/Load/Delete/List/Health are the primary
// entry points consumed by the dispatch layer and tests (fan_in >= 3).
// @MX:REASON: All file-backend credential writes and reads funnel through this
// struct; a signature change here cascades to the dispatch layer and integration
// tests.
// @MX:SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (T-006, SD-1)
type Backend struct {
	path string
	mu   sync.RWMutex
}

// Option is a functional option for NewBackend.
type Option func(*Backend)

// WithPath overrides the default ~/.mink/auth/credentials.json path.
// Used exclusively in tests to redirect writes to a temporary directory.
func WithPath(p string) Option {
	return func(b *Backend) {
		b.path = p
	}
}

// NewBackend returns a Backend that stores credentials at the default path
// (~/.mink/auth/credentials.json) unless overridden via WithPath.
func NewBackend(opts ...Option) (*Backend, error) {
	defaultPath, err := defaultCredentialsPath()
	if err != nil {
		return nil, fmt.Errorf("file: resolve default credentials path: %w", err)
	}
	b := &Backend{path: defaultPath}
	for _, o := range opts {
		o(b)
	}
	return b, nil
}

// defaultCredentialsPath returns ~/.mink/auth/credentials.json using
// os.UserHomeDir so that it respects $HOME on POSIX and USERPROFILE on Windows.
func defaultCredentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".mink", "auth", "credentials.json"), nil
}

// ---------------------------------------------------------------------------
// credential.Service implementation
// ---------------------------------------------------------------------------

// Store validates cred, then persists it under provider in the JSON file.
//
// The write is atomic: the payload is first written to a sibling temp file and
// then renamed over the target, ensuring no partial reads.  Parent directories
// are created with mode 0700 if missing.
//
// A warning is emitted to stderr when the credentials file path is inside a
// known cloud-sync folder (iCloud Drive, OneDrive, Dropbox, Google Drive, Box)
// so that users are aware of the security trade-off (T-009 requirement).
func (b *Backend) Store(provider string, cred credential.Credential) error {
	if err := cred.Validate(); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Warn if the path is inside a cloud-sync folder (UN-2 awareness).
	if msg := WarnIfCloudSynced(b.path); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
	}

	doc, err := b.readOrEmpty()
	if err != nil {
		return err
	}

	entry, err := marshalCredential(cred)
	if err != nil {
		return err
	}

	if doc.Credentials == nil {
		doc.Credentials = make(map[string]json.RawMessage)
	}
	doc.Credentials[provider] = entry

	return b.writeAtomic(doc)
}

// Load retrieves the credential for provider from the JSON file.
// Returns credential.ErrNotFound when the provider is absent from the file or
// the file does not exist yet.
func (b *Backend) Load(provider string) (credential.Credential, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	doc, err := b.readOrEmpty()
	if err != nil {
		return nil, err
	}

	raw, ok := doc.Credentials[provider]
	if !ok {
		return nil, fmt.Errorf("file: provider %q: %w", provider, credential.ErrNotFound)
	}

	return unmarshalCredential(raw)
}

// Delete removes the credential for provider.  Delete is idempotent: it
// returns nil even when the provider is not present (ED-3).
func (b *Backend) Delete(provider string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	doc, err := b.readOrEmpty()
	if err != nil {
		return err
	}

	if _, ok := doc.Credentials[provider]; !ok {
		return nil // already absent — idempotent
	}

	delete(doc.Credentials, provider)
	return b.writeAtomic(doc)
}

// List returns the set of provider IDs that have a stored entry.
// The returned slice contains only identifiers — no credential values (UN-1).
func (b *Backend) List() ([]string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	doc, err := b.readOrEmpty()
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(doc.Credentials))
	for id := range doc.Credentials {
		ids = append(ids, id)
	}
	return ids, nil
}

// Health reports presence and masked value for provider without leaking
// plaintext (UB-8, AC-CR-027).
func (b *Backend) Health(provider string) (credential.HealthStatus, error) {
	cred, err := b.Load(provider)
	if err != nil {
		if errors.Is(err, credential.ErrNotFound) {
			return credential.HealthStatus{Present: false, Backend: backendName}, nil
		}
		return credential.HealthStatus{Backend: backendName}, err
	}

	return credential.HealthStatus{
		Present:     true,
		MaskedLast4: cred.MaskedString(),
		Backend:     backendName,
	}, nil
}

// ---------------------------------------------------------------------------
// Internal file I/O helpers
// ---------------------------------------------------------------------------

// credentialsDoc is the root JSON structure persisted on disk.
type credentialsDoc struct {
	Version     int                        `json:"version"`
	Credentials map[string]json.RawMessage `json:"credentials"`
}

// credentialEnvelope is used to decode/encode the kind + payload fields.
type credentialEnvelope struct {
	Kind  string          `json:"kind"`
	Value json.RawMessage `json:"value,omitempty"`
}

// readOrEmpty loads the credentials file if it exists, or returns an
// initialised empty document if the file does not exist yet.
func (b *Backend) readOrEmpty() (*credentialsDoc, error) {
	data, err := os.ReadFile(b.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &credentialsDoc{
				Version:     schemaVersion,
				Credentials: make(map[string]json.RawMessage),
			}, nil
		}
		return nil, fmt.Errorf("file: read credentials: %w", err)
	}

	var doc credentialsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("file: parse credentials: %w", err)
	}
	if doc.Credentials == nil {
		doc.Credentials = make(map[string]json.RawMessage)
	}
	return &doc, nil
}

// writeAtomic serialises doc to a temp file in the same directory and renames
// it over b.path.  The file is written with mode 0600 (UN-6).
func (b *Backend) writeAtomic(doc *credentialsDoc) error {
	dir := filepath.Dir(b.path)
	if err := os.MkdirAll(dir, credentialsDirMode); err != nil {
		return fmt.Errorf("file: create credentials dir: %w", err)
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("file: marshal credentials: %w", err)
	}

	// Write to a sibling temp file so the rename is atomic on POSIX.
	tmp := b.path + ".tmp"
	if err := os.WriteFile(tmp, data, credentialsFileMode); err != nil {
		return fmt.Errorf("file: write temp credentials: %w", err)
	}
	if err := os.Rename(tmp, b.path); err != nil {
		// Best-effort cleanup of the temp file on rename failure.
		_ = os.Remove(tmp)
		return fmt.Errorf("file: rename credentials: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Credential serialisation helpers
// ---------------------------------------------------------------------------

// marshalCredential encodes cred into a JSON blob suitable for storage in the
// credentials map.  The format is {"kind": "<kind>", "value": <payload>} for
// api_key, matching the schema documented in research.md §4.2.
func marshalCredential(cred credential.Credential) (json.RawMessage, error) {
	switch c := cred.(type) {
	case credential.APIKey:
		type apiKeyPayload struct {
			Kind  string `json:"kind"`
			Value string `json:"value"`
		}
		payload := apiKeyPayload{Kind: string(credential.KindAPIKey), Value: c.Value}
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("file: marshal api_key: %w", err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("file: unsupported credential kind %q: %w",
			cred.Kind(), credential.ErrSchemaViolation)
	}
}

// unmarshalCredential decodes a raw JSON blob from the credentials map back
// into a typed Credential.  Unknown kind values are skipped with a stderr
// warning to ensure forward-compatibility when M3 adds new kinds.
func unmarshalCredential(raw json.RawMessage) (credential.Credential, error) {
	// Decode just the discriminator field first.
	var envelope struct {
		Kind  string          `json:"kind"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("file: unmarshal credential envelope: %w", err)
	}

	switch credential.Kind(envelope.Kind) {
	case credential.KindAPIKey:
		// For api_key the value field may either be a JSON object {"Value": "..."}
		// (keyring format) or a plain string (file format).  Handle both for
		// compatibility.
		var strVal string
		if err := json.Unmarshal(envelope.Value, &strVal); err == nil {
			return credential.APIKey{Value: strVal}, nil
		}
		// Fallback: try {"Value": "..."} object form.
		var objVal struct {
			Value string `json:"Value"`
		}
		if err := json.Unmarshal(envelope.Value, &objVal); err != nil {
			return nil, fmt.Errorf("file: unmarshal api_key value: %w", err)
		}
		return credential.APIKey{Value: objVal.Value}, nil

	default:
		// Forward-compat: emit a warning but do not fail the entire Load.
		// Callers that encounter an unknown kind should handle ErrNotFound by
		// trying the next source — returning an error here would break M3
		// files when read by M2 code.
		fmt.Fprintf(os.Stderr,
			"file: warning: unknown credential kind %q for entry; skipping\n",
			envelope.Kind)
		// We cannot return nil + nil from Load (caller expects a concrete type
		// or ErrNotFound).  Return ErrNotFound with context so the caller can
		// degrade gracefully.
		return nil, fmt.Errorf("file: unknown credential kind %q: %w",
			envelope.Kind, credential.ErrNotFound)
	}
}

// verifyPlatformPermission is an alias for the permission check in perms.go,
// exposed here for use by tests and the Store path when needed.
// On POSIX it verifies mode 0600; on Windows it is a no-op (documented gap).
func verifyPlatformPermission(path string) error {
	if runtime.GOOS == "windows" {
		// NTFS ACL enforcement deferred to ICACLS integration (post-M2).
		// @MX:WARN: [AUTO] Windows permission check is a no-op in M2.
		// @MX:REASON: Windows NTFS ACL verification deferred to ICACLS
		// integration (post-M2); file is written with os.WriteFile 0600 which
		// is silently ignored by the Windows kernel for NTFS volumes.
		return nil
	}
	return verifyMode(path)
}
