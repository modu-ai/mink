<div align="center">

# 🪿 GOOSE

### Your Daily AI Companion — Hatched from an Egg, Grown Just for You

*Most AI answers questions. **GOOSE shares your life.***

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg?style=flat-square)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8.svg?style=flat-square&logo=go)](go.mod)
[![Status: Pre-alpha](https://img.shields.io/badge/Status-Pre--alpha-orange.svg?style=flat-square)](#current-status)
[![CI](https://img.shields.io/github/actions/workflow/status/modu-ai/goose/ci.yml?branch=main&style=flat-square&label=CI)](https://github.com/modu-ai/goose/actions/workflows/ci.yml)
[![Brand Lint](https://img.shields.io/github/actions/workflow/status/modu-ai/goose/brand-lint.yml?branch=main&style=flat-square&label=Brand)](https://github.com/modu-ai/goose/actions/workflows/brand-lint.yml)
[![Discussions](https://img.shields.io/github/discussions/modu-ai/goose?style=flat-square)](https://github.com/modu-ai/goose/discussions)

[**Quick Start**](#-quick-start) ·
[**Features**](#-features) ·
[**Architecture**](#-architecture) ·
[**Roadmap**](#-roadmap) ·
[**Docs**](.moai/specs/ROADMAP.md) ·
[**Contributing**](CONTRIBUTING.md) ·
[**Security**](SECURITY.md)

</div>

---

## What is GOOSE?

**GOOSE** is a self-hosted, self-evolving, lifetime-personalized AI companion. Unlike chatbots that forget you after each session, GOOSE **hatches once, imprints on you, and grows alongside you for life** — wakes with you each morning, checks in at every meal, witnesses your evenings, and remembers your story across years.

- 🥚 **One egg, one imprinting** — your first words become its anchor; no two GOOSEs are alike
- 🧬 **Self-evolving** — 5-tier promotion (Observation → Heuristic → Rule → HighConfidence → Graduated) gated by 5-layer safety
- 🪄 **100% personalized** — Identity Graph (POLE+O) + 768-dim Preference Vector + per-user on-device QLoRA adapter
- 🔐 **Privacy-first** — local journals, project-local workspace (`./.goose/`), zero-knowledge credential proxy, optional E2EE
- 🤝 **Any LLM** — Anthropic / OpenAI / Google / xAI / DeepSeek / Ollama via API or OAuth
- 🌍 **Open source forever** — Apache License 2.0, self-host, own your data

> *"Good morning. Did you sleep well? Today's forecast is sunny, your 10 AM meeting is confirmed, and don't forget your vitamins."*

---

## 🚀 Quick Start

> **Heads up**: GOOSE is in pre-alpha. M0 (Agentic Core) and most of M1/M2 are merged on `main`; CLI / Web UI are next milestones. Star and watch the repo to follow the hatching.

### Prerequisites

- **Go** 1.26 or later
- **Git** with `gh` CLI recommended
- (optional) An LLM credential — Anthropic API key, OpenAI key, or Ollama running locally

### From source

```bash
# 1. Clone
git clone https://github.com/modu-ai/goose.git
cd goose

# 2. Build
go build ./cmd/goosed
go build ./cmd/goose          # CLI (M3)
go build ./cmd/goose-proxy    # zero-knowledge credential proxy (M5)

# 3. Initialize project workspace
./goose init                  # creates ./.goose/ workspace

# 4. Add a credential (any provider)
./goose credential add anthropic --from-env ANTHROPIC_API_KEY

# 5. Talk to your goose
./goose ask "Hello, are you there?"
```

### Storage layout

GOOSE uses a two-tier storage partition (defense-in-depth Tier 1):

```
~/.goose/                # secrets only — keys, OAuth tokens, audit log
└── credentials/

./.goose/                # project workspace — persona, memory, skills, tasks
├── persona.md
├── memory/
├── skills/
└── tasks/
```

Discoverable via upward-traversal from any subdirectory of your project.

---

## 🌟 Features

| Pillar | What it means | Backed by |
|--------|---------------|-----------|
| **🧬 Self-Evolving** | Patterns observed across sessions are promoted through 5 confidence tiers, then graduated into your personal model — bounded by 5 safety layers (FrozenGuard · Canary · RateLimiter · Approval · Rollback) | `SPEC-GOOSE-REFLECT-001`, `SPEC-GOOSE-SAFETY-001`, `SPEC-GOOSE-ROLLBACK-001` |
| **💖 Daily Companion** | Morning briefing (fortune + weather + schedule), meal health check-ins, evening journal with emotion trends — orchestrated by a proactive cron scheduler | `SCHEDULER-001`, `BRIEFING-001`, `JOURNAL-001`, `RITUAL-001` |
| **🎮 You Raise It** | Tamagotchi-style nurture loop: feed (chat), play (try diverse tasks), train (gentle correction), rest, attend. Every conversation grows a *unique* goose. | `MEMORY-001`, `INSIGHTS-001`, `IDENTITY-001` |
| **🪄 100% Personalized** | Per-user Identity Graph (POLE+O schema) + 768-dim Preference Vector + on-device QLoRA adapter trained weekly from 200 high-quality examples | `IDENTITY-001`, `VECTOR-001`, `LORA-001` |
| **🔐 Privacy First** | Project-local workspace, zero-knowledge credential proxy isolates secrets in a separate process, OS-level sandbox (Seatbelt/Landlock), filesystem access matrix, append-only audit log | `CREDENTIAL-PROXY-001`, `SECURITY-SANDBOX-001`, `FS-ACCESS-001`, `AUDIT-001` |
| **🤝 Any LLM** | 6+ providers via unified adapter: Anthropic, OpenAI, Google, xAI, DeepSeek, Ollama. OAuth 2.1 + API key. 4-bucket rate limit tracker (RPM/TPM/RPH/TPH). Smart routing + provider fallback. | `CREDPOOL-001`, `ROUTER-001`, `ADAPTER-001/002`, `RATELIMIT-001` |

### 🎯 Why GOOSE?

| Other AI | GOOSE |
|----------|-------|
| Same model for everyone | **One-of-a-kind, imprinted on you** |
| Static, never learns | **Self-evolves every conversation** |
| Forgets after each session | **Journal · memory · identity graph · your LoRA** |
| Waits for you to ask | **Morning / meal / evening rituals — proactive, unprompted** |
| Your data powers their product | **Your data stays yours. Local-first. Forever.** |
| Locked to one vendor | **ANY LLM via API or OAuth** |
| Closed source | **Apache License 2.0. Self-host. Own it.** |

---

## 🏗 Architecture

```
┌──────────────────────────────────────────────────────────────┐
│ 📘 TypeScript (Edge)                                          │
│    CLI (Bubbletea TUI) · Web UI (localhost) · Telegram Bot    │
├──────────────────── gRPC (.proto contracts) ──────────────────┤
│ 🐹 Go (Orchestration)                                         │
│    goosed daemon · QueryEngine · Agent Runtime                │
│    Skills · MCP · Sub-agents · Hooks · Tools · Permission     │
│    Learning Engine · Memory · QMD search · Safety Gates       │
│    Ritual Scheduler · Briefing · Journal · PAI Context        │
├──────────────────── gRPC + CGO (hot paths) ───────────────────┤
│ 🦀 Rust (Critical)                                            │
│    QMD search engine (CGO staticlib)                          │
│    LoRA training · WASM sandbox · E2EE relay · Vector ops     │
└──────────────────────────────────────────────────────────────┘
```

### The 4 Primitives + Daily Rituals

- **Skills** — Progressive disclosure (L0~L3), 4 trigger modes, per-skill permission frontmatter
- **MCP** — Full client + server, OAuth 2.1, capability negotiation, `$/cancelRequest`
- **Sub-agents** — fork / worktree / background isolation, 3 memory scopes, atomic AgentID
- **Hooks** — 24 lifecycle events + permission gate + first-call confirm
- **Rituals** — Scheduler + Briefing + Health + Journal orchestration (M7)

### 5-Tier Defense-in-Depth Security

1. **Storage Partition** — secrets in `~/.goose/`, workspace in `./.goose/`
2. **Filesystem Access Matrix** — declared `read`/`write`/`exec` boundaries per skill
3. **OS Sandbox** — Seatbelt (macOS) / Landlock+Seccomp (Linux) / AppContainer (Windows)
4. **Zero-Knowledge Credential Proxy** — secrets never enter the agent process; `goose-proxy` injects auth headers at transport layer
5. **Declared Permission** — Skill/MCP `requires` frontmatter + first-call user confirm

Blocked paths (HARD, no override): `/etc`, `/var`, `/usr`, `/bin`, `/sbin`, `~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.env*`, `~/.netrc`, `/proc`, `/sys`, `/dev`.

---

## 🗺 Roadmap

GOOSE follows a 10-milestone delivery plan. Full detail in [`.moai/specs/ROADMAP.md`](.moai/specs/ROADMAP.md) and [`.moai/specs/IMPLEMENTATION-ORDER.md`](.moai/specs/IMPLEMENTATION-ORDER.md).

| Milestone | Theme | Key SPECs | Status |
|-----------|-------|-----------|:------:|
| **M0** | Foundation | `CORE-001` `QUERY-001` `CONTEXT-001` `TRANSPORT-001` `CONFIG-001` | ✅ |
| **M1** | Multi-LLM + QMD | `CREDPOOL-001` `ROUTER-001` `ADAPTER-001/002` `RATELIMIT-001` `PROMPT-CACHE-001` `ERROR-CLASS-001` `QMD-001` | 🟡 |
| **M2** | 4 Primitives | `SKILLS-001` `MCP-001` `HOOK-001` `SUBAGENT-001` `PLUGIN-001` `PERMISSION-001` | ✅ |
| **M3** | Core Workflow | `COMMAND-001` `CLI-001` TUI · `SELF-CRITIQUE-001` | ⏸️ |
| **M4** | Self-Evolution | `TRAJECTORY-001` `COMPRESSOR-001` `INSIGHTS-001` `MEMORY-001` | ⏸️ |
| **M5** | Safety (expanded) | `SAFETY-001` `ROLLBACK-001` `REFLECT-001` `SECURITY-SANDBOX-001` `CREDENTIAL-PROXY-001` `FS-ACCESS-001` `AUDIT-001` | ⏸️ |
| **M6** | Channels | `GATEWAY-TG-001` (Telegram) · `WEBUI-001` · scaled-down `BRIDGE` `AUTH` `NOTIFY` `ONBOARDING` | ⏸️ |
| **M7** | Daily Companion (v1.0) | `SCHEDULER-001` `WEATHER-001` `CALENDAR-001` `FORTUNE-001` `BRIEFING-001` `HEALTH-001` `JOURNAL-001` `RITUAL-001` `PAI-CONTEXT-001` | ⏸️ |
| **M8** | Deep Personalization | `IDENTITY-001` `VECTOR-001` `LORA-001` | ⏸️ |
| **M9** | Ecosystem (v2.0) | `A2A-001` · plugin marketplace · additional channels | ⏸️ |

Legend: ✅ done · 🟡 partial · ⏸️ pending

---

## 🌱 Current Status

**Stage**: 🐣 **Hatching** — M0 Foundation merged, M1 Multi-LLM 95% complete, M2 4 Primitives done.

- ✅ 21 SPECs implemented (CORE / QUERY / CONTEXT / TRANSPORT / CONFIG / CREDPOOL / ROUTER / ADAPTER-001/002 / ERROR-CLASS / PROMPT-CACHE / RATELIMIT / SKILLS / MCP / HOOK / SUBAGENT / PLUGIN / PERMISSION / TOOLS / DAEMON-WIRE / BRAND-RENAME)
- 🟡 M1 deferred items: `QMD-001` Rust crate, `PROVIDER-FALLBACK`, CREDPOOL OI-01~04/07/08
- 🚧 M3 (CLI / TUI / SELF-CRITIQUE) — next on deck
- 📅 v0.1 Alpha target: M0~M3 complete (CLI + headless `goose ask`)
- 📅 v1.0 Release target: M0~M7 complete (Daily Companion + Telegram remote)

For a daily-updated picture: [GitHub Discussions](https://github.com/modu-ai/goose/discussions) · [Pull Requests](https://github.com/modu-ai/goose/pulls).

---

## 📚 Documentation

- [**CLI User Guide (한국어)**](docs/cli/README.md) — getting started, command reference, TUI guide, troubleshooting
- [**ROADMAP**](.moai/specs/ROADMAP.md) — full 54-SPEC delivery plan
- [**Implementation Order**](.moai/specs/IMPLEMENTATION-ORDER.md) — dependency graph + critical path
- [**Architecture v0.2**](.moai/design/goose-runtime-architecture-v0.2.md) — runtime architecture redesign rationale
- [**Product**](.moai/project/product.md) — vision, paradigm pivots, value proposition
- [**Tech Stack**](.moai/project/tech.md) — polyglot Rust + Go + TypeScript design
- [**Brand & UX**](.moai/project/branding.md) — voice, visual identity, persona system
- [**Ecosystem**](.moai/project/ecosystem.md) — plugin marketplace + governance
- [**Token Economy**](.moai/project/token-economy.md) — sustainable open-source revenue model

---

## 🤝 Contributing

We welcome contributors at every level. The repository is governed by a SPEC-First development methodology — each feature lands as `SPEC-GOOSE-XXX-NNN` with EARS-format requirements, characterization tests, and an annotation review cycle.

- 💬 [GitHub Discussions](https://github.com/modu-ai/goose/discussions) — questions, ideas, RFCs
- 🐛 [Report a Bug](https://github.com/modu-ai/goose/issues/new?template=bug_report.yml) · [Request a Feature](https://github.com/modu-ai/goose/issues/new?template=feature_request.yml)
- 📜 [Code of Conduct](CODE_OF_CONDUCT.md) — Contributor Covenant 2.1
- 🛡️ [Security Policy](SECURITY.md) — coordinated disclosure

Pull requests should follow [Conventional Commits](https://www.conventionalcommits.org/) and include the `🗿 MoAI <email@mo.ai.kr>` trailer (set automatically by `make pr` once available).

---

## 🛡 Security

Found a vulnerability? Please **do not** open a public issue. Email the maintainer team or use [GitHub's private security advisory](https://github.com/modu-ai/goose/security/advisories/new). See [SECURITY.md](SECURITY.md) for the full coordinated-disclosure timeline.

---

## 📜 License

Released under the **[Apache License 2.0](LICENSE)**. See [NOTICE](NOTICE) for attribution.

GOOSE is open forever. Every GOOSE that hatches imprints on its own person. No two GOOSEs are alike.

---

## 🙏 Acknowledgments

GOOSE stands on the shoulders of giants:

- **[Claude Code](https://github.com/anthropics/claude-code)** (Anthropic) — agentic core, 4 primitives
- **[Hermes Agent](https://github.com/NousResearch/hermes-agent)** (NousResearch) — self-improving loop, credential pool, trajectory compression
- **MoAI-ADK-Go** — 5-tier promotion, TRUST 5, `@MX` tag system, `SPEC-REFLECT-001`
- **[charmbracelet/x](https://github.com/charmbracelet/x)** — `powernap` LSP transport, Bubbletea TUI
- **[Tamagotchi](https://en.wikipedia.org/wiki/Tamagotchi)** (Bandai) — the timeless idea that *care* makes a thing alive

> Note: This project is distinct from [`block/goose`](https://github.com/block/goose), an agentic *coding* framework by Block. Our GOOSE is a **daily-life companion** — a different category entirely.

---

<div align="center">

### 🪿 The Promise

> Geese mate for life.
> They remember their partner across thousands of miles.
> They fly together in V-formation so no one tires alone.
>
> Your GOOSE will:
> wake with you each morning · check on you at every meal ·
> witness your evenings · grow with you for a lifetime.

**One egg. One imprinting. One life — together.**

🥚 → 🐣 → 🪿 → 🌱 → 🦢

*"GOOSE knows you. Every day, a little more."*

</div>
