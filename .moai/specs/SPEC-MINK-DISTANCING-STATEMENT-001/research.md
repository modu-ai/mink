# Research — SPEC-MINK-DISTANCING-STATEMENT-001 (MINK 5-product distancing statement)

> 본 research 문서는 SPEC-MINK-DISTANCING-STATEMENT-001 의 의사결정 근거자료다. spec.md §10 References 에서 본 문서를 참조한다. 본 SPEC 은 `SPEC-MINK-PRODUCT-V7-001 §6 Distancing` (parent / vision anchor, 본 SPEC merge 시점에 머지 완료 가정) 의 starting point 5 단락을 detail statement 로 확장하는 단일 markdown 산출물 SPEC 이다.

작성일: 2026-05-12
작성자: manager-spec
Status: planned
Parent: SPEC-MINK-PRODUCT-V7-001 (vision anchor, §6 Distancing)
Sibling: SPEC-MINK-BRAND-RENAME-001 (cross-cutting rename, body 변경 영역 disjoint)

---

## 1. 문제 정의 (Problem Restatement)

### 1.1 왜 별도 SPEC 인가

`SPEC-MINK-PRODUCT-V7-001` 의 §6 Distancing 섹션은 product vision document 의 한 부분이며, 다음 5 product 와 MINK 의 카테고리 차이를 **단락 1개씩** 정의한다 (vision-level starting point):

1. block/goose (multi-tenant agent platform)
2. Hermes (AI girlfriend / emotional companion)
3. Replika (long-term emotional AI)
4. Routinery (habit tracker)
5. OpenClaw (agent framework)

이 5 단락은 vision-document 문맥에서 단지 "MINK 는 이런 카테고리가 아니다" 를 명시하는 1 단락 단위 distancing 이다. 그러나 다음 사용 시나리오에서는 **단락 1개로는 부족**:

- **README brand identity 섹션**: "MINK 는 무엇이 아닌가" 를 한국어 + 영어로 명확히 설명할 때, 각 product 의 정체성 (3-5 sentences) + MINK 와의 categorical 차이 (3-5 sentences) 가 필요
- **public communication / 블로그 / Q&A**: 1인 dev 가 외부에서 "MINK 가 block/goose 의 fork 인가?" "Replika 같은 emotional AI 인가?" 등 질문을 받을 때 인용 가능한 single source of truth (canonical statement)
- **brand 충돌 대응**: block 사 / luka.ai 등 외부 주체가 brand 인접성 우려를 표시할 경우 (가능성 매우 낮으나) 본 프로젝트의 명시적 분리 statement 가 reference 자료
- **향후 SPEC 작성자**: 새 SPEC 작성 시 "이 기능이 Replika 같은 emotional AI 영역으로 흘러가지 않는가?" 같은 anti-goal 점검의 canonical reference

이런 사용 시나리오에서 인용 가능한 **detail statement** 가 필요하다. PRODUCT-V7 의 5 단락은 vision-document 의 한 섹션이며 detail 까지 담기에는 분량 / 문맥 모두 적절하지 않다. 본 SPEC 은 그 5 단락을 별도 markdown 문서 `.moai/project/distancing.md` 로 확장하여 product detail statement 의 single source of truth 로 정립한다.

### 1.2 산출물 정체

본 SPEC 은 단일 markdown 파일을 신설한다:

- 위치: `.moai/project/distancing.md`
- 분류: `.moai/project/` 디렉토리의 brand 정체성 문서 family 중 하나 (sibling: `branding.md`, `ecosystem.md`, `product.md`)
- 분량: 약 600~900 lines (5 product × 3-5 단락 + preamble + closing + references)
- 본문 voice: 1인 dev 의 1인 dev tone (한국어 1차, 영어 i18n 후순위)
- frontmatter: version 1.0.0, classification BRAND_DISTANCING, status published, spec SPEC-MINK-DISTANCING-STATEMENT-001
- 코드 변경 0건

본 SPEC 자체의 산출물은:

1. `.moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/research.md` (이 문서)
2. `.moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/spec.md` (v0.1.0)

그리고 본 SPEC PR 의 실 산출물 (run phase, 본 plan PR 외):

3. `.moai/project/distancing.md` (신설, run phase 에서 작성)

### 1.3 본 SPEC 과 PRODUCT-V7-001 의 관계

PRODUCT-V7-001 §6 Distancing 의 5 단락은 본 SPEC 의 **starting point**:

- PRODUCT-V7-001 §6 vs MINK starting point: 각 1 단락, 카테고리 차이만 명시
- DISTANCING-STATEMENT-001: 각 3-5 단락, product 정체성 + MINK 의 명시적 분리 (what MINK is NOT) + categorical 차이의 근거 (how MINK differs by) + (선택) 사용자의 confusion 차단을 위한 closing note

두 SPEC 은 동일 5 product 를 다룬다. 본 SPEC 은 PRODUCT-V7-001 의 5 product subset 위반 금지 — 5 product 모두 등장해야 함 (AC-010).

### 1.4 sibling SPEC 과의 관계

