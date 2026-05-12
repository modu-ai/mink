# Research — SPEC-MINK-PRODUCT-V7-001 (product.md v7.0: 1인 dev personal ritual companion)

> 본 research 문서는 SPEC-MINK-PRODUCT-V7-001의 의사결정 근거자료다. spec.md §References에서 본 문서를 참조한다. 본 SPEC은 IDEA-002 brain 결정의 Wave 3 §4.1 item 2 구현이며, downstream SPEC-MINK-BRAND-RENAME-001 의 constitutional parent (vision anchor) 역할을 한다.

작성일: 2026-05-12
작성자: manager-spec
Status: planned

---

## 1. 문제 정의 (Problem Restatement)

### 1.1 현재 product.md v6.0 의 한계

현재 `.moai/project/product.md` 는 v6.0 (38665 bytes, 2026-04-22 작성, "Daily Companion Edition") 이다. 비전 본문은 다음을 표방한다:

- **글로벌 오픈소스 멀티-테넌트 agent platform**: Apache-2.0, Linux Foundation 가입 목표, 글로벌 100만+ user 지향
- **3-Project heritage**: Claude Code + Hermes + MoAI-ADK-Go (38,700줄 직접 계승)
- **5가지 차별화**: 자기진화 / 100% 개인화 / 오픈소스+프라이버시 / 3-project 상속 / 크로스 플랫폼
- **재무 모델**: Freemium ($9~$29 tier) + Enterprise + Marketplace commission, 3년 ARR $24M 목표
- **로드맵**: Phase 4 Scale (18-24개월) "1M users, ICLR 논문, Linux Foundation Agentic AI 가입"
- **GOOSE brand identity**: "G**enerative **O**wn **O**ne **S**elf-**E**volving companion", "🪿 거위" 메타포, 다마고치 양육 메타포 등

v6.0 의 핵심 문제 (사용자 IDEA-002 brain decision 으로 확정된 인식):

1. **외부지표 지향성**: stars / DAU / MAU / WAU / revenue / Linux Foundation 멤버십 등 외부 검증을 핵심 success metric 으로 채택. 1인 dev 가 6개월 안에 도달 불가능한 야망 위주.
2. **multi-tenant agent platform 비전**: "사용자 1,000만 명" / "Goose Cloud 호스팅" / "Marketplace commission" / "Enterprise SLA" 등 multi-tenant SaaS 구조. 1인 dev personal tool 과 본질적으로 충돌.
3. **block/goose 와 정체성 혼동**: GOOSE brand + 거위 메타포로 인해 block(stripe spinoff) 사의 `goose` agent framework 와 brand 충돌. SPEC-MINK-BRAND-RENAME-001 의 IDEA-002 결정으로 brand 자체가 MINK 로 reset.
4. **사용 패턴 mismatch**: v6.0 의 "Daily Rituals" (아침/점심/저녁) layer 는 본질이 personal companion 인데, 그 본질이 "multi-tenant agent platform" 야망 아래 묻혀 명시되지 않음.
5. **persona 부재**: 1인 dev (Korean, MoAI/Claude Code user) 라는 실제 사용자 persona 가 명시적으로 정의되지 않음. "Alex the developer" 같은 generic persona 만 시나리오로 등장.

### 1.2 product.md v7.0 의 새 비전 (IDEA-002 확정)

사용자 IDEA-002 brain decision 의 8 결정 (orchestrator-collected, 2026-05-12) 핵심:

- **1인 dev personal tool**: 본인이 매일 사용하는 도구로 reset. multi-tenant 야망 폐기.
- **6m success metric**: 본인이 매일 열어봄 + 본인 외 1명이 매일 사용. 외부 stars / WAU / revenue 무관.
- **MINK 브랜드**: Made IN Korea, 한국어 1차 타겟. Hermes / Replika / Routinery / OpenClaw 와 niche 차별화.
- **Personal ritual companion**: journal + scheduler + weather + telegram + ambient LLM context + ritual time-marker 가 핵심 use case. multi-tenant SaaS 기능 (marketplace, enterprise tier) OUT.
- **Distancing**: block/goose 와 정체성 분리 명시. 다른 AI companion (Hermes 등) 과 niche 명시 차별화.

본 SPEC 의 mission: v7.0 본문 작성 + v6.0 archive + downstream SPEC 들의 vision anchor 역할.

---

## 2. v6.0 본문 매핑 (변동 분석)

본 절은 v6.0 본문의 각 영역이 v7.0 에서 어떻게 변환되는지 매핑한다. archive 시 v6.0 본문은 byte-byte identical 로 보존되며 (AC-002), v7.0 은 v6.0 을 대체하는 새 본문이다.

### 2.1 패러다임 진화 표 (v6.0 §0)

v6.0 는 5번의 피벗 (v0.2 → v6.0) 을 표로 기록한다. v7.0 에서는 다음과 같이 처리:

