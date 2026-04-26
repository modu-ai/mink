# Contributing to GOOSE

Thank you for considering a contribution. GOOSE is a long-horizon project — every commit should help GOOSE hatch a little stronger.

This guide is intentionally short. The deeper development methodology lives in [`.moai/specs/`](.moai/specs/) and in agent / skill definitions under `.claude/`.

---

## Quick Reference

| You want to... | Do this |
|----------------|---------|
| Ask a question | [Open a Discussion](https://github.com/modu-ai/goose/discussions) |
| Report a defect | [Open a bug report](https://github.com/modu-ai/goose/issues/new?template=bug_report.yml) |
| Propose a capability | [Open a feature request](https://github.com/modu-ai/goose/issues/new?template=feature_request.yml) |
| Fix a typo or doc nit | Open a small PR directly to `main` |
| Implement a SPEC item | Read [Implementation flow](#implementation-flow) below |
| Report a security issue | [Private advisory](https://github.com/modu-ai/goose/security/advisories/new) — do **not** open a public issue |

---

## Development Setup

```bash
git clone https://github.com/modu-ai/goose.git
cd goose
go mod download
go build ./...
go test ./...
```

Pre-commit hooks are wired to enforce `gofmt`, `go vet`, brand-lint, and conventional-commit message format. Do not bypass them with `--no-verify`.

---

## Implementation Flow

GOOSE follows a SPEC-First methodology. Substantial features go through three phases:

1. **Plan** — write or update `SPEC-GOOSE-XXX-NNN/spec.md` with EARS-format requirements and acceptance criteria. The plan-auditor agent reviews independently before implementation begins.
2. **Run** — implement to satisfy the SPEC, with TDD (RED → GREEN → REFACTOR) for greenfield code or DDD (ANALYZE → PRESERVE → IMPROVE) for legacy zones. Coverage target: 85%+ at the sub-package level.
3. **Sync** — update `.moai/project/codemaps/`, README, and CHANGELOG; create the PR.

Smaller changes (typos, single-file refactors, doc edits) can skip the SPEC phase and go directly to a PR.

---

## Pull Request Conventions

- **Branch name**: `feature/SPEC-GOOSE-XXX-short-description` for SPEC work, `fix/short-description` for bug fixes, `chore/short-description` for governance.
- **Title**: [Conventional Commits](https://www.conventionalcommits.org/) format, under 70 characters. Example: `feat(credential): SPEC-GOOSE-CREDPOOL-001 v0.3.0 — Zero-Knowledge Pool`.
- **Body**: Use the project [PR template](.github/PULL_REQUEST_TEMPLATE.md) — Korean prose for the body, Conventional English for the title.
- **Trailer**: Include `🗿 MoAI <email@mo.ai.kr>` at the bottom of commits.
- **Squash merge** for `feature/*` and `fix/*`; **merge commit** only for `release/*`.
- **Branch deletion**: PRs are merged with `--delete-branch`.

A pull request is ready when:

- [ ] CI is green (Go build/vet/gofmt/test, Brand Lint, Release Drafter)
- [ ] CodeRabbit review has been addressed (or scope-out documented in the PR body)
- [ ] Coverage of touched sub-packages is at or above 85%
- [ ] Relevant `@MX:NOTE / WARN / ANCHOR / TODO` annotations are placed
- [ ] Documentation is updated (`.moai/project/codemaps/`, README, ROADMAP if scope changes)

---

## Quality Gates (TRUST 5)

Every change must satisfy:

- **T**ested — coverage, characterization tests for legacy code
- **R**eadable — clear naming, English comments (per `language.yaml`)
- **U**nified — formatter-clean, no style drift
- **S**ecured — input validated at boundaries, defense-in-depth respected
- **T**rackable — Conventional Commit, SPEC reference, issue reference

---

## License of Contributions

By contributing, you agree that your contributions are licensed under the [Apache License 2.0](LICENSE). The Apache 2.0 patent grant means: when you submit a contribution, you grant the project (and downstream users) a non-exclusive, royalty-free patent license for any patent claims your contribution would otherwise infringe.

We do not require a separate Contributor License Agreement (CLA); the Apache 2.0 license terms apply automatically.

---

## Community

- 💬 [Discussions](https://github.com/modu-ai/goose/discussions) — questions, ideas, RFCs
- 📜 [Code of Conduct](CODE_OF_CONDUCT.md) — community standards
- 🛡️ [Security Policy](SECURITY.md) — coordinated disclosure
- 📋 [Project Roadmap](.moai/specs/ROADMAP.md) — what we are building, in what order

---

## Reviewer Etiquette

If you are reviewing a PR:

- Lead with what works before what needs change
- Anchor critique to a specific line / SPEC reference / test case
- Distinguish "blocker" from "suggestion" so the author can prioritize
- Approve when the change satisfies the SPEC and TRUST 5, even if you would have written it differently

---

Last updated: 2026-04-27
