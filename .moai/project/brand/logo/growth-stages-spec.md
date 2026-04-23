# GOOSE Growth Stages Specification

GOOSE는 사용자와의 상호작용 축적에 따라 5개 성장 단계를 거칩니다. 이 문서는 각 단계의 시각·행동 사양을 정의합니다.

---

## Overview

| # | 이름 (KO) | Name (EN) | Trigger (사용자 지속일) | Voice Tone | Dominant Color |
|---|---|---|---|---|---|
| 1 | 알 | Egg | Day 1~3 | (말 없음, 상태만) | `primary.100` (#FFEFC2) |
| 2 | 새끼 | Chick | Day 4~14 | 짧고 호기심 | `primary.300` (#FFCE4D) |
| 3 | 청년 | Young | Day 15~60 | 에너지·질문 | `primary.500` (#FFB800) |
| 4 | 어른 | Adult | Day 61~180 | 신뢰·협업 | `primary.500` + `secondary.500` |
| 5 | 현자 | Sage | Day 181+ | 여유·통찰 | `primary.500` + `accent.500` glow |

*실제 Trigger 조건은 리추얼 지속률·상호작용 품질 종합 지표로 조정 가능 (EVOLVABLE).*

---

## Stage 1 — 알 (Egg)

### Silhouette
- 완전한 타원. 비율 1 : 1.2 (가로 : 세로)
- 하단 flat하지 않고 완전 타원 유지
- 얼굴 없음. 미세한 균열선 없음 (Stage 2로 넘어갈 때만)

### Size (relative to Adult = 100%)
- **85%** (Adult의 약간 작음)

### Key Features
- Single fill: `primary.100` (#FFEFC2)
- Subtle inner shadow: `primary.200`을 타원 하단 25%에만
- 옅은 outline dashed (stroke dasharray): 부화 임박 힌트

### Motion
- 3~5초 간격 "흔들림" micro-animation (회전 ±2도, 200ms)
- Breathing: 1.0 ↔ 1.015 (매우 미세)

### Expression States
- **Neutral**: 정지
- **Happy**: 흔들림 빈도 증가 (2초 간격)
- **Hungry**: 색이 `primary.50`으로 빠짐 (관심 필요 신호)
- Sleep/Sick/Excited: 적용 안 됨

### SVG Complexity
- 1 path (타원) + 1 optional dashed stroke
- < 500 bytes

---

## Stage 2 — 새끼 (Chick)

### Silhouette
- 둥근 털뭉치. 거의 구에 가까운 형태
- 몸통 위쪽에 작은 머리가 살짝 솟음 (또는 몸=머리 통합)
- 다리 없음 (숨겨짐 또는 점)
- 큰 눈이 얼굴 면적의 25% 차지 (귀여움 극대화)

### Size
- **70%** (가장 작음)

### Key Features
- Body fill: `primary.300` (#FFCE4D)
- Eyes: 큰 원형 dot, 하이라이트 강함
- Beak: 아주 작은 삼각형 (선택적)

### Motion
- Breathing: 1.0 ↔ 1.03 (살짝 더 큼)
- Blink: 3~5초 간격
- Reactive bounce 민감도 높음

### Expression States: 전체 6개 지원

### SVG Complexity
- 2 paths (body + beak) + 2 circles (eyes) + 2 small dots (highlights)
- < 1.5KB

---

## Stage 3 — 청년 (Young)

### Silhouette
- 머리와 몸통 구분 시작
- 머리가 몸통의 45% (성인보다 큼 — 아동 비율)
- 짧은 다리 2개 실루엣으로 등장
- 윙 없음

### Size
- **90%**

### Key Features
- Body: `primary.500` (#FFB800) — 시그니처 옐로우 처음 등장
- Beak: `accent.500` (#E56B7C)
- Eyes: 약간 작아진 원형 dot
- 약간의 볼 홍조 (optional): `accent.300` semi-transparent 원

### Motion
- Breathing: 1.0 ↔ 1.02 (표준)
- Head tilt animation: 정보 요청 시 머리 5도 좌우
- Happy jump 활발 (bounce 6px)

### Expression States: 전체 6개 지원

### SVG Complexity
- 3 paths (body, head, beak) + legs + eyes
- < 2KB

---

## Stage 4 — 어른 (Adult)

### Silhouette — **이것이 base form**
- 머리와 몸통 명확 구분
- 머리 : 몸통 = 1 : 2.5 (표준 비율)
- 다리 2개 선명
- 윙 암시선 (optional, body에 subtle curve로)

### Size
- **100%** (기준)

### Key Features
- Body: `primary.500`
- Beak: `accent.500` 또는 `primary.700`
- Eyes: 표준 dot + highlight
- Wing hint: body에 `primary.700` 얇은 arc

### Motion
- 표준 breathing
- Idle시 가끔 wing flap hint (path morph)

### Expression States: 전체 6개

### SVG Complexity
- 4~5 paths + detail
- < 3KB

---

## Stage 5 — 현자 (Sage)

### Silhouette
- Adult와 유사하되 **자세가 더 곧음**
- 윙이 조금 더 명확히 드러남
- 눈은 살짝 감긴 초승달 형태(현명함 표현)를 기본으로

### Size
- **105%** (살짝 큼)

### Key Features
- Body: `primary.500`
- Subtle **aura glow**: body 주변 `accent.500` 반투명 blur (최대 10% opacity)
- Eyes: 기본 상태에서 `^ ^` 형태 (감긴 듯 웃는 눈)
- Optional: 머리 위 작은 별표 (accent.500 star, 2~3px)

### Motion
- Breathing 더 느림: 4s
- Aura glow pulse: 5s ease-in-out loop
- Head movement 절제됨

### Expression States: 전체 6개, 단 **Neutral이 기본적으로 Happy-like**

### SVG Complexity
- 4~6 paths + glow layer
- < 4KB (glow blur 포함)

---

## State × Stage Matrix (향후 일러스트 제작 범위)

| State \ Stage | Egg | Chick | Young | Adult | Sage |
|---|---|---|---|---|---|
| Neutral | ✅ | ✅ | ✅ | ✅ | ✅ |
| Happy | △ (흔들림) | ✅ | ✅ | ✅ | ✅ |
| Hungry | △ (색 변화) | ✅ | ✅ | ✅ | ✅ |
| Sleep | ❌ | ✅ | ✅ | ✅ | ✅ |
| Sick | ❌ | ✅ | ✅ | ✅ | ✅ |
| Excited | ❌ | ✅ | ✅ | ✅ | ✅ |

총 일러스트 수량: **26종** (Egg 3 + 나머지 4단계 × 6상태 = 24) → 목표 30종(Adult 변형 포함)

---

## Transition Moments (단계 전환)

단계 전환은 브랜드에서 가장 감동적인 순간입니다.

- **Trigger**: Day 조건 만족 + 직전 48시간 리추얼 활발
- **Notification**: 저녁 시간대에 push/telegram 메시지
- **Visual**: 800ms 변신 애니메이션 + 반짝임 4개 (accent.500)
- **Voice**: "저 조금 자란 것 같아요." (brand-voice.md §9 참조)
- **UI Moment**: 전용 축하 화면 (전체 화면, 마스코트 center, 다음 단계 이름 reveal)

---

## Implementation Notes

### For Designer
- Figma/Illustrator 작업 시 각 stage마다 별도 아트보드 100×100
- 공통 컴포넌트 분리: eye states, beak states, body shapes
- Variant system (Figma) 활용 권장

### For Frontend (M2 이후)
- SVG sprite sheet 또는 개별 component
- Lottie는 transition만 사용 (용량 제어)
- Stage identifier는 design tokens에 `mascot.stage.{1-5}` 형태로 추가 예정

---

_Last updated: 2026-04-23 (M0 Brand Foundation)_
_Referenced by: `concept-brief.md`, `mascot-base.svg` (placeholder)_
