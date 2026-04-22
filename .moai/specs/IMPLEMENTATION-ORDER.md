# GOOSE-AGENT 구현 순서 종합 보고서 v4.0

> **작성일**: 2026-04-22 (v4.0 Cross-Platform + Daily Companion 통합)
> **대상**: 43 SPEC (Phase 0~9 전체)
> **근거**: ROADMAP v4.0 + 10 Phase 신규 작성 에이전트 보고 종합
> **방법론**: TDD (RED→GREEN→REFACTOR)
> **목적**: 의존성 그래프 기반 최적 구현 순서 + 병렬화 지점 + Milestone

---

## 0. 요약

- **전체**: 43 active SPEC, 약 **720+ REQ / 420+ AC**
- **Go LoC**: ~35,000 + Rust LoRA/crypto crate 위임
- **Critical Path**: CORE → QUERY → CREDPOOL → ADAPTER → HOOK → SUBAGENT → DESKTOP → BRIDGE → MOBILE → SCHEDULER → BRIEFING → JOURNAL → RITUAL
- **10 Milestone**: M0(Core) → M1(LLM) → M2(Primitives) → M3(Dev CLI) → M4(Evolution) → M5(Safety) → **M6(Cross-Platform)** → **M7(Daily Companion)** → M8(Personalization) → M9(Ecosystem)
- **v1.0 Release 기준**: M7 완료 시점 (일상 반려 AI 완성) ~23주

---

## 1. 43 SPEC 전체 목록

### Phase 0 — Agentic Core (5)
CORE · QUERY★ · CONTEXT · TRANSPORT · CONFIG

### Phase 1 — Multi-LLM Infrastructure (5)
CREDPOOL★ · ROUTER · RATELIMIT · PROMPT-CACHE · ADAPTER★

### Phase 2 — 4 Primitives (5)
SKILLS · MCP · HOOK★ · SUBAGENT · PLUGIN

### Phase 3 — Agentic Primitives (3)
TOOLS · COMMAND · CLI (개발자용)

### Phase 4 — Self-Evolution (5)
TRAJECTORY · COMPRESSOR · INSIGHTS · ERROR-CLASS★ · MEMORY★

### Phase 5 — Promotion & Safety (3)
REFLECT★ · SAFETY · ROLLBACK

### **Phase 6 — Cross-Platform Clients (5) 🆕 v4.0**
DESKTOP★ · BRIDGE★ · RELAY · MOBILE★ · GATEWAY

### Phase 7 — Daily Companion (8)
SCHEDULER★ · WEATHER · FORTUNE · CALENDAR★ · BRIEFING★ · HEALTH · JOURNAL★ · RITUAL★

### Phase 8 — Deep Personalization (3)
IDENTITY · VECTOR · LORA

### Phase 9 — Ecosystem (1)
A2A

★ = Critical Path (14건)

---

## 2. 상세 의존성 그래프 (Cross-Phase)

```
[Phase 0 Foundation]
CORE ─┬─ CONFIG ────────────────────────┐
      │                                   │
      ├─ TRANSPORT ───────────────────────┤
      │                                   │
      └─ QUERY ── CONTEXT                 │
                    │                     │
[Phase 1 Multi-LLM] │                     │
                    │                     │
   CREDPOOL ─┬─ ROUTER ─┬─ ADAPTER ◀─────┤
             ├─ RATELIMIT│      │         │
             └─ PROMPT-CACHE    │         │
                                │         │
[Phase 2 Primitives]            │         │
   SKILLS ── HOOK ── SUBAGENT ─┼──── MCP ─┤
                 │                         │
[Phase 3 Primitives]                      │
   TOOLS ── COMMAND ── CLI (개발용)        │
                                           │
[Phase 4 Self-Evolution]                  │
   MEMORY ─┬─ TRAJECTORY ── COMPRESSOR   │
           ├─ INSIGHTS                    │
           └─ ERROR-CLASS ◀────────────────┘
                        │
[Phase 5 Safety]        │
   REFLECT ── SAFETY ── ROLLBACK

[Phase 6 ★v4.0 Cross-Platform]
   DESKTOP (Tauri) ◀────── BRIDGE ◀───── RELAY (Rust)
        │                    ▲            │
        └─ QR pairing         │            │
                              ▼            │
                     MOBILE (RN) ──────────┘
                              │
                     GATEWAY (Telegram/Discord/KakaoTalk)

[Phase 7 Daily Companion]
   SCHEDULER ──┬─ WEATHER
               ├─ FORTUNE (← IDENTITY, Phase 8)
               ├─ CALENDAR
               └─ HEALTH
                    │
                BRIEFING + JOURNAL ── RITUAL

[Phase 8 Personalization]
   IDENTITY ── VECTOR ── LORA (Rust 위임)

[Phase 9]
   A2A-001
```

