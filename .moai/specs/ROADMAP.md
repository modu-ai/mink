# GOOSE-AGENT SPEC 로드맵 v4.0

> **프로젝트**: GOOSE-AGENT v6.0 Daily Companion Edition (Cross-Platform)
> **작성일**: 2026-04-22
> **이전**: v3.0 (38 SPEC, Daily Companion 추가), v2.0 (30 SPEC), v1.0 (22 SPEC)
> **대화 언어**: 한국어 · **코드 식별자**: 영어
> **개발 방법론**: TDD (`development_mode: tdd`)
> **라이선스**: MIT
> **상태**: 아키텍처 설계 완료, 구현 0%

---

## 0. v4.0 재설계 근거

### 사용자 누적 지시 (2026-04-22)

**v3.0 (Daily Companion)**:
> "매일 아침 운세·날씨·일정 브리핑, 매 끼니 이후 건강/약 안내, 저녁 자기전 안부+일기 메모, 감성적으로 함께 성장하는 반려AI"

**v4.0 추가 (Cross-Platform)**:
> "CLI가 아닌 데스크탑 앱으로 모바일 앱으로 항상 함께 할 수 있도록. 기본 설치는 pc이지만 모바일 클라우드 연동으로 앱에서 pc를 제어 또는 지시 가능. hermes-agent, claude code sourmap 분석 자료로 장점 흡수해서 goose 재설계"

### v3.0 → v4.0 변화

| 축 | v3.0 | **v4.0** |
|----|----|----|
| 총 SPEC | 38 | **43** (+5) |
| Phase | 8 | **10** |
| 기본 UI | CLI 중심 | **Desktop App (Tauri v2)** |
| Mobile | 언급 없음 | **Mobile App (React Native) + PC 원격 제어** |
| 원격 연동 | 없음 | **E2EE Bridge + Relay** |
| 플랫폼 게이트웨이 | 없음 | **Telegram/Discord/Slack/KakaoTalk/WeChat/Webhook** |
| Phase 6 | Personalization | **Cross-Platform Clients (신규)** |
| Phase 8 | Ecosystem | **Personalization (기존 Phase 6에서 이동)** |

### Claude Code + Hermes 장점 흡수 (v4.0 신규)

| 원형 | 흡수 대상 | GOOSE 반영 SPEC |
|------|----------|---------------|
| Claude Code `bridge/` 33 파일 | PC↔Mobile 원격 세션 | **BRIDGE-001** 전체 포팅 |
| Claude Code `remoteBridgeCore.ts` | REPL 원격 | BRIDGE-001 §SessionRunner |
| Claude Code `jwtUtils.ts` + `trustedDevice.ts` | 인증·보안 | BRIDGE-001 §auth |
| Claude Code `flushGate.ts` + `capacityWake.ts` | Backpressure + Wake | BRIDGE-001 §reliability |
| Claude Code 146 UI 컴포넌트 | Desktop UX | **DESKTOP-001** |
| Hermes `gateway/` 7개 플랫폼 | 메신저 봇 | **GATEWAY-001** |
| Mullvad GotaTun (Rust) | WireGuard E2EE | **RELAY-001** (Rust crate 위임) |

---

## 1. 네이밍 규약

형식: `SPEC-GOOSE-{DOMAIN}-{NNN}`

DOMAIN 30개 (v2.0) + 8개 (v3.0 Daily Companion) + 5개 (v4.0 Cross-Platform) = **43개**.

v4.0 신규 DOMAIN: `DESKTOP`, `MOBILE`, `BRIDGE`, `RELAY`, `GATEWAY`

## 2. 우선순위 / 범위

| P0 | blocker | P1 | 가치 핵심 | P2 | 차별화 |
| S | ~500~1500 LoC | M | ~1500~4000 | L | ~4000~8000 |

---

## 3. 전체 SPEC 목록 (43건, 10 Phase)

### Phase 0 — Agentic Core (5 SPEC, P0)
| # | SPEC-ID | 제목 | 우선 | 범위 |
|---|---------|-----|----|----|
| 01 | SPEC-GOOSE-CORE-001 | goosed 데몬 부트스트랩 | P0 | S |
| 02 | **SPEC-GOOSE-QUERY-001** ★ | QueryEngine + queryLoop | P0 | L |
| 03 | SPEC-GOOSE-CONTEXT-001 | Context Window + compaction | P0 | M |
| 04 | SPEC-GOOSE-TRANSPORT-001 | gRPC 서버 + proto | P0 | M |
| 05 | SPEC-GOOSE-CONFIG-001 | 계층형 설정 로더 | P0 | S |