- v7.0 §0 (패러다임 진화) 표에 1행 추가: `v7.0 | MINK (1인 dev personal ritual companion) | 매일 ritual + 한국어 1차 | 본인 + 1명 | personal tool, multi-tenant 야망 폐기`
- v6.0 까지의 5개 행은 historical 기록으로 보존 (v6.0 archive 에는 그대로, v7.0 본문에도 그대로 inline 인용).

### 2.2 §1 프로젝트 개요 (v6.0)

| v6.0 항목 | v7.0 변환 |
|---|---|
| 프로젝트명: AI.GOOSE | MINK |
| 코드명: goose | mink |
| 완전형: Generative Own One Self-Evolving companion | 폐기 (확장형 없음, 단일 4글자 brand) |
| 라이선스: Apache-2.0 | 보존 |
| 저장소: github.com/modu-ai/goose | github.com/modu-ai/mink (downstream SPEC-MINK-BRAND-RENAME-001 결과) |
| 비전: "Every morning I greet you. Every meal I remind you..." | 보존 (정서 유효) + 한국어 tagline 추가: "매일 아침, 매일 저녁, 너의 MINK." |

### 2.3 §2 4-Layer Daily Companion Architecture (v6.0)

v6.0 의 4-layer 모델은 본질적으로 personal ritual companion 의 archtecture 이며 v7.0 비전에 부합한다 → **보존하되 표현 단순화**:

- Layer 1: Agentic Core (기존 30 SPEC, 자기진화 엔진, 27 에이전트)
- Layer 2: Nurture Loop (Feed/Play/Train/Rest/Attention) → v7.0 에서 단순화 (다마고치 메타포 약화)
- Layer 3: Daily Rituals (Morning brief / Meal check / Evening journal) → **v7.0 의 핵심**
- Layer 4: Emotional Bond (journal 누적, 1년 후 추억) → **v7.0 의 핵심**

v7.0 에서는 Layer 3 + Layer 4 를 본질로 부각하고 Layer 1 + Layer 2 는 기반 인프라로 위치 조정.

### 2.4 §3 자기진화 + 100% 개인화 + 일상 돌봄 (v6.0)

| v6.0 강조점 | v7.0 처리 |
|---|---|
| Layer 1 Short-term (implicit feedback) | 보존 (Daily Ritual 의 미시 적응 메커니즘) |
| Layer 2 Medium-term (Markov / K-means) | 보존하되 약화 (1인 dev tool 에서 통계적 robustness 무의미) |
| Layer 3 Long-term (User Identity Graph + LoRA + Continual Learning) | **부분 폐기**. POLE+O RDF graph + LoRA adapter 같은 heavy stack 은 multi-tenant SaaS 가정. v7.0 은 SQLite + 단순 vector index 위주로 단순화 (downstream 결정). |
| Federated Learning | **폐기**. multi-tenant 가정. v7.0 에서 OUT. |

### 2.5 §3.2~§3.3 Heritage (Claude Code / Hermes / MoAI-ADK-Go)

v6.0 의 "3-project heritage" 는 implementation provenance 의 정직한 기록. v7.0 에서는 보존하되 **vision 차원의 핵심 차별화에서 강조 제거**. heritage 는 implementation 영역의 사실로 부각하지 않음 — vision 은 outcome 중심.

### 2.6 §5 사용 시나리오 (25가지, v6.0)

v6.0 §5 의 25가지 시나리오 중 v7.0 비전 부합 / 비부합 분류:

| 시나리오 분류 | v6.0 시나리오 | v7.0 처리 |
|---|---|---|
| **v7.0 핵심 (보존)** | A: 아침 브리핑, B: 식사 health check, C: 저녁 journal, 1년 후 추억 | 핵심 use case 로 §In-Scope §4에 명시 |
| **v7.0 부합 (보존)** | 1~4 (학습 초기), 5~8 (패턴 형성), 9~12 (개인화 완성), 13~16 (Year 1) | personal ritual companion 일관 |
| **v7.0 비부합 (폐기)** | 17 다국어 자동 전환, 18 Family 공유 모드, 19 오프라인 모드 LoRA, 20 데이터 이전 | 1인 dev 단일 사용자 + 한국어 single locale 가정에서 over-engineered. v7.0 OUT. |

### 2.7 §6 경쟁 분석 (v6.0)

v6.0 §6.1 의 8개 글로벌 경쟁사 비교 (ChatGPT/Claude/Gemini/Apple Intelligence/Grok/OpenClaw/Hermes/Inflection Pi) → v7.0 §7 Distancing 에서 **4개로 축소** (Hermes / Replika / Routinery / OpenClaw — IDEA-002 결정의 4개 distancing 대상). 나머지는 vision 차원에서 비교 무의미 (1인 dev personal tool 은 ChatGPT/Claude 같은 일반 LLM 과 비교 자체가 카테고리 오류).

v6.0 §6.2 의 5가지 positioning (vs Static AI / 오픈소스 / 기업 AI / 폐쇄형 / 학습 부족) → v7.0 에서 **2가지로 축소** (vs block/goose multi-tenant agent platform / vs niche companion 4종).

### 2.8 §7 오픈소스 전략 (v6.0)

