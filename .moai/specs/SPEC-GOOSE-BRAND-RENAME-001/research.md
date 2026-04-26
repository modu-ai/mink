# Research — SPEC-GOOSE-BRAND-RENAME-001 (AI.GOOSE 브랜드 통일)

> 본 research 문서는 SPEC-GOOSE-BRAND-RENAME-001의 의사결정 근거자료다. spec.md §10 References에서 본 문서를 참조한다.

작성일: 2026-04-26
작성자: manager-spec
Status: planned

---

## 1. 문제 정의

`goose` 프로젝트는 현재 다양한 user-facing 텍스트(문서·README·CLAUDE.md·코드 주석·로그 메시지)에서 프로젝트 식별 표기가 일관되지 않다. 한 문서 안에서도 `goose`, `Goose`, `GOOSE`, `GOOSE-AGENT` 가 혼용되고 있어 다음 문제가 발생한다.

- 외부 노출 시 브랜드 인식이 통일되지 않음 (검색 SEO 저하, 신규 사용자 혼란)
- 후속 SPEC 작성자가 "어느 표기를 써야 하는가" 매번 결정에 시간을 소모
- AI.GOOSE라는 공식 브랜드명을 도입하려 해도 코드 식별자(Go module path, package, struct)와의 경계가 모호

본 SPEC은 **사용자 의사결정에 따라** 공식 브랜드명을 `AI.GOOSE`로 통일하면서, 코드 식별자(`goose` 소문자)와 URL slug(`ai-goose` 케밥)를 명확히 분리한 표기 규범을 수립한다.

---

## 2. 현황 조사 (grep 기반 카운트)

### 2.1 .moai/project/ 디렉토리 (user-facing 메타 문서)

| 파일 | `goose` 등장 횟수 | 비고 |
|------|------------------|------|
| product.md | 100 | 가장 많은 brand 표기 — `GOOSE-AGENT`, `GOOSE` 위주 |
| tech.md | 64 | 기술 스택 문서, brand 언급 다수 |
| structure.md | 54 | 디렉토리 구조 설명, brand + 식별자 혼재 |
| branding.md | 53 | 기존 branding 문서, AI.GOOSE 도입 시점에 통합 검토 필요 |
| learning-engine.md | 49 | 자기진화 엔진 문서 |
| migration.md | 44 | 마이그레이션 문서 |
| ecosystem.md | 27 | 생태계 설명 |
| adaptation.md | 20 | 적응 시스템 |
| token-economy.md | 10 | |
| research/claude-primitives.md | 11 | research 하위, 정정 우선순위 낮음 |
| research/hermes-learning.md | 10 | |
| research/hermes-llm.md | 9 | |
| research/claude-core.md | 8 | |
| brand/README.md | 6 | brand 디렉토리, 통일 대상 |
| brand/logo/concept-brief.md | 4 | |
| brand/logo/README.md | 3 | |
| brand/logo/growth-stages-spec.md | 2 | |
| db/README.md | 1 | |

총 18개 파일. 표기 변형 분포:
- `goose` (소문자): 155회 — 코드 식별자/도메인 용어가 다수
- `GOOSE` (대문자): 117회 — 현재 brand 표기로 가장 많이 쓰이는 형태
- `Goose` (Title case): 7회 — 산발적

### 2.2 핵심 루트 문서

| 파일 | `goose` 등장 횟수 | 비고 |
|------|------------------|------|
| README.md | 4 | `🪿 GOOSE — Your Daily Companion` 제목 등 brand 표기 |
| CHANGELOG.md | 2 | 헤더 위주 |
| CLAUDE.md | 0 | brand 표기 없음 (직접 정정 대상 적음) |

`README.md` h1: `# 🪿 GOOSE — Your Daily Companion, Hatched Just for You`
`product.md` h1: `# GOOSE-AGENT - 제품 문서 v4.0 GLOBAL EDITION`

### 2.3 .claude/ 하위

| 영역 | brand 정정 대상 추정 |
|------|---------------------|
| .claude/rules/ | 0건 (rules는 MoAI 자체 규약, goose 본문 없음) |
| .claude/agents/ | 0건 (agents 정의 본문에 goose 미언급) |
| .claude/skills/ | 4건 (moai-domain-database SKILL.md 등 도메인 문서에 산발적 등장) |
| .claude/commands/ | 0건 |

`.claude/`는 brand 정정 영향이 작음. moai-domain-database/SKILL.md 등 일부 skill 모듈만 점검.