### Phase 1 — Multi-LLM Infrastructure (5 SPEC, P0) ★
| 06 | **SPEC-GOOSE-CREDPOOL-001** ★ | Credential Pool (OAuth/API) | P0 | L |
| 07 | SPEC-GOOSE-ROUTER-001 | Smart Model Routing | P0 | M |
| 08 | SPEC-GOOSE-RATELIMIT-001 | Rate Limit Tracker | P0 | S |
| 09 | SPEC-GOOSE-PROMPT-CACHE-001 | Prompt Caching | P1 | S |
| 10 | **SPEC-GOOSE-ADAPTER-001** ★ | 6 Provider 어댑터 | P0 | L |

### Phase 2 — 4 Primitives (5 SPEC, P0) ★
| 11 | SPEC-GOOSE-SKILLS-001 | Progressive Disclosure Skill | P0 | L |
| 12 | SPEC-GOOSE-MCP-001 | MCP Client/Server | P0 | L |
| 13 | **SPEC-GOOSE-HOOK-001** ★ | 24 Lifecycle Hooks + permission | P0 | M |
| 14 | SPEC-GOOSE-SUBAGENT-001 | Sub-agent Runtime | P0 | L |
| 15 | SPEC-GOOSE-PLUGIN-001 | Plugin Host | P1 | M |

### Phase 3 — Agentic Primitives (3 SPEC, P0)
| 16 | SPEC-GOOSE-TOOLS-001 | Tool Registry + ToolSearch | P0 | M |
| 17 | SPEC-GOOSE-COMMAND-001 | Slash Command System | P1 | S |
| 18 | SPEC-GOOSE-CLI-001 | goose CLI (개발·헤드리스용, 기본 아님) | P0 | M |

### Phase 4 — Self-Evolution (5 SPEC, P0) ★
| 19 | SPEC-GOOSE-TRAJECTORY-001 | Trajectory 수집 | P0 | S |
| 20 | SPEC-GOOSE-COMPRESSOR-001 | Trajectory Compressor | P0 | M |
| 21 | SPEC-GOOSE-INSIGHTS-001 | Insights 추출 | P1 | M |
| 22 | **SPEC-GOOSE-ERROR-CLASS-001** ★ | Error Classifier (14 FailoverReason) | P0 | S |
| 23 | **SPEC-GOOSE-MEMORY-001** ★ | Pluggable Memory Provider | P0 | M |

### Phase 5 — Promotion & Safety (3 SPEC, P1) ★
| 24 | **SPEC-GOOSE-REFLECT-001** ★ | 5-tier 승격 | P1 | L |
| 25 | SPEC-GOOSE-SAFETY-001 | 5-layer Safety | P1 | M |
| 26 | SPEC-GOOSE-ROLLBACK-001 | Regression Rollback | P1 | S |

### Phase 6 — **Cross-Platform Clients (5 SPEC, P0) ★ v4.0 신규**

> 목표: CLI가 아닌 **Desktop App 기본 + Mobile 동반**. Claude Code bridge + Hermes gateway 흡수.

| # | SPEC-ID | 제목 | 우선 | 범위 | 의존성 |
|---|---------|-----|-----|-----|-------|
| 27 | **SPEC-GOOSE-DESKTOP-001** ★ | Tauri v2 Desktop App (기본 UI) | P0 | L | TRANSPORT, QUERY |
| 28 | **SPEC-GOOSE-BRIDGE-001** ★ | PC↔Mobile 원격 세션 (Claude Code bridge/ 33 파일 포팅) | P0 | L | TRANSPORT |
| 29 | SPEC-GOOSE-RELAY-001 | E2EE Relay (Mullvad GotaTun 패턴, Rust crate) | P1 | L | BRIDGE |
| 30 | **SPEC-GOOSE-MOBILE-001** ★ | React Native Mobile (iOS/Android, PC 원격 제어) | P0 | L | BRIDGE, RELAY |
| 31 | SPEC-GOOSE-GATEWAY-001 | Multi-platform (Telegram/Discord/Slack/Matrix/KakaoTalk/WeChat/Webhook) | P2 | M | BRIDGE, MCP |

### Phase 7 — **Daily Companion (8 SPEC, P0~P1) ★ v3.0 유지**

> 목표: 양방향 반려. Morning/Meals×3/Evening 3회 리추얼로 감정적 유대 구축.