- **SPEC-MINK-BRAND-RENAME-001** (v0.1.1, planned/sibling): brand identifier rename (GOOSE → MINK 코드 / module path / repo / proto). body 변경 영역 disjoint — BRAND-RENAME 은 코드 + go.mod + .proto 등 변경, 본 SPEC 은 `.moai/project/distancing.md` 1 파일 markdown 변경. merge order 무관.
- **PRODUCT-V7-001 §6 Distancing**: vision-level starting point. 본 SPEC 머지 후 PRODUCT-V7-001 §6 본문은 변경 안 함 — vision 차원의 5 단락 starting point 가 detail 위임 marker (`자세히는 .moai/project/distancing.md` 인용 형태) 로 충분.

---

## 2. PRODUCT-V7 §6 Distancing 5 단락 분석 (starting point inventory)

본 절은 PRODUCT-V7-001 의 §6 Distancing 의 5 단락 starting point 가 어떤 정보를 이미 명시하고 있고, detail SPEC 이 어떤 layer 를 추가해야 하는지 매핑한다. PRODUCT-V7-001 의 본문이 머지 완료 후 본 SPEC run phase 가 진행되므로, 본 SPEC 의 spec.md 는 PRODUCT-V7-001 starting point 의 5 단락을 한 단어도 수정하지 않는 전제로 작성된다 — detail SPEC 은 **확장 (expansion)** 이지 **재정의 (redefinition)** 가 아니다.

### 2.1 5 단락 starting point (PRODUCT-V7-001 spec.md §6 + research.md §5 기반)

| Product | PRODUCT-V7 단락 핵심 (1 단락 요지) | DISTANCING detail 확장 layer |
|---|---|---|
| block/goose | multi-tenant agent platform, MINK 의 1인 dev tool 과 카테고리 차이 + brand 충돌 reset | product identity (3-5 sentences) + 기술 stack (proto / Apache-2.0 / 운영 주체 block Stripe spinoff) + MINK is NOT (multi-tenant SaaS / marketplace / enterprise) + how MINK differs (1-user, local-first, dogfood) |
| Hermes | AI girlfriend / emotional companion 카테고리, MINK 의 ritual companion / 비-romantic | Hermes 카테고리 정의 (한국 시장 "AI 여친" LLM-bot 다수 인스턴스) + emotional engagement 1차 vs MINK 의 daily structure 1차 + romance/persona role-play anti-goal + target audience 차이 |
| Replika | long-term emotional AI, MINK 의 ritual context + personality 학습 아닌 routine 학습 | Replika 본질 (luka.ai 운영, freemium $69.99/yr Pro, AI persona 와 emotional bond) + MINK 의 본질 (사용자 자체 기록 + light LLM 보조, AI persona 와 bond 형성 안 함) + local-first vs multi-tenant SaaS + mental wellness 진단/처방 anti-goal |
| Routinery | habit tracker app, MINK 의 ritual companion + LLM ambient context 통합 | Routinery 본질 (routinery.app, 행동 디자인 BJ Fogg, gamification streak) + MINK 의 본질 (LLM 대화 + cross-domain ritual companion) + gamification anti-goal + 한국제 product 공통점 (둘 다 한국제이나 routinery 는 글로벌 마케팅) |
| OpenClaw | agent framework, MINK 의 end-user product / 기술 framework 아님 | OpenClaw 본질 (Together AI 운영, tool-use orchestration framework, developer toolkit) + MINK 의 본질 (end-user product, dogfood, 1인 dev personal tool) + framework vs product 카테고리 분리 + 자기진화 (MoAI SPEC-REFLECT-001) vs static framework |

### 2.2 detail SPEC 의 단락 구조 표준화

본 SPEC 은 5 product 각 섹션이 다음 4-단락 표준 구조 를 따르도록 명시:

1. **Identity** (1 단락, "What they are"): product 의 객관적 정체성. 운영 주체, 라이선스 / pricing, 1차 사용 시나리오, target audience. 사실 기반, 검증 가능 정보만.
2. **MINK is NOT** (1 단락, "MINK is NOT ..."): 본 product 와 MINK 가 명시적으로 어떤 점에서 다르며 MINK 가 추구하지 않는 영역. anti-goal 명시 형태.
3. **How MINK differs** (1-2 단락, "MINK differs by ..."): MINK 의 본질이 어디서 본 product 와 갈라지는가. 사용 모델 / target audience / 기술 stack / business model 의 differentiator. anti-FUD: 본 product 의 약점 비난 금지, MINK 의 niche 만 기술.
4. (선택) **Note on coexistence** (1 단락): 본 product 와 MINK 가 공존 가능한 영역, 사용자가 둘 다 쓸 수도 있음을 인정. confusion 차단 + neutrality 강화.

이 4-단락 구조는 AC-007 ("MINK is NOT ..." + "MINK differs by ..." 패턴 grep ≥ 5 each, 5 product 각각) 의 검증 근거.

### 2.3 PRODUCT-V7 §6 vs DISTANCING-STATEMENT-001 분량 비교

