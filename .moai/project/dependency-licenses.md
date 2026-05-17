# MINK Dependency License Inventory

This file tracks third-party Go modules added to `go.mod` that require explicit
license review for AGPL-3.0 compatibility (ADR-002).

All entries below are AGPL-3.0 compatible permissive licenses.

Last updated: 2026-05-17

---

## Auth / Credential subsystem (SPEC-MINK-AUTH-CREDENTIAL-001)

| Module | Version | License | Use |
|--------|---------|---------|-----|
| `github.com/zalando/go-keyring` | v0.2.x (v0.2.8) | MIT | OS keyring backend — macOS Keychain / Linux Secret Service / Windows Credential Manager |
| `github.com/godbus/godbus/v5` | (transitive, Linux only) | BSD-2-Clause | Linux Secret Service D-Bus client (pulled in by go-keyring on linux builds) |
| `golang.org/x/oauth2` | (latest, reserved) | BSD-3-Clause | Reserved for M3 T-010 — Codex OAuth 2.1 + PKCE flow. **Not yet imported** into go.mod. |

> `golang.org/x/oauth2` is listed here for planning purposes only.
> It will be added to `go.mod` in M3 T-010.

---

## License Compatibility Summary

| License | AGPL-3.0 compatible? | Notes |
|---------|----------------------|-------|
| MIT | Yes | Permissive, no copyleft |
| BSD-2-Clause | Yes | Permissive, no copyleft |
| BSD-3-Clause | Yes | Permissive, no copyleft |
