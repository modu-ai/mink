# MINK 구현 순서 종합 보고서 v6.2 (★ v0.2 재편 amendment 적용)

> **⚠ v0.2 Amendment (2026-04-24)**
> SPEC-GOOSE-ARCH-REDESIGN-v0.2 에 따라 Milestone 재정렬. 아래 본문(v6.2)은 **참조용**이며, 실제 구현은 v0.2 Milestone 순서를 따른다.
>
> **v0.2 Milestone 순서 (최종)** (★ 2026-04-30 진행 상태 갱신):
> - **M0 Foundation** ✅ **완료** — CORE-001 ✅ · QUERY-001 ✅ · CONTEXT-001 ✅ · TRANSPORT-001 ✅ · CONFIG-001 ✅ (5/5)
> - **M1 Multi-LLM + QMD** ✅ **완료** — CREDPOOL ✅ · ROUTER ✅ · RATELIMIT ✅ · PROMPT-CACHE ✅ · **ADAPTER-001 ✅ (Round 1+2 완료 2026-04-27, M0~M5 전수 GREEN)** · QMD-001 🟡 · PROVIDER-FALLBACK 📋 · **ADAPTER-001-AMEND-001 📋 (deferred AC-016/017)**
> - **M2 4 Primitives** ✅ **완료** — SKILLS ✅ · MCP ✅ · HOOK ✅ · SUBAGENT ✅ · PLUGIN ✅ · PERMISSION-001 ✅ (6/6)
> - **M3 Core Workflow** 🟡 **진행 중** — COMMAND ✅ · TOOLS ✅ · **CLI-001 🟡 TUI 보강** · Plan-Run-Sync 📋 · SELF-CRITIQUE-001 ✅
> - **M4 Self-Evolution** 🟡 **부분** — ERROR-CLASS ✅ · MEMORY-001 ✅ · TRAJECTORY 📋 · COMPRESSOR 📋 · INSIGHTS 📋 · auto-skill 📋
> - **M5 Safety (★ 대폭 확장)** 🟡 **부분** — AUDIT-001 ✅ · CREDENTIAL-PROXY-001 🟡 · FS-ACCESS-001 🟡 · SECURITY-SANDBOX-001 🟡 · SIGNING-001 🟡 · SAFETY 📋 · ROLLBACK 📋
> - **M6 Channels (축소)** 📋 **계획** — TELEGRAM-001 📋 · WEBUI-001 📋 (MOBILE/WIDGET/SYNC/CLOUD/DISCOVERY/Email 제거)
> - **M7 Daily Companion v1.0** 🟡 **부분 진행** — RITUAL-001 📋 · **BRIEFING-001 ✅ (v0.3.0 implemented 2026-05-14, AC 16/16 GREEN, PR #178/#182/#183)** · **JOURNAL-001 ✅ (v0.3.0 completed)** · **WEATHER-001 ✅ (v0.2.0 completed)** · **MSG-TELEGRAM-001 ✅ (v0.1.3 completed)** · **SCHEDULER-001 ✅ (v0.2.x completed)** · PAI-CONTEXT-001 📋 · GROWTH 📋
> - **M8 Deep Personalization** 📋 **계획** — IDENTITY-001 📋 · VECTOR 📋 · LORA 📋 · Kuzu 도입 📋 · Affective 📋
> - **M9 Ecosystem v2.0** 📋 **계획** — plugin marketplace · additional channels
>
> **진행 요약** (2026-05-14 갱신): M0/M1/M2 완료 ✅ · M3/M4/M5 부분 진행 🟡 · **M7 Daily Companion v1.0 부분 진행 🟡 (BRIEFING/JOURNAL/WEATHER/TELEGRAM/SCHEDULER 5 SPEC implemented)** · M6/M8/M9 계획 📋
> **MINK rebrand 시리즈 완료** (2026-05-14): BRAND-RENAME-001 ✅ + DISTANCING-STATEMENT-001 ✅ + USERDATA-MIGRATE-001 ✅ + ENV-MIGRATE-001 ✅ + PRODUCT-V7-001 ✅ + Draft 3종 rebrand ✅ (PR #180) + 메타문서 rebrand ✅ (PR #181) + 95-SPEC 리뷰 ✅ (PR #178)
> **다음 Critical Path**: CLI-001 TUI 보강 (BriefingPanel bubbletea integration + /briefing slash dispatch) → Phase 4 Self-Evolution (TRAJECTORY → COMPRESSOR → INSIGHTS) → OBS-METRICS-001 / 신규 MINK-{CROSSPLAT,I18N,ONBOARDING}-001 implementation 진입
>
> **메타 정정 (2026-04-30)**: ADAPTER-001 / ALIAS-CONFIG-001 모두 spec.md frontmatter `completed: 2026-04-27` 정합. 이전 'Round 2 진행 중' / 'planned' 표기는 progress.md 미갱신에 따른 메타 불일치였으며, 실제 구현은 완료 상태. 추가 개선은 별도 amendment SPEC(ADAPTER-001-AMEND-001, ALIAS-CONFIG-001-AMEND-001)로 분리.
>
> 자세한 근거: `.moai/design/mink-runtime-architecture-v0.2.md` §10 · `.moai/specs/SPEC-GOOSE-ARCH-REDESIGN-v0.2/spec.md`

---

## v0.1.0 MVP Milestone (FROZEN — 2026-05-17 사용자 확정)

> ROADMAP §0 (v0.1.0 MVP 범위) 의 IMPLEMENTATION-ORDER 측 대응 섹션. v6.2 본문의 M0~M9 milestone 은 **참조용 장기 로드맵**이며, **실 구현 우선순위는 본 §v0.1.0 + §0.2.0 이월 표가 우선**한다.

### v0.1.0 Critical Path (직선)

```
[완료된 Foundation — 다시 진입 불필요]
M0 Foundation        ✅ CORE → CONFIG → TRANSPORT → QUERY → CONTEXT
M1 Multi-LLM         ✅ CREDPOOL → ROUTER → RATELIMIT → PROMPT-CACHE → ADAPTER
M2 4 Primitives      ✅ SKILLS → MCP → HOOK → SUBAGENT → PLUGIN → PERMISSION
M3 Core Workflow     ✅ TOOLS → COMMAND → CLI(베이스)
M4 Evolution(부분)   ✅ ERROR-CLASS → MEMORY → TRAJECTORY(일부)
M5 Safety(부분)      ✅ AUDIT
M7 Daily(부분)       ✅ SCHEDULER → WEATHER → JOURNAL → BRIEFING (v0.3.1)
                     ✅ MSG-TELEGRAM (v0.1.4)
MINK 전환            ✅ BRAND-RENAME, DISTANCING, PRODUCT-V7, USERDATA-MIGRATE,
                       ENV-MIGRATE, LOCALE-001(v0.4), AUTH-CREDENTIAL(v0.3)

[v0.1.0 Critical Path — 완료해야 launch 가능]
1. MEMORY-QMD M5b              🟡 M1~M5a 완료, M5b cross-SPEC wiring 잔여
2. LLM-ROUTING-V2-AMEND M2~M5  🟡 M1 stub 완료, 5 provider 실 구현 + 2 패턴 인증
3. CLI-TUI-003-AMEND           ⏸️ TUI/CLI 패리티 (코드 진입 필요)
4. MSG-SLACK + MSG-DISCORD     ⏸️ 채널 3종 완비 (코드 진입 필요)
5. WEB-CONFIG                  ⏸️ 설치 후 설정 페이지 (코드 진입 필요)
6. ONBOARDING-001 Phase 5 AMEND ⏸️ Web Step 2 LLM provider 로그인 wiring 🔴
   └─ launch 차단 요소
```

### v0.1.0 의존성 정렬 (실 진입 순서)

```
[Step 1] MEMORY-QMD M5b
    └─ JOURNAL/BRIEFING/WEATHER publish hooks 가 0.1.0 의 cross-SPEC wiring 풀어줌
    └─ 차단 해소 후 [Step 2~6] 모두 진입 가능
[Step 2] LLM-ROUTING-V2-AMEND M2~M5 (병행 가능)
    └─ ONBOARDING Phase 5 의 Web Step 2 placeholder 해소 선행 의존
[Step 3] ONBOARDING-001 Phase 5 AMEND (Step 2 후 진입)
    └─ Web Step 2 (LLM 로그인) + Step 3 (채널 연결) wiring
[Step 4] MSG-SLACK + MSG-DISCORD (병행 가능, Step 3 와 무관)
[Step 5] CLI-TUI-003-AMEND (병행 가능, 어느 시점이든)
[Step 6] WEB-CONFIG (병행 가능, Step 3 후 진입 권장)
```

병렬 처리: Step 2/4/5 는 Step 1 완료 후 동시 진입 가능. Step 3 는 Step 2 의존.

### v0.2.0 이월 10건 (2026-05-17 사용자 확정)

| SPEC | M | Phase | 원래 priority | 이월 priority | 이월 사유 |
|---|---|---|---|---|---|
| CALENDAR-001 | M7 | 7 | P0 | P0(유지) | 아침 브리핑 통합 일정은 v0.2.0 |
| FORTUNE-001 | M7 | 7 | P1 | P1(유지) | 엔터테인먼트, 후순위 |
| HEALTH-001 | M7 | 7 | P1 | P1(유지) | 의료 규제 의존 |
| LORA-001 | M8 | 6 | P2 | P2(유지) | Deep personalization, Rust 위임 |
| GEMMA4-001 | M1 | 1 | P0 | P1(강등) | 0.1.0 은 qwen2.5:3b 유지 |
| TRAIN-001 | M8 | 2 | P1 | P2(강등) | GEMMA4 의존 |
| AUTH-001 | M6 | 6 | P0 | P1(강등) | AUTH-CREDENTIAL 가 0.1.0 인증 담당 |
| A2A-001 | M9 | 7 | P2 | P2(유지) | v2.0+ 검토 |
| WEBUI-001 | M6 | 6 | P0 | P1(강등) | 0.1.0 은 ONBOARDING 설치 위저드 + 채널 |
| DESKTOP-001 | M6 | 6 | critical | P1(강등) | 0.1.0 은 CLI/TUI 한정 |

frontmatter 마킹: 각 SPEC `target_milestone: v0.2.0` + `mvp_status: deferred` + `deferred_reason` 필드.

### v0.1.0 잔여 backlog — 별도 결정 필요

10건 외에 `planned` 상태로 남는 SPEC 들 (Self-Evolution Layer 2~4 / Identity·Vector / 보안 4건 / Channel / CMDCTX amendments / PAI-CONTEXT / REGION-SKILLS) 은 본 결정에서 명시적 이월 대상 아님. 0.1.0 launch 직전 별도 라운드로 진입 여부 결정.

---

> **작성일**: 2026-04-22 (v6.2) · 2026-04-24 (v0.2 amendment)
> **대상**: v6.2 58 SPEC + v0.2 재편 (-5 +9) = 약 54 SPEC
> **근거**: ROADMAP v6.2 → v0.2 amendment + 7-round 소크라테스식 인터뷰
> **방법론**: TDD (RED→GREEN→REFACTOR)
> **목적**: 의존성 그래프 기반 최적 구현 순서 + 병렬화 + Milestone

---

## 0. 요약

- **전체**: 58 active SPEC (+ 5 옵션 v2.0+), 약 **950+ REQ / 560+ AC**
- **Go LoC**: ~40,000 + Rust LoRA/crypto/signal crate 위임
- **Critical Path**: CORE → QUERY → CREDPOOL → ADAPTER → HOOK → SUBAGENT → DESKTOP → BRIDGE → MOBILE → SCHEDULER → BRIEFING → JOURNAL → RITUAL (14 SPEC)
- **10 Milestone**:
  - M0 Core → M1 Multi-LLM → M2 Primitives → M3 Dev CLI → M4 Evolution → M5 Safety
  - **M6 Cross-Platform + Localization + 3-Tier (6주)**
  - **M7 Daily Companion + Telegram (4.5주)** ← **v1.0 Release**
  - M7.5 Messenger 확장 (2주) ← v1.1~1.2
  - M8 Personalization → M9 Ecosystem
- **v1.0 기준 소요**: 팀 3명 **~17.5주** / 팀 2명 ~25.5주 / 솔로 ~32.5주

---

## 1. 58 SPEC 전체 목록

### Phase 0 — Agentic Core (5)
`CORE` · `QUERY★` · `CONTEXT` · `TRANSPORT` · `CONFIG`

### Phase 1 — Multi-LLM Infrastructure (5)
`CREDPOOL★` · `ROUTER` · `RATELIMIT` · `PROMPT-CACHE` · `ADAPTER★`

### Phase 2 — 4 Primitives (5)
`SKILLS` · `MCP` · `HOOK★` · `SUBAGENT` · `PLUGIN`

### Phase 3 — Agentic Primitives (3)
`TOOLS` · `COMMAND` · `CLI`

### Phase 4 — Self-Evolution (5)
`TRAJECTORY` · `COMPRESSOR` · `INSIGHTS` · `ERROR-CLASS★` · `MEMORY★`

### Phase 5 — Promotion & Safety (3)
`REFLECT★` · `SAFETY` (+ Channel HARD) · `ROLLBACK`

### Phase 6 — Cross-Platform + Localization + 3-Tier (15) ★ v6.2 핵심
**6a Cross-Platform (5)**: `DESKTOP★` · `BRIDGE★` · `RELAY` · `MOBILE★` · `GATEWAY★` (umbrella)
**6b Localization (4)**: `LOCALE★` · `I18N★` · `REGION-SKILLS` · `ONBOARDING★`
**6c 3-Tier (4)**: `CLOUD★` · `DISCOVERY` · `AUTH` · `SYNC`
**6d Notifications (2)**: `NOTIFY` · `WIDGET`

### Phase 7 — Daily Companion (8)
`SCHEDULER★` · `WEATHER` · `FORTUNE` · `CALENDAR★` · `BRIEFING★` · `HEALTH` · `JOURNAL★` · `RITUAL★`

### Phase 8 — Deep Personalization (3)
`IDENTITY` · `VECTOR` · `LORA`

### Phase 9 — Ecosystem + Messenger 확장 (6)
`GATEWAY-TG★` (v1.0) · `GATEWAY-DC` · `GATEWAY-SL` · `GATEWAY-MX` · `GATEWAY-SG` · `A2A` (v2.0)

### v2.0+ 옵션 (SPEC 번호 미할당, 사업자 전제)
- `GATEWAY-KR` (KakaoTalk, 한국 법인)
- `GATEWAY-CN` (WeChat, 중국/홍콩 법인)
- `GATEWAY-JP` (LINE, 일본/태국 법인)
- `GATEWAY-SMS` (Twilio, Tier 2 연계)
- `VOICE` (Siri/Google Assistant 정식)

★ = Critical Path (14건)

---

## 2. 상세 의존성 그래프 (Cross-Phase, v6.2)

```
[Phase 0 Foundation]
CORE ─┬─ CONFIG ──────────────────────────────────┐
      ├─ TRANSPORT ─────────────────────────────────┤
      └─ QUERY ── CONTEXT                          │
                    │                               │
[Phase 1 Multi-LLM] │                               │
  CREDPOOL ─┬─ ROUTER ─┬─ ADAPTER ◀────────────────│
            ├─ RATELIMIT│        │                   │
            └─ PROMPT-CACHE     │                   │
                                 │                   │
[Phase 2 Primitives]             │                   │
  SKILLS ── HOOK ── SUBAGENT ─┬──│─── MCP ──────────│
                 │             │  │                   │
[Phase 3]        │             │  │                   │
  TOOLS ── COMMAND ── CLI      │  │                   │
                                │  │                   │
[Phase 4 Evolution]             │  │                   │
  MEMORY ─┬─ TRAJECTORY ── COMPRESSOR                 │
          ├─ INSIGHTS                                   │
          └─ ERROR-CLASS ◀─────┘                       │
                                                         │
[Phase 5 Safety]                                        │
  REFLECT ── SAFETY(+Ch-HARD) ── ROLLBACK              │
                                                         │
[Phase 6 ★v6.2 Cross-Platform + 3-Tier]                │
  병렬1: LOCALE → I18N → REGION-SKILLS                 │
  병렬2: DISCOVERY + AUTH ─── CLOUD                    │
                          └── NOTIFY                   │
  순차: DESKTOP (Tauri) ◀── ONBOARDING                │
            │                                           │
            └── BRIDGE ◀── RELAY (Rust mink-crypto)    │
                    │                                   │
                    └── MOBILE (RN + Widget + Siri) ◀──│
                                    │                   │
  umbrella: GATEWAY-001 재정의                         │
                                                         │
[Phase 7 Daily Companion]                               │
  SCHEDULER ──┬─ WEATHER                                │
              ├─ FORTUNE                                │
              ├─ CALENDAR                               │
              └─ HEALTH                                 │
                    │                                   │
            BRIEFING + JOURNAL ── RITUAL ◀─────────────┘
                    │
                    └── (Telegram 전송: JOURNAL 제외, Ch HARD)

[Phase 8 Personalization]
  IDENTITY ── VECTOR ── LORA (Rust)

[Phase 9]
  GATEWAY-TG (v1.0, Telegram)
  GATEWAY-DC/SL (v1.1)
  GATEWAY-MX/SG (v1.2, E2EE)
  A2A (v2.0)
```

---

## 3. 최적 구현 순서 (Milestone별)

### M0 — Agentic Foundation (2주) → v0.1 Alpha
```
CORE → [CONFIG ∥ TRANSPORT] → QUERY★ → CONTEXT
```
**완료 기준**: Mock LLM으로 `<-chan SDKMessage` streaming 성공

### M1 — Multi-LLM + Error Handling (3주) → v0.1 Alpha
```
CREDPOOL → [ROUTER ∥ RATELIMIT ∥ ERROR-CLASS 선행] → PROMPT-CACHE → ADAPTER★
```
**완료 기준**: `mink ask "hello"` → 실제 Anthropic/OpenAI 응답

### M2 — 4 Primitives (4주) → v0.2 Beta
```
[SKILLS ∥ HOOK ∥ MCP] → SUBAGENT → PLUGIN
```
**완료 기준**: 외부 MCP, Sub-agent fork, Skill 로드 동작

### M3 — Developer CLI (1주) → v0.3 Beta
```
TOOLS → COMMAND → CLI
```
**완료 기준**: `mink ask --json --prompt "..."` 헤드리스 모드

### M4 — Self-Evolution (3주)
```
[TRAJECTORY ∥ MEMORY] → COMPRESSOR → INSIGHTS
```

### M5 — Safety (2주)
```
REFLECT → SAFETY (+Channel HARD rule) → ROLLBACK
```

### M6 — Cross-Platform + Localization + 3-Tier (6주) → v0.4 Public Beta ★

**Week 1**: Localization 기초
```
병렬: LOCALE → I18N → REGION-SKILLS
```

**Week 2**: 3-Tier 인프라 기초
```
병렬: DISCOVERY + AUTH → CLOUD (Zero-Knowledge 골격)
병렬: NOTIFY (APNs/FCM 연동)
```

**Week 3~4**: Desktop + Onboarding
```
DESKTOP (Tauri v2) → ONBOARDING (6단계: Welcome→Locale→Consent→Tier→LLM→Channel)
```

**Week 5**: Bridge + Mobile
```
BRIDGE (mDNS + STUN/TURN 계단식) ── RELAY (Rust crate) 병렬 가능
  └── MOBILE (RN + Widget + Live Activity + Siri Shortcut + Share Sheet)
```

**Week 6**: Gateway umbrella + WIDGET 마감
```
GATEWAY-001 재정의 (Tier A 5종 인터페이스만, 구체 구현은 M7+)
WIDGET (iOS/Android/Desktop Widget 마감)
```

**완료 기준**:
- PC Desktop App 실행 → 6단계 ONBOARDING (~3분)
- 국가/언어 자동 감지 → 국가별 Skill 10개 자동 번들
- 20+ 언어 UI + RTL
- Mobile App QR 페어링 (Tier 0) 또는 이메일 가입 (Tier 1)
- Tier 1 Cloud Free: 외부망 Mobile 대화 + APNs/FCM 푸시
- Apple Native: Siri Shortcut + Share Sheet + Live Activity 동작
- v0.4 Public Beta Release (Tier 0 + Tier 1)

### M7 — Daily Companion + Telegram (4.5주) → v1.0 Release ★

**Week 1~3**: Daily Ritual
```
SCHEDULER → [WEATHER ∥ CALENDAR ∥ HEALTH]
         → FORTUNE → [BRIEFING ∥ JOURNAL] → RITUAL
```

**Week 4 (병행)**: Telegram Bot
```
GATEWAY-TG-001 (long-polling, inline buttons, Trusted User 인증)
```

**Week 4.5**: 통합 테스트
- 07:00 자동 아침 브리핑 (Desktop Tray + Mobile Lock Screen + Telegram DM)
- 식후 3회 식사/복약 알림 (Mobile + Telegram, Journal 제외)
- 23:30 저녁 일기 프롬프트 (Desktop + Mobile, Telegram 금지)
- 3회 리추얼 통합 오케스트레이션

**완료 기준**: **v1.0 Release = 일상 반려 AI + Telegram 원격 리모컨 완성**

### M7.5 — Messenger 확장 (2주) → v1.1~v1.2

**Week 1 (v1.1)**: Discord + Slack
```
GATEWAY-DC-001 (Discord Gateway WS + Application) → GATEWAY-SL-001 (Slack Socket Mode)
```

**Week 2 (v1.2)**: Matrix + Signal (E2EE)
```
GATEWAY-MX-001 (Matrix sync, E2EE 옵션) → GATEWAY-SG-001 (Signal libsignal, E2EE HARD)
```
Matrix/Signal E2EE 채널에서만 Journal 본문 송출 허용 (사용자 opt-in 필수).

### M8 — Deep Personalization (4주) → v1.5
```
[IDENTITY ∥ VECTOR] → LORA (Rust crate 위임)
```
주간 로컬 QLoRA 재훈련, 사용자 고유 말투 학습.

### M9 — Ecosystem (옵션) → v2.0

**Biz-Messenger 진입** (사업자 등록 후):
```
GATEWAY-KR (Kakao 알림톡 + Solapi) → GATEWAY-CN (WeChat) → GATEWAY-JP (LINE)
```

**SMS + Voice**:
```
GATEWAY-SMS (Twilio) 병행
VOICE (Siri Shortcut 정식 + Google Assistant Intent + Alexa Skill)
```

**A2A**:
```
A2A-001 (단, Journal/Health/Identity 전송 HARD 금지 유지)
```

---

## 4. Critical Path 분석 (v6.2)

### 4.1 최단 경로 (순차 필수)
```
CORE → CONFIG → TRANSPORT → QUERY → CONTEXT
→ CREDPOOL → ROUTER → ADAPTER
→ HOOK → SUBAGENT
→ TOOLS
→ LOCALE → I18N → DISCOVERY → AUTH → CLOUD → NOTIFY → DESKTOP → ONBOARDING
→ BRIDGE → MOBILE → GATEWAY (umbrella)       [v0.4 Public Beta]
→ TRAJECTORY → MEMORY → INSIGHTS
→ REFLECT → SAFETY
→ SCHEDULER → BRIEFING + JOURNAL → RITUAL
→ GATEWAY-TG-001                              [v1.0 Release]
→ VECTOR → LORA                               [v1.5]
```
총 **29 SPEC**. 나머지 29 SPEC은 critical path 외.

### 4.2 병렬화 기회

| Milestone | 버전 | 병렬 그룹 | 병렬 수 | 절감 |
|---|---|---|---|---|
| M0 | v0.1 | CONFIG / TRANSPORT | 2 | 30% |
| M1 | v0.1 | ROUTER / RATELIMIT / ERROR-CLASS | 3 | 40% |
| M2 | v0.2 | SKILLS / HOOK / MCP | 3 | 50% |
| M4 | v0.5 | TRAJECTORY / MEMORY | 2 | 30% |
| **M6 L1** | v0.4 | LOCALE / I18N / REGION-SKILLS | 3 | 40% |
| **M6 L2** | v0.4 | DISCOVERY / AUTH / NOTIFY | 3 | 40% |
| **M6 L3** | v0.4 | RELAY / MOBILE / WIDGET | 3 | 30% |
| **M7** | v1.0 | WEATHER / CALENDAR / HEALTH | 3 | 40% |
| **M7** | v1.0 | [Phase 7 main path] ∥ [Telegram bot] | 2 | 25% |
| M7.5 | v1.1 | Discord / Slack | 2 | 35% |
| M7.5 | v1.2 | Matrix / Signal | 2 | 30% |
| M8 | v1.5 | IDENTITY / VECTOR | 2 | 40% |

팀 2~3명 병렬 시 **32.5주 → 17.5주 (~4개월)** 압축.

### 4.3 Blocker SPEC (후속 대거 의존, 리소스 집중 대상)

1. **QUERY-001** (20+ 후속): M0 최우선
2. **MEMORY-001** (8 후속: INSIGHTS/IDENTITY/VECTOR/LORA/REFLECT/HEALTH/JOURNAL/RITUAL): M4 초기
3. **ADAPTER-001** (5 후속, 모든 provider): M1 마지막
4. **HOOK-001** (5 후속: SUBAGENT/PLUGIN/SAFETY/SCHEDULER/RITUAL): M2 중앙
5. **BRIDGE-001** (Mobile + Gateway 차단): M6 중앙
6. **SCHEDULER-001** (Phase 7 전체): M7 최우선
7. **🆕 CLOUD-001** (Tier 1/2 인프라 차단): M6 초기
8. **🆕 NOTIFY-001** (Morning Push 전체 차단): M6 중간

→ **8개 Blocker에 리소스 집중**.

---

## 5. v6.2 신규 구현 고려사항

### 5.1 3-Tier 인프라 (CLOUD-001)

**Zero-Knowledge 원칙 구현**:
- STUN/TURN: Pion (Go native, OSS)
- Push Relay: 암호화 payload 저장소 (Redis + PostgreSQL), 건드릴 수 없는 blob
- Device Registry: 공개키 보관 (개인키 절대 미저장)
- Auth: bcrypt + sha256 email hash (rainbow table 방지 salt)

**배포**: Hetzner/Cloudflare Workers/Fly.io 중 선택. 지역별 edge 배치 (KR/JP/US/EU 최소).

**코드 분리**: `mink-cloud/` 별도 저장소, OSS 공개, reproducible build. 본 monorepo와 독립.

### 5.2 5-Channel 통합 (GATEWAY-001 재정의)

**umbrella SPEC**: 각 Tier A 메신저를 동일 인터페이스로 추상화
```go
type MessengerAdapter interface {
    Connect(ctx context.Context, token string) error
    Subscribe() <-chan IncomingMessage
    Send(msg OutgoingMessage) error
    Disconnect() error
}
```

**구현체 별 특성**:
- Telegram: Bot API long-polling (30초 간격), inline keyboard, MarkdownV2
- Discord: Gateway WebSocket, Slash command, Embed
- Slack: Socket Mode, Block Kit, thread reply
- Matrix: sync long-polling, E2EE 선택 (olm)
- Signal: libsignal-client (Rust FFI 필요 가능성 높음)

### 5.3 iMessage 대응 (Apple Native)

**MOBILE-001 SPEC scope 확장**:
- App Intents API (iOS 16+) — Siri Shortcut 자동 등록
- Share Extension target (Xcode) — Safari/Mail 등 공유 시트 진입
- ActivityKit (iOS 16.1+) — Live Activity 구현
- Intents.framework — "Ask MINK" custom intent

**플랫폼별 대응**:
- iPhone 15 Pro+: Action Button 설정 → MINK 즉시 호출
- Apple Watch: watchOS 10+ Smart Stack 위젯
- Mac: macOS Shortcuts.app + Global Shortcut (Desktop SPEC)

### 5.4 채널별 프라이버시 라우팅 (SAFETY-001 강화)

```go
func (s *Safety) RouteMessage(msg OutMessage, channel Channel) error {
    if msg.Category.IsSensitive() && !channel.IsE2EE() {
        return ErrChannelForbidden{
            Msg:   "Journal/Health content cannot be sent to non-E2EE channels",
            Channel: channel.Name,
        }
    }
    if detector.ContainsCrisis(msg.Body) {
        s.emergencyNotify(userCrisisHotline[msg.Locale])
        return s.requireUserConfirm(msg)
    }
    return nil
}
```

**Channel HARD rule 매트릭스**:
| Channel | Journal | Health | Identity | Calendar | Weather |
|---|---|---|---|---|---|
| Desktop (local) | ✅ | ✅ | ✅ | ✅ | ✅ |
| Mobile App | ✅ (E2EE) | ✅ | ✅ | ✅ | ✅ |
| Telegram | ❌ HARD | ❌ HARD | ❌ HARD | ✅ | ✅ |
| Discord | ❌ HARD | ❌ HARD | ❌ HARD | ✅ | ✅ |
| Slack | ❌ HARD | ❌ HARD | ❌ HARD | ✅ | ✅ |
| Matrix (E2EE) | ⚠️ opt-in | ⚠️ opt-in | ❌ HARD | ✅ | ✅ |
| Signal | ✅ | ✅ | ⚠️ opt-in | ✅ | ✅ |

### 5.5 TDD 엄격도 (v6.2 확장)

- Backend Go: 단위 85%+, integration 70%+
- Frontend Desktop: Vitest (Tauri + React)
- Frontend Mobile: Vitest (RN) + Detox (E2E)
- Cross-platform E2E: Playwright (Desktop) + Detox (Mobile)
- Bridge: integration (PC↔Mobile pairing scenarios)
- **신규**: Cloud integration test (STUN/TURN/Push 실제 작동)
- **신규**: Messenger gateway mock (Telegram Bot API mock server)
- **신규**: Channel HARD rule unit test (Journal 본문이 Telegram으로 안 나가는지 verify)

---

## 6. 인터페이스 계약 (v6.2 확장)

| 인터페이스 | 정의 SPEC | 구현 SPEC |
|---|---|---|
| `LLMCall` | QUERY-001 | ADAPTER-001 |
| `Executor` | QUERY-001 | TOOLS-001 |
| `Compactor` | CONTEXT-001 | COMPRESSOR-001 |
| `Summarizer` | COMPRESSOR-001 | ADAPTER(cheap) |
| `MemoryProvider` | MEMORY-001 | Builtin + Plugin |
| `HookHandler` | HOOK-001 | QUERY + SCHEDULER |
| `SafetyGate` | SAFETY-001 | REFLECT + Channel HARD |
| `BridgeSession` | BRIDGE-001 | DESKTOP + MOBILE |
| `CryptoProvider` | RELAY-001 | Rust mink-crypto |
| `RitualOrchestrator` | RITUAL-001 | BRIEFING + HEALTH + JOURNAL |
| `ScheduledEvent` | SCHEDULER-001 | HOOK-001 emit |
| **🆕 `CloudRelay`** | CLOUD-001 | mink-cloud 서비스 |
| **🆕 `DiscoveryProvider`** | DISCOVERY-001 | mDNS (zeroconf) |
| **🆕 `PushProvider`** | NOTIFY-001 | APNs + FCM + Desktop native |
| **🆕 `AuthProvider`** | AUTH-001 | Zero-Knowledge email + keys |
| **🆕 `SyncProvider`** | SYNC-001 | Tier 2 CRDT or op-log |
| **🆕 `MessengerAdapter`** | GATEWAY-001 | TG/DC/SL/MX/SG 개별 |

---

## 7. 예상 공수 및 Release (v6.2)

### 7.1 인력별 일정 (TDD 엄격)

| Milestone | 버전 | 순차 | 팀 2 | 팀 3 |
|---|---|---|---|---|
| M0 Foundation | v0.1 | 3주 | 2주 | 1.5주 |
| M1 Multi-LLM | v0.1 | 4주 | 3주 | 2주 |
| M2 4 Primitives | v0.2 | 5주 | 4주 | 2.5주 |
| M3 Dev CLI | v0.3 | 1.5주 | 1주 | 1주 |
| M4 Self-Evolution | v0.5 | 4주 | 3주 | 2주 |
| M5 Safety | v0.5 | 2.5주 | 2주 | 1.5주 |
| **M6 Cross-Platform + Loc + 3-Tier** | **v0.4** | **9주** | **7주** | **5주** |
| **M7 Daily Companion + Telegram** | **v1.0** | **6주** | **4.5주** | **3주** |
| M7.5 Msgr 확장 | v1.1~1.2 | 3주 | 2.5주 | 2주 |
| M8 Personalization | v1.5 | 5주 | 4주 | 3주 |
| **M0~M7 → v1.0** | | **35주** | **26.5주** | **17.5주** |
| M0~M8 → v1.5 | | 43주 | 33주 | 22.5주 |

### 7.2 Release 타임라인 (v6.2 확정)

| Release | Milestone | Tier | Channel | 핵심 기능 |
|---|---|---|---|---|
| v0.1 Alpha | M0~M1 | Tier 0 | CLI | `mink ask` 헤드리스 |
| v0.2 Beta | M0~M2 | Tier 0 | CLI | 4 Primitive |
| v0.3 Beta | M0~M3 | Tier 0 | CLI | Dev CLI 안정화 |
| **v0.4 Public Beta** | **M0~M6** | **Tier 0+1** | **Ch.1~3** | **Desktop+Mobile+Apple Native** |
| v0.5 RC | +M4+M5 | Tier 0+1 | Ch.1~3 | Safety |
| **v1.0 Release** | **M0~M7** | **Tier 0+1** | **Ch.1~4(TG)** | **Daily Companion + Telegram** |
| v1.1 | +GATEWAY-DC/SL | Tier 0+1 | +Discord+Slack | 협업 도구 확장 |
| v1.2 | +GATEWAY-MX/SG | Tier 0+1 | +Matrix+Signal E2EE | 프라이버시 파워 유저 |
| v1.3 | +Tier 2 | Tier 0+1+2 | Ch.1~4 | PC OFF fallback + 백업 |
| v1.5 | +M8 | 전 Tier | Ch.1~4 | LoRA 개인화 |
| **v2.0** | +M9+Biz+SMS+Voice | 전 Tier | **Ch.1~5** | Kakao/WeChat/LINE/SMS/A2A (사업자) |

---

## 8. 즉시 실행 가능한 다음 액션

### 8.1 선결 의사결정 6건 확정 (v0.1 진입 전)
1. Go 버전 고정 (1.26+ 권장)
2. sqlite 드라이버 선택 (modernc vs mattn)
3. Tokenizer 라이브러리 (tiktoken-go vs 자체)
4. proto commit 정책 (매 변경마다 vs 주기적)
5. LLM Stream 인터페이스 규약 (SSE vs gRPC streaming)
6. Rust crate 배포 방식 (embedded vs 별도 바이너리)

### 8.2 병행 사전 준비
- `internal/contracts/` 인터페이스 패키지 (17개 interface 선제 선언, +5 v6.2)
- `.moai/project/security.md` (privacy 거버넌스, Channel HARD rule 명문화)
- Rust `crates/mink-ml/`, `crates/mink-crypto/`, `crates/mink-signal/` 리포 초기화
- `proto/` 디렉토리 + Connect-gRPC 스키마
- `packages/mink-desktop/` (Tauri), `packages/mink-mobile/` (RN) 스캐폴드
- `mink-cloud/` 별도 리포 스캐폴드 (v6.1~6.2 신규)
- Tauri updater ed25519 키페어 + CI secret
- APNs p8 Key + FCM Project (v6.0 신규)

### 8.3 첫 SPEC 실행
```bash
/moai run SPEC-GOOSE-CORE-001
```

### 8.4 v0.4 진입 전 준비 (M6 시작 4주 전)
- Apple Developer 계정 (iOS TestFlight)
- Google Play Console (Android internal testing)
- GeoLite2 DB 배포 전략 확정
- 20+ 언어 번역 1차 초안 (Tier 1 4개 네이티브 + Tier 2 LLM 초벌)
- Cloud 서비스 지역 선정 (KR/JP/US/EU)

### 8.5 v1.0 진입 전 준비 (M7 시작 2주 전)
- Telegram @BotFather 사용자 가이드 문서화
- 식약처 DUR API 접근 신청 (HEALTH-001)
- Naver Calendar API 파트너 문의 (CALENDAR-001)
- 20+ 언어 Tier 2 커뮤니티 리뷰어 10명 섭외

---

## 9. v6.2 최종 권장 순서

```
M0 Foundation (2주)        → v0.1 Alpha
  CORE → [CONFIG ∥ TRANSPORT] → QUERY★ → CONTEXT

M1 Multi-LLM (3주)         → v0.1 Alpha
  CREDPOOL → [ROUTER ∥ RATELIMIT ∥ ERROR-CLASS] → PROMPT-CACHE → ADAPTER★

M2 4 Primitives (4주)      → v0.2 Beta
  [SKILLS ∥ HOOK ∥ MCP] → SUBAGENT → PLUGIN

M3 Dev CLI (1주)           → v0.3 Beta
  TOOLS → COMMAND → CLI

M4 Self-Evolution (3주)
  [TRAJECTORY ∥ MEMORY] → COMPRESSOR → INSIGHTS

M5 Safety (2주)
  REFLECT → SAFETY(+Ch-HARD) → ROLLBACK

M6 Cross-Platform + Loc + 3-Tier (6주) → v0.4 Public Beta ★
  W1: [LOCALE → I18N → REGION-SKILLS]
  W2: [DISCOVERY ∥ AUTH] → CLOUD + NOTIFY
  W3-4: DESKTOP → ONBOARDING(6단계)
  W5: BRIDGE → [RELAY ∥ MOBILE(+Siri+Widget+Live Activity)]
  W6: GATEWAY(umbrella) + WIDGET finalize

M7 Daily Companion + Telegram (4.5주) → v1.0 Release ★
  W1-3: SCHEDULER → [WEATHER ∥ CALENDAR ∥ HEALTH] → FORTUNE → [BRIEFING ∥ JOURNAL] → RITUAL
  W4(병행): GATEWAY-TG-001 (Telegram long-polling bot)
  W4.5: 통합 테스트

M7.5 Msgr 확장 (2주) → v1.1~1.2
  [GATEWAY-DC ∥ GATEWAY-SL] → [GATEWAY-MX(E2EE) ∥ GATEWAY-SG(E2EE HARD)]

M8 Personalization (4주) → v1.5
  [IDENTITY ∥ VECTOR] → LORA (Rust)

M9 Ecosystem (옵션) → v2.0
  [GATEWAY-KR ∥ GATEWAY-CN ∥ GATEWAY-JP] → GATEWAY-SMS → VOICE → A2A
```

**첫 실행 커맨드**:
```bash
cd /Users/goos/MoAI/AgentOS
# 선결 의사결정 6건 확정 후
/moai run SPEC-GOOSE-CORE-001
```

---

**Version**: 6.2.0
**License**: Apache-2.0
**Next action**: 선결 의사결정 6건 확정 → `SPEC-GOOSE-CORE-001` TDD RED → M0 진입 → v0.1 Alpha → v0.4 Public Beta → **v1.0 Release (Daily Companion + Telegram Remote Control + Zero-Knowledge Cloud)**