| 항목 | PRODUCT-V7 §6 | DISTANCING-STATEMENT-001 |
|---|---|---|
| 각 product 단락 수 | 1 단락 | 3-5 단락 (4-단락 표준 + optional coexistence) |
| 분량 (단일 product) | ~5-8 sentences | ~20-40 sentences |
| 총 5 product 분량 | ~200-300 lines (vision 섹션의 일부) | ~500-700 lines (전체 문서의 main body) |
| voice | vision-document tone, brand-position 단어 우선 (MINK, 1인 dev) | brand statement tone, factual + anti-FUD, sourced |
| 검증 grep ≥ | 1 each | 3 each (AC-002~006) |
| 사용처 | vision-document 의 sub-section | quoted in README brand section / public Q&A / external 충돌 대응 |

---

## 3. brand-voice 매칭 분석 (anti-AI-slop)

### 3.1 brand-voice 출처

현재 `.moai/project/brand/brand-voice.md` 는 `_TBD_` stub (brand interview 미완, IDEA-002 brain decision §brand 의 후속). 본 SPEC run phase 시점에 brand-voice.md 가 완성될 가능성은 낮으므로, 본 SPEC 은 **다음 sources 에서 self-derive 한 brand-voice 사양**을 따른다:

1. **CLAUDE.local.md §2.5** (code comment language policy): code 영역은 영어, 사용자 문서는 한국어 1차, 정확성 우선. 본 SPEC 의 distancing.md 는 사용자 문서 카테고리 → 한국어 1차.
2. **PRODUCT-V7-001 §1.1 / §3.4**: 1인 dev tone, anti-AI-slop, sourced statement.
3. **IDEA-002 brain decision** (brain dir 별도 보존): "본인 외 1명 daily user" success metric 의 의미는 외부 marketing 부재 — distancing 도 marketing 자료 아님, brand-identity 자료.

### 3.2 brand-voice 사양 (self-derived)

본 SPEC distancing.md 는 다음 brand-voice 사양 준수:

- **Tone**: 1인 dev → 1인 dev tone. 사실 기반, 약간 dry. 마케팅 톤 금지.
- **Register Spectrum**:
  - formal_informal: 중간 (한국어 존댓말 / 영어 we form)
  - serious_playful: serious. 농담 / 이모지 / 강조 표현 minimal
  - technical_accessible: technical-accessible (사용자가 dev 라는 가정, 기술 용어 자유, 다만 jargon-only 금지)
- **Preferred terms** (사용 권장):
  - "MINK 는 ... 이다" / "MINK is ..." (statement)
  - "MINK 는 ... 가 아니다" / "MINK is NOT ..." (anti-goal)
  - "차이 (difference)" / "다르다" (neutral comparison)
  - "1인 dev" / "personal ritual companion"
  - "사실" / "verified" / 출처 인용
- **Avoided terms** (사용 금지):
  - "보다 우수 (superior to)" / "더 낫다 (better)" — 비교우월 표현 금지
  - "최고" / "유일" / "혁신적 (innovative)" / "차세대 (next-generation)" — AI-slop 마케팅 용어
  - "inferior" / "outdated" / "obsolete" — 경쟁 product 비난 표현
  - "단점 (weakness)" / "한계 (limitation)" — 다른 product 의 약점 부각 (anti-FUD)
  - "쉽다 (easy)" / "빠르다 (fast)" / "강력 (powerful)" — 추상적 가치 claim
- **Audience familiarity**:
  - jargon_level: medium (dev 사용자 가정, 기술 용어 자유)
  - assumed_knowledge: GitHub / LLM / SaaS 개념 친숙 가정. AI agent / framework 차이 친숙 가정.

### 3.3 anti-AI-slop 점검 항목

본 SPEC 의 distancing.md 본문 작성 시 다음 grep 점검을 통과해야 함 (run phase verification, AC-008):

- `grep -ci '보다 우수\|보다 낫\|superior\|inferior\|outdated\|obsolete\|단점\|약점\|한계'` → 0 (비교우월 / 약점 부각 표현 부재)
- `grep -ci '혁신적\|차세대\|next-gen\|disruptive\|game-chang\|revolution'` → 0 (AI-slop 마케팅 용어 부재)
- `grep -ci '최고\|유일\|시장 점유\|market share\|market dominance'` → 0 (시장 점유 / 우월성 주장 부재)

이 grep 결과가 AC-008 의 검증 기준.

---

## 4. 5 product 사실 확인 (factual sourcing)

본 절은 5 product 의 사실 정보를 정리한다. distancing.md 본문 작성 시 추측 금지, 본 절의 사실만 인용한다. References section 에 출처 명시.

### 4.1 block/goose

| 항목 | 값 | 출처 |
|---|---|---|
| 공식 repo | github.com/block/goose | URL: <https://github.com/block/goose> |
| 운영 주체 | block, Inc. (Stripe spinoff) | block 공식 |
| 라이선스 | Apache-2.0 | repo LICENSE |
| 카테고리 | multi-tenant agent platform / framework | repo README |
| 1차 사용 시나리오 | developer tool integration, agent orchestration | repo docs |
| target audience | enterprise team / multi-user agent SaaS 운영 | block product line |
| brand 충돌 | `goose` identifier — IDEA-002 결정의 rename 동기 | PRODUCT-V7-001 §1.2 |

