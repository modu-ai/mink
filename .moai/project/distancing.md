---
version: 1.0.0
status: published
created_at: 2026-05-12
updated_at: 2026-05-12
brand: MINK
classification: BRAND_DISTANCING
parent: SPEC-MINK-PRODUCT-V7-001
spec: SPEC-MINK-DISTANCING-STATEMENT-001
language: ko
---

# MINK — 5-product Brand Distancing Statement

## HISTORY

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 1.0.0 | 2026-05-12 | manager-spec | Initial draft. PRODUCT-V7-001 §6 detail document. 5-product distancing statement: block/goose, Hermes, Replika, Routinery, OpenClaw. 각 4-단락 표준 (Identity + MINK is NOT + How MINK differs + Note on coexistence). Anti-FUD, anti-AI-slop, 1인 dev tone, 한국어 1차. |

---

## Preamble

이 문서는 MINK가 다음 5가지 AI/automation 제품과 명시적으로 어떻게 다른지 정립한 **brand identity statement**입니다.

본 SPEC `SPEC-MINK-PRODUCT-V7-001` 의 §6 Distancing 섹션의 5개 단락 starting point를 detail 4-단락 구조로 확장한 것입니다. 각 제품이 누구인지(Identity), MINK가 그 영역을 추구하지 않는 이유(MINK is NOT), MINK의 niche가 어디서 갈라지는지(How MINK differs), 그리고 두 제품이 공존할 수 있는 방식(Note on coexistence)을 설명합니다.

**본 문서의 의도:**
- README의 brand section에 인용되는 canonical reference
- "MINK가 block/goose의 fork인가?" "Replika 같은 AI인가?" 같은 외부 질문에 명확한 답을 제시
- 향후 SPEC 작성자가 새 기능을 추가할 때 "이 기능이 Hermes/Routinery 영역으로 흘러가지 않는가?" 점검하는 anti-goal reference
- Brand 충돌 시 공식 distancing statement

**본 문서는 각 제품 간 명확한 차이를 기술하되, 어느 제품이 우월하거나 열등하다고 판단하지 않습니다.** MINK의 niche가 어디서 그들과 갈라지는지 사실만 기술합니다. 각 제품은 그 영역에서 훌륭한 선택이며, MINK는 다른 영역의 선택일 뿐입니다.

**brand voice:** 1인 dev tone, 한국어 1차, 사실 기반. AI-slop 마케팅 톤, 우월성 주장, 시간 추정 부재.

---

## vs block/goose

### Identity: 다중 테넌트 에이전트 플랫폼