| v6.0 §7 항목 | v7.0 처리 |
|---|---|
| Apache 2.0 라이선스 | 보존 (코드 공개 자체는 의도) |
| OpenCollab community model + RFC | **폐기** (1인 dev tool, RFC 절차 over-engineered) |
| Linux Foundation Agentic AI 가입 (2026-Q3 목표) | **폐기** (외부 인증 추구 anti-goal) |
| GitHub Stars 로드맵 (6m=1K, 1y=10K, 3y=100K) | **폐기** (anti-goal 의 직접 표현) |
| Contributor 시스템 / 배지 / spotlights | **폐기** (1인 dev 가정) |
| Microsoft Agent Governance Toolkit 준수 (OWASP Top 10) | **부분 폐기** (보안 best practice 는 유지하되 governance 인증 추구 OUT) |
| 커뮤니티 채널 (Discord/Reddit/Blog/YouTube) | **폐기** (1인 dev personal tool) |

### 2.9 §8 지니/거위 메타포 시스템 (v6.0)

v6.0 §8 의 거위 (Goose) + Gaggle / Lineage / Honk Protocol / Migration 등 메타포 시스템 → **전면 폐기**. MINK brand reset 의 핵심 동기.

v7.0 brand 메타포: minimalist, 메타포 없음. "MINK" 자체가 4글자 짧은 brand. 동물/사물 연상 약화.

### 2.10 §9 재무 모델 (v6.0)

| v6.0 §9 항목 | v7.0 처리 |
|---|---|
| Freemium tier ($9~$29) | **폐기** |
| Goose Cloud 호스팅 (60% revenue) | **폐기** |
| Enterprise Support (20% revenue) | **폐기** |
| Marketplace Commission | **폐기** |
| 3년 ARR $24M | **폐기 (anti-goal 직접 표현)** |
| Crypto 옵션 (2027+) | **폐기** |

v7.0 에서는 §재무 모델 섹션 자체를 **제거**. 1인 dev personal tool 은 수익 모델 부재가 정상 상태.

### 2.11 §10 로드맵 (v6.0)

| v6.0 phase | 기간 | v7.0 처리 |
|---|---|---|
| Phase 1: Foundation | 0-6개월 | 보존 (core build) |
| Phase 2: Personalization | 6-12개월 | 부분 보존 (LoRA 폐기, journal/scheduler/weather 강조) |
| Phase 3: Ecosystem | 12-18개월 | **폐기** (Marketplace / Linux Foundation / Enterprise) |
| Phase 4: Scale | 18-24개월 | **폐기** (1M users / ICLR / DAO governance) |

v7.0 의 로드맵은 Wave 기반 (이미 진행 중인 SPRINT-1 / SPRINT-2 결과 + 향후 Wave 3 / 4) 으로 재작성. 시간 추정 금지 (priority 라벨 사용).

### 2.12 §11 핵심 차별화 5가지 (v6.0)

| v6.0 차별화 | v7.0 처리 |
|---|---|
| 1️⃣ 자기진화 (SPEC-REFLECT-001) | 보존 (implementation 핵심) — 단, vision 차원 강조 약화 |
| 2️⃣ 100% 개인화 (LoRA + Identity Graph) | **단순화** (1인 dev 단일 user 가정에서 단순 SQLite + vector index 로 충분) |
| 3️⃣ 오픈소스 + 프라이버시 (FL + DP) | **부분 폐기** (FL multi-tenant 가정, DP over-engineered). 단순 local-first 만 명시. |
| 4️⃣ 3-project heritage | implementation 영역에 격하 |
| 5️⃣ 크로스 플랫폼 (CLI/Desktop/Mobile/Web/OnDevice) | **CLI + Desktop only** (1인 dev personal tool, Mobile/Web 야망 OUT) |

v7.0 의 5가지 차별화 재정의 (잠정, downstream 결정):

1. 매일 ritual flow (morning brief / journal / weather) — vertical 단순화
2. 한국어 1차, 일본어/중국어 i18n 후순위
3. local-first (개인 데이터 외부 전송 없음)
4. MoAI-ADK 직접 dogfooding (자기 SPEC 으로 build)
5. 1인 dev personal tool (multi-tenant SaaS anti-goal)

### 2.13 §13 MoAI 명령어 통합 (v6.0)

v6.0 §13 의 `/moai plan` `/moai run` `/moai sync` `/moai design` `/moai db` 명령어 흐름 → v7.0 에서 보존 (implementation 영역). vision 본문에 inline 으로 인용하지 않고, "본 프로젝트는 MoAI-ADK 명령어로 build 됨" 1 줄 reference 로 격하.

---

## 3. 1인 dev personal ritual companion 비전 정의

### 3.1 핵심 정의

**MINK 는 1인 dev 의 매일 ritual 을 함께하는 personal AI companion 이다.**

- "1인 dev": 본 프로젝트 owner (GOOS행님) + 본인 외 1명의 daily user. 글로벌 1M user 아님.
- "매일 ritual": 아침 brief / journal write / weather check / scheduler review / 저녁 reflection. 우발적 chat 이 아닌 reproducible daily pattern.
- "함께한다": MINK 가 사용자의 ritual time 을 인지하고 자발적으로 trigger. 사용자가 매번 "/journal" 같은 명시 명령 입력 minimize.