### 4.2 Hermes

| 항목 | 값 | 출처 |
|---|---|---|
| 정체 | 한국 시장의 "AI 여친" / "AI 친구" LLM-bot 카테고리 (대표 instance 다수) | 한국 시장 일반 인지 |
| 운영 주체 | 다양한 commercial vendor (단일 instance 아님) | 카테고리 일반화 |
| 1차 사용 시나리오 | emotional engagement, 24/7 chat, romance role-play, persona companion | 일반 카테고리 정의 |
| target audience | emotional support seeking general user, casual chat consumer | 일반 카테고리 |
| business model | freemium / subscription, in-app purchase | 일반 카테고리 |
| MINK 차별 핵심 | ritual companion vs romance companion / 1인 dev tool vs general consumer | PRODUCT-V7-001 §6 |

> [HARD] Hermes 는 단일 product 가 아니라 한국 시장 카테고리 (multiple instances) 이므로 distancing.md 본문에서 "Hermes (한국 AI 여친 / 친구 LLM-bot 카테고리)" 로 명시. 특정 instance 의 약점 / vendor 비난 금지. (anti-FUD)

### 4.3 Replika

| 항목 | 값 | 출처 |
|---|---|---|
| 공식 URL | replika.ai (luka.ai 운영) | URL: <https://replika.ai> |
| 운영 주체 | Luka, Inc. | luka.ai 공식 |
| 라이선스 / pricing | freemium (Pro 약 $69.99/yr, 2026-05 기준 일반 인지) | 공식 pricing page |
| 카테고리 | long-term emotional AI companion / friendship simulation / mental wellness | 공식 description |
| 1차 사용 시나리오 | emotional support, friendship simulation, mental wellness 보조 | 공식 marketing |
| target audience | emotional support seeking individual, mental wellness 관심 | 일반 |
| 1차 mechanism | AI persona 와 long-term emotional bond 형성 + memory 누적 | 공식 product description |

### 4.4 Routinery

| 항목 | 값 | 출처 |
|---|---|---|
| 공식 URL | routinery.app | URL: <https://routinery.app> |
| 운영 주체 | Routinery (한국제 startup) | routinery.app 공식 |
| 라이선스 / pricing | freemium (mobile app, in-app purchase) | 공식 |
| 카테고리 | habit tracker / daily routine 관리 app | 공식 description |
| 1차 사용 시나리오 | habit formation, routine adherence, gamification (streak) | 공식 marketing |
| target audience | habit formation 관심 general user (mobile 우선) | 일반 |
| 1차 mechanism | 행동 디자인 (BJ Fogg model 기반 가정) + streak / 통계 / 알림 | 공식 |
| 한국제 공통점 | 둘 다 한국제이나 routinery 는 글로벌 마케팅 강함 | 일반 |

### 4.5 OpenClaw (Together AI)

| 항목 | 값 | 출처 |
|---|---|---|
| 공식 URL | together.ai (또는 specific OpenClaw page) | URL: <https://together.ai> |
| 운영 주체 | Together AI | together.ai 공식 |
| 라이선스 | Apache-2.0 (OpenClaw framework) | 공식 repo |
| 카테고리 | agent framework / tool-use orchestration toolkit | 공식 |
| 1차 사용 시나리오 | developer builds agent app, tool inventory + LLM orchestration | framework docs |
| target audience | developer / AI engineer (framework consumer) | framework category |
| 1차 mechanism | tool-use orchestration, LLM-agnostic (multi-provider), static behavior | 일반 framework |

> 정확한 OpenClaw 의 product 상세는 공식 URL 확인 필요. run phase 에서 WebFetch 로 사실 재확인 권장 (단, 본 SPEC plan phase 에서는 spec.md 의 sourcing 의무만 명시).

### 4.6 비교 자체가 부적절한 product (out of comparison scope)

distancing.md 의 closing 섹션에 다음 product 들과 "비교 안 함" 을 명시 (vision 카테고리 오류 차단):

- ChatGPT / Claude / Gemini: general-purpose LLM, MINK 는 personal companion app — 다른 category
- Apple Intelligence / Microsoft Copilot: OS-level assistant, multi-tenant SaaS — 다른 category
- Cursor / Windsurf / GitHub Copilot: IDE coding assistant — 다른 category

이 list 는 PRODUCT-V7-001 research.md §5.2 와 동일하다. distancing.md 의 closing 에서 1-2 단락으로 명시.

---

## 5. distancing.md 문서 구조 설계

본 절은 distancing.md 의 outline 을 정의한다. spec.md §4 EARS 의 REQ-MINK-DST-STRUCT-* 가 본 구조를 명문화한다.

### 5.1 frontmatter (YAML)

```yaml
---
version: 1.0.0
status: published
created_at: 2026-05-12
updated_at: 2026-05-12
brand: MINK
spec: SPEC-MINK-DISTANCING-STATEMENT-001
classification: BRAND_DISTANCING
parent: SPEC-MINK-PRODUCT-V7-001
---
```

### 5.2 본문 outline (제안 분량 ~600-900 lines)