| # | SPEC-ID | 제목 | 우선 | 범위 | 의존성 |
|---|---------|-----|-----|-----|-------|
| 32 | **SPEC-GOOSE-SCHEDULER-001** ★ | Proactive Cron Scheduler | P0 | M | HOOK, CORE |
| 33 | SPEC-GOOSE-WEATHER-001 | 날씨 (OpenWeather + 기상청 KMA) | P1 | S | TOOLS |
| 34 | SPEC-GOOSE-FORTUNE-001 | 개인화 운세 (사주·바이오리듬·LLM) | P1 | M | ADAPTER, IDENTITY |
| 35 | **SPEC-GOOSE-CALENDAR-001** ★ | 일정 (Google/Apple/Outlook/Naver) | P0 | M | MCP |
| 36 | **SPEC-GOOSE-BRIEFING-001** ★ | 아침 브리핑 오케스트레이션 | P0 | M | FORTUNE, WEATHER, CALENDAR, SCHEDULER |
| 37 | SPEC-GOOSE-HEALTH-001 | 식사·복약 트래커 (한국 식약처 DUR) | P1 | M | SCHEDULER, MEMORY |
| 38 | **SPEC-GOOSE-JOURNAL-001** ★ | 저녁 일기 + 감정 태깅 + 추억 호출 | P0 | M | MEMORY, INSIGHTS, SAFETY |
| 39 | **SPEC-GOOSE-RITUAL-001** ★ | 3회 리추얼 통합 오케스트레이션 | P0 | L | BRIEFING, HEALTH, JOURNAL |

### Phase 8 — Deep Personalization (3 SPEC, P2)
> v3.0의 Phase 6에서 이동. Phase 6/7 후에 LoRA·Identity Graph 개인화.

| 40 | SPEC-GOOSE-IDENTITY-001 | Identity Graph (POLE+O, Kuzu) | P2 | L |
| 41 | SPEC-GOOSE-VECTOR-001 | Preference Vector (768-dim) | P2 | M |
| 42 | SPEC-GOOSE-LORA-001 | User QLoRA Trainer (Rust 위임) | P2 | L |

### Phase 9 — Ecosystem (1 SPEC, P2)
| 43 | SPEC-GOOSE-A2A-001 | Agent Communication Protocol | P2 | L |

**합계**: **43 SPEC · 10 Phase · 총 약 720+ REQ / 420+ AC**

★ = Critical Path

---

## 4. 4-Layer ↔ Phase 매핑 (v4.0 완성)

```
┌────────────────────────────────────────────────────────┐
│ Layer 4 💞 Emotional Bond   → JOURNAL(§38) + REFLECT   │
│ Layer 3 📅 Daily Rituals    → Phase 7 (§32~39) ★v3.0   │
│ Layer 2 🐣 Nurture Loop     → QUERY + INSIGHTS + TUI   │
│ Layer 1 🧠 Agentic Core     → Phase 0~5 (26 SPEC)      │
├────────────────────────────────────────────────────────┤
│ 🖥️ Presentation              → Phase 6 ★v4.0 신규       │
│   - Desktop App (기본)                                   │
│   - Mobile App (동반)                                    │
│   - Bridge + Relay (E2EE)                                │
│   - Gateway (Telegram/KakaoTalk 등)                      │
├────────────────────────────────────────────────────────┤
│ 🧬 Advanced                  → Phase 8~9                 │
│   - Identity Graph + LoRA + A2A                          │
└────────────────────────────────────────────────────────┘
```

---

## 5. Phase별 요약

| Phase | 이름 | SPEC 수 | 핵심 가치 |
|-------|-----|--------|----------|
| 0 | Agentic Core | 5 | async streaming query loop |
| 1 | Multi-LLM | 5 | 15+ provider OAuth/API |
| 2 | 4 Primitives | 5 | Skills/MCP/Agents/Hooks |
| 3 | Agentic Primitives | 3 | Tool Registry + CLI(개발용) |
| 4 | Self-Evolution | 5 | Trajectory → Insights → Memory |
| 5 | Promotion & Safety | 3 | 5-tier + 5-layer |
| **6** | **🆕 Cross-Platform Clients** | **5** | **Desktop + Mobile + Bridge + Relay + Gateway** |
| 7 | Daily Companion | 8 | 일상 리추얼 3회 |
| 8 | Deep Personalization | 3 | Identity + Vector + LoRA |
| 9 | Ecosystem | 1 | A2A |
| **합계** | — | **43** | — |

---