---

## 3. 최적 구현 순서 (Milestone별)

### M0 — Agentic Foundation (2주)
```
CORE → [CONFIG + TRANSPORT 병렬] → QUERY★ → CONTEXT
```
**완료 기준**: Mock LLM으로 `<-chan SDKMessage` streaming 성공

### M1 — Multi-LLM + Error Handling (3주)
```
CREDPOOL → [ROUTER + RATELIMIT + ERROR-CLASS 병렬 3개]
        → PROMPT-CACHE → ADAPTER★ (Anthropic + OpenAI 먼저)
```
**완료 기준**: `goose ask "hello"` → 실제 Anthropic/OpenAI 응답

### M2 — 4 Primitives (4주)
```
[SKILLS + HOOK + MCP 병렬 3개] → SUBAGENT → PLUGIN
```
**완료 기준**: 외부 MCP, Sub-agent fork, Skill 로드 동작

### M3 — Developer CLI (1주) ← 범위 축소 (v3.0 대비)
```
TOOLS → COMMAND → CLI (개발·디버그·헤드리스 전용)
```
**완료 기준**: `goose ask --json --prompt "..."` 헤드리스 모드

### M4 — Self-Evolution (3주)
```
[TRAJECTORY + MEMORY 병렬] → COMPRESSOR → INSIGHTS
```

### M5 — Safety (2주)
```
REFLECT → SAFETY → ROLLBACK
```

### **M6 — Cross-Platform Clients (4주) 🆕 v0.4 Public Beta**
```
DESKTOP (Tauri v2) ─ 기본 UI 시작
        │
        └─ BRIDGE (Claude Code bridge/ 포팅)
                  │
                  ├─ RELAY (Rust crate) 병렬 가능
                  │
                  └─ MOBILE (React Native)
                              │
                              └─ GATEWAY (옵션, 후속)
```
**완료 기준**:
- PC Desktop App 실행 → goosed daemon 자동 시작 → 메인 채팅 + 트레이 아이콘
- Mobile App에서 QR 페어링 → PC와 원격 세션 수립
- 아침 리추얼 푸시 알림 (Phase 7과 연계)
- v0.4 Public Beta Release 가능

### **M7 — Daily Companion (4주) 🏆 v1.0 Release**
```
SCHEDULER → [WEATHER + CALENDAR + HEALTH 병렬 3개]
         → FORTUNE → BRIEFING + JOURNAL → RITUAL
```
**완료 기준**:
- 07:00 자동 아침 브리핑 (운세 + 날씨 + 일정)
- 식사 후 건강/약 알림
- 23:30 저녁 일기 자동 프롬프트
- Mobile Push로 리추얼 알림
- **v1.0 Release Milestone = 일상 반려 AI 완성**

### M8 — Deep Personalization (4주) v1.5
```
[IDENTITY + VECTOR 병렬] → LORA (Rust 위임)
```
**완료 기준**: Weekly QLoRA 재훈련, 개체 고유성 확립

### M9 — Ecosystem (옵션) v2.0
A2A-001

---

## 4. Critical Path 분석 (업데이트)

### 4.1 최단 경로 (순차 필수)
```
CORE → CONFIG → TRANSPORT → QUERY → CONTEXT
→ CREDPOOL → ROUTER → ADAPTER
→ HOOK → SUBAGENT
→ TOOLS
→ DESKTOP → BRIDGE → MOBILE          [v0.4 Public Beta]
→ TRAJECTORY → MEMORY → INSIGHTS
→ REFLECT → SAFETY
→ SCHEDULER → BRIEFING + JOURNAL → RITUAL [v1.0 Release]
→ VECTOR → LORA                       [v1.5]
```
총 **25 SPEC**. 나머지 18 SPEC은 critical path 외.

### 4.2 병렬화 기회

| Milestone | 병렬 그룹 | 병렬 수 | 절감 |
|-----------|---------|-------|------|
| M0 | CONFIG/TRANSPORT | 2 | 30% |
| M1 | ROUTER/RATELIMIT/ERROR-CLASS | 3 | 40% |
| M2 | SKILLS/HOOK/MCP | 3 | 50% |
| M4 | TRAJECTORY/MEMORY | 2 | 30% |
| **M6** | **BRIDGE/RELAY → MOBILE** | 부분 | 25% |
| **M7** | **WEATHER/CALENDAR/HEALTH** | 3 | 40% |
| M8 | IDENTITY/VECTOR | 2 | 40% |

