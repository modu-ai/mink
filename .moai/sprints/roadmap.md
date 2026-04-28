# AI-GOOSE Sprint Roadmap

> **프로젝트**: GOOSE -- Daily Companion AI (self-hosted, project-local workspace)
> **작성일**: 2026-04-28
> **근거**: IMPLEMENTATION-ORDER v6.2 + v0.2 Amendment + SPEC frontmatter 의존성 분석
> **개발 방법론**: TDD (`development_mode: tdd`)
> **참조**: `.moai/specs/IMPLEMENTATION-ORDER.md`, `.moai/specs/ROADMAP.md`

---

## Overview

| 항목 | 수치 |
|------|------|
| 구현 완료 | 27 SPEC (622 REQ / 468 AC) |
| 잔여 Planned | 44 SPEC |
| 잔여 Draft | 6 SPEC |
| 총 잔여 | 50 SPEC |
| 스프린트 수 | 8 (Sprint 0--7) |
| 스프린트당 규모 | 4--9 SPEC |
| Critical Path | ARCH-REDESIGN -> CLI -> BRIDGE -> DESKTOP + TRAJECTORY -> COMPRESSOR + CREDENTIAL-PROXY -> AUTH |

---

## Dependency Graph

```
                         ┌─────────────────────┐
                         │  ARCH-REDESIGN-v0.2  │ (draft, blocks 12+ SPECs)
                         │  QMD-001 (draft)     │
                         └──────────┬──────────┘
                                    │
            ┌───────────────────────┼────────────────────────┐
            │                       │                        │
     ┌──────▼──────┐      ┌────────▼────────┐      ┌───────▼────────┐
     │  Phase 0-1  │      │    Phase 3-4    │      │    Phase 5     │
     │ AGENT-001   │      │ CLI-001         │      │ SAFETY-001     │
     │ LLM-001     │      │ SELF-CRITIQUE   │      │ AUDIT-001      │
     │             │      │ TRAJECTORY-001  │      │ FS-ACCESS-001  │
     └─────────────┘      │ COMPRESSOR-001  │      │ CREDENTIAL-PROX│
                          │ MEMORY-001      │      │ SECURITY-SANDBOX│
                          │ INSIGHTS-001    │      │ ROLLBACK-001   │
                          └───────┬─────────┘      └───────┬────────┘
                                  │                        │
                          ┌───────▼────────────────────────▼─────┐
                          │            Phase 6                   │
                          │  BRIDGE-001 ──> DESKTOP-001          │
                          │  BRIDGE-001 ──> GATEWAY-001          │
                          │  BRIDGE-001 ──> RELAY-001            │
                          │  CREDENTIAL-PROXY ──> AUTH-001       │
                          │  WEBUI-001, LOCALE-001, I18N-001     │
                          │  ONBOARDING-001, NOTIFY-001          │
                          └───────────────┬──────────────────────┘
                                          │
                          ┌───────────────▼──────────────────────┐
                          │            Phase 7                   │
                          │  SCHEDULER-001 (draft)               │
                          │  RITUAL-001 ──> BRIEFING-001 (draft) │
                          │  JOURNAL, CALENDAR, WEATHER          │
                          │  FORTUNE, HEALTH, PAI-CONTEXT        │
                          └───────────────┬──────────────────────┘
                                          │
                          ┌───────────────▼──────────────────────┐
                          │       Phase 8-9 (Advanced)           │
                          │  REFLECT ──> IDENTITY ──> VECTOR     │
                          │  LORA, SIGNING, GATEWAY-TG, A2A     │
                          └──────────────────────────────────────┘
```

### Critical Path

```
ARCH-REDESIGN-v0.2 --+-- CLI-001 -- CMDCTX-CLI-INTEG -- CMDCTX-DAEMON-INTEG
                      |
                      +-- TRAJECTORY-001 -- COMPRESSOR-001
                      |
                      +-- SAFETY-001 -- REFLECT-001 -- IDENTITY-001
                      |
                      +-- CREDENTIAL-PROXY-001 -- AUTH-001
                      |
                      +-- BRIDGE-001 --+-- DESKTOP-001 -- SIGNING-001
                      |               +-- GATEWAY-001 -- GATEWAY-TG-001
                      |               +-- RELAY-001
                      |
                      +-- RITUAL-001 -- BRIEFING-001
                                         SCHEDULER-001
```