## 6. 주요 의존성 그래프 (v4.0 상위)

```
[Phase 0] CORE ──┬─ CONFIG
                 ├─ TRANSPORT ──────────────────────────────┐
                 └─ QUERY ── CONTEXT                         │
                                 │                           │
[Phase 1] CREDPOOL ── ROUTER ── ADAPTER ────┐               │
                         ├─ RATELIMIT         │               │
                         └─ PROMPT-CACHE      │               │
                                              │               │
[Phase 2] SKILLS ── HOOK ── SUBAGENT ────────┼──── MCP ───────┤
                                              │               │
[Phase 3] TOOLS ── COMMAND ── CLI(개발)       │               │
                                              │               │
[Phase 4] MEMORY ── TRAJECTORY ── COMPRESSOR │               │
                        ├─ INSIGHTS           │               │
                        └─ ERROR-CLASS ←──────┘               │
                                                              │
[Phase 5] REFLECT ── SAFETY ── ROLLBACK                       │
                                                              │
[Phase 6 ★v4.0] ────────────────────────────────────────────▶│
  DESKTOP (Tauri)  ◀─────── BRIDGE ◀──── RELAY (Rust)         │
       │                      ▲                               │
       └─ QR pairing          │                               │
                              ▼                               │
                      MOBILE (RN)                             │
                              │                               │
                      GATEWAY (Telegram/Discord/Kakao)        │
                                                              │
[Phase 7] SCHEDULER ──┬─ WEATHER                              │
                      ├─ FORTUNE                              │
                      ├─ CALENDAR ───┐                        │
                      └─ HEALTH      │                        │
                                     ▼                        │
                             BRIEFING + JOURNAL ── RITUAL ←───┘
                                                              
[Phase 8] IDENTITY ── VECTOR ── LORA (Rust)
                                                              
[Phase 9] A2A-001
```

---

## 7. 실행 순서 권장 (Milestone)

### M0 — Agentic Foundation (2주)
CORE → CONFIG + TRANSPORT(병렬) → QUERY → CONTEXT

### M1 — Multi-LLM (3주)
CREDPOOL → [ROUTER+RATELIMIT+ERROR-CLASS] 병렬 → PROMPT-CACHE → ADAPTER

### M2 — 4 Primitives (4주)
[SKILLS+HOOK+MCP] 병렬 → SUBAGENT → PLUGIN

### M3 — Developer CLI (1주) **← 기존 v3.0 MVP CLI에서 범위 축소**
TOOLS → COMMAND → CLI-001 (개발/헤드리스 용도)

### M4 — Self-Evolution (3주)
[TRAJECTORY+MEMORY] 병렬 → COMPRESSOR → INSIGHTS

### M5 — Safety (2주)
REFLECT → SAFETY → ROLLBACK

### **M6 — Cross-Platform Clients (4주) ★ v4.0 신규**
DESKTOP ↔ BRIDGE ── RELAY ── MOBILE 순차·병렬 → GATEWAY
**→ v0.3 Public Beta 가능 시점 (Desktop+Mobile 동작)**

### **M7 — Daily Companion (4주) ← v1.0 Release**
SCHEDULER → [WEATHER+CALENDAR+HEALTH] 병렬 → FORTUNE → BRIEFING + JOURNAL → RITUAL
**→ v1.0 Release: 일상 반려 AI 완성**

### M8 — Deep Personalization (4주) ← v1.5 Release
[IDENTITY+VECTOR] 병렬 → LORA (Rust 위임)

### M9 — Ecosystem (옵션)
A2A-001

### 총 기간 (팀 2명, TDD)
- M0~M7 v1.0 Release: **~23주** (~5.5개월)
- M0~M8 v1.5: **~27주** (~6.3개월)
- 팀 3명 병렬 최대: **~17주** (~4개월)

---

## 8. 버전 Release 타임라인

| Release | Milestone 포함 | 기능 |
|---------|-------------|-----|
| v0.1 Alpha | M0~M1 | `goose ask "hello"` 동작 (CLI) |
| v0.2 Beta | M0~M2 | 4 Primitive + MVP Skill 로드 |
| v0.3 Beta | M0~M3 | 개발자 CLI 안정화 |
| **v0.4 Public Beta** | **M0~M6** | **Desktop + Mobile + Bridge 첫 공개** |
| v0.5 RC | M0~M6+M5 | Safety 게이트 + PR 품질 |
| **v1.0 Release** | **M0~M7** | **일상 반려 AI 완성 (Daily Companion)** |
| v1.5 | M0~M8 | 개인화 LoRA |
| v2.0 | M0~M9 | A2A + Ecosystem |