팀 2~3명 병렬 시: **23주 → 17주 (~4개월)**로 압축 가능.

### 4.3 Blocker SPEC (후속 대거 의존)

1. **QUERY-001** — 20+ 후속 SPEC 의존
2. **MEMORY-001** — 8 후속 (INSIGHTS/IDENTITY/VECTOR/LORA/REFLECT/HEALTH/JOURNAL/RITUAL)
3. **ADAPTER-001** — 5 후속
4. **HOOK-001** — 5 후속 (SUBAGENT/PLUGIN/SAFETY/SCHEDULER/RITUAL)
5. **🆕 BRIDGE-001** — Mobile + Gateway 차단
6. **🆕 SCHEDULER-001** — Phase 7 전체 차단

→ 6 blocker에 리소스 집중 권장.

---

## 5. v4.0 신규 구현 고려사항

### 5.1 Desktop App 기본 UI 전환 (Phase 6 핵심)

**기존 v3.0**: CLI = MVP
**v4.0**: Desktop App = MVP, CLI = 개발·헤드리스 보조

배포 대상:
- macOS (Intel + Apple Silicon)
- Linux (Ubuntu/Fedora/Arch)
- Windows 10/11

Tauri v2 선택 이유:
- React + Rust backend = 가벼움(~10MB) + 안전성
- Native OS 통합 (tray, notification, keyboard shortcut)
- Auto-update via Tauri updater

CI 필요: GitHub Actions 5 플랫폼 매트릭스 빌드 + ed25519 서명

### 5.2 Mobile Companion 페어링 (v4.0 핵심)

**첫 사용 플로우**:
1. PC Desktop App 실행 → QR 코드 표시
2. Mobile App 스캔 → JWT 교환 → Trusted Device 등록
3. Bridge 세션 수립 → E2EE (Noise Protocol via RELAY)
4. 이후: Mobile은 Bridge 경유 PC 제어 가능

**보안 계층**:
- PC 로컬 저장 (일기/건강/Identity Graph)
- JWT 24h 만료 + refresh token
- Biometric lock (Mobile 로컬 캐시)
- Post-quantum preshared key 옵션

### 5.3 Phase 6 ↔ Phase 7 긴밀 결합

Daily Rituals는 Desktop/Mobile UI 없이 체감 불가:
- 🌅 Morning: Desktop 트레이 팝업 OR Mobile 푸시
- 🍽️ Meals: Mobile 푸시 (외출 중에도 체감)
- 🌙 Evening: Desktop 자연스러운 프롬프트

→ **M6 완료 전 M7 시작 불가** (의존 관계 엄격).

### 5.4 TDD 엄격도 (강화)

Phase 6 Desktop/Mobile은 UI 포함:
- Backend: Go unit test (단위당 85%+)
- Frontend: Vitest/RTL (React), Detox (RN E2E)
- Cross-platform E2E: Playwright (Desktop), Detox (Mobile)
- Bridge: integration test (PC↔Mobile pairing scenarios)

---

## 6. 인터페이스 계약 (v4.0 확장)

| 인터페이스 | 정의 SPEC | 구현 SPEC |
|----------|---------|---------|
| `LLMCall` | QUERY-001 | ADAPTER-001 |
| `Executor` (tool runner) | QUERY-001 | TOOLS-001 |
| `Compactor` | CONTEXT-001 | COMPRESSOR-001 |
| `Summarizer` | COMPRESSOR-001 | ADAPTER(cheap) |
| `MemoryProvider` | MEMORY-001 | Builtin + Plugin |
| `HookHandler` | HOOK-001 | QUERY + SCHEDULER |
| `SafetyGate` | SAFETY-001 | REFLECT 소비 |
| **🆕 `BridgeSession`** | BRIDGE-001 | DESKTOP + MOBILE |
| **🆕 `CryptoProvider`** | RELAY-001 | Rust goose-crypto |
| **🆕 `RitualOrchestrator`** | RITUAL-001 | BRIEFING + HEALTH + JOURNAL |
| **🆕 `ScheduledEvent`** | SCHEDULER-001 | HOOK-001 emit |
| **🆕 `GatewayProvider`** | GATEWAY-001 | 7 플랫폼 구현 |

---

## 7. 예상 공수 및 Release

### 7.1 인력별 일정 (TDD 엄격)