### 3.2 핵심 ritual flow 3-5개

다음 use case 가 v7.0 §In-Scope §4 의 핵심:

1. **Morning brief** (07:00~09:00): 운세 + 날씨 + 오늘 일정 + 어제 mood trend. SPEC-WEATHER-001 / SPEC-SCHEDULER-001 / SPEC-FORTUNE (Wave 3) / SPEC-JOURNAL-001 통합.
2. **Journal write** (저녁 ritual): emotion 태깅 (LLM 보조), crisis word 감지, 1년 후 추억 기능. SPEC-JOURNAL-001 (완료, 26/26 AC GREEN).
3. **Long-term memory recall**: 1년 전 오늘 / 분기별 emotion trend / 검색 / summary. SPEC-JOURNAL-001 M2 (완료).
4. **Ambient LLM context**: weather / 일정 / journal 누적 결과를 LLM 응답에 자동 inject. ritual 외 ad-hoc chat 도 personal context 인지.
5. **Telegram bridge**: 알림 / 짧은 input (외부 device 에서 journal entry 추가 등). SPEC-MSG-TELEGRAM-001 (완료).

### 3.3 6개월 success metric (anti-goal 명시 포함)

- **필수 조건**: 본인 (project owner) 이 매일 1회 이상 MINK 를 연다.
- **success ceiling**: 본인 외 daily user 1명. 2~3명 이상 추구 안 함.
- **anti-goals** (명시 폐기):
  - GitHub stars 추구 (현재 stars 수 무관)
  - WAU / MAU / DAU 외부 지표 추구 (analytics 자체 미설치 권장)
  - revenue (구독 / commission / consulting) 추구
  - Linux Foundation 등 외부 governance 인증 추구
  - public marketing / blog / YouTube 채널 운영
  - multi-tenant SaaS 전환

### 3.4 6m metric 정당화

사용자 IDEA-002 brain decision §metric 의 핵심 reasoning:

1. **사용자가 본인 외 1명을 매일 사용시키는 것이 가장 어려운 ceiling**: stars/WAU 같은 외부 지표는 marketing 으로 inflate 가능 (게임 가능). 본인 외 1명 daily user 는 inflate 불가능 — 실제 product-market fit 의 진정한 minimal evidence.
2. **1인 dev 의 시간 분산 방지**: marketing / community management / OSS contributor governance 등 운영 비용은 1인 dev 의 시간 자원을 분산시킴. anti-goal 로 명시하여 시간 보호.
3. **dogfooding 강제**: success 의 필수조건이 "본인 매일 사용" 이라면 본인이 안 쓰는 기능은 추가 동기 부재. 자연스레 1인 dev 가 본인이 매일 쓸 기능에만 집중.
4. **MINK 의 본질 보존**: personal companion 의 본질은 1대1 친밀도. 사용자 수가 100명을 넘어가는 순간 personal 본질 손상.

---

## 4. Persona 정의 근거

### 4.1 Primary persona

**이름**: 1인 dev (project owner + close peer)
**연령대**: 30~45세 (한국 dev 의 mid-career 영역)
**역할**: solo developer 또는 small team senior dev. 일상 workflow 가 코드 작성 + git + Claude Code / ChatGPT / Cursor 등 LLM tool 사용 위주.
**위치**: 한국 거주, Korean 1차 언어, 영어 reading 능숙.
**기술 stack**: MoAI-ADK user, Claude Code subscriber, Go / Python / TypeScript multi-lang, terminal + tmux + VSCode + git workflow.
**시간 패턴**: 7~9시 wake, 22~24시 sleep, 점심/저녁 식사 일정 reproducible.
**감정 패턴**: 정기적 stress (deadline / code review / 1:1) + 정기적 회복 (가족 / 산책 / 친구 약속).

### 4.2 Persona 정당화

이 persona 는 IDEA-002 brain decision 의 "본인 + 1명" success metric 의 본인 (GOOS행님) 의 profile 을 1차 모델로 한다:

- **MoAI-ADK user**: 본 프로젝트 owner 가 매일 MoAI-ADK 명령어를 사용하므로, MoAI workflow 와 통합되는 ritual companion 이 자연스러운 dogfood.
- **Korean speaker**: brand MINK (Made IN Korea) + IDEA-002 §brand 의 한국어 1차 정책.
- **Solo developer**: multi-tenant 야망 폐기 와 일관 — single user 가정.
- **Mid-career**: ritual / journal / health check 같은 self-care use case 가 mid-career dev 에게 가장 부합 (junior 는 chat-style LLM 더 사용, senior 는 personal companion 까지 도달).

### 4.3 Persona OUT (anti-persona)

다음 persona 는 v7.0 명시 anti-persona (이 사용자 group 을 위한 기능 추가 거부):

