# GOOSE Logo & Mascot Assets

이 디렉터리는 GOOSE 브랜드의 시각 자산을 보관합니다. 현재 포함된 SVG들은 **컨셉 플레이스홀더**이며, 최종 일러스트는 디자이너 또는 dedicated design agent에 의해 대체됩니다.

---

## Files

| File | Purpose | Status |
|---|---|---|
| `mascot-base.svg` | 기본 마스코트 (Adult 단계, neutral 표정) | Placeholder (geometric) |
| `wordmark.svg` | "GOOSE" 워드마크 | Placeholder |
| `concept-brief.md` | 마스코트 디자인 브리프 | Final |
| `growth-stages-spec.md` | 5단계 성장 사양 | Final |

---

## Logo Usage Rules

### Primary Logo (Mascot + Wordmark)
- 사용처: 앱 스플래시, 랜딩 히어로, 공식 문서 표지
- 최소 크기: 마스코트 120px 높이
- Clearspace: 마스코트 높이의 25% 사방 여백

### Mascot Only
- 사용처: 앱 아이콘, 파비콘, 네비 아바타, 인앱 컴패니언
- 최소 크기: 80×80px (앱 내) / 16×16px (favicon — 단순화 버전 필요)
- 항상 `radius.xl` 컨테이너 또는 원형 배경 위에 배치

### Wordmark Only
- 사용처: 푸터, 이메일 서명, 텍스트 중심 맥락
- 최소 크기: 높이 16px
- Clearspace: 워드마크 높이의 50%

---

## Color Variants

| Context | Mascot Fill | Wordmark Fill |
|---|---|---|
| Light background (`neutral.50`) | `primary.500` (#FFB800) | `neutral.700` (#3A352E) |
| Dark background (`neutral.900`) | `primary.300` (#FFCE4D) | `neutral.50` (#FAF8F4) |
| Monochrome print | `neutral.900` | `neutral.900` |
| Single-color brand | `primary.500` | `primary.500` |

---

## DO / DON'T

### DO
- 반드시 vector(SVG) 포맷 사용
- 배경 대비 유지 (WCAG AA 이상)
- `primary.500` 또는 `neutral.50/900` 조합을 기본으로

### DON'T
- 마스코트를 회전·왜곡·기울임
- 마스코트에 그림자·광택 효과 추가 (flat 유지)
- 브랜드 외 색으로 채색
- 마스코트와 워드마크 비율 임의 변경

---

## Replacement Workflow

현재 SVG들은 기하학적 플레이스홀더입니다. 최종 아트는 다음 순서로 교체합니다.

1. 디자이너가 `concept-brief.md` + `growth-stages-spec.md` 기준으로 일러스트 작성
2. 동일 파일명으로 교체 (`mascot-base.svg`, `wordmark.svg`)
3. 5단계 전체 일러스트는 `growth-stages/` 하위 디렉터리에 추가:
   - `stage-1-egg.svg`
   - `stage-2-chick.svg`
   - `stage-3-young.svg`
   - `stage-4-adult.svg` (= base)
   - `stage-5-sage.svg`
4. 각 단계별 expression 변형 (`-happy`, `-hungry`, `-sleep`, `-sick`, `-excited`)

---

## Export Settings

- Format: SVG (optimized via SVGO)
- viewBox: `0 0 100 100` for mascot, dynamic for wordmark
- Stroke: 벡터에 outline 포함 금지 (fill 기반)
- File size target: < 5KB per asset

---

_Last updated: 2026-04-23 (M0 Brand Foundation)_
