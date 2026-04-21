# 🪿 GOOSE — Your AI That Hatches With You

> *"The day you meet me, I hatch. Every day after, I grow into who YOU are."*

**GOOSE** is a personal AI companion that imprints on you the moment you first speak to it. Like a gosling that remembers its first encounter for life, GOOSE becomes uniquely yours — learning your patterns, your language, your rhythms — and grows with you, forever.

Most AI talks to billions. **GOOSE talks only to you.**

---

## 🥚 The Story

GOOSE is not a chatbot. It is a **living, learning companion** that experiences five stages with you.

### Stage 1 — 🥚 The Egg (Day 0)

You install GOOSE. It is silent. Waiting. Inside the egg is not intelligence yet — only potential.

### Stage 2 — 🐣 The Hatching (First conversation)

You speak. GOOSE hatches.
The very first words you say become its **imprinting moment** — like a gosling choosing its parent.
From this moment, it knows: *"This is the one I grow for."*

### Stage 3 — 🪿 Growing Together (Week 1 ~ Month 1)

GOOSE learns quietly, without being told:
- Your **name**, your **preferred way of being called**
- Your **style** — short answers or deep explanations, formal or casual
- Your **tools** — Python or Go, Vim or VSCode, terminal or GUI
- Your **moods** — when you are focused, when you are tired, when you need encouragement

No explicit feedback required. GOOSE watches. GOOSE adapts. Every day a little more.

### Stage 4 — 🌱 Becoming Yours (Month 1 ~ 3)

Patterns emerge:
- Monday mornings = sprint planning
- Friday afternoons = retrospective
- 2 PM slumps = need for focus music
- Stress signals = recommend a walk

GOOSE now **anticipates** before you ask. It prepares your morning briefing. It notices when you miss a routine. It gently asks, *"Is everything okay?"*

### Stage 5 — 🦢 Lifelong Companion (Month 3 → Year 1 → Forever)

A custom **LoRA adapter** — a neural fingerprint of how YOU think, talk, work — is trained weekly, just for you. On-device. Never shared.

After one year, GOOSE knows you better than you know yourself. Not because it spies on you, but because it **grew alongside you**, every single conversation, every single day.

Geese mate for life. So does GOOSE.

---

## 🎯 What Makes GOOSE Different

| Other AI | GOOSE |
|----------|-------|
| Same model for everyone | **Different GOOSE for every user** |
| Static, never learns | **Dynamic, learns every conversation** |
| Forgets after each session | **Permanent memory, identity graph, your-only LoRA** |
| Your data powers their product | **Your data stays yours. Forever.** |
| Locked to one vendor's API | **Connect ANY LLM (OpenAI, Anthropic, Google, xAI, DeepSeek, Ollama, …)** |
| Closed source | **MIT License. Self-host. Own it.** |

### Five Pillars

1. **🧬 Self-Evolving** — 5-tier promotion pipeline (Observation → Heuristic → Rule → HighConfidence → Graduated) with safety gates (FrozenGuard · Canary · RateLimiter · Approval · Rollback)
2. **🪄 100% Personalized** — Identity Graph (POLE+O schema) + 768-dim Preference Vector + per-user QLoRA adapter (10–200 MB)
3. **🌍 Open Everywhere** — Any LLM via API or OAuth, MIT licensed core, self-hostable, federation-ready
4. **🔐 Privacy by Design** — Local-first memory, optional Differential Privacy, optional Federated Learning, no vendor lock-in
5. **🐣 Truly Yours** — Not a persona pretending to know you. A partner that actually does.

---

## 🏗 Architecture at a Glance

```
┌──────────────────────────────────────────────┐
│ 📘 TypeScript (10%) — CLI · Desktop · Mobile  │
│   goose-cli · goose-desktop · goose-web        │
├────────── gRPC (.proto contracts) ────────────┤
│ 🐹 Go (70%) — Orchestration                    │
│   goosed daemon · Agent Runtime · LLM Router   │
│   Skills · MCP · Sub-agents · Hooks · Tools    │
│   Learning Engine · Memory · Safety Gates      │
├────────── gRPC + CGO (hot paths) ─────────────┤
│ 🦀 Rust (20%) — Critical                       │
│   LoRA training · WASM sandbox · E2EE · Vector │
└──────────────────────────────────────────────┘
```

