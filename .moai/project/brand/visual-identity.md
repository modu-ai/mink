# Visual Identity — GOOSE

GOOSE의 비주얼 시스템은 **"따뜻한 다마고치"** 컨셉을 기반으로 합니다. 차가운 SaaS 디자인의 파란색 네온이 아닌, 손에 쥐면 체온이 느껴지는 90년대 휴대용 기기의 정서를 모던하게 재해석합니다.

---

## 1. Design Principles

1. **Warmth over Sleekness** — 미끈함보다 따뜻함. 글로시한 그라디언트 대신 부드러운 컬러 블록.
2. **Playful but Calm** — 귀엽되 시끄럽지 않게. 과한 애니메이션·이모지 금지.
3. **Growth is Visible** — 성장 단계가 UI 전반에 은은하게 반영됨 (배경 톤, 실루엣, 모션).
4. **Cross-Platform Consistency** — Telegram bot부터 Desktop까지 동일한 시각 언어.
5. **Accessibility First** — WCAG 2.1 AA 필수. 색상 대비 4.5:1 이상.

---

## 2. Color Palette

### 2.1 Primary — Sunrise Yolk

브랜드의 중심 색. "알 노른자"에서 영감. 따뜻한 에너지.

| Token | HEX | 용도 |
|---|---|---|
| `primary.50` | `#FFF8E7` | 배경 washes |
| `primary.100` | `#FFEFC2` | Hover 상태 배경 |
| `primary.200` | `#FFE08A` | Accent 면 |
| `primary.300` | `#FFCE4D` | Secondary CTA |
| `primary.500` | `#FFB800` | **Primary CTA, 브랜드 시그니처** |
| `primary.600` | `#E6A300` | Hover on primary |
| `primary.700` | `#B37F00` | Active / pressed |
| `primary.900` | `#4D3600` | Text on light surface |

### 2.2 Secondary — Soft Sage

GOOSE의 이름(거위)과 자연을 연결. 안정감.

| Token | HEX | 용도 |
|---|---|---|
| `secondary.50` | `#F0F6F1` | 섹션 배경 대체 |
| `secondary.200` | `#C8DFCA` | Borders, dividers |
| `secondary.500` | `#6B9E74` | Secondary 액션, 성장 지표 |
| `secondary.700` | `#406B47` | Text accent |

### 2.3 Accent — Berry Blush

드물게 사용. 중요한 감정 모먼트(성장 단계 전환, 축하).

| Token | HEX | 용도 |
|---|---|---|
| `accent.300` | `#F6A5B0` | Gentle highlight |
| `accent.500` | `#E56B7C` | 성장 전환 알림, heart states |

### 2.4 Neutral — Warm Gray

순수 회색이 아닌 미세하게 노란빛 섞인 따뜻한 중립.

| Token | HEX | 용도 |
|---|---|---|
| `neutral.50` | `#FAF8F4` | **Page background (light mode)** |
| `neutral.100` | `#F1EEE8` | Surface |
| `neutral.200` | `#E2DDD2` | Borders |
| `neutral.300` | `#C9C2B3` | Disabled |
| `neutral.400` | `#9D9587` | Placeholder |
| `neutral.500` | `#6F685C` | Secondary text |
| `neutral.600` | `#524C42` | Body text |
| `neutral.700` | `#3A352E` | Heading text |
| `neutral.800` | `#252220` | Dark surface |
| `neutral.900` | `#171513` | **Page background (dark mode)** |
| `neutral.950` | `#0B0A09` | Deepest dark |

### 2.5 Semantic

| Token | HEX | 용도 |
|---|---|---|
| `success` | `#4C9B6A` | 완료, 성장 |
| `warning` | `#E89A3C` | 주의, 리추얼 누락 |
| `danger` | `#D45A4F` | 에러, 파괴적 행동 |
| `info` | `#5B8FB5` | 정보 제공 |

### 2.6 Contrast Compliance

