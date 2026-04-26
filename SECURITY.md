# Security Policy

GOOSE is a daily AI companion that handles deeply personal data — journals, health check-ins, identity graphs, on-device LoRA adapters, and credentials for multiple LLM providers. Security is treated as a constitutional concern, not a checklist.

This policy describes how we accept vulnerability reports, the timeline you can expect, and the scope of our security commitment.

---

## 🛡️ Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Instead, report privately through one of:

1. **GitHub Security Advisory (preferred)** — [Open a private advisory](https://github.com/modu-ai/goose/security/advisories/new). This creates a private collaboration space between you and the maintainers.
2. **Email** — `security@mo.ai.kr` (encrypt with our PGP key when available).

When reporting, please include:

- A clear description of the issue (impact + reproduction steps)
- Affected version / commit SHA / branch
- Proof-of-concept code or commands (if any)
- Your assessment of severity (CVSS optional)
- Whether you would like public credit after disclosure

We thank you in advance for **coordinated disclosure** — please give us the response window below before publishing.

---

## ⏱️ Response Timeline

| Phase | Target | What happens |
|-------|--------|--------------|
| **Acknowledgment** | 48 hours | Maintainer responds confirming receipt |
| **Triage** | 7 days | Severity assessment + initial fix plan |
| **Fix delivery** | 30 days (critical), 90 days (high/medium) | Patch merged + release published |
| **Public disclosure** | 7 days after patched release | CVE assigned + advisory published |

For active exploits in the wild, we may compress this timeline and coordinate emergency releases.

---

## 🎯 Scope

### In Scope

- All code in `cmd/`, `internal/`, `pkg/`, `crates/` of this repository
- Default configuration files in `.moai/config/`
- The `goosed` daemon, `goose` CLI, `goose-proxy` credential proxy
- gRPC `.proto` contracts and transport layer
- Skills / MCP servers / hooks distributed as part of this repo
- Storage layout (`~/.goose/`, `./.goose/`) and the FS access matrix
- Default brand-lint, CI workflows, and `.github/` automation

### Out of Scope

- Third-party LLM providers (Anthropic, OpenAI, Google, etc.) — please report directly to those vendors
- User-installed plugins or skills not bundled with the official release
- Local privilege escalation requiring physical access to an unlocked workstation
- Self-XSS or social-engineering attacks
- Issues already addressed in `main` or in an open PR
- DoS issues that require resource exhaustion of >100x normal load on a self-hosted instance
- Vulnerabilities in dependencies that have not yet released a fix (please coordinate with upstream first)

---

## 🔐 Defense Architecture (Quick Reference)

GOOSE implements 5-tier defense-in-depth (see `SPEC-GOOSE-ARCH-REDESIGN-v0.2`):

1. **Storage Partition** — secrets in `~/.goose/`, workspace in `./.goose/`
2. **Filesystem Access Matrix** — declared `read`/`write`/`exec` boundaries (`SPEC-GOOSE-FS-ACCESS-001`)
3. **OS Sandbox** — Seatbelt / Landlock+Seccomp / AppContainer (`SPEC-GOOSE-SECURITY-SANDBOX-001`)
4. **Zero-Knowledge Credential Proxy** — secrets isolated in `goose-proxy` process (`SPEC-GOOSE-CREDENTIAL-PROXY-001`)
5. **Declared Permission** — Skill / MCP `requires` frontmatter + first-call confirm (`SPEC-GOOSE-PERMISSION-001`)

Hard-blocked paths (cannot be overridden by user config):
`/etc`, `/var`, `/usr`, `/bin`, `/sbin`, `~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.env*`, `~/.netrc`, `/proc`, `/sys`, `/dev`.

All security-sensitive operations are recorded in an append-only audit log (`SPEC-GOOSE-AUDIT-001`).

---

## 🔍 Reproducible Builds

Releases ship with checksums and (planned) Sigstore signatures. The credential proxy and OAuth flow are independently reproducible from source. For full provenance, see `.github/workflows/release.yml` and the SLSA attestation bundled with each release artifact (M5+).

---

## 🤝 Recognition

We maintain a public [Security Hall of Fame](https://github.com/modu-ai/goose/security/advisories) listing researchers who have reported coordinated vulnerabilities (with their permission). At present we do not run a paid bug bounty program, but contributions are credited and may be eligible for swag once the program launches.

---

## 📜 References

- [`SPEC-GOOSE-ARCH-REDESIGN-v0.2`](.moai/specs/SPEC-GOOSE-ARCH-REDESIGN-v0.2/spec.md) — runtime architecture and 5-tier defense model
- [`SPEC-GOOSE-PERMISSION-001`](.moai/specs/SPEC-GOOSE-PERMISSION-001/) — declared permission system
- [Apache License 2.0](LICENSE) — license terms
- [Code of Conduct](CODE_OF_CONDUCT.md) — community expectations

---

Last updated: 2026-04-27
