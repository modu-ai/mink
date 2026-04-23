# GOOSE-AGENT SPEC 로드맵 v6.2 (★ v0.2 재편 amendment 적용)

> **⚠ v0.2 Amendment (2026-04-24)**
> SPEC-GOOSE-ARCH-REDESIGN-v0.2 확정본에 따라 주요 변경 발생. 아래 본문은 v6.2 기준이며, **실제 구현 우선순위와 범위는 재편본을 따른다**.
> 자세한 재편 내용: `.moai/design/goose-runtime-architecture-v0.2.md` + `.moai/specs/SPEC-GOOSE-ARCH-REDESIGN-v0.2/spec.md`
>
> **v0.2 주요 변화 요약**:
> - **삭제** (5건): MOBILE-001, WIDGET-001, SYNC-001, CLOUD-001, DISCOVERY-001
> - **축소** (4건): AUTH-001, NOTIFY-001, BRIDGE-001, ONBOARDING-001
> - **신규** (9건): QMD-001, WEBUI-001, SELF-CRITIQUE-001, PAI-CONTEXT-001, PERMISSION-001, SECURITY-SANDBOX-001, CREDENTIAL-PROXY-001, FS-ACCESS-001, AUDIT-001
> - **채널 변경**: v0.1 Alpha는 CLI/TUI + Telegram + Web UI 3종 (Email 제거, Mobile 제거, Apple Native 제거)
> - **스토리지 2원화**: `~/.goose/` (secrets only) + `./.goose/` (workspace)
> - **Module path**: `github.com/modu-ai/goose`
> - **SPEC 총량**: 58 → 약 54 (-5 +9 = +4 순증)
>
> 기존 v6.2 본문은 **참조용**으로 보존되며, 새 Milestone 순서는 v0.2 문서의 §10을 기준으로 한다.

---

> **프로젝트**: GOOSE — Daily Companion AI (self-hosted, project-local workspace)
> **작성일**: 2026-04-22 (v6.2) · 2026-04-24 (v0.2 amendment)
> **이전 버전**: v5.0 (47 SPEC) · v6.0 (48) · v6.1 (52) · v6.2 (58) · **v0.2 (54, 재편본)**
> **대화 언어**: 한국어 · **코드 식별자**: 영어
> **개발 방법론**: TDD (`development_mode: tdd`)
> **라이선스**: MIT
> **상태**: 아키텍처 재설계 완료 (v0.2), M0 CORE-001 구현 완료

---

## 0. v6.2 재설계 근거 (Progressive Evolution)

### 진화 경로
| 버전 | 핵심 변화 | SPEC 수 |
|---|---|---|
| v4.0 | Cross-Platform (Desktop+Mobile) 추가 | 43 |
| v5.0 | Global Localization (20+ 언어, 국가별 Skill) | 47 |
| v6.0 | Hermes Gateway 폐기 → App-First | 48 |
| v6.1 | 3-Tier (Local/Cloud Free/Cloud Plus) | 52 |
| **v6.2** | **5-Channel (LAN/Cloud/Apple/Self-msgr/Biz-msgr) + Gateway 재정의** | **58** |

### v6.2 핵심 통찰

1. **Hermes Gateway는 2종류로 분리**:
   - Tier A (**클라우드 0, 계정 0**): Telegram/Discord/Slack/Matrix/Signal — PC에서 long-polling만으로 작동
   - Tier B (**사업자 등록 필수**): Kakao/WeChat/LINE — v2.0+ 분리
2. **iMessage 직접 통합 불가** (Apple 정책): Siri Shortcut + Share Sheet + Live Activity로 대체
3. **Tier A 메신저는 Tier 1 Cloud의 강력한 대안**: Tier 0 + Telegram만으로도 외부망 대화+푸시 확보

### 사용자 누적 지시 (2026-04-22 세션)

> "에르메스처럼 챗봇 연결 X, 앱 개발해서 연동" (1차)
> "모바일↔PC 바로 연결되나? 클라우드 필요? 메신저 환경 복구?" (2차)
> "텔레그램에서 PC GOOSE에게 지시 가능한 Hermes 패턴 복원" (3차)
> "iMessage 추가 가능하면 추가" (4차)

---

## 1. 네이밍 규약