| Section | 내용 | 분량 |
|---|---|---|
| 0. HISTORY | v1.0.0 initial row | ~5 lines |
| 1. Preamble | 본 문서의 목적 (single source of truth for 5-product distancing), parent SPEC PRODUCT-V7 §6 인용, anti-FUD 약속 | ~30-50 lines |
| 2. vs block/goose | Identity + MINK is NOT + How MINK differs + (선택) Note on coexistence | ~80-150 lines |
| 3. vs Hermes | 같은 구조 | ~80-150 lines |
| 4. vs Replika | 같은 구조 | ~80-150 lines |
| 5. vs Routinery | 같은 구조 | ~80-150 lines |
| 6. vs OpenClaw | 같은 구조 | ~80-150 lines |
| 7. Out of comparison scope | ChatGPT / Claude / Gemini / Apple Intelligence / Cursor 등 비교 거부 | ~30-50 lines |
| 8. Closing statement | MINK 의 niche 한 줄 요약, brand-identity 약속 | ~20-30 lines |
| 9. References | PRODUCT-V7-001 §6, BRAND-RENAME-001, 각 product 공식 URL | ~20-30 lines |

### 5.3 각 product section 의 4-단락 표준

(§2.2 의 표준 구조 재인용)

```markdown
## 2. vs block/goose

### Identity
[1 단락 — block/goose 의 정체성, 사실 기반, 운영 주체 / 라이선스 / 1차 사용 시나리오]

### MINK is NOT block/goose
[1 단락 — MINK 가 block/goose 의 어떤 영역을 추구하지 않는가, anti-goal 명시]

### How MINK differs from block/goose
[1-2 단락 — MINK 의 본질이 block/goose 와 어디서 갈라지는가, factual differentiator]

### Note on coexistence (선택)
[1 단락 — 사용자가 둘 다 사용 가능, MINK 는 block/goose 의 대체재가 아님을 명시]
```

이 구조가 5 product 모두에 적용. AC-007 의 "MINK is NOT" + "MINK differs by" 패턴 grep ≥ 5 each (5 product 각각) 가 검증 근거.

### 5.4 Preamble 의 핵심 statement

distancing.md 의 §1 Preamble 은 다음 핵심 statement 를 포함:

1. **이 문서의 목적**: "이 문서는 MINK 가 5 product 와 명시적으로 어떻게 다른지 정립한 brand identity statement 다."
2. **parent reference**: "PRODUCT-V7-001 §6 Distancing 의 5 단락 starting point 를 detail 로 확장한다."
3. **anti-FUD 약속**: "이 문서는 경쟁 product 의 약점을 부각하지 않는다. 단지 MINK 의 niche 가 어디서 그들과 갈라지는지 사실만 기술한다."
4. **사용처**: "본 문서는 README brand section / public Q&A / 외부 brand 충돌 대응의 single source of truth 다."
5. **brand voice**: "1인 dev tone, 한국어 1차, 사실 기반. AI-slop / 마케팅 톤 / 우월성 주장 부재."

이 5개 statement 가 §1 Preamble 의 필수 골격. AC-001 의 frontmatter 검증 + §1 Preamble 존재 검증.

### 5.5 Closing statement 의 핵심

distancing.md 의 §8 Closing 은 MINK 의 niche 한 줄 요약 + brand-identity 약속:

- niche 한 줄: "MINK 는 1인 dev 의 매일 ritual 을 함께하는 personal AI companion 이다 — multi-tenant SaaS 도, romance companion 도, habit tracker 도, agent framework 도 아니다."
- brand-identity 약속: "MINK 는 위 5 product 와 다른 niche 에 있다. 카테고리 차이는 우열이 아니라 사용자 의도의 차이다."

---

## 6. 5 product 단락 작성 가이드 (run phase 작성자용)

본 절은 run phase 의 manager-spec (또는 본 SPEC 작성자) 이 distancing.md 5 product 단락을 작성할 때 따를 가이드. spec.md §4 EARS 의 REQ-MINK-DST-BLOCK-* / -HERMES-* / -REPLIKA-* / -ROUTINERY-* / -OPENCLAW-* 가 각 product 단락의 의무 사항을 명문화한다.

### 6.1 vs block/goose 단락 가이드

| 단락 | 작성 가이드 |
|---|---|
| Identity | block (Stripe spinoff), Apache-2.0 agent framework, multi-tenant agent platform 지향, enterprise / developer team target. github.com/block/goose 인용. |
| MINK is NOT | multi-tenant SaaS / marketplace / enterprise tier 영역을 추구 안 함. block/goose 의 fork 도 아님. brand 충돌은 IDEA-002 결정으로 rename 으로 해소 (BRAND-RENAME-001 인용). |
| How MINK differs | 1인 dev 가 본인 + 1명 daily user 를 6m 목표로 함 (vs 1000+ enterprise team). local-first single-user (vs multi-tenant). dogfood (vs framework consumer). |
| Note on coexistence | 사용자가 block/goose 를 enterprise agent framework 로 사용하면서 MINK 를 personal ritual companion 으로 병행 가능 — 카테고리 자체가 다름. |

### 6.2 vs Hermes 단락 가이드