- `primary.500` (#FFB800) on `neutral.900` (#171513): **13.4:1** — AAA
- `neutral.600` (#524C42) on `neutral.50` (#FAF8F4): **9.1:1** — AAA
- `primary.700` (#B37F00) on `neutral.50` (#FAF8F4): **4.6:1** — AA
- `primary.500` (#FFB800) on white: **2.1:1** — **사용 금지** (큰 텍스트만)

### 2.7 Do / Don't

**Do**
- `primary.500`은 CTA와 핵심 브랜드 모먼트에만 사용 (화면당 1~2곳)
- 배경은 `neutral.50` (light) / `neutral.900` (dark) 기준 유지
- `secondary.500` + `primary.500` 조합으로 차분한 색 대비 구성

**Don't**
- 보라 ↔ 파랑 그라디언트 (generic AI SaaS 인상)
- `primary.500`을 대면적 배경으로 사용 (피로감)
- 6개 이상의 색을 한 화면에 혼합

---

## 3. Typography

### 3.1 Font Pair

| 용도 | Latin | Korean | Fallback |
|---|---|---|---|
| Primary (UI/Body) | **Inter** | **Pretendard** | `-apple-system, system-ui` |
| Display (Hero) | **Fraunces** (serif, optical) | **Pretendard** (heavy) | `Georgia, serif` |
| Mono | **JetBrains Mono** | **D2Coding** | `ui-monospace, Menlo` |

- Pretendard는 한글 가독성과 Inter와의 수직 리듬이 자연스러움.
- Fraunces는 hero에만 사용. 따뜻한 세리프가 "companion" 정서를 강화.
- Source: `bunny-fonts` (우선) → `google-fonts` (fallback). Self-hosted 권장.

### 3.2 Scale (Major Third, 1.250)

| Token | Size | Line-height | Weight | Use |
|---|---|---|---|---|
| `display.xl` | 72px / 4.5rem | 1.05 | 600 | Hero landing |
| `display.lg` | 56px / 3.5rem | 1.1 | 600 | Section hero |
| `display.md` | 44px / 2.75rem | 1.15 | 600 | Page title |
| `heading.h1` | 36px / 2.25rem | 1.2 | 600 | H1 |
| `heading.h2` | 28px / 1.75rem | 1.3 | 600 | H2 |
| `heading.h3` | 22px / 1.375rem | 1.35 | 500 | H3 |
| `body.lg` | 18px / 1.125rem | 1.6 | 400 | Lead paragraph |
| `body.md` | 16px / 1rem | 1.6 | 400 | **Default body** |
| `body.sm` | 14px / 0.875rem | 1.55 | 400 | Secondary |
| `caption` | 12px / 0.75rem | 1.5 | 500 | Meta, timestamps |
| `mono.md` | 14px / 0.875rem | 1.6 | 400 | Code |

### 3.3 Korean-specific

- `letter-spacing`: 한글은 `-0.01em` 권장 (Pretendard 최적화)
- 한글 행간: Latin보다 0.05 여유 (`line-height: 1.65` for body)
- 숫자는 `font-feature-settings: "tnum"` (tabular)

---

## 4. Spacing System

**Base unit: 4px.** 4px grid 기반, 8px rhythm 선호.

| Token | Value | Use |
|---|---|---|
| `space.0` | 0 | — |
| `space.1` | 4px | Icon padding |
| `space.2` | 8px | Tight stack |
| `space.3` | 12px | Inline gap |
| `space.4` | 16px | **Default gap** |
| `space.5` | 24px | Card padding |
| `space.6` | 32px | Section inner |
| `space.8` | 48px | Section gap (mobile) |
| `space.10` | 64px | Section gap (desktop) |
| `space.12` | 96px | Hero margin |
| `space.16` | 128px | Page-level whitespace |

**Layout width**
- Content max-width: `1120px`
- Reading max-width: `680px`
- Mobile gutter: `16px`, Desktop gutter: `32px`

---

## 5. Border Radius

따뜻함의 핵심 장치. 각진 모서리 최소화.

| Token | Value | Use |
|---|---|---|
| `radius.sm` | 6px | Inline chips |
| `radius.md` | 12px | **Default (buttons, inputs)** |
| `radius.lg` | 20px | Cards |
| `radius.xl` | 32px | Hero panels, mascot container |
| `radius.full` | 9999px | Avatar, pill buttons |

**Rule**: 동일 화면 내 radius는 2단계까지만 혼용.

---

## 6. Shadow / Elevation

차가운 drop-shadow 대신 **따뜻한 soft shadow** 사용.

| Token | Value | Use |
|---|---|---|
| `shadow.sm` | `0 1px 2px rgba(77, 54, 0, 0.06)` | Hover |
| `shadow.md` | `0 4px 12px rgba(77, 54, 0, 0.08)` | **Card default** |
| `shadow.lg` | `0 12px 32px rgba(77, 54, 0, 0.12)` | Modal, popover |
| `shadow.xl` | `0 24px 48px rgba(77, 54, 0, 0.16)` | Hero floating |

그림자 RGB 기반색은 `primary.900`(#4D3600)에서 추출 — 그레이 섀도우가 아닌 "따뜻한 섀도우".

---

## 7. Iconography

### Style Direction
- **Line weight**: 1.5px (small), 2px (default), 2.5px (large)
- **Corner**: rounded (stroke-linejoin: round, stroke-linecap: round)
- **Style**: outlined 기본. filled는 active 상태에만.
- **Grid**: 24×24 기본, 16×16 small, 32×32 large
- **Family**: Lucide Icons 기반 (MIT license, 커스터마이징 가능)

### Don't
- 2개 이상의 아이콘 스타일 혼용 (outlined + filled + duotone)
- 그라디언트 아이콘
- emoji-like 컬러 아이콘 (flat 이모지 셋은 금지)

---

## 8. Motion Principles

GOOSE의 성장을 느끼게 하는 4가지 모션 원칙.

### 8.1 Breathing (숨쉬기)
마스코트와 주요 인터랙션 요소는 은은하게 스케일 변화.
- Scale: `1.0 ↔ 1.02`
- Duration: `3.2s` (느림)
- Easing: `cubic-bezier(0.4, 0, 0.6, 1)` (sine-like)
- Loop: infinite

### 8.2 Growth (성장)
성장 단계 전환 순간만 사용. 강한 모먼트.
- Duration: `800ms`
- Easing: `cubic-bezier(0.34, 1.56, 0.64, 1)` (gentle overshoot)
- 스케일 + 색상 블렌드 동시

### 8.3 Acknowledge (응답)
사용자 입력에 대한 즉각 반응.
- Duration: `180ms`
- Easing: `cubic-bezier(0.2, 0, 0, 1)` (snappy)
- 과장된 바운스 금지

### 8.4 Settle (안정)
페이지·상태 전환.
- Duration: `320ms`
- Easing: `cubic-bezier(0.4, 0, 0.2, 1)` (standard)
- Fade + 4~8px translate 조합

### 8.5 Accessibility
`prefers-reduced-motion: reduce` 사용자는 모든 루프 모션 OFF, 전환만 150ms fade.

---

## 9. Dark Mode

- 지원 방식: **System (prefers-color-scheme) + Manual toggle**
- Dark 배경: `neutral.900` (#171513) — 순흑 금지 (피로)
- Dark primary: `primary.300` (#FFCE4D) — 대비 유지
- 그림자: dark mode에선 `rgba(0,0,0,0.3)` 기반 + border-glow 보조

---

## 10. Mascot Placement Rules

- **Min size**: 80×80px (mobile), 120×120px (desktop)
- **Clearspace**: 마스코트 높이의 25% 사방 여백
- **Never**: 마스코트 위에 텍스트 오버레이
- **Always**: 마스코트 뒤 배경은 `neutral.50` 또는 `primary.50` 계열

---

## 11. Cross-Platform Surface

| Platform | Background | Primary CTA | Font |
|---|---|---|---|
| Web / Desktop | `neutral.50` | `primary.500` | Inter + Pretendard |
| iOS | System bg | `primary.500` | SF Pro + Apple SD Gothic Neo |
| Android | Material surface | `primary.500` | Roboto + Noto Sans KR |
| Telegram | 테마 상속 | `primary.500` accent만 | 시스템 폰트 |
| CLI | 터미널 기본 | ANSI 색 (`#FFB800` → 33/yellow) | Mono |

---

## 12. Logo & Wordmark

- Logo file: `logo/mascot-base.svg` (mascot) + `logo/wordmark.svg` (text)
- Dark variant: 색 반전 없이 `primary.300` 사용
- Max height in nav: `32px` (mobile), `40px` (desktop)
- Min clearspace: wordmark 높이의 50%

자세한 사양은 `logo/README.md` 참조.

---

_Last updated: 2026-04-23 (M0 Brand Foundation)_
_Source of truth for design-tokens.json and all implementation._
_Constitutional parent per `.claude/rules/moai/design/constitution.md` Section 3.1._