형식: `SPEC-GOOSE-{DOMAIN}-{NNN}`

**DOMAIN 총계**:
- v2.0 core (22) + v3.0 Daily Companion (8) + v4.0 Cross-Platform (5) + v5.0 Localization (4) + v6.0 App-First (2: NOTIFY/WIDGET) + v6.1 3-Tier (5: CLOUD/DISCOVERY/SYNC/AUTH/VOICE(opt)) + v6.2 Gateway 확장 (5: GATEWAY-TG/DC/SL/MX/SG) = **51**
- 추가 umbrella: GATEWAY-001 재정의
- v2.0+ 옵션: GATEWAY-KR/CN/JP, SMS (Twilio) = 4

**활성 SPEC 58개** (v1.x 범위), **옵션 4개** (v2.0+).

---

## 2. 우선순위 / 범위

| 우선 | 정의 | | 범위 | LoC 기준 |
|---|---|---|---|---|
| P0 | blocker | | S | ~500~1500 |
| P1 | 가치 핵심 | | M | ~1500~4000 |
| P2 | 차별화 | | L | ~4000~8000 |

---

## 3. 3-Tier × 5-Channel 아키텍처 ★ v6.2 신규

### 3.1 Tier 축 (사용자 선택제)

| Tier | 회원가입 | 월 비용 | 기능 범위 |
|---|---|---|---|
| **Tier 0 Local** (기본값) | 없음 | $0 | Desktop 완전 + Mobile 홈WiFi + Self-msgr |
| **Tier 1 Cloud Free** (권장) | 이메일 1회 | $0 | +Mobile 외부망 + APNs/FCM 푸시 + Live Activity |
| **Tier 2 Cloud Plus** | 이메일 + 결제 | $9/mo | +PC OFF fallback + 암호화 백업 + 멀티기기 |

Cloud는 **Zero-Knowledge**: 이메일 해시, 장치 공개키, 암호화 APNs token, 암호화 push payload만 저장. Journal/Health/Identity Graph는 영구 로컬.

### 3.2 Channel 축 (통신 채널)

| Ch. | 이름 | 대상 플랫폼 | 클라우드? | 버전 |
|---|---|---|---|---|
| **Ch.1** | Mobile App LAN | iOS/Android (홈 WiFi) | ❌ mDNS | v0.4 |
| **Ch.2** | Mobile App Cloud | iOS/Android (외부망) | Tier 1+ 필요 | v0.4 |
| **Ch.3** | Apple Native | Siri Shortcut + Share Sheet + Live Activity | ❌ 온디바이스 | v1.0 |
| **Ch.4** | Self-hosted Messenger | Telegram/Discord/Slack/Matrix/Signal | ❌ long-polling | v1.0~1.2 |
| **Ch.5** | Business Messenger | Kakao/WeChat/LINE/SMS | 사업자 등록 | v2.0+ |

### 3.3 Tier × Channel 직교성

사용자는 Tier × Channel의 임의 조합을 선택 가능 (12가지 패턴). 예시 조합:

- **프라이버시 극단주의자**: Tier 0 + Ch.1 + Ch.4 Signal (E2EE)
- **일반 Telegram 사용자**: Tier 0 + Ch.4 Telegram (클라우드 0으로 완전 사용)
- **앱 중심 권장 프리셋**: Tier 1 + Ch.1 + Ch.2 + Ch.3 Apple Native
- **출장족**: Tier 2 + 전 채널
- **B2B 한국 시장 (v2.0)**: Tier 1 + Ch.5 Kakao

---

## 4. 전체 SPEC 목록 (58건, 10 Phase)

### Phase 0 — Agentic Core (5 SPEC, P0)
| # | SPEC-ID | 제목 | 우선 | 범위 |
|---|---|---|---|---|
| 01 | `SPEC-GOOSE-CORE-001` | goosed 데몬 부트스트랩 | P0 | S |
| 02 | `SPEC-GOOSE-QUERY-001` ★ | QueryEngine + queryLoop | P0 | L |
| 03 | `SPEC-GOOSE-CONTEXT-001` | Context Window + compaction | P0 | M |
| 04 | `SPEC-GOOSE-TRANSPORT-001` | gRPC 서버 + proto | P0 | M |
| 05 | `SPEC-GOOSE-CONFIG-001` | 계층형 설정 로더 | P0 | S |