---

## Sprint 0: Foundation & Architecture

**Goal**: 아키텍처 재설계 확정 + 기반 인터페이스 구현 + QMD 메모리 기반 확보

| SPEC | 제목 | 상태 | 우선순위 | 규모 | 의존성 |
|------|------|------|---------|------|--------|
| SPEC-GOOSE-ARCH-REDESIGN-v0.2 | Architecture Redesign v0.2 | draft | P0 | 대(L) | 없음 (12+ SPEC 차단) |
| SPEC-GOOSE-QMD-001 | QMD Embedded Hybrid Memory Search | draft | critical | 대(L) | 없음 |
| SPEC-GOOSE-AGENT-001 | Agent Runtime 최소 생애주기 + Persona | planned | P0 | 중(M) | 없음 |
| SPEC-GOOSE-LLM-001 | LLM Provider 인터페이스 + Ollama 어댑터 | planned | P0 | 중(M) | 없음 |
| SPEC-AGENCY-CLEANUP-002 | Legacy Agency 파일 정리 | planned | P2 | 소(S) | AGENCY-ABSORB-001 (완료) |

**병렬 가능**: AGENT-001 || LLM-001 || AGENCY-CLEANUP-002 (QMD, ARCH는 선행 필요)
**완료 기준**: ARCH-REDESIGN v0.2 draft -> planned 전환, QMD 핵심 인터페이스 구현, Agent/LLM Provider 기본 동작

---

## Sprint 1: CLI & Context Integration

**Goal**: goose CLI 완성 + ContextAdapter 전 경로 wiring + 명령 체계 확장

| SPEC | 제목 | 상태 | 우선순위 | 규모 | 의존성 |
|------|------|------|---------|------|--------|
| SPEC-GOOSE-CLI-001 | goose CLI (cobra + Connect-gRPC + bubbletea TUI) | planned | P0 | 중(M) | COMMAND-001 (완료) |
| SPEC-GOOSE-CMDCTX-CLI-INTEG-001 | CLI 진입점 ContextAdapter / Dispatcher Wiring | planned | P1 | 중(M) | CLI-001, CMDCTX-001 (완료) |
| SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 | Daemon 진입점 ContextAdapter Wiring | planned | P2 | 중(M) | CMDCTX-CLI-INTEG-001 |
| SPEC-GOOSE-PLANMODE-CMD-001 | /plan 빌트인 명령 PlanMode 트리거 | planned | P3 | 소(S) | CMDCTX-001 (완료) |
| SPEC-GOOSE-SELF-CRITIQUE-001 | Task Self-Critique (Reflect Phase) | planned | P0 | 중(M) | REFLECT-001 (Sprint 7, 인터페이스만 선행) |

**순차 체인**: CLI-001 -> CLI-INTEG -> DAEMON-INTEG (직렬), PLANMODE-CMD (CLI-INTEG 후), SELF-CRITIQUE (REFLECT 인터페이스에만 의존, 구현은 Sprint 7)
**주의**: SELF-CRITIQUE는 REFLECT-001에 의존하나, 인터페이스만 먼저 정의하고 Sprint 7에서 REFLECT 구현 완료 후 통합

---

## Sprint 2: Self-Evolution Pipeline

**Goal**: Trajectory 수집 -> 압축 -> 인사이트 추출 + 메모리 제공자 + 관측 기반

| SPEC | 제목 | 상태 | 우선순위 | 규모 | 의존성 |
|------|------|------|---------|------|--------|
| SPEC-GOOSE-TRAJECTORY-001 | Trajectory 수집 + 익명화 (ShareGPT JSON-L) | planned | P0 | 소(S) | 없음 |
| SPEC-GOOSE-COMPRESSOR-001 | Trajectory Compressor (Protected Head/Tail + LLM Middle) | planned | P0 | 중(M) | TRAJECTORY-001, CONTEXT-001 (완료) |
| SPEC-GOOSE-MEMORY-001 | Pluggable Memory Provider (Builtin + 외부 1개 Plugin) | planned | P0 | 중(M) | QMD-001 (Sprint 0) |
| SPEC-GOOSE-INSIGHTS-001 | 다차원 Insights 추출 (Pattern/Preference/Error/Opportunity) | planned | P1 | 중(M) | 없음 |
| SPEC-GOOSE-CMDCTX-TELEMETRY-001 | ContextAdapter 호출 카운트 / Latency Metrics | planned | P3 | 중(M) | CMDCTX-001 (완료) |

