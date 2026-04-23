# GOOSE Brand Foundation

이 디렉터리는 **GOOSE 브랜드의 헌법적 기준(constitutional parent)**입니다. 모든 디자인·카피·개발 산출물은 이 디렉터리의 내용을 기준으로 생성·검증됩니다.

> `.claude/rules/moai/design/constitution.md` Section 3.1에 따라, 이 디렉터리의 내용은 프로젝트의 모든 phase를 관통하는 제약 조건입니다.

---

## Directory Map

```
.moai/project/brand/
├── README.md                 ← 이 파일 (인덱스)
├── brand-voice.md            ← 톤, 매너, 용어집, growth-stage별 보이스
├── visual-identity.md        ← 컬러, 타이포, 스페이싱, 모션 원칙
├── target-audience.md        ← 3 personas + Tier × Channel usage scenarios
├── design-tokens.json        ← W3C DTCG 포맷, visual-identity의 기계 판독형
└── logo/
    ├── README.md             ← 로고 사용 규칙
    ├── concept-brief.md      ← 마스코트 디자인 브리프 (거위×다마고치)
    ├── growth-stages-spec.md ← 5단계 성장 시각 사양
    ├── mascot-base.svg       ← 기본 마스코트 (Adult, Neutral) — placeholder
    └── wordmark.svg          ← "GOOSE" 워드마크 — placeholder
```

---

## What Lives Here (vs. `.moai/design/`)

브랜드 시스템은 두 계층으로 나뉩니다. 이는 헌법 Section 3.3의 원칙입니다.

| | `.moai/project/brand/` (**여기**) | `.moai/design/` |
|---|---|---|
| **질문** | GOOSE는 누구인가 (WHO) | 이번 iteration은 무엇을 만드나 (WHAT) |
| **수명** | 장기·거의 변하지 않음 | 프로젝트·iteration 단위 |
| **변경 주체** | 사용자 명시 승인 (헌법) | `/moai design` per-iteration |
| **예시** | 브랜드 색, 마스코트 컨셉, 페르소나 | 랜딩 페이지 spec, research 노트, pencil plan |
| **충돌 시** | **우선** | 브랜드에 종속 |

---

## How to Use This Directory

### For Copywriting Tasks
1. `brand-voice.md`의 tone·vocabulary·signature phrases 참조
2. `target-audience.md`의 persona vocabulary(§6)를 카피 기준 어휘로 사용
3. Growth stage에 따른 톤 조정 (brand-voice §4)

### For Design/UI Tasks
1. `visual-identity.md`의 color·typography·spacing·motion 준수
2. 구현 시 `design-tokens.json`을 단일 소스로 사용 (hardcoded value 금지)
3. 마스코트 사용 시 `logo/README.md`의 clearspace·min size 규칙 준수

### For Implementation Tasks
```js
// 토큰 로드 예시 (Web)
import tokens from '.moai/project/brand/design-tokens.json';
const primary = tokens.color.primary['500'].$value; // "#FFB800"
```

---

## Evolution Protocol

이 디렉터리는 **FROZEN zone**에 준하는 보호 수준을 가집니다. 변경은 다음 프로세스를 따릅니다.

### 변경이 가능한 경우
- 사용자가 명시적으로 브랜드 수정을 요청 (`/moai design` 또는 직접 편집 지시)
- Growth stage 추가 (EVOLVABLE per constitution §2)
- Design tokens의 미세 조정 (tonal shift, 신규 semantic color 등)

### 변경이 불가능한 경우 (FROZEN)
- 핵심 브랜드 정체성: 이름(GOOSE), 거위×다마고치 컨셉, 5 growth stages 구조
- `primary.500` (#FFB800) 시그니처 옐로우 — 브랜드의 fingerprint
- "동반자" 포지셔닝 (도구·비서 반대)

### Evolution Flow (`/moai design` 경유 시)
1. `/moai design` → path A 또는 B 선택
2. Learner가 제안 (canary 통과)
3. 사용자 승인 (AskUserQuestion)
4. 변경 로깅: `.moai/research/evolution-log.md`
5. 이 디렉터리 업데이트

---

## Current Status

- **Version**: 1.0.0
- **Phase**: M0 Brand Foundation (2026-04-23)
- **Placeholders**: `logo/mascot-base.svg`, `logo/wordmark.svg` (기하학적 대체물 — 디자이너가 교체 예정)
- **Final**: voice, visual-identity, audience, tokens, mascot concept & spec

---

## Next Milestones

| Milestone | Owner | Deliverables |
|---|---|---|
| **M1** — Copy Foundation | copywriter / moai-domain-copywriting | hero/features/cta 카피 JSON |
| **M2** — Landing Site | expert-frontend + moai-domain-brand-design | goose.ai landing (path B) |
| **M3** — Mascot Illustration | Designer (external) | 30종 마스코트 아트 + Lottie transitions |
| **M4** — Telegram Sticker Pack | Designer | 512×512 sticker set |

---

## Validation Checklist

새로운 산출물이 브랜드를 준수하는지 확인.

- [ ] 컬러는 `design-tokens.json`에 정의된 것만 사용
- [ ] 텍스트의 톤이 `brand-voice.md` §5 DO 규칙과 일치
- [ ] 금지 표현(`brand-voice.md` §10)이 없음
- [ ] 타이포는 정의된 scale 내 (`visual-identity.md` §3.2)
- [ ] 스페이싱은 4/8px grid 준수
- [ ] 마스코트 사용 시 clearspace·min size 준수
- [ ] 대상 사용자가 `target-audience.md`의 페르소나와 일치
- [ ] `prefers-reduced-motion` 접근성 대응

---

_Last updated: 2026-04-23 (M0 Brand Foundation)_
_Constitutional authority: `.claude/rules/moai/design/constitution.md` Section 3.1_