### Phase 1 — Multi-LLM Infrastructure (5 SPEC, P0) ★
| 06 | `SPEC-GOOSE-CREDPOOL-001` ★ | Credential Pool (OAuth/API) | P0 | L |
| 07 | `SPEC-GOOSE-ROUTER-001` | Smart Model Routing | P0 | M |
| 08 | `SPEC-GOOSE-RATELIMIT-001` | Rate Limit Tracker | P0 | S |
| 09 | `SPEC-GOOSE-PROMPT-CACHE-001` | Prompt Caching | P1 | S |
| 10 | `SPEC-GOOSE-ADAPTER-001` ★ | 6 Provider 어댑터 | P0 | L |

### Phase 2 — 4 Primitives (5 SPEC, P0) ★
| 11 | `SPEC-GOOSE-SKILLS-001` | Progressive Disclosure Skill | P0 | L |
| 12 | `SPEC-GOOSE-MCP-001` | MCP Client/Server | P0 | L |
| 13 | `SPEC-GOOSE-HOOK-001` ★ | 24 Lifecycle Hooks + permission | P0 | M |
| 14 | `SPEC-GOOSE-SUBAGENT-001` | Sub-agent Runtime | P0 | L |
| 15 | `SPEC-GOOSE-PLUGIN-001` | Plugin Host | P1 | M |

### Phase 3 — Agentic Primitives (3 SPEC, P0)
| 16 | `SPEC-GOOSE-TOOLS-001` | Tool Registry + ToolSearch | P0 | M |
| 17 | `SPEC-GOOSE-COMMAND-001` | Slash Command System | P1 | S |
| 18 | `SPEC-GOOSE-CLI-001` | goose CLI (개발·헤드리스용) | P0 | M |

### Phase 4 — Self-Evolution (5 SPEC, P0) ★
| 19 | `SPEC-GOOSE-TRAJECTORY-001` | Trajectory 수집 | P0 | S |
| 20 | `SPEC-GOOSE-COMPRESSOR-001` | Trajectory Compressor | P0 | M |
| 21 | `SPEC-GOOSE-INSIGHTS-001` | Insights 추출 | P1 | M |
| 22 | `SPEC-GOOSE-ERROR-CLASS-001` ★ | Error Classifier (14 FailoverReason) | P0 | S |
| 23 | `SPEC-GOOSE-MEMORY-001` ★ | Pluggable Memory Provider | P0 | M |

### Phase 5 — Promotion & Safety (3 SPEC, P1) ★
| 24 | `SPEC-GOOSE-REFLECT-001` ★ | 5-tier 승격 | P1 | L |
| 25 | `SPEC-GOOSE-SAFETY-001` | 5-layer Safety + Channel HARD rule | P1 | M |
| 26 | `SPEC-GOOSE-ROLLBACK-001` | Regression Rollback | P1 | S |

### Phase 6 — Cross-Platform + Localization + 3-Tier (15 SPEC, P0~P2) ★ v6.2 핵심

**6a. Cross-Platform (5 SPEC, v4.0)**
| 27 | `SPEC-GOOSE-DESKTOP-001` ★ | Tauri v2 Desktop App + Tray + Widget + Global Shortcut | P0 | L |
| 28 | `SPEC-GOOSE-BRIDGE-001` ★ | PC↔Mobile + mDNS + STUN/TURN 계단식 fallback | P0 | L |
| 29 | `SPEC-GOOSE-RELAY-001` | E2EE Relay 클라이언트 (Noise Protocol, Rust crate) | P1 | L |
| 30 | `SPEC-GOOSE-MOBILE-001` ★ | RN + Widget + Live Activity + Siri Shortcut + Share Sheet | P0 | L |
| 31 | `SPEC-GOOSE-GATEWAY-001` ★ | **Self-hosted Messenger Bridge umbrella (Tier A 5종)** | P1 | L |

**6b. Global Localization (4 SPEC, v5.0)**
| 32 | `SPEC-GOOSE-LOCALE-001` ★ | OS locale + IP fallback | P0 | S |
| 33 | `SPEC-GOOSE-I18N-001` ★ | 20+ 언어 UI (go-i18n + i18next + ICU + RTL) | P0 | M |
| 34 | `SPEC-GOOSE-REGION-SKILLS-001` | 국가별 Skill 자동 활성화 | P1 | M |
| 35 | `SPEC-GOOSE-ONBOARDING-001` ★ | 6단계 온보딩 (Tier 선택 포함) | P0 | M |