- **Enterprise team**: SLA / RBAC / audit log governance 등 enterprise 기능 anti-goal.
- **Marketplace plugin developer**: marketplace 자체가 anti-goal.
- **English-only global user**: 한국어 1차, 다국어는 후순위.
- **Mobile-first user**: CLI + Desktop 1차, mobile is anti-goal (1인 dev 의 native 환경은 terminal).
- **Casual chat user**: ChatGPT 대체 wannabe 사용자 — MINK 는 ritual companion, casual chat 1차 use case 아님.

---

## 5. Competitive landscape (Distancing)

### 5.1 v7.0 4개 distancing 대상 (IDEA-002 brain 결정)

다음 4개 product 와 MINK 의 차별화는 v7.0 §Distancing 에 명시 (each 1 단락):

#### 5.1.1 vs block/goose (multi-tenant agent platform)

**block/goose** (github.com/block/goose, Apache-2.0): block (Stripe spinoff) 의 agent framework. multi-tenant SaaS, enterprise governance, marketplace 지향. brand 충돌 (`goose` 이름).

**MINK 차별화**:
- multi-tenant agent platform 야망 폐기 — 1인 dev personal tool
- enterprise governance / SLA / RBAC anti-goal
- marketplace anti-goal
- brand reset (GOOSE → MINK) 으로 명시적 분리

#### 5.1.2 vs Hermes (AI girlfriend / emotional companion)

**Hermes** (대표 instance: 한국 시장에서 "AI 여친" 카테고리의 LLM-bot 서비스): emotional engagement / 24/7 chat / romance role-play 가 핵심 value.

**MINK 차별화**:
- chat 자체가 핵심 use case 아님 — ritual companion (정해진 morning/evening flow)
- emotional engagement 가 goal 이 아님 — daily structure 가 goal
- romance / persona role-play 자체가 anti-goal
- target audience: emotion-seeking general user (Hermes) → 1인 dev (MINK)
- relationship pattern: companion-as-romance (Hermes) → tool-as-ritual-anchor (MINK)

#### 5.1.3 vs Replika (long-term emotional AI)

**Replika** (luka.ai, 글로벌): personal AI for emotional support, friendship simulation, mental wellness 지원. multi-tenant SaaS, freemium ($69.99/yr Pro tier).

**MINK 차별화**:
- emotional simulation 1차 goal 아님 (MINK 의 journal 은 사용자 자체 기록 + light LLM 보조이지 AI persona 와 emotional bond 형성 아님)
- multi-tenant SaaS 아님 — local-first / 자기 호스팅
- mental wellness 진단 / 처방 anti-goal (crisis word 감지로 외부 hotline 안내 만)
- 사용 모델: AI 와 emotional bond 형성 (Replika) → 사용자 본인의 ritual 안에서 AI 가 보조 (MINK)

#### 5.1.4 vs Routinery (habit tracker app)

**Routinery** (routinery.app, 한국 + 글로벌): daily routine / habit tracking, gamification (streak), 행동 디자인 (BJ Fogg model) 기반.

**MINK 차별화**:
- gamification / streak 추구 anti-goal (1인 dev personal tool, behavioral nudge 가 핵심 가치 아님)
- habit tracker app vs ritual companion: routinery 는 행동 측정 도구, MINK 는 시간 동반자 (morning brief / journal / memory recall 의 narrative companion 적 본질)
- LLM context inject 부재 (Routinery) → ambient LLM context inject (MINK)
- 한국어 native (Routinery 도 한국제이나 글로벌 마케팅 강함, MINK 는 명시적 한국어 1차)

#### 5.1.5 vs OpenClaw (agent framework, Together AI)

**OpenClaw** (Together AI 운영, Apache-2.0): tool-use agent framework. 자기진화 없음, tool inventory + LLM orchestration 중심. developer toolkit 성격.

**MINK 차별화**:
- agent framework vs personal companion: OpenClaw 는 build 도구, MINK 는 end-user product
- 자기진화 (MoAI-ADK SPEC-REFLECT-001) — OpenClaw static
- multi-user agent platform 가정 (OpenClaw) → single-user personal tool (MINK)
- target audience: developer (OpenClaw) → 1인 dev 본인 + 1명 (MINK, end-user 위치)

### 5.2 비교 자체가 부적절한 product (v7.0 에서 비교 거부)

다음은 v7.0 §Distancing 에 명시 안 함 (비교 카테고리 오류):

- **ChatGPT / Claude / Gemini**: general-purpose LLM, MINK 는 personal companion app — 다른 category
- **Apple Intelligence / Microsoft Copilot**: OS-level assistant, multi-tenant SaaS — 다른 category
- **Cursor / Windsurf / GitHub Copilot**: IDE coding assistant — 다른 category

이런 product 들과 MINK 는 "비교 안 함" 이 명시적 입장. v7.0 §Distancing 본문에 "out of comparison scope" 로 short note 가능.

### 5.3 Competitive niche

v7.0 의 niche 한 줄 정의: **"1인 dev 의 매일 ritual companion (한국어 1차, local-first, journal + scheduler + ambient context 통합)"**.