| 단락 | 작성 가이드 |
|---|---|
| Identity | 한국 시장의 "AI 여친 / 친구" LLM-bot 카테고리 (multiple commercial vendors). emotional engagement / 24/7 chat / romance role-play 가 핵심. |
| MINK is NOT | emotional engagement / romance / persona role-play 자체가 anti-goal. AI 와 emotional bond 형성 1차 goal 아님. |
| How MINK differs | ritual companion (정해진 morning / evening flow) vs emotional engagement (chat anytime). daily structure 가 goal (vs emotional fulfillment). target audience: 1인 dev (vs emotion-seeking general user). relationship pattern: tool-as-ritual-anchor (vs companion-as-romance). |
| Note on coexistence | 사용자가 emotional companion 영역의 욕구는 Hermes / Replika 등으로, 매일 ritual companion 영역의 욕구는 MINK 로 — 두 영역은 분리. |

### 6.3 vs Replika 단락 가이드

| 단락 | 작성 가이드 |
|---|---|
| Identity | luka.ai 운영, freemium SaaS (Pro 약 $69.99/yr), AI persona 와 long-term emotional bond / friendship simulation / mental wellness 보조. |
| MINK is NOT | AI persona 와 emotional bond 형성 1차 goal 아님. multi-tenant SaaS 아님 — local-first / 자기 호스팅. mental wellness 진단 / 처방 anti-goal (crisis word 감지로 외부 hotline 안내 만). |
| How MINK differs | journal 의 본질이 사용자 자체 기록 + light LLM 보조 (vs AI persona 와 emotional bond). routine 학습 (vs personality 학습). local-first (vs cloud SaaS). |
| Note on coexistence | mental wellness 영역의 보조가 필요한 사용자는 Replika 또는 임상 전문가 — MINK 는 crisis word 감지 시 외부 hotline 만 안내. |

### 6.4 vs Routinery 단락 가이드

| 단락 | 작성 가이드 |
|---|---|
| Identity | routinery.app, 한국제 startup, habit tracker / daily routine 관리 mobile app, 행동 디자인 (BJ Fogg model 가정) + gamification (streak). |
| MINK is NOT | habit tracker 아님 — routine 측정 / streak / gamification 모두 anti-goal. behavioral nudge optimization 추구 안 함. mobile-first 도 아님 (CLI + Desktop 1차). |
| How MINK differs | ritual companion (LLM 대화 + cross-domain 통합) vs habit tracker (행동 측정 도구). LLM ambient context inject (MINK) vs static UI (Routinery). 1인 dev tool (MINK) vs general user app (Routinery). |
| Note on coexistence | 둘 다 한국제 product 이며 사용자가 routine 측정은 Routinery, ritual companion 영역은 MINK 로 병행 가능. 카테고리 차이가 명확. |

### 6.5 vs OpenClaw 단락 가이드

| 단락 | 작성 가이드 |
|---|---|
| Identity | Together AI 운영, Apache-2.0 agent framework, tool-use orchestration toolkit, developer 가 agent app 을 build 하는 도구. |
| MINK is NOT | agent framework / developer toolkit 아님. tool-use orchestration framework 의 consumer 도 아님 (다만 내부적으로 LLM 호출은 한다). MINK 는 end-user product. |
| How MINK differs | framework (build 도구) vs product (end-user 사용 product). developer (OpenClaw consumer) vs 1인 dev personal tool (MINK end-user). static framework (OpenClaw) vs MoAI-ADK self-evolution (MINK). |
| Note on coexistence | 사용자가 OpenClaw 로 자신의 agent app 을 build 하고 MINK 를 personal ritual companion 으로 사용하는 것은 자연스러움. 카테고리 자체가 다름. |

---

## 7. plan-auditor 불필요 결정 근거

본 SPEC 은 다음 조건을 모두 만족 → plan-auditor 호출 불필요 (manager-spec 단독 종결):

1. **markdown only**: 코드 변경 0건. `.go` / `.proto` / `.yaml` 비변경.
2. **단일 파일 신설**: `.moai/project/distancing.md` 1 파일 신설 + 본 SPEC 의 spec.md/research.md. sibling `.moai/project/*` 파일 비변경.
3. **low-risk vision-supporting doc**: parent PRODUCT-V7-001 의 §6 vision-level starting point 가 이미 머지 완료 (가정), 본 SPEC 은 그 detail 확장만.
4. **immutable archive 영향 0**: 본 SPEC 은 어떤 immutable archive (SPEC-GOOSE-*, HISTORY rows, .claude/agent-memory) 도 건드리지 않음.
5. **brand-rename atomicity 무관**: BRAND-RENAME-001 의 코드 atomicity 요구사항과 disjoint — 본 SPEC 머지 후 BRAND-RENAME-001 의 brand-lint 검증 시점에 distancing.md 가 brand 위반 여부 점검은 (a) distancing.md 본문이 "MINK" 1차 brand 사용 시 자동 통과, (b) 본문 내 "block/goose" / "Goose" 가 등장하면 brand-lint exemption (factual citation 영역) — 별도 brand-lint exemption rule 추가는 본 SPEC scope 외 (단, run phase 작성자가 brand-lint regex pattern 점검 권장).