**6c. 3-Tier Infrastructure (4 SPEC, v6.1)**
| 36 | `SPEC-GOOSE-CLOUD-001` ★ | Zero-Knowledge Thin Cloud (STUN+TURN+Push+Registry) | P0 | L |
| 37 | `SPEC-GOOSE-DISCOVERY-001` | mDNS LAN P2P 발견 + QR pairing | P0 | S |
| 38 | `SPEC-GOOSE-AUTH-001` | 이메일+장치키 Zero-Knowledge 관리 | P0 | M |
| 39 | `SPEC-GOOSE-SYNC-001` | 멀티 디바이스 암호화 싱크 (Tier 2) | P1 | M |

**6d. App Native Notifications (2 SPEC, v6.0)**
| 40 | `SPEC-GOOSE-NOTIFY-001` | 통합 Push (APNs + FCM + Desktop native) | P0 | M |
| 41 | `SPEC-GOOSE-WIDGET-001` | iOS/Android/Desktop Widget + Live Activity | P1 | M |

### Phase 7 — Daily Companion (8 SPEC, P0~P1) ★ v3.0 유지
| 42 | `SPEC-GOOSE-SCHEDULER-001` ★ | Proactive Cron Scheduler | P0 | M |
| 43 | `SPEC-GOOSE-WEATHER-001` | 날씨 (OpenWeather + KMA + 국가별) | P1 | S |
| 44 | `SPEC-GOOSE-FORTUNE-001` | 개인화 운세 (사주·바이오리듬·LLM) | P1 | M |
| 45 | `SPEC-GOOSE-CALENDAR-001` ★ | 일정 (Google/Apple/Outlook/Naver) | P0 | M |
| 46 | `SPEC-GOOSE-BRIEFING-001` ★ | 아침 브리핑 오케스트레이션 (i18n) | P0 | M |
| 47 | `SPEC-GOOSE-HEALTH-001` | 식사·복약 트래커 (국가별 약품 DB) | P1 | M |
| 48 | `SPEC-GOOSE-JOURNAL-001` ★ | 저녁 일기 + 감정 태깅 + 추억 호출 | P0 | M |
| 49 | `SPEC-GOOSE-RITUAL-001` ★ | 3회 리추얼 통합 오케스트레이션 | P0 | L |

### Phase 8 — Deep Personalization (3 SPEC, P2)
| 50 | `SPEC-GOOSE-IDENTITY-001` | Identity Graph (POLE+O, Kuzu) | P2 | L |
| 51 | `SPEC-GOOSE-VECTOR-001` | Preference Vector (768-dim) | P2 | M |
| 52 | `SPEC-GOOSE-LORA-001` | User QLoRA Trainer (Rust crate) | P2 | L |

### Phase 9 — Ecosystem + Messenger 확장 (6 SPEC, v1.1~v2.0)
| 53 | `SPEC-GOOSE-GATEWAY-TG-001` | Telegram Bot (long-polling, inline buttons) | P1 | M | **v1.0** |
| 54 | `SPEC-GOOSE-GATEWAY-DC-001` | Discord Gateway (Slash + Application) | P2 | M | v1.1 |
| 55 | `SPEC-GOOSE-GATEWAY-SL-001` | Slack Socket Mode + Block Kit | P2 | M | v1.1 |
| 56 | `SPEC-GOOSE-GATEWAY-MX-001` | Matrix sync (E2EE 옵션) | P2 | M | v1.2 |
| 57 | `SPEC-GOOSE-GATEWAY-SG-001` | Signal libsignal (E2EE HARD) | P2 | L | v1.2 |
| 58 | `SPEC-GOOSE-A2A-001` | Agent Communication Protocol | P2 | L | v2.0 |