### 2.4 .moai/specs/ (SPEC 본문)

98개 SPEC 마크다운 파일에서 `goose` 등장. 상위 분포:

| SPEC 파일 | `goose` 등장 횟수 |
|-----------|------------------|
| SPEC-GOOSE-CLI-001/spec.md | 100 |
| SPEC-GOOSE-QMD-001/spec.md | 62 |
| SPEC-GOOSE-ONBOARDING-001/spec.md | 52 |
| SPEC-GOOSE-CORE-001/spec.md | 40 |
| SPEC-GOOSE-CLI-001/research.md | 34 |
| SPEC-GOOSE-DAEMON-WIRE-001/spec.md | 32 |
| SPEC-GOOSE-DESKTOP-001/spec.md | 26 |
| SPEC-GOOSE-TRANSPORT-001/spec.md | 21 |
| SPEC-GOOSE-LORA-001/research.md | 21 |
| ... 외 87개 SPEC 파일 |

대부분의 등장은 SPEC ID(`SPEC-GOOSE-XXX-NNN`), 코드 식별자(`goose`, `goosed`), 도메인 용어(`goose agent loop`)이며 brand 표기는 일부에 한정. **선별 정정 필요** (단순 일괄치환 금지).

### 2.5 Go 코드 (참고 통계)

본 SPEC 작업 시점에 Go 소스 트리는 별도 grep을 통해 user-facing 문자열(log/error/CLI help)만 선별 정정. `package goose`, `type Goose*`, `func Goose*`는 식별자로 보존.

---

## 3. `goose` 단어의 4가지 의미 분류

본 SPEC의 핵심은 단어의 의미 분류다. 모든 정정 작업은 다음 분류표를 기준으로 한다.

### 3.1 분류표

| 의미 카테고리 | 정의 | 예시 | 정정 정책 |
|-------------|------|------|----------|
| (A) Brand (공식 브랜드) | 외부에 노출되는 프로젝트 정체성 | "GOOSE 프로젝트", "Welcome to Goose", README h1 | **`AI.GOOSE`로 정정** |
| (B) Module Path / Repo (URL/식별 인프라) | Git 인프라 식별자 | `github.com/modu-ai/goose`, `modu-ai/goose-agent` | **보존** (변경 시 build 손상) |
| (C) Code Identifier (코드 식별자) | Go package/struct/func/var/binary | `package goose`, `type Goose*`, `cmd/goose`, `goosed` | **보존** (변경 시 컴파일 오류) |
| (D) Domain Term (도메인 용어) | 프로젝트 내부 개념의 약칭 | "goose agent loop", "goose CLI", "goosed daemon" | **보존** (작은 따옴표/백틱으로 인용 형태 유지) |

### 3.2 판정 절차

각 등장 위치에 대해 다음 순서로 판정:

1. 백틱/코드블록 안에 있는가? → (B)/(C)/(D), 보존
2. URL/import path/Git remote의 일부인가? → (B), 보존
3. Go 식별자 (package/type/func/var) 인가? → (C), 보존
4. "the goose CLI", "goosed daemon" 처럼 시스템 컴포넌트의 약칭인가? → (D), 인용 형태(`goose CLI`)로 보존
5. 그 외 산문 본문에서 프로젝트 자체를 가리키는가? → (A), `AI.GOOSE`로 정정

### 3.3 경계 사례

| 표현 | 분류 | 정정 결과 |
|------|------|----------|
| "GOOSE-AGENT 제품 문서" | (A) | "AI.GOOSE 제품 문서" |
| "the goose CLI" | (D) | `goose CLI` (백틱 인용) — 보존 |
| `package goose` | (C) | 보존 |
| "We use github.com/modu-ai/goose" | (B) | 보존 |
| "Goose는 거위(goose)에서 따왔다" | (A) → 정정, 단 어원 설명 시 (D) 보존 가능 | "AI.GOOSE는 거위(goose)에서 따왔다" |
| `goose agent loop` | (D) | 백틱 인용 형태 보존 |

---

## 4. 유사 사례 분석 (산업 관행)

### 4.1 Next.js — 점 표기와 패키지 이름의 분리

- 공식 브랜드명: **Next.js** (점 포함, 마케팅·문서·로고)
- npm 패키지 이름: **`next`** (점 없음, 식별자)
- GitHub repo: **`vercel/next.js`** (URL slug, 점 포함)