**순차 체인**: TRAJECTORY-001 -> COMPRESSOR-001 (직렬), MEMORY-001, INSIGHTS-001, TELEMETRY-001 (병렬)
**병렬 가능**: TRAJECTORY || MEMORY || INSIGHTS || TELEMETRY (TRAJECTORY 완료 후 COMPRESSOR 시작)

---

## Sprint 3: Safety & Security

**Goal**: 5-Layer Safety Architecture + Zero-Knowledge Credential Proxy + OS-Level Sandbox + 감사 로그

| SPEC | 제목 | 상태 | 우선순위 | 규모 | 의존성 |
|------|------|------|---------|------|--------|
| SPEC-GOOSE-SAFETY-001 | 5-Layer Safety Architecture | planned | P1 | 중(M) | HOOK-001 (완료) |
| SPEC-GOOSE-AUDIT-001 | Append-Only Audit Log | planned | P0 | 소(S) | ARCH-REDESIGN (Sprint 0) |
| SPEC-GOOSE-FS-ACCESS-001 | Filesystem Access Matrix Engine | planned | P0 | 중(M) | ARCH-REDESIGN (Sprint 0) |
| SPEC-GOOSE-CREDENTIAL-PROXY-001 | Zero-Knowledge Credential Proxy | planned | P0 | 대(L) | ARCH-REDESIGN (Sprint 0) |
| SPEC-GOOSE-SECURITY-SANDBOX-001 | OS-Level Sandbox | planned | P0 | 대(L) | ARCH-REDESIGN (Sprint 0) |
| SPEC-GOOSE-ROLLBACK-001 | Regression 자동 감지 및 30일 Staleness Rollback | planned | P1 | 소(S) | 없음 |

**순차 체인**: SAFETY-001 -> REFLECT-001 (Sprint 7에서 이어짐), CREDENTIAL-PROXY -> AUTH-001 (Sprint 4)
**병렬 가능**: SAFETY || AUDIT || FS-ACCESS || CREDENTIAL-PROXY || SECURITY-SANDBOX || ROLLBACK (대부분 독립)
**주의**: CREDENTIAL-PROXY-001과 SECURITY-SANDBOX-001은 대형(L) SPEC으로 별도 worktree 권장

---

## Sprint 4: Cross-Platform Core + Auth + Localization

**Goal**: Desktop App 기반 + Web UI + 인증 + 국제화 기반 + 온보딩

| SPEC | 제목 | 상태 | 우선순위 | 규모 | 의존성 |
|------|------|------|---------|------|--------|
| SPEC-GOOSE-BRIDGE-001 | Daemon <-> Web UI Local Bridge | planned | P0 | 중(M) | ARCH-REDESIGN, TRANSPORT-001 (완료) |
| SPEC-GOOSE-AUTH-001 | Local Token-Based Authentication | planned | P0 | 중(M) | CREDENTIAL-PROXY-001 (Sprint 3) |
| SPEC-GOOSE-LOCALE-001 | Locale Detection + Cultural Context Injection | planned | P0 | 소(S) | 없음 |
| SPEC-GOOSE-DESKTOP-001 | GOOSE Desktop App (Tauri v2) | planned | critical | 대(L) | BRIDGE-001, CLI-001 (Sprint 1) |
| SPEC-GOOSE-WEBUI-001 | Localhost Web UI (비개발자 대응) | planned | P0 | 대(L) | ARCH-REDESIGN (Sprint 0) |
| SPEC-GOOSE-I18N-001 | UI Internationalization (20+ Languages, Plurals, RTL) | draft | P0 | 중(M) | 없음 |

