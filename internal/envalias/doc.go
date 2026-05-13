// Package envalias provides a single entry point for GOOSE_* → MINK_* environment
// variable alias resolution during the MINK brand-rename transition period.
//
// # Overview
//
// Per SPEC-MINK-ENV-MIGRATE-001, every env var read in the MINK binary MUST go
// through this package's API instead of calling os.Getenv directly. The package:
//
//   - Maintains a static 22-key mapping table (GOOSE_X ↔ MINK_X).
//   - Implements MINK_X priority over GOOSE_X (NEW > OLD, Vault/Terraform pattern).
//   - Emits per-key per-process sync.Once deprecation warnings (zap WARN) when a
//     caller receives a value from the legacy GOOSE_X key.
//   - Supports an optional strict mode for detecting unregistered key access.
//
// # REQ coverage
//
//   - REQ-MINK-EM-001: all env var reads centralised through this package.
//   - REQ-MINK-EM-002: 22-key mapping table (21 single-key + GOOSE_AUTH_* prefix
//     handled separately in internal/hook/isolation_*.go).
//   - REQ-MINK-EM-003: MINK_X priority when both keys are set.
//   - REQ-MINK-EM-004: per-key per-process deprecation warning (sync.Once).
//   - REQ-MINK-EM-005: no warning when only MINK_X is set.
//   - REQ-MINK-EM-006: conflict warning (one-time) when both MINK_X and GOOSE_X set.
//   - REQ-MINK-EM-009: strict mode warning for unregistered keys.
//
// # GOOSE_AUTH_* prefix
//
// The GOOSE_AUTH_* / MINK_AUTH_* prefix deny-list used by the env-scrub in
// internal/hook/isolation_*.go is NOT part of the single-key mapping table.
// Use [MinkPrefixes] to obtain the known MINK_* prefix list for deny-list extension.
//
// # Migration path
//
// Phase 1 (this package): alias loader skeleton.
// Phase 2: internal/config.envOverlay adopts loader for 5 keys.
// Phase 3: 11 distributed os.Getenv read sites adopt loader.
// Phase 4: deny-list + 28 t.Setenv test migrations.
// Phase 5: prose / error message / comment cleanup.
// Phase 6: integration tests + final verification.
//
// Full scope: SPEC-MINK-ENV-MIGRATE-001 spec.md §6.
package envalias