핵심 시사점: brand 표기와 식별자 표기가 다르다. 사용자는 `npm install next`를 입력하지만 문서·로고는 항상 `Next.js`. 이는 본 SPEC의 dual representation 전략과 일치한다.

### 4.2 Mistral AI — 공백·점 dual

- 공식 브랜드: **Mistral AI** (공백 포함)
- 도메인: **mistral.ai** (점 포함)
- 모델 이름: `mistral-7b`, `mistral-medium` (식별자)

본 프로젝트의 `.moai/project/tech.md`에서도 `Mistral AI`로 표기. 산업 표준 사례.

### 4.3 Anthropic / OpenAI — 단일 표기

- 단순한 단일 단어 브랜드(`Anthropic`, `OpenAI`)는 dual representation 불필요.
- 본 프로젝트는 `goose` 단어가 흔한 영어 명사이고 코드 식별자로 광범위하게 사용되므로 **dual representation 전략이 필수**.

### 4.4 결론

`AI.GOOSE` (brand) / `goose` (식별자) / `ai-goose` (slug) 세 가지 표기를 명확히 분리하는 것은 산업 관행과 일치하며, 검색·식별·발음 모두에서 이점을 가진다.

---

## 5. 위험 식별 (Build/Test 손상 가능 영역)

본 SPEC 구현 단계에서 **잘못된 정정으로 build/test가 깨질 수 있는 영역**을 사전 식별한다. 모든 영역은 spec.md §3.2 Scope OUT으로 명시된다.

### 5.1 Critical (build 직접 손상)

- Go module path: `github.com/modu-ai/goose` — `go.mod`, 모든 import 문
- Go package 선언: `package goose`, `package goosed`, `package goosecli`
- Type 이름: `type Goose*`, `type Goosed*` 시리즈
- Function/method 이름
- Binary 이름: `goose`, `goosed`, `goose-cli` (Makefile, CI workflow, 설치 스크립트)
- proto package: `goose.v1`

### 5.2 High (test/runtime 손상)

- 환경 변수 prefix: `GOOSE_*` (만약 존재한다면 — 별도 grep 검증 필요)
- 설정 파일 키: `.moai/config/sections/*.yaml`의 `goose` 필드 이름

### 5.3 Medium (URL/외부 식별)

- GitHub repo 이름: `modu-ai/goose-agent`
- 도메인 (미래): `ai-goose.dev` 등 새 도메인 도입 시 `ai-goose` slug 사용

### 5.4 Low (이력 immutable)

- 과거 commit message
- 과거 CHANGELOG entry
- 종료된 SPEC HISTORY entry — `## HISTORY` 표 안의 기존 row

위 영역은 spec.md AC-BR-007/008/009/011로 검증한다.

---

## 6. 변경 영역 매핑 (Phase별 대상)

각 phase는 별도 commit으로 분리. 단일 PR에 포함(squash merge).

### Phase 1 — 표기 규범 commit (이 SPEC + style-guide.md 신설)

대상 파일:
- `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/spec.md` (새로 작성)
- `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/research.md` (이 문서)
- `.moai/project/brand/style-guide.md` (구현 단계에서 신설, frozen reference)

규모: 3 파일

### Phase 2 — 핵심 다큐먼트 일괄 정정

대상 파일 (현황 조사 기반):
- `README.md` (4건)
- `CHANGELOG.md` (header 추가 entry부터, 기존 entry는 immutable)
- `CLAUDE.md` (0건이지만 brand 도입 문구 추가 가능)
- `.moai/project/product.md` (100건)
- `.moai/project/tech.md` (64건)
- `.moai/project/structure.md` (54건)
- `.moai/project/branding.md` (53건)
- `.moai/project/learning-engine.md` (49건)
- `.moai/project/migration.md` (44건)
- `.moai/project/ecosystem.md` (27건)
- `.moai/project/adaptation.md` (20건)
- `.moai/project/token-economy.md` (10건)
- `.moai/project/brand/README.md` (6건)
- `.moai/project/brand/logo/*.md` (9건)

규모: 약 14 파일, 약 480건 등장 위치 중 brand 표기 부분(추정 30~40%)만 정정.

### Phase 3 — Claude rules / agents / skills / commands

대상 파일:
- `.claude/skills/moai-domain-database/SKILL.md` (3건)
- `.claude/skills/moai/workflows/project.md` (1건)
- `.claude/skills/moai-domain-database/modules/mongodb.md` (1건)
- `.claude/skills/moai-domain-database/modules/INDEX.md` (1건)