| Milestone | 순차 | 팀 2명 | 팀 3명 |
|-----------|------|--------|--------|
| M0 Foundation | 3주 | 2주 | 1.5주 |
| M1 Multi-LLM | 4주 | 3주 | 2주 |
| M2 4 Primitives | 5주 | 4주 | 2.5주 |
| M3 Dev CLI | 1.5주 | 1주 | 1주 |
| M4 Self-Evolution | 4주 | 3주 | 2주 |
| M5 Safety | 2주 | 2주 | 1.5주 |
| **M6 Cross-Platform** | **6주** | **4주** | **2.5주** |
| **M7 Daily Companion** | **5주** | **4주** | **2.5주** |
| M8 Personalization | 5주 | 4.5주 | 3주 |
| **M0~M7 v1.0** | **30.5주** | **23주** | **15.5주** |
| M0~M8 v1.5 | 35.5주 | 27.5주 | 18.5주 |

### 7.2 Release 타임라인

| Release | Milestone 포함 | 핵심 기능 |
|---------|-------------|---------|
| v0.1 Alpha | M0~M1 | goose ask CLI 동작 (헤드리스) |
| v0.2 Beta | M0~M2 | 4 Primitive 완성 |
| v0.3 Beta | M0~M3 | Developer CLI 안정화 |
| **v0.4 Public Beta** | **M0~M6** | **Desktop + Mobile + Bridge** |
| v0.5 RC | + M5 | Safety 게이트 + 품질 |
| **v1.0 Release** | **M0~M7** | **일상 반려 AI 완성** ✨ |
| v1.5 | + M8 | 개인화 LoRA |
| v2.0 | + M9 | A2A + Ecosystem |

---

## 8. 즉시 실행 가능한 다음 액션

### 8.1 권장: Phase 0 CORE-001부터 TDD RED
```
/moai run SPEC-GOOSE-CORE-001
```
- manager-tdd 서브에이전트가 AC-CORE-01~06 실패 테스트 작성
- Go 1.26+ 버전 확정 후 go.mod 초기화
- cmd/goosed + internal/core + internal/health 스켈레톤

### 8.2 병행 준비 작업
- **`internal/contracts/` 인터페이스 패키지** 선제 생성 (14개 interface 순수 선언)
- `.moai/project/security.md` 작성 (privacy 거버넌스)
- Rust `crates/goose-ml/`, `crates/goose-crypto/` 별도 리포 초기화 준비
- `proto/` 디렉토리 초기 스키마 + Connect-gRPC client/server proto
- **🆕 `packages/goose-desktop/`, `packages/goose-mobile/` 스캐폴드 사전 배치 (Tauri create + RN init)**
- **🆕 Tauri updater ed25519 키페어 생성 + CI secret 등록**

### 8.3 의사결정 필요 항목 (구현 진입 전 확정, 20건)

위 ROADMAP §12 참조. 주요 6건:
1. Go 버전 고정
2. Rust crate 배포 방식 (embedded vs 별도 바이너리)
3. Tokenizer 선택
4. Graph DB (Kuzu vs Neo4j)
5. Tauri updater 키 관리
6. 카카오 알림톡 벤더 (Solapi vs 직접 계약)

---

## 9. v4.0 최종 권장 순서

```
M0 Foundation (2주)
  CORE → [CONFIG + TRANSPORT] 병렬 → QUERY★ → CONTEXT

M1 Multi-LLM (3주)
  CREDPOOL → [ROUTER + RATELIMIT + ERROR-CLASS] 병렬 → PROMPT-CACHE → ADAPTER★

M2 4 Primitives (4주)
  [SKILLS + HOOK + MCP] 병렬 → SUBAGENT → PLUGIN

M3 Dev CLI (1주)
  TOOLS → COMMAND → CLI

M4 Self-Evolution (3주)
  [TRAJECTORY + MEMORY] 병렬 → COMPRESSOR → INSIGHTS

M5 Safety (2주)
  REFLECT → SAFETY → ROLLBACK

M6 Cross-Platform (4주) ← v0.4 Public Beta
  DESKTOP → BRIDGE → [RELAY + MOBILE] 부분병렬 → GATEWAY

M7 Daily Companion (4주) ← v1.0 Release
  SCHEDULER → [WEATHER + CALENDAR + HEALTH] 병렬
           → FORTUNE → BRIEFING + JOURNAL → RITUAL

M8 Personalization (4주) ← v1.5
  [IDENTITY + VECTOR] 병렬 → LORA (Rust 위임)

M9 Ecosystem (선택, v2.0)
  A2A
```

**첫 실행 커맨드** (Go 버전 확정 후):
```bash
cd /Users/goos/MoAI/AgentOS
/moai run SPEC-GOOSE-CORE-001
```

---

**Version**: 4.0.0
**License**: MIT (본 문서 포함)
**Next action**: 최종 승인 → Go 버전 확정 → `/moai run SPEC-GOOSE-CORE-001` TDD RED 진입