이 niche 는 위 4개 distancing 대상 어디에도 해당 안 됨:
- Hermes/Replika 는 emotional companion (1인 dev tool 아님)
- Routinery 는 habit tracker (LLM ambient context 부재)
- OpenClaw 는 framework (end-user product 아님)
- block/goose 는 multi-tenant agent platform

---

## 6. 현재 .moai/project/ 구조 + product.md archive 결정

### 6.1 archive 방법론

`.moai/project/product.md` v6.0 본문은 byte-byte identical 로 다음 위치에 archive:

`.moai/project/product-archive/product-v6.0-2026-04-27.md`

archive 파일명 컨벤션: `product-v{version}-{file-mtime-date}.md`. v6.0 의 file modification date (또는 본문 HISTORY 명시일) 인 `2026-04-27` 을 suffix.

archive 디렉토리 `.moai/project/product-archive/` 는 본 SPEC PR 의 commit 에서 신설 (현재 부재). git mv 가 아닌 cp + 신규 product.md 작성 (v7.0) 방식. v6.0 의 byte 가 보존되어야 archive 의 의미가 있으므로 git history 가 아닌 별도 파일로 archive.

### 6.2 product.md HISTORY 형식

v7.0 의 product.md 는 frontmatter 와 HISTORY 표를 갖는다 (다른 SPEC 스타일에 정렬):

```markdown
---
version: 7.0.0
status: published
created_at: 2026-05-12
updated_at: 2026-05-12
brand: MINK
spec: SPEC-MINK-PRODUCT-V7-001
classification: VISION_DOCUMENT
---

# MINK — 1인 dev personal ritual companion

## HISTORY

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 7.0.0 | 2026-05-12 | manager-spec | v7.0 신설 — 1인 dev personal ritual companion 비전 정립. v6.0 ("Daily Companion Edition", multi-tenant agent platform 야망) → product-archive/product-v6.0-2026-04-27.md 로 archive. IDEA-002 brain Wave 3 §4.1 item 2 구현. SPEC-MINK-PRODUCT-V7-001 로 추적. |
```

이전 v6.0 까지의 HISTORY 는 archive 파일에 보존되고, v7.0 의 HISTORY 는 1행 (v7.0 신설) 부터 시작.

### 6.3 다른 .moai/project/ 파일과의 관계

v7.0 product.md 는 다음 sibling 파일들의 vision anchor 역할:

| 파일 | v7.0 와의 관계 |
|---|---|
| `.moai/project/tech.md` | implementation stack (Go + TS), v7.0 vision 의 stack 결정 근거로 인용 가능 |
| `.moai/project/structure.md` | repo layout, v7.0 와 직접 관계 없음 |
| `.moai/project/branding.md` | 옛 GOOSE brand 자료, downstream SPEC-MINK-BRAND-RENAME-001 으로 정정 예정 |
| `.moai/project/learning-engine.md` | v6.0 자기진화 stack 의 상세, v7.0 단순화 영향 받음 (단, 본 SPEC 에서 직접 수정 안 함) |
| `.moai/project/ecosystem.md` | v6.0 의 marketplace/Linux Foundation 계획, v7.0 anti-goal — downstream SPEC 으로 정정 또는 archive |
| `.moai/project/migration.md` | v6.0 의 GOOSE → MINK 이행 계획, MINK-BRAND-RENAME-001 으로 흡수 |
| `.moai/project/adaptation.md` | v6.0 의 적응형 학습 stack 상세, v7.0 단순화 영향 |
| `.moai/project/token-economy.md` | v6.0 의 freemium tier / pricing, v7.0 anti-goal — archive 권고 |
| `.moai/project/brand/*` | brand 자료 (별도 downstream SPEC-MINK-BRAND-RENAME-001 정정) |

본 SPEC `.moai/specs/SPEC-MINK-PRODUCT-V7-001/spec.md` 는 위 sibling 파일들의 정정 작업을 **명시 안 함** (downstream SPECs 또는 후속 정정 작업으로 위임). 본 SPEC 의 변경 범위는 `.moai/project/product.md` 1 파일 (재작성) + `.moai/project/product-archive/product-v6.0-2026-04-27.md` 1 파일 (신설) 2개로 제한.

---

## 7. Downstream SPEC 관계

### 7.1 SPEC-MINK-PRODUCT-V7-001 의 parent role

본 SPEC 은 다음 SPEC 들의 **vision anchor (constitutional parent)** 역할:

1. **SPEC-MINK-BRAND-RENAME-001** (planned, v0.1.1): brand identifier rename (`AI.GOOSE` → `MINK`, `goose` → `mink`). v7.0 vision 의 "MINK 1인 dev personal ritual companion" 정의가 brand rename 의 정당화 근거. spec.md §1 Background 에 v7.0 비전 인용 가능.
2. **SPEC-MINK-DISTANCING-STATEMENT-001** (proposed, Wave 3 §4.1 item 3): block/goose + Hermes/Replika/Routinery/OpenClaw distancing 의 detailed statement document. 본 SPEC §Distancing 의 4 단락이 그 detail SPEC 의 시작점. detail SPEC 은 marketing material / FAQ / website copy 영역.
3. **SPEC-MINK-USERDATA-MIGRATE-001** (downstream): `./.goose/` → `./.mink/` 실제 마이그레이션. v7.0 의 "local-first" + "single-user" 가정이 마이그레이션 정책의 base assumption.
4. **SPEC-MINK-ENV-MIGRATE-001** (downstream): env var `GOOSE_*` → `MINK_*` deprecation alias loader. 같은 base assumption.
5. **향후 Wave 3 SPEC 들** (FORTUNE / HEALTH / CALENDAR / RITUAL / BRIEFING): v7.0 의 핵심 ritual flow (§3.2) 가 신규 SPEC 의 use case 정의 근거.

### 7.2 backward 영향 (이미 완료된 SPEC 들과의 관계)

다음 완료 SPEC 들은 v7.0 vision 에 부합 — 별도 정정 불필요:

- **SPEC-GOOSE-CLI-001 ~ SPEC-GOOSE-CLI-TUI-003** (completed): CLI + TUI, v7.0 의 CLI 1차 지원 부합
- **SPEC-GOOSE-WEATHER-001** (completed): morning brief 의 weather component
- **SPEC-GOOSE-JOURNAL-001** (completed v0.3.0, 26/26 AC): v7.0 의 핵심 ritual (evening journal + long-term memory)
- **SPEC-GOOSE-SCHEDULER-001** (completed v0.2.2): morning brief + ritual time-marker
- **SPEC-GOOSE-MSG-TELEGRAM-001** (completed v0.1.3): mobile bridge (단, mobile-first 가 아닌 telegram bridge로 한정 — v7.0 부합)
- **SPEC-GOOSE-TOOLS-WEB-001** (completed): ambient web context
- **SPEC-GOOSE-OBS-METRICS-001** (completed): self-observability (단, external analytics 아님 — v7.0 부합)
- **SPEC-GOOSE-BRIDGE-001** + AMEND-001 (completed): cross-conn 인증

다음 완료 SPEC 들은 v7.0 vision 과 부합 여부 재검토 필요 (단, 본 SPEC 에서 직접 정정 안 함):

- **SPEC-GOOSE-COMPRESSOR-001** (completed): 본질이 LLM context compression — v7.0 의 ambient context inject 에 부합
- **SPEC-GOOSE-INSIGHTS-001** (completed): self-insight tracker, 본인 본인-매일 metric 추적 도구로 활용 가능 — v7.0 부합 (단, 외부 analytics 로 흘러가면 anti-goal)

이런 retro 정합성 평가는 본 SPEC scope 외, 후속 audit 으로 위임.

---

## 8. Risk Analysis