**v2.0+ 옵션 (사업자 전제, SPEC 번호 미할당)**:
- `SPEC-GOOSE-GATEWAY-KR-001` (KakaoTalk, 한국 법인)
- `SPEC-GOOSE-GATEWAY-CN-001` (WeChat, 중국/홍콩 법인)
- `SPEC-GOOSE-GATEWAY-JP-001` (LINE, 일본/태국 법인)
- `SPEC-GOOSE-GATEWAY-SMS-001` (Twilio, Tier 2 Cloud Plus 연계)
- `SPEC-GOOSE-VOICE-001` (Siri/Google Assistant 정식 확장)

**합계**: **58 active · 5 옵션 · 10 Phase · 약 950+ REQ / 560+ AC**

★ = Critical Path

---

## 5. 4-Layer ↔ Phase 매핑 (v6.2)

```
┌────────────────────────────────────────────────────────────┐
│ Layer 4 💞 Emotional Bond   → JOURNAL + REFLECT            │
│ Layer 3 📅 Daily Rituals    → Phase 7 (8 SPEC)             │
│ Layer 2 🐣 Nurture Loop     → QUERY + INSIGHTS             │
│ Layer 1 🧠 Agentic Core     → Phase 0~5 (26 SPEC)          │
├────────────────────────────────────────────────────────────┤
│ 🖥️  Presentation              → Phase 6 (15 SPEC)            │
│   ├─ Core: Desktop + Mobile + Bridge + Relay                │
│   ├─ Localization: Locale + I18N + Region-Skills + Onboard  │
│   ├─ 3-Tier: Cloud + Discovery + Auth + Sync                │
│   ├─ Notifications: Notify + Widget                         │
│   └─ Gateway: Self-hosted Msgr umbrella                     │
├────────────────────────────────────────────────────────────┤
│ 🌐 Channel Expansion         → Phase 9 (6 SPEC)            │
│   ├─ Tier A Msgrs (v1.0~1.2): TG/DC/SL/MX/SG               │
│   └─ A2A (v2.0)                                            │
├────────────────────────────────────────────────────────────┤
│ 🧬 Advanced                  → Phase 8 (3 SPEC)            │
│   └─ Identity + Vector + LoRA                              │
└────────────────────────────────────────────────────────────┘
```

---

## 6. Phase별 요약

| Phase | 이름 | SPEC 수 | 핵심 가치 |
|---|---|---|---|
| 0 | Agentic Core | 5 | async streaming query loop |
| 1 | Multi-LLM | 5 | 15+ provider OAuth/API |
| 2 | 4 Primitives | 5 | Skills/MCP/Agents/Hooks |
| 3 | Agentic Primitives | 3 | Tool Registry + Dev CLI |
| 4 | Self-Evolution | 5 | Trajectory → Insights → Memory |
| 5 | Promotion & Safety | 3 | 5-tier + 5-layer + Channel HARD |
| **6** | **Cross-Platform + Loc + 3-Tier** | **15** | **Desktop + Mobile + Localization + Cloud + Push** |
| 7 | Daily Companion | 8 | 3회 리추얼 |
| 8 | Deep Personalization | 3 | Identity + Vector + LoRA |
| 9 | Ecosystem + Msgrs | 6 | Telegram+Discord+Slack+Matrix+Signal+A2A |
| **합계** | — | **58** | — |

---

## 7. 주요 의존성 그래프 (v6.2 상위)

```
[Phase 0] CORE ──┬─ CONFIG
                 ├─ TRANSPORT ─────────────────────┐
                 └─ QUERY ── CONTEXT               │
                                                    │
[Phase 1] CREDPOOL ── ROUTER ── ADAPTER ───────────│
                                                    │
[Phase 2] SKILLS ── HOOK ── SUBAGENT ── MCP ───────│
                                                    │
[Phase 3] TOOLS ── COMMAND ── CLI                  │
                                                    │
[Phase 4] MEMORY ── TRAJECTORY ── COMPRESSOR ──────│
                    └─ INSIGHTS                     │
                    └─ ERROR-CLASS ←────────────────│
                                                    │
[Phase 5] REFLECT ── SAFETY ── ROLLBACK            │
                                                    │
[Phase 6 ★v6.2 Cross-Platform + 3-Tier]           │
  LOCALE ── I18N ── REGION-SKILLS                  │
     │                                              │
  DISCOVERY (mDNS) ──┐                              │
                     ├── BRIDGE (P2P + NAT)         │
  AUTH (Zero-K) ─────┤                              │
                     ├── CLOUD (STUN+TURN+Push)     │
  NOTIFY ────────────┤                              │
                     │                              │
  DESKTOP (Tauri) ◀──┼──── BRIDGE ──── RELAY (Rust)│
       │             │       ▲                      │
       └─ ONBOARDING │       │                      │
                     ▼       │                      │
                  MOBILE (RN + Widget + Siri) ◀────│
                     │                              │
  GATEWAY (umbrella) ┴── TG/DC/SL/MX/SG (long-poll)│
                                                    │
[Phase 7] SCHEDULER ──┬─ WEATHER                   │
                      ├─ FORTUNE                   │
                      ├─ CALENDAR                  │
                      └─ HEALTH                    │
                              │                    │
                     BRIEFING + JOURNAL ── RITUAL ◀┘

[Phase 8] IDENTITY ── VECTOR ── LORA (Rust)

[Phase 9] A2A + Msgr 개별 구현
```