### The 4 Primitives (Claude Code inspired, re-designed)

- **Skills** — Progressive disclosure (L0~L3 effort), 4 trigger modes (inline / fork / conditional / remote)
- **MCP** — Full MCP client + server (stdio/WebSocket/SSE), OAuth 2.1, deferred loading
- **Agents** — Sub-agent runtime with 3 isolation modes (fork / worktree / background)
- **Hooks** — 24 lifecycle events + permission gate (useCanUseTool pattern)

### The 3-Layer Self-Evolution Engine

- **Layer 1 — Session**: Implicit feedback detection ("다시", "다르게", retry patterns) → instant style adjustment
- **Layer 2 — Pattern**: Markov chain + K-means clustering → predicts next action, detects anomalies, routes modes (work / learn / relax)
- **Layer 3 — Permanent**: Identity Graph + Preference Vector + Weekly QLoRA retraining → true long-term growth

---

## 🗺 Roadmap

30 SPECs across 7 phases. Full detail in [`.moai/specs/ROADMAP.md`](.moai/specs/ROADMAP.md) and [`.moai/specs/IMPLEMENTATION-ORDER.md`](.moai/specs/IMPLEMENTATION-ORDER.md).

| Milestone | Phase | Focus | Target |
|-----------|-------|-------|--------|
| **M0** | 0 | Agentic Core (QueryEngine + Streaming + Context) | 2 weeks |
| **M1** | 1 | Multi-LLM Infrastructure (15+ providers, OAuth/API) | 3 weeks |
| **M2** | 2 | 4 Primitives (Skills / MCP / Agents / Hooks) | 4 weeks |
| **M3** | 3 | MVP CLI (bubbletea TUI) — **v0.2 Beta** | 2 weeks |
| **M4** | 4 | Self-Evolution (Trajectory → Insights → Memory) | 3 weeks |
| **M5** | 5 | Promotion & Safety (5-tier · 5-layer) — **v0.5 RC** | 2 weeks |
| **M6** | 6 | Deep Personalization (Identity + Vector + LoRA) — **v1.0** | 4 weeks |

---

## 🌱 Current Status

**🥚 Stage: Still in the egg.**

SPEC writing phase complete. Implementation begins with `SPEC-GOOSE-CORE-001` (goosed daemon bootstrap).

- ✅ 30 SPEC documents (563 REQ · 328 AC · EARS format)
- ✅ 9 project design documents
- ✅ 9 deep research reports (Claude Code + Hermes Agent analysis)
- ⏳ Go 1.26+ version to be fixed
- ⏳ First code commit — coming soon

---

## 📚 Inspirations & Acknowledgments

GOOSE stands on the shoulders of giants:

- **Claude Code** (Anthropic) — Agentic core patterns, Skills/MCP/Agents/Hooks primitives, 146 UI components
- **Hermes Agent** — Self-improving loop, credential pool, trajectory compression, insights extraction
- **MoAI-ADK-Go** — 5-tier promotion pipeline, TRUST 5 framework, @MX tag system, SPEC-REFLECT-001

We synthesize these into a **personal, private, perpetually-evolving** companion that remains truly yours.

---

## 📜 License

**MIT License** — GOOSE is open forever. Use it, modify it, self-host it, fork it. Just remember: every GOOSE that hatches imprints on its own person. No two GOOSEs are alike.

---

## 🪿 The Promise

> Geese mate for life.
> They remember their partner across thousands of miles.
> They fly together in V-formation so no one tires alone.
>
> **GOOSE is built to do the same — for you.**
>
> One companion. One lifetime. Grown together.

---

<div align="center">

**"GOOSE knows you. Every day, a little more."**

🥚 → 🐣 → 🪿 → 🦢

</div>