**순차 체인**: BRIDGE-001 -> DESKTOP-001 (직렬), CREDENTIAL-PROXY(Sprint 3) -> AUTH-001
**병렬 가능**: LOCALE || I18N || WEBUI || AUTH (BRIDGE 선행)
**주의**: DESKTOP-001은 critical + 대형으로 별도 worktree + 장기 실행 권장

---

## Sprint 5: Localization & Channels + Onboarding

**Goal**: 온보딩 마법사 + 지역 Skill Bundle + 메신저 알림 + 게이트웨이 + E2EE 중계

| SPEC | 제목 | 상태 | 우선순위 | 규모 | 의존성 |
|------|------|------|---------|------|--------|
| SPEC-GOOSE-ONBOARDING-001 | CLI + Web UI Install Wizard | draft | critical | 중(M) | ARCH-REDESIGN (Sprint 0) |
| SPEC-GOOSE-NOTIFY-001 | Messenger Gateway Notifications | planned | P0 | 중(M) | ARCH-REDESIGN (Sprint 0) |
| SPEC-GOOSE-REGION-SKILLS-001 | Regional Skill Bundles + Locale-aware Activation | planned | P1 | 중(M) | LOCALE-001 (Sprint 4) |
| SPEC-GOOSE-GATEWAY-001 | Self-hosted Messenger Bridge (umbrella) | planned | P1 | 대(L) | BRIDGE-001 (Sprint 4), MCP-001 (완료) |
| SPEC-GOOSE-RELAY-001 | E2EE 중계 서비스 (Noise Protocol) | planned | P1 | 대(L) | BRIDGE-001 (Sprint 4) |
| SPEC-GOOSE-CMDCTX-HOTRELOAD-001 | ContextAdapter Registry / AliasMap Hot-Reload | planned | P4 | 중(M) | CMDCTX-001 (완료) |
| SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 | Permissive Alias Mode | planned | P4 | 극소(XS) | CMDCTX-001 (완료) |

**순차 체인**: BRIDGE-001(Sprint 4) -> GATEWAY-001 / RELAY-001, LOCALE-001(Sprint 4) -> REGION-SKILLS-001
**병렬 가능**: ONBOARDING || NOTIFY || GATEWAY || RELAY || HOTRELOAD || PERMISSIVE-ALIAS
**주의**: GATEWAY-001과 RELAY-001은 대형(L) + BRIDGE-001 선행 필수

---

## Sprint 6: Daily Companion v1.0

**Goal**: Daily Ritual + Morning Briefing + Evening Journal + Calendar + Weather + Fortune + Health + PAI Context + Scheduler

| SPEC | 제목 | 상태 | 우선순위 | 규모 | 의존성 |
|------|------|------|---------|------|--------|
| SPEC-GOOSE-SCHEDULER-001 | Proactive Ritual Scheduler (Cron-like, Timezone-aware) | draft | critical | 중(M) | HOOK-001 (완료) |
| SPEC-GOOSE-RITUAL-001 | Daily Ritual Orchestrator (Morning + Meals + Evening) | planned | P0 | 대(L) | ARCH-REDESIGN (Sprint 0) |
| SPEC-GOOSE-BRIEFING-001 | Morning Briefing Orchestrator | draft | critical | 중(M) | RITUAL-001 (Sprint 내) |
| SPEC-GOOSE-JOURNAL-001 | Evening Journal + Emotion Tagging + Long-term Memory Recall | planned | P0 | 중(M) | 없음 |
| SPEC-GOOSE-CALENDAR-001 | Calendar Integration (CalDAV + Native APIs) | planned | P0 | 중(M) | 없음 |
| SPEC-GOOSE-WEATHER-001 | Weather Report Tool (Global + Korean, Air Quality) | planned | P1 | 소(S) | 없음 |
| SPEC-GOOSE-FORTUNE-001 | Personalized Fortune Generator (사주/바이오리듬, Opt-in) | planned | P1 | 중(M) | 없음 |
| SPEC-GOOSE-HEALTH-001 | Meal/Medication/Hydration Tracker | planned | P1 | 중(M) | 없음 |
| SPEC-GOOSE-PAI-CONTEXT-001 | PAI Identity Context Files | planned | P0 | 중(M) | ARCH-REDESIGN (Sprint 0) |