---

## 8. 실행 순서 권장 (Milestone)

### M0 — Agentic Foundation (2주) → v0.1 Alpha
`CORE` → [`CONFIG` ∥ `TRANSPORT`] → `QUERY★` → `CONTEXT`

### M1 — Multi-LLM (3주) → v0.1 Alpha
`CREDPOOL` → [`ROUTER` ∥ `RATELIMIT` ∥ `ERROR-CLASS`] → `PROMPT-CACHE` → `ADAPTER★`

### M2 — 4 Primitives (4주) → v0.2 Beta
[`SKILLS` ∥ `HOOK★` ∥ `MCP`] → `SUBAGENT` → `PLUGIN`

### M3 — Dev CLI (1주) → v0.3 Beta
`TOOLS` → `COMMAND` → `CLI`

### M4 — Self-Evolution (3주)
[`TRAJECTORY` ∥ `MEMORY`] → `COMPRESSOR` → `INSIGHTS`

### M5 — Safety (2주)
`REFLECT` → `SAFETY` (+ Channel HARD rule) → `ROLLBACK`

### M6 — Cross-Platform + Localization + 3-Tier (6주) → v0.4 Public Beta ★
```
병렬 L1: LOCALE → I18N → REGION-SKILLS
병렬 L2: DISCOVERY + AUTH → CLOUD
순차:    DESKTOP → ONBOARDING → BRIDGE → [RELAY ∥ MOBILE] → NOTIFY + WIDGET
umbrella: GATEWAY-001 재정의
```

### M7 — Daily Companion + Telegram (4.5주) → v1.0 Release ★
```
SCHEDULER → [WEATHER ∥ CALENDAR ∥ HEALTH]
         → FORTUNE → [BRIEFING ∥ JOURNAL] → RITUAL
병행:    GATEWAY-TG-001 (Telegram bot, 1주)
```

### M7.5 — Msgr 확장 (2주) → v1.1~1.2
`GATEWAY-DC` → `GATEWAY-SL` → `GATEWAY-MX` → `GATEWAY-SG`

### M8 — Deep Personalization (4주) → v1.5
[`IDENTITY` ∥ `VECTOR`] → `LORA`

### M9 — Ecosystem (옵션) → v2.0
`A2A` + `GATEWAY-KR/CN/JP` + `SMS` + `VOICE` (사업자 등록 후)

### 총 기간
- **M0~M7 → v1.0 Release**: 팀 1명 32.5주 / 팀 2명 25.5주 / **팀 3명 17.5주**
- **M0~M8 → v1.5**: +4주
- **M0~M9 → v2.0**: 사업성 평가 후

---

## 9. Release 타임라인