| # | 리스크 | 가능성 | 영향 | Mitigation |
|---|--------|--------|------|------|
| R1 | v7.0 본문이 너무 야망 축소되어 implementation 방향 보수 화 | 중 | 중 | "anti-goal" 명시 (multi-tenant / WAU / marketplace) 가 implementation 의 의도적 절제. 보수가 아닌 self-imposed constraint. R1 자체가 의도된 효과. |
| R2 | downstream Wave 3 SPEC (FORTUNE/HEALTH/CALENDAR) 가 v7.0 ritual flow 와 misalign | 중 | 중 | v7.0 §In-Scope §4 의 ritual flow 정의가 downstream SPEC 의 acceptance criteria 근거. 본 SPEC merge 후 Wave 3 plan 에서 v7.0 §4 cross-reference 강제. |
| R3 | v6.0 archive 본문의 license / IP 문제 | 매우 낮 | 낮 | v6.0 본문 자체가 Apache-2.0 license (현재 본문 §15 명시). archive 도 동일 license. byte-identical 보존이 license/IP 영역 추가 영향 없음. |
| R4 | v7.0 의 "본인 외 1명" 6m metric 이 너무 낮아 product 야망 부재로 인식 | 중 | 낮 | IDEA-002 brain reasoning (§3.4) 을 v7.0 §Success Metric 본문에 inline 인용. "낮은 ceiling 은 anti-goal 명시의 직접 표현, 의도된 절제" 명문화. |
| R5 | "1인 dev" persona 가 너무 좁아 본인 외 1명 사용자 자체가 어려움 | 중 | 중 | "본인 외 1명" 의 1명은 random user 가 아니라 본인의 close peer (개발자 친구). persona 영역도 close peer (Korean dev, similar workflow) 로 명시. |
| R6 | block/goose distancing 이 legal/trademark 영역으로 확대 | 매우 낮 | 매우 낮 | distancing 은 vision 차원의 카테고리 차별화 (multi-tenant agent platform vs personal ritual companion). trademark / legal claim 아님. v7.0 §Distancing 본문에서 명시. |
| R7 | Hermes / Replika / Routinery 의 사용자 일부가 MINK 와 confusion | 낮 | 낮 | §Distancing 4 단락이 정체성 명시 → confusion 해소. 외부 marketing 안 하므로 confusion 자체 도달성 낮음. |
| R8 | v7.0 vision 변경 → 기존 .moai/project/* sibling 파일 (tech.md / structure.md / 등) 의 일관성 손상 | 중 | 중 | 본 SPEC 은 product.md 1 파일만 변경. sibling 파일 정정은 별도 후속 audit/SPEC. 단, 본 SPEC §Downstream Influence 에 sibling 파일 영향 명시. |

---

## 9. References

### 9.1 사용자 결정 trail

- **IDEA-002 brain decision** (2026-05-12 결정 trail, brain 디렉토리는 별도 프로젝트 `/Users/goos/Projects/GooseBot/.moai/brain/IDEA-002/` 에 보존): GooseBot 한국시장 단타 LLM-bot 별도 분리 + AI.GOOSE → MINK rename + 1인 dev personal tool 결정. 8개 결정사항 (brand / metric / scope / supersede / branch / mode / prefix / immutable).
- 사용자 8개 확정사항 (orchestrator-collected, 2026-05-12, project_idea_002_mink_brain_complete memory): scope / 6m metric / brand / target audience / language / multi-tenant 폐기 / anti-goals / Wave 3 후속 SPEC roadmap.

### 9.2 관련 SPEC

- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/spec.md` v0.1.1 (planned, downstream child): brand identifier rename. 본 SPEC merge 후 brand rename 의 정당화 근거로 §1 Background 인용.
- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/research.md` (planned): brand rename 의 현황 + research. v6.0 → v7.0 vision 변환의 implementation evidence.

### 9.3 본 프로젝트 자료

- `.moai/project/product.md` v6.0 (38665 bytes, 2026-04-22, "Daily Companion Edition"): 본 SPEC 의 변환 대상
- `.moai/project/brand/{brand-voice,target-audience,visual-identity}.md`: 현재 모두 _TBD_ stub (brand interview 미완). 본 SPEC scope 외 (별도 brand interview 완료 후 정정).
- `CLAUDE.md` §1 Core Identity: "MoAI is the Strategic Orchestrator" — v7.0 의 dogfooding 정당화 근거
- `CLAUDE.local.md` §1 Git Flow: feature branch + squash PR — 본 SPEC merge 경로

### 9.4 외부 참조

- **block/goose**: <https://github.com/block/goose> (multi-tenant agent framework, brand 충돌 source)
- **Hermes** (한국 AI 여친 LLM-bot 시장 카테고리): 별도 commercial instance 다수
- **Replika**: <https://replika.ai> (luka.ai 운영, emotional AI companion)
- **Routinery**: <https://routinery.app> (habit tracker, 한국제 + 글로벌)
- **OpenClaw / Together AI**: <https://together.ai> (agent framework)

### 9.5 grep / count reproducibility

본 research 의 v6.0 본문 분석은 다음으로 재현 가능:

```bash
# §2.1 product.md v6.0 file size + line count
ls -l .moai/project/product.md           # 38665 bytes
wc -l .moai/project/product.md           # ~1189 lines
head -3 .moai/project/product.md         # v4.0 GLOBAL EDITION header

# §6.1 archive target verification
ls .moai/project/product-archive/ 2>&1   # No such file or directory (archive 부재, 본 SPEC PR 에서 신설)

# §2.6 v6.0 의 OUT scope identifier
grep -c '거위\|GOOSE\|Goose\|goose\|🪿' .moai/project/product.md   # 200+

# §7.1 downstream SPEC 존재
ls -d .moai/specs/SPEC-MINK-*            # SPEC-MINK-BRAND-RENAME-001/, SPEC-MINK-PRODUCT-V7-001/ (본 SPEC)
```

---

## 10. Decision Snapshot (preview of spec.md)

본 research 기반으로 spec.md 에 다음 결정 명문화:

1. **product.md v7.0 신설**: 본문 작성, `.moai/project/product.md` 위치 그대로.
2. **v6.0 archive**: `.moai/project/product-archive/product-v6.0-2026-04-27.md` 신설 (byte-identical preservation).
3. **vision 한 줄**: "MINK 는 1인 dev 의 매일 ritual 을 함께하는 personal AI companion".
4. **6m success metric**: 본인 매일 + 1명 daily user. 외부 stars/WAU/revenue anti-goal.
5. **persona**: 1인 dev (30~45세, 한국, MoAI/Claude Code user, Korean speaker).
6. **in-scope ritual flow 5종**: morning brief / journal / long-term memory / ambient LLM context / telegram bridge.
7. **anti-goals 명시**: multi-tenant SaaS / marketplace / enterprise tier / GitHub stars / Linux Foundation / global marketing / mobile-first.
8. **distancing 4종**: block/goose / Hermes / Replika / Routinery / OpenClaw (5개 — Wave 3 §4.1 item 3 detail SPEC 으로 detail 위임 가능).
9. **roadmap**: Wave 기반, 시간 추정 금지.
10. **scope 제한**: 본 SPEC 은 product.md 1 파일 + archive 1 파일 의 변경 만. sibling .moai/project/* 정정은 OUT.

---

Version: 0.1.0
Status: planned
Last Updated: 2026-05-12