규모: 약 4 파일, 6건. 대부분 도메인 용어 인용 형태이므로 정정 대상은 1~2건 수준.

### Phase 4 — 기존 SPEC 본문 선별 정정

대상: 98개 SPEC 마크다운 파일 중 brand 표기로 goose를 언급하는 부분.

전략:
- 각 SPEC 본문의 첫 페이지(Overview/Background section)에서 "프로젝트 명칭"으로 goose를 언급한 부분만 정정
- SPEC ID, HISTORY 표, 코드 인용은 보존
- 결과는 `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/migration-log.md`에 기록 (변경된 SPEC 목록 + diff 카운트)

규모: 추정 20~30 파일, 50~80 건 정정.

### Phase 5 — 코드 내 user-facing 문자열

대상:
- log/error message: `logger.Info("starting goose daemon...")` → `"starting AI.GOOSE daemon..."`
- CLI help text: `cmd.Long = "goose is a daily companion..."` → `"AI.GOOSE is a daily companion..."`
- doc-comment의 brand 언급 (예: `// Package goose implements the AI.GOOSE agent runtime.`)

전략: `grep -rn "goose"` 결과 중 user-facing 문자열만 선별. 식별자(`type Goose*`, `package goose`)는 절대 변경 금지.

규모: 사전 grep 통계 미실시 (구현 단계 초반에 baseline 측정).

### Phase 6 — 검증

대상:
- `.moai/project/brand/style-guide.md` 또는 `scripts/check-brand.sh` 신설
- (선택) CI gate: `make brand-lint` target 추가
- AC-BR-007/008/009 baseline 비교 스크립트

규모: 1~2 파일.

---

## 7. 누락 위험 완화 전략

70여 SPEC 본문 일괄 정정 시 누락/오변경 위험에 대한 완화 전략:

1. **migration-log.md 기록**: Phase 4 결과를 별도 파일로 추적. 변경된 SPEC 목록과 diff 카운트.
2. **Spot-check QA**: 무작위 5건 SPEC을 사람이 직접 비교 검토.
3. **자동 검증 스크립트**: `scripts/check-brand.sh`로 다음 패턴이 0건인지 확인:
   - `Goose 프로젝트` (Title case 잔존)
   - `GOOSE-AGENT` 단독 표기 (코드 식별자가 아닌 brand 위치)
   - `goose project` (영문 brand 위치, 백틱 외부)
4. **베이스라인 비교**: AC-BR-007/008/009로 코드 식별자 변화 0건 검증.

---

## 8. 산업 사례 부록 (참고)

| 프로젝트 | Brand | 패키지/식별자 | URL/Repo |
|----------|-------|--------------|---------|
| Next.js | `Next.js` | `next` | `vercel/next.js` |
| Mistral AI | `Mistral AI` | `mistral-*` | `mistralai/mistral-*` |
| Tailwind CSS | `Tailwind CSS` | `tailwindcss` | `tailwindlabs/tailwindcss` |
| AI.GOOSE (본 SPEC) | `AI.GOOSE` | `goose` | `modu-ai/goose-agent`, `ai-goose` (future) |

---

## 9. 결정 사항 요약

본 research를 기반으로 spec.md에 다음 결정을 명문화한다:

1. 공식 브랜드명은 **`AI.GOOSE`** (점 1개, 모두 대문자)
2. 코드 식별자/짧은 약칭은 **`goose`** (소문자, 백틱 인용 권장)
3. URL slug/도메인은 **`ai-goose`** (케밥 케이스, 미래 도메인용)
4. 기존 immutable 기록(commit message, 종료 SPEC HISTORY, 과거 CHANGELOG entry)은 변경 금지
5. 코드 식별자 일체 보존 (Go module path, package, struct, binary, proto)
6. `.moai/project/brand/style-guide.md`를 frozen reference로 신설
7. 검증은 `scripts/check-brand.sh` 또는 `make brand-lint`로 자동화

---

## 10. References

- 산업 사례: Next.js (vercel/next.js), Mistral AI, Tailwind CSS
- 본 프로젝트 기존 분석: `.moai/project/branding.md`, `.moai/project/brand/`
- 사용자 의사결정 trail: orchestrator로부터 위임된 본 SPEC 작성 prompt (2026-04-26)
- grep 카운트: 본 문서 §2 현황 조사 표