| Release | Milestone 포함 | Tier 지원 | Channel 지원 | 핵심 기능 |
|---|---|---|---|---|
| v0.1 Alpha | M0~M1 | Tier 0 | CLI only | `goose ask "hello"` |
| v0.2 Beta | M0~M2 | Tier 0 | CLI | 4 Primitive 완성 |
| v0.3 Beta | M0~M3 | Tier 0 | CLI | 헤드리스 개발자 CLI |
| **v0.4 Public Beta** | **M0~M6** | **Tier 0 + 1** | **Ch.1 + Ch.2** | **Desktop+Mobile 공개, 20+ 언어** |
| v0.5 RC | +M4+M5 | Tier 0 + 1 | Ch.1 + Ch.2 | Safety + Self-evolution |
| **v1.0 Release** | **M0~M7 + Telegram** | **Tier 0 + 1** | **Ch.1~4 (일부)** | **Daily Companion + Telegram** |
| v1.1 | +Discord+Slack | Tier 0 + 1 | Ch.1~4 전체 | 협업 도구 유저 확장 |
| v1.2 | +Matrix+Signal | Tier 0 + 1 | Ch.1~4 전체 + E2EE | 프라이버시 파워 유저 |
| v1.3 | +Tier 2 Plus | Tier 0 + 1 + 2 | Ch.1~4 | PC OFF fallback |
| v1.5 | +M8 | 전 Tier | Ch.1~4 | LoRA 개인화 |
| **v2.0** | +M9+Biz-Msgrs | 전 Tier | **Ch.1~5** | Kakao/WeChat/LINE/SMS + A2A |

---

## 10. 설계 원칙 15개 (v6.2 누적)

**v0.1~0.3 Agentic Foundation**:
1. One QueryEngine per conversation
2. Streaming mandatory
3. Credential Pool first
4. 4 Primitive first-class

**v0.5 Safety**:
5. Self-evolution with safety gates (5-tier + 5-layer)

**v1.0 Daily Companion**:
6. Proactive over Reactive
7. Bidirectional Care
8. Privacy-First Intimacy
9. Ritualized Presence

**v0.4 Cross-Platform**:
10. PC-First, Mobile-Companion
11. E2EE Always
12. Progressive Disclosure on UI

**v0.4 Localization**:
13. Global First, Local Second
14. Cultural Adaptation

**v6.2 Channel**:
15. **Optional Multi-Channel Reach** — 사용자가 원하는 진입 채널 선택 (앱/메신저/Siri/음성), 단 Channel HARD rule로 민감 데이터 송출 차단

---

## 11. 법적·윤리적 HARD Rule

### Phase별 제약
| SPEC | 제약 |
|---|---|
| FORTUNE | 엔터테인먼트, opt-in OFF 기본, 의학·금융·복권 guard |
| HEALTH | 의료기기 아님, 응급 시 119 자동 안내, DUR severe HARD block |
| JOURNAL | 로컬 only, A2A 전송 HARD 금지, crisis keyword 시 1577-0199 |
| CALENDAR | OAuth 최소 권한 |
| SCHEDULER | Quiet hours 23-06시 HARD floor |
| RITUAL | Guilt-free 언어, 스킵 자유 |
| BRIDGE | Trusted Device, JWT 24h, session revoke |
| RELAY | Noise Protocol, plaintext 접근 HARD 불가 |
| MOBILE | Biometric lock, 로컬 캐시 암호화 |

### Channel HARD Rule (v6.2 신규)
- [HARD] Journal/Health/Identity Graph **본문**은 Tier B 메신저(Telegram/Discord/Slack) 전송 금지
- [HARD] E2EE 채널(Signal/Matrix E2EE)만 Journal 본문 허용, 사용자 명시적 opt-in 필요
- [HARD] 메신저 Bot Token은 OS 키체인에만 저장, 설정 파일 평문 금지
- [HARD] 메신저 수신 명령은 Crisis keyword 이중 검사 후 LLM 실행
- [HARD] Trusted User 권한 세분화: Calendar-read / Tool-exec / Journal-write / Admin

### Tier HARD Rule
- [HARD] Cloud가 저장 가능한 데이터 한정: email sha256+salt, device public key, encrypted APNs/FCM token, encrypted push payload
- [HARD] Tier 2 LLM Cloud 실행 시 prompt ↔ email 연결 영구 분리 (pepper 단방향 토큰)
- [HARD] Cloud 코드 100% OSS + reproducible build + 3rd-party audit
- [HARD] Journal/Health/Identity Graph는 어느 Tier에서도 평문 클라우드 저장 금지

---

## 12. OUT OF SCOPE

- Rust ML crate 구현 상세 → `ROADMAP-RUST.md`
- iMessage 직접 봇 통합 → Apple 정책상 불가 (Siri Shortcut + Share Sheet + Live Activity로 대체)
- KakaoTalk/WeChat/LINE → v2.0 사업자 전용 (별도 SPEC)
- 클라우드 백업 구체 스킴 → v1.3 Tier 2 별도 SPEC
- 기업 자체 Relay 호스팅 가이드 → v1.5+
- Agent Teams 병렬 실행 → MoAI-ADK-Go link 시점 재평가