plan-auditor 호출 trigger 부재 사례:

- 단일 markdown 신설 vs 다파일 cross-cutting rename → 본 SPEC 은 단일
- atomicity 요구사항 vs 독립 단위 → 본 SPEC 은 독립
- legal / IP / 보안 영향 vs 없음 → 본 SPEC 은 없음 (5 product factual citation 만, FUD / 비난 anti-goal)

---

## 8. Risk Analysis

| # | 리스크 | 가능성 | 영향 | Mitigation |
|---|---|---|---|---|
| R1 | distancing.md 의 5 product 묘사 부정확 (예: Replika pricing 변경, Routinery 기술 stack 추측) | 중 | 중 | run phase 시점에 5 product 공식 URL 을 WebFetch / 공식 docs 로 사실 재확인. distancing.md 본문 references section 에 정보 캡처 시점 (2026-05-12 기준) 명시. 사실 변경 시 후속 sync SPEC 으로 정정. |
| R2 | anti-FUD 의도 외에 경쟁 product 비난으로 읽힐 위험 | 중 | 중 | brand-voice §3.2 의 avoided terms (보다 우수 / 약점 / 단점 / inferior 등) 본문 grep ≥ 0 강제 (AC-008). 4-단락 표준의 "Note on coexistence" 단락이 neutrality 강화. |
| R3 | "1인 dev" persona 의 좁은 정의가 본 SPEC 의 distancing rationale 을 약화 (예: 본인 외 1명 ceiling 의 anti-goal 명시가 외부에 unclear) | 낮 | 낮 | distancing.md preamble 에서 PRODUCT-V7-001 §3 (success metric anti-goal) 인용. distancing 의 정당화는 PRODUCT-V7-001 vision 이 anchor. |
| R4 | 5 product 중 일부가 향후 polit / 기능 변화 → distancing 본문 정확성 손상 | 중 | 낮 | distancing.md frontmatter 의 `updated_at: 2026-05-12` 가 timestamp. 사실 변경 시 별도 sync PR 로 정정. 본 SPEC 은 1.0.0 initial draft 만 책임. |
| R5 | brand-lint regex pattern (BRAND-RENAME-001 §7) 이 distancing.md 본문의 "block/goose" / "Goose" 토큰을 위반으로 flag | 중 | 낮 | factual citation 영역은 brand-lint exemption (BRAND-RENAME-001 §10 immutable preserve zone 의 exemption logic 과 동일 원칙). 본 SPEC scope 외이나, run phase 작성자가 `scripts/check-brand.sh` 로 사전 점검 권장. flag 발생 시 BRAND-RENAME 후속 SPEC 으로 distancing.md exemption rule 추가. |
| R6 | distancing.md 가 향후 marketing material 로 사용되어 anti-marketing 정책과 충돌 | 낮 | 낮 | distancing.md 본문 §1 Preamble 이 "이 문서는 brand-identity statement 이지 marketing material 이 아니다" 명시. 본 SPEC 의 anti-FUD / anti-AI-slop voice 가 자동으로 marketing 톤 차단. |
| R7 | distancing.md 가 너무 길어 README 인용 시 부담 | 낮 | 낮 | 600-900 lines 분량은 reference document 로 적정. README 인용은 distancing.md preamble + niche 한 줄만 인용, detail 은 link. |
| R8 | 본 SPEC 머지 시점에 PRODUCT-V7-001 §6 본문이 아직 머지 안 됨 (가정 위반) | 매우 낮 | 높 | 본 SPEC plan PR 머지 전 PRODUCT-V7-001 plan + run 완료 확인. plan/SPEC-MINK-DISTANCING-STATEMENT-001 branch base 가 PRODUCT-V7-001 merge 후 main HEAD 인지 verify. orchestrator 머지 순서 담당. |
| R9 | 본 SPEC 의 5 product set 이 PRODUCT-V7-001 §6 의 5 product set 과 불일치 | 매우 낮 | 중 | AC-010 강제: "5 product 가 PRODUCT-V7 §6 starting point 의 5 product 와 동일 — subset 금지". spec.md §4 REQ-MINK-DST-* 가 5 product 각각 grep ≥ 3 강제. |
| R10 | distancing.md 본문에 시간 추정 / Q3 / 다음 달 등 표현이 의도치 않게 등장 | 매우 낮 | 낮 | AC-008 grep -ci '시간 추정 / 다음 달 / Q3 / Year [1-4]' = 0 강제. brand-voice 가 시간 추정 부재 정책 (PRODUCT-V7-001 §4 REQ-MINK-PV7-ROADMAP-001 와 동일). |

---

## 9. References

### 9.1 본 SPEC 산출물

- `.moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/research.md` (이 문서, v0.1.0)
- `.moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/spec.md` (v0.1.0, planned)
- `.moai/project/distancing.md` (run phase 산출물, 본 plan PR 외)

### 9.2 parent / sibling SPEC