---

## 9. 주요 설계 원칙 (v4.0 누적)

1. **One QueryEngine per conversation** (Claude Code)
2. **Streaming mandatory** (async channel)
3. **Credential Pool first** (모든 LLM은 pool 경유)
4. **4 Primitive first-class** (Skills/MCP/Agents/Hooks)
5. **Self-evolution with safety gates** (5-tier + 5-layer)
6. **Proactive over Reactive** (GOOSE가 먼저 말 건다)
7. **Bidirectional Care** (사용자⇄GOOSE 서로 돌봄)
8. **Privacy-First Intimacy** (친밀함은 로컬 저장으로만)
9. **Ritualized Presence** (매일 같은 시간에 나타나는 존재)
10. **🆕 PC-First, Mobile-Companion** (PC가 메인, Mobile은 항상 함께)
11. **🆕 E2EE Always** (PC↔Mobile 간 plaintext 접근 불가능)
12. **🆕 Progressive Disclosure on UI** (Desktop 풀 UI → Mobile 핵심 기능 → CLI 헤드리스)

---

## 10. 법적·윤리적 제약 (v4.0 업데이트)

| SPEC | 제약 |
|------|------|
| FORTUNE-001 | 엔터테인먼트, opt-in OFF 기본, 의학·금융·복권 키워드 guard |
| HEALTH-001 | 의료 기기 아님, 응급 시 119 자동 안내, 식약처 DUR severe interaction HARD block |
| JOURNAL-001 | 로컬 only 기본, A2A 전송 HARD 금지, crisis keyword 시 1577-0199 |
| CALENDAR-001 | OAuth 최소 권한, Cross-origin redirect 차단 |
| SCHEDULER-001 | Quiet hours 23-06시 HARD floor |
| RITUAL-001 | Guilt-free 언어, 스킵 자유 |
| **BRIDGE-001** | **Trusted Device 관리, JWT 24h, session revoke** |
| **RELAY-001** | **Plaintext 접근 절대 불가 (Noise Protocol 증명)** |
| **MOBILE-001** | **Biometric lock, 로컬 캐시 암호화** |
| **GATEWAY-001** | **OAuth minimum scope, 사용자 명시적 동의** |

---

## 11. OUT OF SCOPE

- Rust ML crate (LoRA 구현 상세): `ROADMAP-RUST.md`
- 클라우드 백업 구체 스킴: Phase 8+ 별도 SPEC
- 기업 내 자체 Relay 호스팅 가이드: v1.5 이후
- Agent Teams 병렬 실행: MoAI-ADK-Go 직접 link 시점에 재평가

---

## 12. 오픈 이슈 (v4.0 업데이트, 20건)

### 기존 (v3.0)
1. Go 버전 고정
2. sqlite 드라이버 (modernc vs mattn)
3. Tokenizer 라이브러리
4. Graph DB (Kuzu vs Neo4j)
5. LoRA Base Model
6. LLM Stream 인터페이스
7. proto commit 정책
8. Rust goose-ml 배포
9. 운세 문화 범위 (한국 vs 서양)
10. 건강 DB 소스 (식약처 vs WHO)
11. 캘린더 기본 프로바이더
12. 일기 저장 위치 (로컬 SQLite vs 파일)
13. 리추얼 음성 TTS

### v4.0 신규
14. **Tauri updater ed25519 키 배포**: CI 주입 vs 소스 임베드
15. **Relay Go↔Rust FFI**: gRPC(기본) vs CGO(핫패스)
16. **Porcupine 라이센스**: 상용 모듈 + GOOSE MIT 호환
17. **Whisper 모델 배포**: 번들 150MB vs on-demand
18. **카카오 알림톡 벤더**: Solapi vs 직접 계약
19. **WeChat 중국 법인**: v0.1 스텁만 or 제외
20. **HOOK-001 EventName 확장**: SCHEDULER가 5개 신규 요청 → HOOK-001 minor bump

---

## 13. 첫 번째 실행 SPEC

**Phase 0, 순번 01**: `SPEC-GOOSE-CORE-001`

v4.0 첫 Cross-Platform 진입: `SPEC-GOOSE-DESKTOP-001` (M6 시작)

---

**Version**: 4.0.0
**License**: MIT (이 문서 포함)
**Next action**: Phase 0 CORE-001 RED → M7 완료 시 v1.0 Daily Companion Release