block/goose는 **Stripe 계열사인 block, Inc.가 운영하는 다중 테넌트 에이전트 플랫폼**입니다. Apache-2.0 라이선스 오픈소스이며 (<https://github.com/block/goose>), 기업 팀이 여러 LLM 제공자(OpenAI, Anthropic, etc.)를 통합하여 에이전트 앱을 구축하는 개발자 도구입니다. Enterprise 팀, SaaS 운영자, agent marketplace 참여자가 주요 사용자입니다.

### MINK is NOT block/goose

MINK는 다중 테넌트 플랫폼이 아닙니다. Enterprise SaaS, 마켓플레이스, 중앙 호스팅, RBAC/audit log 같은 기업 거버넌스 요구사항을 추구하지 않습니다. block/goose의 fork도 아니며, 그것의 대체제가 아닙니다. MINK는 block/goose와 완전히 다른 카테고리의 제품입니다.

### How MINK differs from block/goose

block/goose는 **개발자 도구(developer toolkit)**이고, MINK는 **end-user 개인용 제품(personal product)**입니다.

- **사용자 규모:** block/goose는 1000명 이상의 enterprise 팀을 지원하도록 설계. MINK는 본인이 매일 쓰고, 본인 외 1명이 매일 쓰는 수준이 성공 metric. (PRODUCT-V7 success metric 참조)
- **호스팅 모델:** block/goose는 중앙 호스팅 / 마켓플레이스. MINK는 로컬 우선(local-first), 사용자 본인이 호스팅하거나 자기 서버에 배포.
- **도구 vs 제품:** block/goose는 개발자가 agent app을 build하는 도구. MINK는 1인 dev가 본인의 daily ritual을 위해 직접 쓰는 end-user 제품.
- **목표 고객:** block/goose는 기술 팀의 DevOps/infra engineer. MINK는 1인 dev 또는 시니어 개발자 본인.

### Note on coexistence

개발자가 block/goose로 기업 agent app을 구축하면서 MINK를 개인 ritual companion으로 동시에 사용하는 것은 자연스럽습니다. 둘은 완전히 다른 층(infrastructure vs end-user experience)에 있기 때문입니다.

---

## vs Hermes

### Identity: AI 여친/친구 LLM-bot (한국 시장 카테고리)

Hermes는 한국 AI 여친/친구 카테고리의 일반명입니다. 단일 제품이 아니라 여러 상업 vendor가 제공하는 "24/7 가능한 AI 여친" LLM-bot 카테고리입니다. 핵심은 **emotional engagement와 romance role-play**입니다. 사용자는 AI와 지속적인 대화를 통해 emotional support, companionship, persona connection을 추구합니다.

### MINK is NOT Hermes

MINK는 emotional engagement를 1차 goal로 하지 않습니다. AI와 romance/emotional relationship을 형성하는 것도 anti-goal입니다. MINK의 AI는 보조 역할이며, 사용자 본인의 daily ritual이 중심입니다.

### How MINK differs from Hermes

Hermes는 **시간 독립적 engagement (anytime chat, 24/7 available)**를 추구하고, MINK는 **시간 앵커 ritual (정해진 아침/저녁 ritual)**을 추구합니다.

- **Engagement 패턴:** Hermes는 사용자가 원할 때 언제든지 AI와 대화하고 emotional bond를 형성. MINK는 아침(운세·날씨·일정), 저녁(일기·감정)처럼 정해진 time anchor에 나타나는 ritual companion.
- **목표 감정:** Hermes는 emotional fulfillment, companionship, romance 추구. MINK는 일상의 structure 제공, 스트레스 신호 감지, ritual anchor 역할.
- **타겟 사용자:** Hermes는 emotional support 추구하는 일반 사용자 (casual chat consumer). MINK는 1인 dev, daily structure와 personal context 중심인 기술자.
- **Relationship pattern:** Hermes는 AI와의 romantic/emotional relationship. MINK는 tool-as-ritual-anchor (AI는 도구, ritual의 시간 앵커).

### Note on coexistence

사용자가 emotional companion이 필요한 순간에 Hermes/Replika를 사용하고, daily ritual의 structure와 context가 필요할 때 MINK를 사용하는 것은 자연스럽습니다. 두 need는 다른 영역입니다.

---

## vs Replika

### Identity: 장기 emotional AI companion

Replika는 **Luka, Inc. (luka.ai)가 운영하는 freemium SaaS 서비스**입니다. Pro 구독 약 $69.99/year 비용. 핵심은 **AI persona와의 long-term emotional bond 형성**입니다. 사용자는 시간이 지나면서 AI persona의 personality를 배우고, emotional connection을 심화시키며, friendship simulation을 경험합니다. mental wellness 보조도 명시적 목표입니다.

### MINK is NOT Replika

MINK는 AI persona와의 emotional bond 형성을 1차 goal로 하지 않습니다. Replika의 요금제 모델(freemium SaaS)을 따르지 않으며, 다중 테넌트 cloud 서비스가 아닙니다(MINK는 로컬 우선). Mental wellness 진단이나 처방을 제공하지 않습니다(crisis word 감지 시 외부 hotline 안내만).

### How MINK differs from Replika

Replika는 **AI persona와의 emotional learning**을 중심으로 하고, MINK는 **사용자 본인의 기록과 routine learning**을 중심으로 합니다.

- **중심 데이터:** Replika는 AI persona의 personality 학습(사용자와 AI의 대화 history로 AI가 진화). MINK는 사용자 본인의 일기, 습관, 감정 패턴 기록(AI는 light LLM 보조).
- **Memory focus:** Replika는 "AI가 나를 기억한다" (persona memory). MINK는 "내가 내 일상을 기록한다"(user memory).
- **Hosting model:** Replika는 cloud SaaS (multi-tenant, luka.ai 서버). MINK는 로컬 우선(local SQLite, 사용자 본인 호스팅).
- **Mental health positioning:** Replika는 mental wellness 보조 명시. MINK는 crisis word 감지 시 외부 전문 hotline만 안내(진단/처방 거부).

### Note on coexistence

Mental wellness 영역의 전문 보조가 필요한 사용자는 Replika 또는 임상 전문가 상담을 추구해야 합니다. MINK는 그 영역을 대체할 수 없으며, 본인 기록과 light ambient context만 제공합니다.

---

## vs Routinery

### Identity: 습관 추적 모바일 앱

Routinery는 **한국제 startup이 운영하는 습관 추적 모바일 앱** (<https://routinery.app>)입니다. 행동 디자인(behavioral design, BJ Fogg model 기반 추정) 원칙으로 daily routine 관리를 돕습니다. Gamification을 강조하며, streak(연속성)과 통계로 사용자를 nudge합니다. 모바일 우선 설계이며 freemium 비즈니스 모델입니다.

### MINK is NOT Routinery

MINK는 habit tracker가 아닙니다. Routine 측정, streak count, gamification 요소 모두 anti-goal입니다. Behavioral nudge optimization을 추구하지 않습니다. Mobile-first 설계도 아닙니다(MINK는 CLI + Desktop TUI 1차).

### How MINK differs from Routinery

Routinery는 **행동 측정과 동기화(behavior measurement + gamification)**를 중심으로 하고, MINK는 **ritual 동반과 cross-domain context integration**을 중심으로 합니다.

- **중심 개념:** Routinery는 "routine 완료 여부를 추적하고 streak으로 동기 부여". MINK는 "daily ritual의 시간대에 나타나 cross-domain context 제공".
- **Measurement vs Context:** Routinery는 action metrics(completed/not completed, streak). MINK는 ambient context(journal entry, weather, mood trend, scheduler 통합).
- **LLM 역할:** Routinery는 LLM 미포함(행동 디자인 중심). MINK는 LLM ambient context injection이 핵심(light language model이 ritual 대화에 context 추가).
- **Platform:** Routinery는 모바일 우선. MINK는 CLI + Desktop TUI 우선 (모바일은 Telegram bridge 한정).

### Note on coexistence

둘 다 한국제 제품이며, 사용자가 routine 측정은 Routinery로, ritual companion 영역은 MINK로 나누어 사용하는 것은 자연스럽습니다. Measurement와 companionship은 다른 need이기 때문입니다.

---

## vs OpenClaw

### Identity: Agent framework (Together AI)

OpenClaw는 **Together AI가 운영하는 agent framework 및 developer toolkit** (<https://together.ai>)입니다. Apache-2.0 라이선스. 핵심은 **tool-use orchestration과 LLM-agnostic agent 구축**입니다. Developer가 자신의 agent app을 build하기 위해 필요한 framework, library, documentation을 제공합니다.

### MINK is NOT OpenClaw

MINK는 agent framework가 아닙니다. Developer toolkit도 아닙니다. OpenClaw 같은 tool-use orchestration framework의 consumer도 아닙니다(내부적으로 LLM을 호출하기는 하지만, framework consumer가 아님). MINK는 end-user product이며, 개발자가 자신의 agent를 build하는 도구가 아니라, 1인 dev가 본인의 daily ritual을 위해 직접 사용하는 product입니다.

### How MINK differs from OpenClaw

OpenClaw는 **정적 framework(static tools and orchestration)**이고, MINK는 **end-user product with self-evolution capability**입니다.

- **Audience:** OpenClaw는 developer(agent builder의 대상). MINK는 1인 dev 본인 + 1명 daily user (framework consumer가 아닌 end-user).
- **Purpose:** OpenClaw는 "agent app을 build하기 위한 infrastructure". MINK는 "personal ritual companion product를 use하기".
- **Evolution model:** OpenClaw는 static framework (developer가 update하면 consumer는 새 버전으로 upgrade). MINK는 self-evolution (MoAI-ADK SPEC-REFLECT-001, 자기 기록에서 배워서 자체 개선).
- **Flexibility:** OpenClaw는 framework consumer의 custom needs에 맞게 extend 가능. MINK는 1인 dev의 본인 ritual에만 최적화.

### Note on coexistence

Developer가 OpenClaw로 자신의 agent app을 build하고, MINK를 personal ritual companion으로 동시에 사용하는 것은 자연스럽습니다. 서로 다른 purpose(build vs use)를 가진 도구이기 때문입니다.

---

## Out of Comparison Scope

다음 제품들과는 MINK가 비교 대상이 되지 않습니다:

- **ChatGPT, Claude, Gemini:** 범용 LLM (general-purpose language models). MINK는 personal ritual companion 제품이며 LLM 그 자체가 아님.
- **Apple Intelligence, Microsoft Copilot:** OS-level assistant (OS 레벨의 다중 테넌트 비서). MINK는 single-user personal tool.
- **Cursor, Windsurf, GitHub Copilot:** IDE coding assistant. MINK는 code editor 대상이 아니라 daily ritual 대상.

---

## Closing Statement

**MINK는 1인 dev의 매일 ritual을 함께하는 personal AI companion입니다.**

위 5가지 제품(block/goose, Hermes, Replika, Routinery, OpenClaw)과 MINK는 다른 niche에 있습니다. 각각의 카테고리 차이는 우열(better/worse)이 아니라 **사용자 intention의 차이**입니다. 본인이 매일 쓰고, 본인의 close peer 1명이 매일 쓰는 것이 MINK의 성공입니다. 우리는 그 niche를 깊게 파기로 약속합니다.

---

## References

### 본 문서의 정의

- **SPEC-MINK-DISTANCING-STATEMENT-001:** 본 문서를 정의하는 SPEC. `.moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/spec.md`
- **SPEC-MINK-PRODUCT-V7-001:** 본 문서의 parent SPEC. 5-product starting point의 vision-level definition. `.moai/specs/SPEC-MINK-PRODUCT-V7-001/spec.md`

### Sibling SPEC

- **SPEC-MINK-BRAND-RENAME-001:** Brand identifier rename (GOOSE→MINK). 본 SPEC과 body 변경 영역은 disjoint.

### 5 Product 공식 정보

- **block/goose:** <https://github.com/block/goose> (Apache-2.0, Stripe spinoff)
- **Hermes:** 한국 시장 "AI 여친/친구" LLM-bot 카테고리 (multiple commercial vendors, single canonical URL 부재)
- **Replika:** <https://replika.ai> (Luka, Inc. 운영, <https://luka.ai>)
- **Routinery:** <https://routinery.app> (한국제 startup)
- **OpenClaw:** <https://together.ai> (Together AI 운영, agent framework)

### 결정 근거

- **IDEA-002 brain decision** (2026-05-12): 8개 확정사항, §7 distancing 5 product 명시
- **PRODUCT-V7-001 research.md:** §5 Competitive landscape (Distancing) 의 5 product 상세 분석

---

**Version:** 1.0.0  
**Status:** published  
**Created:** 2026-05-12  
**Classification:** BRAND_DISTANCING  
**License:** Apache-2.0
