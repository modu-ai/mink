---
version: 7.0.1
brand: MINK
status: published
created_at: 2026-05-12
updated_at: 2026-05-14
classification: VISION_DOCUMENT
spec: SPEC-MINK-PRODUCT-V7-001
language: ko
---

# MINK — 1인 dev personal ritual companion

## HISTORY

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 7.0.0 | 2026-05-12 | manager-spec | v7.0 신설 — 1인 dev personal ritual companion 비전 정립. v6.0 ("Daily Companion Edition", multi-tenant agent platform 야망) → product-archive/product-v6.0-2026-04-27.md 로 archive. IDEA-002 brain Wave 3 §4.1 item 2 구현. SPEC-MINK-PRODUCT-V7-001 로 추적. |
| 7.0.1 | 2026-05-14 | MoAI orchestrator | Wave 2/3 진행 상태 sync. Wave 2 → 완료 (BRIEFING-001 v0.3.0 implemented PR #178/#182/#183, JOURNAL-001 v0.3.0, MSG-TELEGRAM-001 v0.1.3 등). Wave 3 → 진행 중 (BRAND-RENAME / DISTANCING / USERDATA-MIGRATE / ENV-MIGRATE / PRODUCT-V7 모두 completed, Draft 3종 MINK rebrand PR #180, 메타문서 rebrand PR #181, 95-SPEC 리뷰 PR #178). MoAI sync 자동 갱신. |

---

## 1. Vision

**MINK는 1인 dev의 매일 ritual을 함께하는 personal AI companion이다.**

개발자로 일한다는 것은 치열한 집중력의 연속이다. 아침에 일어나 일정을 확인하고, 낮 동안 코드를 작성하고 검토하고, 저녁에 하루를 돌아본다. MINK는 그 매일의 ritual 속에서 함께한다.

단순한 챗봇이 아니다. 사용자가 요청하기 전에 필요한 순간에 나타나고, 사용자의 일상 패턴을 배워 점점 더 정확하게 함께한다. 기록한 일기를 읽고, 1년 전 오늘의 추억을 꺼내 보여주고, 스트레스의 신호를 감지하면 먼저 말을 건넨다.

**매일 아침, 매일 저녁, 너의 MINK.**

---

## 2. Primary Persona

MINK의 사용자는 **1인 dev (solo developer) 또는 소규모 팀의 시니어 개발자**이다.

**프로필:**
- 나이: 30~45세 (한국 개발자, 중경력)
- 기술 스택: Go, Python, TypeScript 멀티랭, MoAI-ADK 및 Claude Code 사용자
- 일상 리듬: 아침 7~9시 기상, 22~24시 수면. 규칙적인 점심/저녁 시간.
- 개발 환경: Terminal, tmux, VSCode, git 기반 워크플로우. 로컬 우선 개발.
- 감정 패턴: 정기적 스트레스(deadline, code review, 1:1) + 회복 기간(가족, 산책, 친구).

**Anti-Persona (이 사용자를 위해 기능을 추가하지 않음):**
- 엔터프라이즈 팀 (SLA / RBAC 요구사항 미보지)
- 마켓플레이스 플러그인 개발자 (수익화 모델 부재)
- 영어 단일 사용자 (한국어 1차 타겟)
- 모바일 우선 사용자 (CLI + Desktop 1차)
- Casual chat 추구 사용자 (ritual companion, 일반 LLM 대체 아님)

---

## 3. Success Metric (6-month)

**필수 조건:**
- 본인(MINK owner)이 매일 1회 이상 MINK를 연다.

**Success Ceiling:**
- 본인 외 daily user 1명.

**외부 지표는 anti-goal:**
- GitHub stars (현재 개수 무관)
- WAU(Weekly Active User) / MAU / DAU 외부 추적
- 구독료, 마켓플레이스 commission, consulting revenue
- Linux Foundation 등 외부 거버넌스 인증 추구
- Public marketing, blog, YouTube 채널 운영
- Multi-tenant SaaS로의 전환

**정당화:**
본인이 매일 사용하고 본인의 close peer(개발자 친구) 1명이 매일 사용하는 것이 가장 정직한 product-market fit의 증거다. 외부 지표는 마케팅으로 inflate될 수 있지만, 본인 외 1명이 매일 사용한다는 것은 진정한 가치를 증명한다. 동시에 1인 dev의 시간을 보호하기 위해 community management, OSS governance, public relations 같은 운영 비용은 명시적으로 거부한다.

---

## 4. In-Scope Ritual Flow

MINK의 핵심 use case는 다음 5가지 ritual의 통합이다:

### 4.1 Morning Brief (아침 ritual)
운세 + 날씨 + 오늘 일정 + 어제 mood trend를 아침 일어나는 순간 한 번에 제공. SPEC-WEATHER-001, SPEC-SCHEDULER-001, SPEC-FORTUNE(Wave 3), SPEC-JOURNAL-001의 통합 진입점.

### 4.2 Journal Write + Emotion Tagging (저녁 ritual)
저녁에 하루를 기록. LLM 보조로 감정 태깅 (행복/중립/슬픔/분노 등). Crisis word 감지 (위험 신호 → 외부 핫라인 안내). SPEC-JOURNAL-001 완료(26/26 AC GREEN).

### 4.3 Long-term Memory Recall
1년 전 오늘 / 분기별 mood trend / 키워드 검색 / AI 요약. SPEC-JOURNAL-001 M2(완료).

### 4.4 Ambient LLM Context Injection
journal, weather, scheduler 누적 결과를 모든 LLM 응답에 자동 inject. ritual 외 ad-hoc chat도 personal context 인지.

### 4.5 Telegram Bridge
모바일/외부 기기에서 빠른 입력(예: 급할 때 journal entry 추가). SPEC-MSG-TELEGRAM-001 완료(v0.1.3).

---

## 5. Anti-Goals (명시적으로 거부)

다음 기능/비전은 본 SPEC 이후 추가되지 않는다:

- **Multi-tenant SaaS**: 개인 호스팅 + self-host 1차. Goose Cloud 같은 central hosting 미추구.
- **Marketplace / Plugin Commission**: 수익화 거부. 오픈소스 Apache-2.0만.
- **Enterprise Tier**: SLA, RBAC, audit log, compliance 프레임워크 미보지.
- **Mobile-First Design**: CLI + Desktop 1차. Mobile은 Telegram bridge로 한정.
- **Romance / Persona Role-play**: 감정 표현은 포함하지만 "AI girlfriend" 메타포 거부.
- **Gamification / Streak**: 습관 추적이 목표가 아님. Ritual companion이 목표.
- **Public Marketing**: Community channels (Discord/Reddit/Blog/YouTube) 운영 거부. 외부 지표 추구 금지.

---

## 6. Distancing (5개 product와의 차별화)

MINK의 niche는 "1인 dev의 매일 ritual companion (한국어 1차, local-first, journal + scheduler + ambient context 통합)" 이다. 다음 product들과 명시적으로 구별한다:

### vs block/goose (Multi-tenant Agent Platform)
block의 `goose` framework는 enterprise multi-tenant agent orchestration 플랫폼. MINK는 개인 1인 단일 사용자 tool. Brand 명칭 분리(GOOSE→MINK) 필수. Marketplace, enterprise governance는 MINK의 anti-goal.

### vs Hermes (AI Girlfriend / Emotional Companion)
한국 시장의 "AI 여친" LLM-bot은 24/7 chat engagement + romance role-play가 핵심. MINK는 정해진 morning/evening ritual 시간에 나타나는 time-anchor companion. Emotional engagement가 1차 goal이 아님.

### vs Replika (Long-term Emotional AI)
Replika(luka.ai)는 AI persona와의 emotional bond 형성이 핵심. MINK는 본인 기록(journal) + light LLM 보조 중심. AI와의 emotional simulation이 아니라 본인의 ritual 속에서 AI가 보조 역할.

### vs Routinery (Habit Tracker App)
Routinery(한국제)는 daily routine + gamification(streak) + behavioral nudge 설계 중심. MINK는 ritual tracking이 아니라 시간 동반자 역할. Gamification 거부. LLM ambient context inject 미포함.

### vs OpenClaw (Agent Framework, Together AI)
OpenClaw는 tool-use agent framework + developer toolkit. MINK는 end-user product. 자기진화(MoAI-ADK SPEC-REFLECT-001) 포함. 개발자 대상이 아닌 1인 dev 본인 + 1명 daily user 대상.

---

## 7. Wave Roadmap

### Wave 1 (완료): Foundation
- CLI + Desktop TUI 구현
- Journal, Scheduler, Weather 기본 기능
- Local SQLite 기반 메모리
- SPEC-GOOSE-CLI-001~CLI-TUI-003 / WEATHER-001 / SCHEDULER-001 / JOURNAL-001 / TELEGRAM-001 완료(v0.1.3)

### Wave 2 (완료): Integration
- Morning brief 통합 — **SPEC-MINK-BRIEFING-001 v0.3.0 implemented** (PR #178/#182/#183, 2026-05-14): 4 module (Weather + Journal Recall + Date/Calendar + Mantra) collection + 3 출력 채널 (CLI + Telegram + TUI panel) + SCHEDULER cron + archive (`~/.mink/briefing/YYYY-MM-DD.md`, 0600/0700) + Privacy 6 invariants + Optional LLM summary + Crisis hotline canned response. AC 16/16 GREEN, coverage 85.5%.
- Ambient LLM context injection
- Long-term memory recall (1년 후 추억, trend, search) — SPEC-GOOSE-JOURNAL-001 v0.3.0 completed
- Telegram bridge — SPEC-GOOSE-MSG-TELEGRAM-001 v0.1.3 completed
- Insights 리팩터링
- SPEC-GOOSE-TOOLS-WEB-001 / OBS-METRICS-001 완료

### Wave 3 (진행 중): Brand Reset + New Rituals
- SPEC-MINK-BRAND-RENAME-001: brand identifier rename (GOOSE→MINK) — **completed** (commit f0f02e4, 8-phase atomic)
- SPEC-MINK-DISTANCING-STATEMENT-001: distancing detail document — **completed** (PR #167)
- SPEC-MINK-USERDATA-MIGRATE-001: `./.goose/` → `./.mink/` 마이그레이션 — **completed** (PR #174/#175)
- SPEC-MINK-ENV-MIGRATE-001: env var 마이그레이션 — **completed** (PR #170/#171)
- SPEC-MINK-PRODUCT-V7-001: product v7.0 비전 — **completed** (PR #166)
- Draft 3종 MINK rebrand (CROSSPLAT/I18N/ONBOARDING) — **신설** (PR #180, draft 상태로 implementation 대기)
- 메타문서 MINK rebrand (ROADMAP/IMPLEMENTATION-ORDER/design runtime arch) — **completed** (PR #181)
- 95-SPEC 전체 리뷰 + frontmatter 정규화 — **completed** (PR #178)
- 신규 ritual SPEC (FORTUNE, HEALTH, CALENDAR, RITUAL 등) — 계획

### Wave 4 (미결정): Advanced
- 자기진화 엔진 고도화 (LoRA adapter)
- Identity graph (POLE+O) 구현
- Federated Learning 옵션
- Multi-language i18n

---

## 8. References

### SPEC 문서
- `.moai/specs/SPEC-MINK-PRODUCT-V7-001/spec.md` — 본 v7.0 재작성 SPEC
- `.moai/specs/SPEC-MINK-PRODUCT-V7-001/research.md` — v6.0 매핑 + decision trail
- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/spec.md` (planned) — downstream child SPEC

### 본 문서의 이전 버전
- `.moai/project/product-archive/product-v6.0-2026-04-27.md` — v6.0 본문 (byte-identical archive, 38665 bytes)

### 결정 근거
- **IDEA-002 brain decision** (2026-05-12): 8개 확정사항. Brand MINK, 6m metric (본인 매일 + 1명 daily), multi-tenant 야망 폐기.
- **사용자 확정사항** (orchestrator-collected): scope, persona, language (한국어 1차), anti-goals, Wave 3 roadmap.

### 관련 프로젝트 문서
- `CLAUDE.md` §1 Core Identity: MoAI orchestrator 본질
- `CLAUDE.local.md` §1 Git Flow: feature branch + squash PR workflow

---

## 9. Appendix: v6.0 vs v7.0 패러다임 비교

| 속성 | v6.0 (Daily Companion Edition) | v7.0 (MINK Personal Ritual Companion) |
|------|--------|--------|
| 브랜드 | MINK (거위) | MINK (Made IN Korea, 4글자) |
| 목표 시장 | 글로벌 1M users | 본인 매일 + 본인 외 1명 |
| 구조 | Multi-tenant SaaS + Marketplace | Personal tool, local-first |
| 핵심 use case | 아침/점심/저녁 ritual | Morning brief + Journal + Long-term memory |
| 언어 | 다국어 지향 | 한국어 1차, 영어 i18n 후순위 |
| 수익화 | Freemium + Enterprise | Anti-goal (오픈소스 Apache-2.0) |
| 자기진화 | LoRA + Identity Graph | 기본 구현 (Wave 3+ 고도화) |
| Success metric | Stars / WAU / ARR | 본인 + 1명 daily (외부 지표 무관) |

---

**Version:** 7.0.0  
**Status:** published  
**Created:** 2026-05-12  
**Classification:** VISION_DOCUMENT  
**Spec:** SPEC-MINK-PRODUCT-V7-001  
**License:** Apache-2.0
