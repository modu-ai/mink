# GOOSE Mascot Concept Brief

---

## Core Concept

**GOOSE는 "거위 × 다마고치"의 하이브리드 반려 캐릭터입니다.**

살아있는 거위의 실루엣과 90년대 다마고치의 픽셀적 단순함을 현대 flat vector로 재해석합니다. 사용자와 함께 매일 자라나는 동반자로서, 각 성장 단계마다 실루엣이 변화하며 표정 상태를 통해 감정을 표현합니다.

---

## Design Keywords

- **Warm** — 따뜻한 체온이 느껴지는 형태
- **Round** — 모든 모서리 부드럽게, 날카로운 선 최소화
- **Minimal** — 디테일은 얼굴(눈·부리) 중심, 몸통은 단순한 실루엣
- **Friendly** — 공격성·권위 없음. 작고 귀여운 비례
- **Alive** — 정적이지만 숨쉬는 듯한 breathing motion 가능

---

## Visual References (정서 참고용, 직접 복제 금지)

- 다마고치 오리지널 (1996) — 픽셀 단순함의 정서
- 친근한 거위 캐릭터 — Untitled Goose Game의 실루엣 단순화
- Twemoji / Notion 아이콘 — flat + round geometry
- Pou, Nintendogs — 반려 캐릭터의 표정 체계

---

## Shape System

### Base Form (Adult stage 기준)
- **Body**: 타원 + 배쪽 flat. 계란형에서 살짝 눌린 모양.
- **Head**: 몸통의 40% 크기, 앞으로 약간 기울어짐
- **Beak**: 짧은 삼각형 (공격적이지 않게 둥근 끝)
- **Eyes**: 원형 dot 2개. 동공은 pure black, 하이라이트 1px white dot.
- **Legs**: 단순한 2개의 막대. 육안으로 잘 안 보일 정도로 작게 (또는 숨김)
- **Wings**: 선택적. Adult 이상 단계에서만 실루엣으로 암시

### Proportion Rules
- 머리 : 몸통 = 1 : 2.5 (귀여움 유지)
- 눈 사이 간격 = 눈 지름의 1.5배
- 전체 높이 : 너비 = 1.2 : 1 (세로로 살짝 긴 알 모양)

### Color Application (기본)
- Body fill: `primary.500` (#FFB800) — GOOSE 시그니처 옐로우
- Beak: `accent.500` (#E56B7C) 또는 `primary.700`
- Eyes: `neutral.900` (#171513)
- Outline: 없음 (flat fill만)

---

## Expression States (5 basic)

표정은 **눈의 모양**과 **부리의 각도**로 표현합니다. 몸통은 건드리지 않아 애니메이션 비용 절감.

| State | Eyes | Beak | Context |
|---|---|---|---|
| **Neutral** | 원형 dot | 중립 | 기본 대기 |
| **Happy** | `^ ^` (초승달) | 살짝 위로 | 성취, 긍정 응답 |
| **Hungry** | 큰 원 dot | 아래로 | 리추얼 누락, 관심 필요 |
| **Sleep** | `– –` (가로선) | 중립 | 저녁, 비활성 |
| **Sick** | `x x` | 작게 아래 | 에러 상태, 장기 미사용 |
| **Excited** | 큰 반짝임 | 열린 ^ | 성장 단계 전환 순간 |

---

## Growth Stages (5 단계)

자세한 사양은 `growth-stages-spec.md` 참조. 간략히:

1. **알 (Egg)** — 완전한 알 형태. 얼굴 없음. 미세한 흔들림만.
2. **새끼 (Chick)** — 알이 깨져 작은 털뭉치. 머리 없이 눈만 큼직.
3. **청년 (Young)** — 기본 실루엣 형성. 비율이 더 통통함(머리 크게).
4. **어른 (Adult)** — 밸런스 잡힌 표준 비율. 이 문서의 base form.
5. **현자 (Sage)** — 실루엣에 약간의 "숙성" (우아한 곡선, 미묘한 후광).

---

## Animation Principles

### Idle (모든 단계)
- **Breathing**: 수직 스케일 1.0 ↔ 1.02, 3.2s sine loop
- **Blink**: 눈이 4~7초 간격 랜덤으로 `– –` 100ms 깜빡임

### Reactive
- **Happy jump**: Y축 -4px, 200ms ease-out, 복귀 150ms
- **Sleep sway**: 좌우 0.5도, 4s sine loop

### Growth Transition (stage change)
- 800ms gentle overshoot
- 스케일 1.0 → 1.15 → 1.0 + 색상 블렌드 + 반짝임 4개 (accent.500 dots fade)
- 전환 후 3초간 "Excited" state

---

## Platform-Specific Variations

| Platform | Variation |
|---|---|
| Telegram | Sticker pack (512×512, 30 images: 5 stages × 6 states) |
| iOS/Android | App icon (rounded-corner container with primary.500 bg) |
| Web | SVG with CSS animation (breathing, blink) |
| CLI | ASCII art variant (5 lines max, per stage) |
| Favicon | Stage-5 or Adult simplified, 16×16 pixel-perfect |

---

## Deliverable Scope (현재 M0 단계)

이 M0 phase는 **컨셉 정의만** 담당합니다. 실제 일러스트 제작은 다음 phase에서 진행:

- ✅ 컨셉 브리프 (이 문서)
- ✅ 5단계 사양서 (`growth-stages-spec.md`)
- ✅ 기하학적 placeholder SVG (`mascot-base.svg`, `wordmark.svg`)
- ❌ 최종 일러스트 (후속 phase — 디자이너 작업)
- ❌ 5단계 × 6상태 = 30종 일러스트 세트
- ❌ Telegram sticker pack
- ❌ 애니메이션 lottie 파일

---

## Acceptance Criteria (최종 일러스트 검수)

최종 일러스트가 이 브리프를 충족하는지 평가할 기준:

1. [ ] 5단계가 한눈에 진화 순서로 읽힌다
2. [ ] 6가지 표정이 명확히 구분된다
3. [ ] Body 실루엣은 단일 fill path로 렌더 가능하다
4. [ ] 브랜드 팔레트 내 색상만 사용한다
5. [ ] 16×16 scale에서도 정체성이 유지된다 (favicon test)
6. [ ] 공격적·위협적·성적 요소가 전혀 없다
7. [ ] "다마고치" 정서가 느껴진다 (복고 + 반려)

---

_Last updated: 2026-04-23 (M0 Brand Foundation)_