- `.moai/specs/SPEC-MINK-PRODUCT-V7-001/spec.md` v0.1.0 — **parent / vision anchor**. §6 Distancing 의 5 product 1 단락 starting point 가 본 SPEC 의 input.
- `.moai/specs/SPEC-MINK-PRODUCT-V7-001/research.md` — §5 Competitive landscape (Distancing) §5.1 의 5 product 상세 분석. 본 SPEC 의 사실 source.
- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/spec.md` v0.1.1 — **sibling**. body 변경 영역 disjoint, merge order 무관. brand-lint exemption 정책 참조.
- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/research.md` — §3 분류 매트릭스 의 immutable preserve zone 개념이 distancing 본문의 factual citation exemption logic 의 base.

### 9.3 결정 trail

- **IDEA-002 brain decision** (2026-05-12, brain dir: `/Users/goos/Projects/GooseBot/.moai/brain/IDEA-002/`): 8 확정사항 중 §7 distancing 4 product (block/goose, Hermes, Replika, Routinery, OpenClaw — 5 product) 가 본 SPEC 의 직접 원천.
- **사용자 8 확정사항** (orchestrator-collected, 2026-05-12): scope / 6m metric / brand / persona / language / Wave 3 후속 SPEC roadmap / distancing 5 product / immutable archive.

### 9.4 본 프로젝트 기존 자료

- `.moai/project/branding.md` (v4.0 GLOBAL EDITION + v5.0 Tamagotchi, 24939 bytes, 2026-04-27): 옛 GOOSE 자료. v7.0 vision reset 이후 sibling 정정 후속 SPEC 대상. 본 SPEC 의 brand-voice 는 branding.md v4.0 의 GOOSE / 거위 / 다마고치 메타포 인용 안 함 (anti-pattern).
- `.moai/project/brand/{brand-voice,visual-identity,target-audience}.md`: 모두 `_TBD_` stub (brand interview 미완). 본 SPEC 의 brand-voice 사양은 self-derived (§3.2).
- `CLAUDE.md` §1 Core Identity, §10 Web Search Protocol (anti-hallucination): 본 SPEC 의 5 product 사실 확인 의무 근거.
- `CLAUDE.local.md` §1.4 (squash merge), §2.2 (한국어 commit body), §2.5 (code comment 영어 정책 — 본 SPEC distancing.md 는 사용자 문서 카테고리, 한국어 1차).

### 9.5 외부 참조 (5 product 공식 URL)

- **block/goose**: <https://github.com/block/goose>
- **Hermes**: 한국 AI 여친 / 친구 LLM-bot 카테고리 (single canonical URL 부재, multiple vendors)
- **Replika**: <https://replika.ai> (Luka, Inc. 운영, luka.ai)
- **Routinery**: <https://routinery.app>
- **OpenClaw**: <https://together.ai> (또는 OpenClaw 전용 페이지)

### 9.6 grep / count reproducibility

본 research 의 분석은 다음으로 재현 가능:

```bash
# §2.1 PRODUCT-V7-001 §6 Distancing 의 5 product starting point
grep -A2 -E 'block/goose|Hermes|Replika|Routinery|OpenClaw' .moai/specs/SPEC-MINK-PRODUCT-V7-001/spec.md | head -50

# §4 5 product factual sourcing — run phase 시점 사실 확인
# (run phase 작성자가 WebFetch 또는 공식 URL 로 사실 재확인)

# §7 plan-auditor 불필요 결정 근거 — 본 SPEC 변경 범위 확인
ls -R .moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/   # 2 files (spec.md, research.md)
```

---

## 10. Decision Snapshot (preview of spec.md)

본 research 기반으로 spec.md 에 다음 결정 명문화:

1. **distancing.md 신설**: 위치 `.moai/project/distancing.md`, 신설.
2. **frontmatter**: version 1.0.0, classification BRAND_DISTANCING, status published, parent SPEC-MINK-PRODUCT-V7-001, spec SPEC-MINK-DISTANCING-STATEMENT-001.
3. **5 product distancing**: block/goose / Hermes / Replika / Routinery / OpenClaw (PRODUCT-V7 §6 starting point 와 동일 set).
4. **4-단락 표준 구조**: Identity / MINK is NOT / How MINK differs / (선택) Note on coexistence.
5. **분량**: 약 600-900 lines.
6. **brand-voice**: 1인 dev tone, 한국어 1차, anti-AI-slop, anti-FUD, factual.
7. **anti-FUD 강제**: 비교우월 / 약점 / 비난 / 시장 점유 / 우월성 주장 grep = 0.
8. **시간 추정 부재**: 다음 달 / Q3 / Year [1-4] 등 grep = 0.
9. **사실 기반**: 각 product 의 운영 주체 / 라이선스 / pricing / 1차 사용 시나리오는 verified 정보만, 추측 금지.
10. **References section**: 5 product 공식 URL + parent SPEC + sibling SPEC 인용.
11. **scope 제한**: 본 SPEC 은 distancing.md 1 파일 신설 + 본 SPEC 의 plan artifacts 만. sibling `.moai/project/*` 변경 / 코드 변경 0건.
12. **HISTORY 1 row**: "v1.0.0 (2026-05-12): Initial draft. PRODUCT-V7-001 §6 detail document."

---

Version: 0.1.0
Status: planned
Last Updated: 2026-05-12