---

## 13. 오픈 이슈 (v6.2 누적, 30건)

### 기존 v3.0 (13건)
1. Go 버전 고정 (1.26+ 가정)
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

### v4.0 Cross-Platform (4건)
14. Tauri updater ed25519 키 배포
15. Relay Go↔Rust FFI (gRPC vs CGO)
16. Whisper 모델 배포 (번들 vs on-demand)
17. HOOK EventName 확장 (SCHEDULER 5개)

### v5.0 Localization (5건)
18. Tier 2 커뮤니티 번역 리뷰어 모집 (12개 언어)
19. MaxMind GeoLite2 DB 배포 (앱 번들 vs on-demand)
20. Windows locale detection (`chcp` vs `Get-Culture`)
21. 음력 공휴일 DB 정확성 (~2050)
22. 인도 카스트·대만·홍콩 민감 Skill 범위
23. Ollama 모델 번들 전략 (자동 pull vs 수동)
24. 재온보딩 권한 (locale 변경 시 full vs minor)

### v6.1 3-Tier (3건, Kakao/WeChat 3건 삭제됨)
25. APNs 인증서 방식 (p12 vs p8 Key)
26. Android 푸시 프로바이더 (FCM 전용 vs HMS 중국 병행)
27. GOOSE Cloud 호스팅 지역 (Hetzner/AWS/Cloudflare Workers)

### v6.2 Channel (3건)
28. Telegram Bot Token 사용자 지침 (BotFather 문서화)
29. Discord Application 권한 최소화 (slash command scope)
30. Signal libsignal-client Go 포팅 가능성 검증 (또는 Rust FFI 필요)

**삭제된 v4.0 이슈** (App-First 전환으로 소멸): 카카오 알림톡 벤더, WeChat 중국 법인, Porcupine 라이선스, LINE Business 등록

---

## 14. 첫 번째 실행 SPEC

### v0.1 진입 순서
1. **선결 의사결정 6건 확정** (Go 버전, sqlite, Tokenizer, Graph DB, Stream 인터페이스, proto 정책)
2. `go.mod` 초기화 + `cmd/goosed` 스켈레톤
3. **`SPEC-GOOSE-CORE-001` TDD RED 진입**
   ```bash
   /moai run SPEC-GOOSE-CORE-001
   ```

### v0.4 진입 순서 (M6 시작 시)
1. Tauri updater 키페어 생성 + CI secret
2. Apple Developer 계정 + TestFlight 설정
3. APNs/FCM 프로젝트 생성
4. `SPEC-GOOSE-LOCALE-001` + `SPEC-GOOSE-DISCOVERY-001` 병렬 TDD RED

### v1.0 진입 (M7)
1. Telegram @BotFather 준비 가이드
2. `SPEC-GOOSE-SCHEDULER-001` TDD RED
3. `SPEC-GOOSE-GATEWAY-TG-001` 병행 (Telegram only)

---

**Version**: 6.2.0
**License**: MIT
**Next action**: 선결 의사결정 6건 확정 → `SPEC-GOOSE-CORE-001` TDD RED → v0.1 Alpha → v0.4 Beta → v1.0 Release (Daily Companion + Telegram Bridge)

## Change Log
- **v6.2 (2026-04-22)**: 5-Channel 추가 (Apple Native + Self-hosted Msgr Tier A 5종), GATEWAY-001 재정의, iMessage 정책 명시, SPEC 52→58
- v6.1 (2026-04-22): 3-Tier 선택제 (Local/Cloud Free/Cloud Plus), SPEC 48→52
- v6.0 (2026-04-22): App-First 피벗, Hermes Gateway 폐기 시도, SPEC 47→48
- v5.0 (2026-04-22): Global Localization 4 SPEC 추가, 43→47
- v4.0 (2026-04-22): Cross-Platform (Desktop+Mobile+Bridge+Relay+Gateway), 38→43
- v3.0: Daily Companion 8 SPEC, 30→38
- v2.0: Agentic Core 22→30
- v1.0: 초기 22 SPEC