**순차 체인**: SCHEDULER -> RITUAL -> BRIEFING (직렬)
**병렬 가능**: JOURNAL || CALENDAR || WEATHER || FORTUNE || HEALTH || PAI-CONTEXT (독립)
**주의**: Sprint 내 SPEC 수가 9개로 최대치. RITUAL-001은 대형이므로 충분한 시간 확보 필요

---

## Sprint 7: Advanced Features & Ecosystem

**Goal**: Reflect 승급 파이프라인 + Identity Graph + Vector Space + QLoRA + Binary Signing + Telegram Gateway + A2A Protocol

| SPEC | 제목 | 상태 | 우선순위 | 규모 | 의존성 |
|------|------|------|---------|------|--------|
| SPEC-GOOSE-REFLECT-001 | 5단계 승격 파이프라인 (Observation -> Graduated) | planned | P1 | 대(L) | SAFETY-001 (Sprint 3), MEMORY-001 (Sprint 2), INSIGHTS-001 (Sprint 2) |
| SPEC-GOOSE-VECTOR-001 | Preference Vector Space (768-dim, EMA update) | planned | P2 | 중(M) | 없음 |
| SPEC-GOOSE-IDENTITY-001 | Identity Graph (POLE+O, Kuzu 임베디드) | planned | P2 | 대(L) | REFLECT-001, TRAJECTORY-001 (Sprint 2), VECTOR-001 |
| SPEC-GOOSE-LORA-001 | User-specific QLoRA Trainer (Go + Rust) | planned | P2 | 대(L) | 없음 |
| SPEC-GOOSE-SIGNING-001 | Binary Signing & Update Key Distribution | planned | P0 | ? | DESKTOP-001 (Sprint 4) |
| SPEC-GOOSE-GATEWAY-TG-001 | Telegram Bot Gateway (Self-hosted) | planned | P1 | 중(M) | GATEWAY-001 (Sprint 5) |
| SPEC-GOOSE-A2A-001 | Agent Communication Protocol (A2A v0.3 + Hermes ACP) | planned | P2 | 대(L) | MCP-001 (완료), SUBAGENT-001 (완료) |

**순차 체인**: REFLECT -> IDENTITY (REFLECT + TRAJECTORY + VECTOR 완료 후), GATEWAY(Sprint 5) -> GATEWAY-TG
**병렬 가능**: VECTOR || LORA || SIGNING || GATEWAY-TG || A2A (대부분 독립)
**주의**: REFLECT-001은 Sprint 3의 SAFETY + Sprint 2의 MEMORY/INSIGHTS에 의존 -- 선행 스프린트 완료 필수

---

## Draft SPEC Finalization Schedule

Draft SPEC은 requirements가 확정되지 않았으므로, 각 스프린트 시작 전 planned 상태로 승격 필요:

| Draft SPEC | 승격 시점 | 스프린트 배정 |
|------------|----------|--------------|
| SPEC-GOOSE-ARCH-REDESIGN-v0.2 | Sprint 0 시작 전 (최우선) | Sprint 0 |
| SPEC-GOOSE-QMD-001 | Sprint 0 시작 전 (최우선) | Sprint 0 |
| SPEC-GOOSE-I18N-001 | Sprint 4 시작 전 | Sprint 4 |
| SPEC-GOOSE-ONBOARDING-001 | Sprint 5 시작 전 | Sprint 5 |
| SPEC-GOOSE-BRIEFING-001 | Sprint 6 시작 전 | Sprint 6 |
| SPEC-GOOSE-SCHEDULER-001 | Sprint 6 시작 전 | Sprint 6 |

---

## Sprint Dependency Chain

```
Sprint 0 (Foundation)
  │
  ├──> Sprint 1 (CLI & Integration) ──> CLI-001, CMDCTX-*-INTEG
  │         │
  │         └──> Sprint 4 (Cross-Platform) ──> DESKTOP-001
  │                       │
  │                       └──> Sprint 5 (Channels) ──> GATEWAY-001
  │                                    │
  │                                    └──> Sprint 7 ──> GATEWAY-TG-001
  │
  ├──> Sprint 2 (Self-Evolution) ──> TRAJECTORY, COMPRESSOR, MEMORY, INSIGHTS
  │         │
  │         └──> Sprint 7 (Advanced) ──> REFLECT ──> IDENTITY
  │
  ├──> Sprint 3 (Safety & Security) ──> CREDENTIAL-PROXY, SECURITY-SANDBOX
  │         │
  │         ├──> Sprint 4 (Cross-Platform) ──> AUTH-001
  │         └──> Sprint 7 (Advanced) ──> REFLECT ──> IDENTITY
  │
  └──> Sprint 6 (Daily Companion) ──> RITUAL, SCHEDULER, BRIEFING
```

Sprint 0, 1, 2, 3은 Sprint 0 이후 부분 병렬 가능:
- **Track A**: Sprint 1 -> Sprint 4 -> Sprint 5 -> Sprint 7 (일부)
- **Track B**: Sprint 2 -> Sprint 7 (일부)
- **Track C**: Sprint 3 -> Sprint 4 (AUTH) + Sprint 7 (REFLECT)
- **Track D**: Sprint 6 (Sprint 0 완료 후 독립 시작 가능)

---

## Risk Register

| 위험 | 영향 | 확률 | 완화 전략 |
|------|------|------|----------|
| ARCH-REDESIGN-v0.2 draft 지연 | 12+ SPEC 차단, 전체 일정 지연 | 중 | Sprint 0에서 최우선 확정, 나머지 SPEC은 draft 확정 전 병렬 시작 가능 |
| QMD-001 범위 확대 | Memory/Insights/Reflect 전체 영향 | 중 | M(중) 규모로 범위 엄격 관리, 외부 플러그인은 후속 스프린트 |
| DESKTOP-001 Tauri v2 학습 곡선 | Cross-Platform 트랙 전체 지연 | 중 | BRIDGE-001 먼저 완성하여 백엔드 안정화 후 Desktop UI 작업 |
| SECURITY-SANDBOX-001 OS별 구현 | Linux/macOS/Windows 각각 대응 필요 | 높음 | Phase 1은 Linux/macOS만, Windows는 후속 |
| Sprint 6 과적 (9 SPEC) | 일정 지연, 품질 저하 | 높음 | RITUAL + BRIEFING을 먼저, 나머지는 Sprint 6b로 분할 가능 |
| REFLECT-001 다중 의존성 | SAFETY + MEMORY + INSIGHTS 모두 필요 | 중 | Sprint 3과 Sprint 2 완료 후 시작, 임시 인터페이스로 선행 SPEC 연동 |
| GATEWAY-001 umbrella 범위 | TG/DC/SL/MX/MX 등 다중 어댑터 | 중 | v1.0은 TG-001만 포함, 나머지는 v1.1+ |
| Draft 6건 requirements 미확정 | 스프린트 시작 불가 | 높음 | 각 스프린트 시작 1주 전 draft -> planned 승격 완료 필수 |

---

## Summary Statistics

| Sprint | SPEC 수 | 대(L) | 중(M) | 소(S/극소) | Draft | 주요 산출물 |
|--------|---------|-------|-------|-----------|-------|------------|
| Sprint 0 | 5 | 2 | 2 | 1 | 2 | Architecture v0.2 + QMD + Agent/LLM |
| Sprint 1 | 5 | 0 | 4 | 1 | 0 | goose CLI + ContextAdapter wiring |
| Sprint 2 | 5 | 0 | 4 | 1 | 0 | Trajectory/Compressor/Memory/Insights |
| Sprint 3 | 6 | 2 | 2 | 2 | 0 | Safety + Credential + Sandbox + Audit |
| Sprint 4 | 6 | 2 | 3 | 1 | 1 | Desktop + WebUI + Auth + I18N |
| Sprint 5 | 7 | 2 | 3 | 1 | 1 | Onboarding + Gateway + Relay |
| Sprint 6 | 9 | 1 | 7 | 1 | 2 | Daily Companion v1.0 전체 |
| Sprint 7 | 7 | 3 | 3 | 0 | 0 | Reflect + Identity + LoRA + A2A |
| **Total** | **50** | **12** | **28** | **8** | **6** | |

---

*이 로드맵은 SPEC frontmatter 기반으로 자동 생성되었으며, 스프린트 진행에 따라 업데이트됩니다.*
*마지막 갱신: 2026-04-28*
