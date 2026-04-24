---
id: SPEC-GOOSE-ONBOARDING-001
version: 0.1.0
status: planned
created_at: 2026-04-22
updated_at: 2026-04-22
author: manager-spec
priority: P0
issue_number: null
phase: 6
size: 중(M)
lifecycle: spec-anchored
labels: []
---

# SPEC-GOOSE-ONBOARDING-001 — CLI + Web UI Install Wizard

> **v0.2 Amendment (2026-04-24)**: SPEC-GOOSE-ARCH-REDESIGN-v0.2 에 따라 **스코프 축소**.
> 제거: 이메일 가입/로그인 플로우, 모바일 디바이스 페어링 단계, Apple Native 초기 설정.
> 유지: **`goose init` CLI 마법사** + **Web UI 설치·설정 마법사** (비개발자 대응).
> 초기 설정 범위: `./.goose/` 생성 → persona/soul.md 입력 → provider key 저장 (OS keyring) → 첫 messenger 채널 활성화.
> 기존 8-step 플로우는 재구성 필요.

---

## 원본 타이틀: First-Install 8-Step Onboarding Flow ★ 스코프 축소됨

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성. v5.0 ROADMAP Phase 6 Localization 시리즈 4번째(최종). 사용자 지시: "설치시 사용자의 국가와 정보를 수집해서 사용자에 맞게". Desktop App 첫 실행 시 8단계 UX로 locale + identity + daily pattern + interests + ritual + LLM provider + privacy 수집. | manager-spec |

---

## 1. 개요 (Overview)

사용자 최종 지시(2026-04-22):

> "설치시 사용자의 국가와 정보를 수집해서 사용자에 맞게 각 현지화된 스킬들을 추가하도록 하자."

본 SPEC은 **GOOSE 첫 설치 시 사용자를 맞이하는 8단계 온보딩 플로우**를 정의한다. Desktop App(DESKTOP-001)에 호스트되며, Mobile App(MOBILE-001)은 페어링으로 locale 동기화. 소요 시간 목표: **5분 이하**. 모든 단계는 **스킵 가능**(기본값 적용) + **뒤로 가기** + **진행 바** 제공.

8단계:

1. **Welcome** 🥚 — 알 부화 애니메이션 + 짧은 intro
2. **Locale 확인** 🌍 — LOCALE-001이 감지한 country/language/timezone 표시 + 수정 옵션
3. **Identity** 👤 — 이름, 선호 호칭(존칭 레벨), 대명사(선택)
4. **Daily Pattern** ⏰ — 기상/아침/점심/저녁/취침 시간 (SCHEDULER-001 소비)
5. **Interests** 🏷️ — 직업 · 취미 · 관심사 태그 (IDENTITY-001 POLE+O 기초)
6. **Ritual Preferences** 🌅 — Morning/Meals/Evening 리추얼 on/off + 커스터마이징
7. **LLM Provider** 🤖 — Anthropic / OpenAI / Google / Ollama / Custom 선택 + API key 또는 OAuth
8. **Privacy & Consent** 🔒 — 데이터 수집 범위, LoRA 훈련 포함 여부, Telemetry opt-in

완료 후:

- REGION-SKILLS-001 자동 활성화 (LOCALE country 기반)
- Identity Graph 초기 노드 생성 (이름, 관심사)
- Goose 부화 애니메이션 + 첫 인사(i18n 적용, 사용자 언어)
- Mobile 페어링 QR 제안 (선택)

---

## 2. 배경 (Background)

### 2.1 왜 8단계인가

**최소한의 단계로 최대한의 개인화**. 각 단계는 후속 SPEC의 필수 입력:

| Step | 수집 데이터 | 소비 SPEC |
|------|-----------|----------|
| 1 | — (welcome) | — |
| 2 | country/language/timezone | LOCALE-001, I18N-001, REGION-SKILLS-001, SCHEDULER-001 |
| 3 | name, honorific_level, pronouns | IDENTITY-001, ADAPTER-001(말투) |
| 4 | wake/meal/sleep times | SCHEDULER-001, BRIEFING-001, HEALTH-001 |
| 5 | interests tags | IDENTITY-001, VECTOR-001 |
| 6 | ritual on/off | RITUAL-001, BRIEFING-001, JOURNAL-001 |
| 7 | llm_provider, credentials | CREDPOOL-001, ROUTER-001, ADAPTER-001 |
| 8 | consent flags | MEMORY-001, LORA-001, telemetry |

한 단계라도 누락 시 후속 SPEC 기능 저하. 단, 각 단계는 기본값으로 완료 가능하여 **"5분 안에 무조건 끝나는 UX"** 달성.

### 2.2 Desktop-hosted 이유

- Desktop(DESKTOP-001)이 "기본 설치 대상". Mobile은 페어링 경로, CLI는 헤드리스.
- 카메라/마이크/파일 시스템 등 권한 요청이 Desktop에서 가장 자연스러움.
- LLM API key 입력 시 clipboard → 키체인 저장이 Desktop Rust(Tauri Keyring)로 안전.
- Mobile은 페어링 후 locale/identity 동기화만 수행(중복 입력 회피).

### 2.3 branding.md §5.1 First Day with GOOSE

> "Hatching 🐣 — 알에서 깨어나는 최초의 만남. Goose가 사용자를 발견하고 각인(imprinting)하는 순간."

각인(imprinting) 메타포: 첫 입력이 GOOSE의 "어미"를 결정. 본 SPEC은 이 각인 이벤트를 기술적으로 구현 → Step 3(Identity) 완료 시 GOOSE mood가 `calm → imprinting → curious`로 전환, 트레이 아이콘 갱신.

### 2.4 법적 제약

- **GDPR** (EU): 명시적 동의 필수, withdrawal 권리 고지
- **PIPA** (KR): 민감정보 최소 수집
- **CCPA** (US): do-not-sell 옵션
- **LGPD** (BR), **PIPL** (CN), **FZ-152** (RU): 각 country의 법률 flags는 LOCALE-001이 제공, Step 8이 그에 따라 동의 문구 조정

### 2.5 범위 경계

- **IN**: 8단계 UI (React), Tauri backend command, CONFIG-001 저장, LOCALE/I18N/REGION-SKILLS/IDENTITY 초기화 호출, 첫 인사 애니메이션, 페어링 QR 제안.
- **OUT**: LLM provider 상세 OAuth 플로우(CREDPOOL-001), Identity Graph 완전 구축(IDENTITY-001), LoRA 훈련 자체(LORA-001), Mobile 페어링 자체(BRIDGE-001), 법률 문구 전체(외부 법률 검토 후 주입).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. **Desktop UI** — `packages/goose-desktop/src/onboarding/`:
   - 8개 React 컴포넌트 (Step1Welcome, Step2Locale, ..., Step8Privacy)
   - Zustand store로 수집 데이터 임시 보관
   - Framer Motion 애니메이션 (부화, 전환)
   - 뒤로 가기 / 스킵 / 진행 바 (shadcn/ui)
   - 키보드 단축키 (Enter=다음, Esc=스킵)
2. **Backend (Go)** — `internal/onboarding/`:
   - `flow.go` — `OnboardingFlow` 상태 머신
   - `steps.go` — 8 step 정의 + validation
   - `progress.go` — 진행률 계산 및 지속화
   - `completion.go` — 완료 처리 (후속 SPEC 초기화 호출)
3. **Tauri Commands**:
   - `onboarding_start()` — 플로우 시작
   - `onboarding_submit_step(step, data)` — 각 단계 데이터 저장
   - `onboarding_skip_step(step)` — 기본값 적용 + 다음
   - `onboarding_back()` — 이전 단계
   - `onboarding_complete()` — 최종 완료 + 후속 초기화
   - `onboarding_pair_mobile_qr()` — Mobile 페어링 QR 생성 호출
4. **Step 2 (Locale)** 상세:
   - LOCALE-001의 `Detect()` 결과 표시
   - Country, Language, Timezone 각각 수정 가능 (dropdown)
   - Conflict 존재 시 (OS vs IP) 명시 표시
   - "Apply and continue" 클릭 시 `LocaleContext` 저장 + I18N 즉시 적용
5. **Step 3 (Identity)** 상세:
   - Name (필수), Preferred Honorific Level (한국어일 때: 존댓말/해요체/반말), Pronouns (he/she/they/기타)
   - LOCALE-001의 `CulturalContext.formality_default` 따라 기본값 추천
   - Language별 UI (한국 사용자는 한국어로 표시)
6. **Step 4 (Daily Pattern)** 상세:
   - 기상 시간, 아침 식사, 점심 식사, 저녁 식사, 취침 시간 (각각 시간 선택기)
   - Weekday / Weekend 분리 옵션
   - 기본값: 07:00 / 08:00 / 12:30 / 19:00 / 23:00
   - 하루라도 편차 있으면 "평균" 사용 안내
7. **Step 5 (Interests)** 상세:
   - 직업 카테고리 (Developer, Designer, Writer, Student, Manager, Entrepreneur, Other)
   - 취미 태그 (Reading, Gaming, Cooking, Fitness, Travel, Music, Art, Photography, Nature, Coding, ...)
   - Free-form custom tags (최대 10개)
   - 관심 도메인 (Tech, Health, Finance, Entertainment, Education, Politics, Science, ...)
8. **Step 6 (Ritual Preferences)** 상세:
   - Morning Briefing: on/off, 시간 조정 (기본 wake_time + 15min)
   - Meal Reminders: 각 식사별 on/off + 약 복용 여부
   - Evening Journal: on/off, 시간 (기본 sleep_time - 30min)
   - Weekend 다른 패턴: 체크박스
9. **Step 7 (LLM Provider)** 상세:
   - 카드 리스트 UI: Anthropic / OpenAI / Google / Ollama / DeepSeek / Custom
   - Anthropic/OpenAI: OAuth 버튼 (브라우저 열림) + API key 대안
   - Ollama: localhost detection, 모델 드롭다운
   - Custom: URL + API key + model name
   - API key 저장은 OS Keychain (Tauri plugin-keyring)
   - "Skip and configure later" 옵션 (환경변수 의존)
10. **Step 8 (Privacy & Consent)** 상세:
    - 체크박스 UI:
      - [x] 대화 기록 로컬 저장 (필수, 기본 ON)
      - [ ] LoRA 개인 모델 훈련에 사용 (opt-in, 기본 OFF)
      - [ ] Anonymous telemetry (opt-in, 기본 OFF)
      - [ ] 오류 보고 자동 전송 (opt-in, 기본 OFF)
    - GDPR region이면 "I explicitly consent" 명시적 체크 강제
    - "Privacy Policy" 링크 + "Export my data" + "Delete my data" 안내
    - Submit 시 `ConsentFlags` CONFIG-001에 저장
11. **완료 처리**:
    - Goose 부화 애니메이션 (3초)
    - 첫 인사 TTS 또는 텍스트 ("안녕하세요, [이름]님!") — LocaleContext + CulturalContext 적용
    - REGION-SKILLS-001 `ActivateForCountry(country)` 호출
    - Identity Graph 초기 노드 seed (IDENTITY-001의 public API)
    - Mobile 페어링 제안: "모바일에서도 GOOSE를 사용하시겠어요?" → Yes 클릭 시 DESKTOP의 QR 페어링 창 호출
12. **5분 목표 수행 측정**:
    - 각 단계 소요 시간 로그 (익명화)
    - 완료율, 중도 이탈율 추적 (Step 8 opt-in 시에만)

### 3.2 OUT OF SCOPE

- **LLM OAuth 플로우 상세**: CREDPOOL-001 전담. 본 SPEC은 "OAuth 버튼 클릭" 이벤트만 발신.
- **Identity Graph 완전 구축**: 본 SPEC은 초기 seed 노드만. POLE+O 확장은 IDENTITY-001.
- **LoRA 훈련 자체**: LORA-001. 본 SPEC은 동의 flag만 저장.
- **Mobile 페어링 프로토콜**: BRIDGE-001/MOBILE-001. 본 SPEC은 QR 페어링 UI 진입점만.
- **법률 문구 작성**: 외부 법률 검토 후 `docs/privacy-policy.md`에 작성, 본 SPEC은 링크만.
- **Accessibility 화면 전환**(wizard ↔ form): v1.0+.
- **재온보딩 플로우**: 1회성. 추후 "Preferences > Re-run onboarding" 메뉴(v1.0+).
- **A/B 테스트**: v2+.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-OB-001 [Ubiquitous]** — The onboarding flow **shall** complete in 8 steps or fewer and **shall** present a visible progress bar showing current step / total steps at all times.

**REQ-OB-002 [Ubiquitous]** — Every onboarding step **shall** provide three actions: `Next` (submit + advance), `Back` (return to previous step, disabled on Step 1), and `Skip` (apply default values + advance).

**REQ-OB-003 [Ubiquitous]** — The onboarding UI **shall** be rendered in the language determined by LOCALE-001's `primary_language` as soon as Step 2 completes; prior to Step 2, the UI **shall** use the OS-detected language.

**REQ-OB-004 [Ubiquitous]** — All user inputs **shall** be validated before advancing to the next step; validation errors **shall** display inline with field-specific messages in the user's language.

### 4.2 Event-Driven

**REQ-OB-005 [Event-Driven]** — **When** the Desktop App is launched for the first time (no `~/.goose/config.yaml` exists or no `onboarding_completed: true`), the app **shall** start the onboarding flow as a modal full-screen overlay before showing the main UI.

**REQ-OB-006 [Event-Driven]** — **When** Step 2 (Locale) is submitted, the Tauri backend **shall** call LOCALE-001's override API to persist the user's choice and **shall** immediately reload the I18N bundles for the new language.

**REQ-OB-007 [Event-Driven]** — **When** Step 7 (LLM Provider) is submitted with an API key, the Tauri backend **shall** store the secret in the OS keychain via `tauri-plugin-keyring` and **shall not** write plaintext secrets to `config.yaml`.

**REQ-OB-008 [Event-Driven]** — **When** Step 8 (Privacy) is submitted and the user's country is in EU, the form **shall** require explicit `I consent` checkbox before allowing completion; skipping is not permitted in GDPR regions for consent-required fields.

**REQ-OB-009 [Event-Driven]** — **When** the user completes Step 8, the backend **shall** (a) persist all collected data to CONFIG-001, (b) call REGION-SKILLS-001 to activate country skills, (c) seed Identity Graph initial nodes, (d) mark `onboarding_completed: true`, and (e) transition to the main UI with a hatching animation.

**REQ-OB-010 [Event-Driven]** — **When** the user clicks "Skip" on Step 7 (LLM Provider), the backend **shall** record `llm.default_provider: "unset"`, and the main UI **shall** display a non-blocking banner prompting the user to configure a provider later.

### 4.3 State-Driven

**REQ-OB-011 [State-Driven]** — **While** the user is in the middle of onboarding and quits the app, the collected data (partial) **shall** be persisted to a temp file `~/.goose/onboarding-draft.yaml`, and re-opening the app **shall** resume from the last completed step.

**REQ-OB-012 [State-Driven]** — **While** LOCALE-001's `Detect()` returned `LocaleConflict` (OS vs IP mismatch), Step 2 **shall** display both values side-by-side and require the user to choose one before advancing.

**REQ-OB-013 [State-Driven]** — **While** the user is on Step 6 and has unchecked all three ritual options, the UI **shall** display a soft notice: "You can always enable rituals later from Preferences" and allow advance.

### 4.4 Unwanted Behavior

**REQ-OB-014 [Unwanted]** — The onboarding flow **shall not** transmit any user input to external servers except during Step 7 LLM OAuth (initiated by the user) or Step 8 telemetry opt-in (after explicit consent).

**REQ-OB-015 [Unwanted]** — **If** the user enters an API key that fails validation (malformed, wrong prefix for provider), **then** the form **shall** display an error and **shall not** store the invalid key in the keychain.

**REQ-OB-016 [Unwanted]** — The onboarding flow **shall not** request sensitive data that is not strictly needed: no SSN, no biometric templates, no medical records, no government IDs. Profession is a free-form category (no verification).

**REQ-OB-017 [Unwanted]** — **If** Step 3 (Identity) is submitted with a name field containing 500+ characters or shell injection patterns, **then** the backend **shall** reject the submission and log a security event.

### 4.5 Optional

**REQ-OB-018 [Optional]** — **Where** the user has successfully completed Step 7 with a valid provider, the onboarding flow **may** offer to pair with a mobile device at the end via QR code (BRIDGE-001 integration).

**REQ-OB-019 [Optional]** — **Where** accessibility features are enabled in the OS (high contrast, reduced motion), the onboarding UI **shall** respect those preferences (no autoplay animations, WCAG AA contrast).

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-OB-001 — 첫 실행 시 온보딩 시작**
- **Given** fresh install, `~/.goose/config.yaml` 없음
- **When** Desktop App 실행
- **Then** 메인 UI 대신 온보딩 모달이 full-screen으로 표시, Step 1 Welcome 화면

**AC-OB-002 — 8단계 진행 바**
- **Given** 온보딩 진행 중
- **When** Step 3 도착
- **Then** 진행 바가 "3 / 8 (37%)" 표시, Back 버튼 활성, Skip 버튼 존재

**AC-OB-003 — Locale 감지 + 수정**
- **Given** LOCALE-001이 `country="KR"` 감지
- **When** Step 2 표시
- **Then** "거주 국가: 대한민국" + dropdown으로 변경 가능. 사용자가 "일본"으로 변경 후 Next → CONFIG-001에 `country="JP"` 저장, UI 언어는 한국어→일본어로 즉시 전환

**AC-OB-004 — OS vs IP 충돌 해결**
- **Given** `LocaleConflict{os:"KR", ip:"US"}` 감지
- **When** Step 2 표시
- **Then** "OS 설정: 한국 / IP 위치: 미국" 두 라디오 버튼, 하나 선택 강제 후 Next

**AC-OB-005 — 이름 유효성**
- **Given** Step 3에서 name 필드 비어있음
- **When** Next 클릭
- **Then** "이름을 입력해주세요" 인라인 에러, 전진 불가

**AC-OB-006 — Skip 기본값 적용 (Step 4 Daily Pattern)**
- **Given** Step 4, 사용자가 아무것도 입력 안 함
- **When** Skip 클릭
- **Then** wake=07:00, breakfast=08:00, lunch=12:30, dinner=19:00, sleep=23:00 저장, Step 5로 진행

**AC-OB-007 — API key 키체인 저장**
- **Given** Step 7, 사용자가 Anthropic 선택 + `sk-ant-xxx` 입력
- **When** Next 클릭
- **Then** 키체인에 `anthropic.api_key` 저장, CONFIG-001 `~/.goose/config.yaml`에는 `providers.anthropic.api_key_source: keychain` 만 저장(평문 없음)

**AC-OB-008 — GDPR 명시적 동의**
- **Given** `country="DE"` (EU), Step 8
- **When** 사용자가 "I consent" 체크 없이 Submit 클릭
- **Then** 에러 "Explicit consent required", Submit 차단

**AC-OB-009 — 완료 후 Region Skills 활성화**
- **Given** 사용자가 `country="KR"`로 온보딩 완료
- **When** Step 8 Submit
- **Then** REGION-SKILLS-001 `korean-holidays`, `kakao-talk`, `jondaetmal-etiquette` 자동 활성. CONFIG-001 `skills.region.active` 배열에 3개 ID 등록

**AC-OB-010 — 부화 애니메이션 + 첫 인사**
- **Given** 온보딩 완료
- **When** Step 8 Submit 후 3초
- **Then** GOOSE egg → hatching → baby goose 애니메이션 재생, 한국어 사용자에겐 "안녕하세요, [이름]님! 오늘 처음 뵙네요." 인사 표시

**AC-OB-011 — 중도 이탈 후 재개**
- **Given** Step 5 완료 후 사용자가 앱 종료
- **When** Desktop 재실행
- **Then** `~/.goose/onboarding-draft.yaml`에 Step 1~5 데이터 존재, UI는 Step 6부터 재시작

**AC-OB-012 — LLM provider 스킵**
- **Given** Step 7에서 Skip 클릭
- **When** Step 8로 진행
- **Then** `llm.default_provider: "unset"` 저장. 온보딩 완료 후 메인 UI 상단에 "LLM 공급자를 설정해주세요" 배너 표시 (닫기 가능)

**AC-OB-013 — Mobile 페어링 제안**
- **Given** Step 7 완료(유효한 provider 설정)
- **When** Step 8 Submit 후
- **Then** 완료 화면에 "모바일에서도 사용하시겠어요?" 버튼, 클릭 시 BRIDGE-001의 QR 페어링 창 호출

**AC-OB-014 — Skip 불허 (GDPR consent)**
- **Given** `country="FR"`, Step 8
- **When** Skip 클릭
- **Then** 에러 "명시적 동의는 스킵할 수 없습니다" (프랑스어), 전진 불가

**AC-OB-015 — 입력 사니타이징 (이름)**
- **Given** Step 3 name = `"Hacker<script>alert(1)</script>"`
- **When** Next 클릭
- **Then** 정규식 검증 실패, 에러 표시, 저장 안 됨, security event 로그

**AC-OB-016 — 5분 이내 완료 가능성**
- **Given** 모든 Skip 사용 + 기본값 수용
- **When** Step 1 → Step 8 전체 진행
- **Then** 총 소요 시간 ≤ 5분 (수동 측정 CI test), Skip 8회 + 최소 입력(Step 3 name 필수)만으로 완료 가능

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
packages/goose-desktop/src/onboarding/
├── OnboardingModal.tsx          # 상위 컨테이너
├── steps/
│   ├── Step1Welcome.tsx
│   ├── Step2Locale.tsx
│   ├── Step3Identity.tsx
│   ├── Step4DailyPattern.tsx
│   ├── Step5Interests.tsx
│   ├── Step6Rituals.tsx
│   ├── Step7LLMProvider.tsx
│   └── Step8Privacy.tsx
├── ProgressBar.tsx
├── store.ts                      # zustand
├── types.ts                      # OnboardingData, ConsentFlags
└── __tests__/

internal/onboarding/
├── flow.go                       # OnboardingFlow 상태 머신
├── steps.go                      # Step definitions + validators
├── progress.go                   # Draft 저장/로드
├── completion.go                 # 완료 처리 + 후속 초기화
└── *_test.go
```

### 6.2 핵심 타입 (TypeScript)

```typescript
// packages/goose-desktop/src/onboarding/types.ts

export interface OnboardingFlow {
  currentStep: 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8;
  totalSteps: 8;
  data: Partial<OnboardingData>;
  advance(): Promise<void>;
  goBack(): Promise<void>;
  skip(): Promise<void>;
  complete(): Promise<void>;
  save(): Promise<void>;   // draft 저장
}

// 각 단계별 데이터
export interface OnboardingStep<T> {
  id: number;
  title: string;            // i18n key
  validator: (data: T) => ValidationResult;
  defaults: T;
  canSkip: boolean;
}

// 전체 수집 데이터
export interface OnboardingData {
  locale: LocaleChoice;     // Step 2
  identity: IdentityProfile; // Step 3
  dailyPattern: DailyPattern; // Step 4
  interests: Interests;      // Step 5
  rituals: RitualPreferences; // Step 6
  llmProvider: LLMProviderChoice; // Step 7
  consent: ConsentFlags;     // Step 8
}

export interface LocaleChoice {
  country: string;          // ISO 3166-1
  primaryLanguage: string;  // BCP 47
  secondaryLanguage?: string;
  timezone: string;         // IANA
  method: "detected" | "user_override";
}

export interface IdentityProfile {
  name: string;
  preferredHonorific?: "formal" | "casual" | "intimate"; // 한국/일본에서 의미 있음
  pronouns?: string;
}

export interface DailyPattern {
  wakeTime: string;         // "07:00" (HH:mm)
  breakfastTime: string;
  lunchTime: string;
  dinnerTime: string;
  sleepTime: string;
  weekendDifferent: boolean;
}

export interface Interests {
  profession: string;
  hobbies: string[];        // predefined tags
  customTags: string[];
  interestDomains: string[];
}

export interface RitualPreferences {
  morningEnabled: boolean;
  morningTime?: string;
  mealRemindersEnabled: boolean;
  medicationTrackingEnabled: boolean;
  eveningJournalEnabled: boolean;
  eveningTime?: string;
  weekendDifferent: boolean;
}

export interface LLMProviderChoice {
  provider: "anthropic" | "openai" | "google" | "ollama" | "deepseek" | "custom" | "unset";
  authMethod?: "oauth" | "api_key" | "env";
  apiKeyPresent?: boolean;  // true면 키체인에 있음
  customEndpoint?: string;
  preferredModel?: string;
}

export interface ConsentFlags {
  conversationStorageLocal: boolean; // 기본 true (필수)
  loraTrainingAllowed: boolean;       // 기본 false
  telemetryEnabled: boolean;          // 기본 false
  crashReportingEnabled: boolean;     // 기본 false
  gdprExplicitConsent?: boolean;     // EU 사용자만
}
```

### 6.3 Tauri Backend (Rust) — Commands

```rust
// packages/goose-desktop/src-tauri/src/onboarding.rs

#[tauri::command]
async fn onboarding_start(app: tauri::AppHandle) -> Result<OnboardingSession, String> {
    // session ID 생성, draft 파일 준비
}

#[tauri::command]
async fn onboarding_submit_step(
    session: OnboardingSession,
    step: u8,
    data: serde_json::Value,
) -> Result<(), String> {
    // validate + draft 저장
}

#[tauri::command]
async fn onboarding_complete(session: OnboardingSession) -> Result<(), String> {
    // CONFIG-001 persist, 후속 SPEC 초기화 호출
}
```

### 6.4 Backend Go 타입

```go
// internal/onboarding/flow.go

type OnboardingFlow struct {
    SessionID    string
    CurrentStep  int
    Data         OnboardingData
    StartedAt    time.Time
    CompletedAt  *time.Time
}

type OnboardingStep interface {
    ID() int
    Title() string
    Validate(data any) error
    DefaultValues() any
    CanSkip() bool
}

type UserProfile struct {
    Locale     locale.LocaleContext
    Identity   IdentityProfile
    Daily      DailyPattern
    Interests  Interests
    Rituals    RitualPreferences
    LLM        LLMProviderChoice
    Consent    ConsentFlags
}

// 공개 함수:
func StartFlow(ctx context.Context, locale *locale.LocaleContext) (*OnboardingFlow, error)
func (f *OnboardingFlow) SubmitStep(step int, data any) error
func (f *OnboardingFlow) Complete() (*UserProfile, error)
```

### 6.5 UI 흐름 (Framer Motion)

- 전체 전환: `motion.div` slide left/right
- Step 1 Welcome: egg 이미지 ↓ gentle bounce
- Step 8 완료: egg → cracking → baby goose 애니메이션 (3초, `useAnimationControls`)
- `prefers-reduced-motion` 존중 → fade only

### 6.6 국가별 Consent 문구 차이

| Country | GDPR | 문구 차이 |
|---------|------|---------|
| EU (DE, FR, IT, ...) | ✅ | "I explicitly consent" 체크박스 필수 |
| UK | ✅ (UK GDPR) | EU와 동일 |
| KR | PIPA | "개인정보 수집·이용 동의" 문구 + 주민등록번호 수집 안 함 명시 |
| US (CA) | CCPA | "Do not sell my personal information" 옵션 |
| BR | LGPD | EU 유사 |
| CN | PIPL | "境内数据处理" 명시, 데이터 해외 전송 동의 |
| JP | APPI | "個人情報の利用目的" 명시 |
| 기타 | — | 일반 문구 |

LOCALE-001의 `legal_flags`를 기반으로 Step 8 UI가 조건부 분기.

### 6.7 라이브러리 결정

| 용도 | 라이브러리 | 결정 근거 |
|------|----------|---------|
| React UI | `react` 19.x | DESKTOP-001과 공유 |
| State | `zustand` 5.x | DESKTOP-001 공유 |
| Animation | `framer-motion` 11.x | 부화 애니메이션 |
| 폼 검증 | `zod` 3.x + `react-hook-form` 7.x | 선언적 validation |
| UI 컴포넌트 | `shadcn/ui` | Desktop과 일관 |
| Keychain (Rust) | `tauri-plugin-keyring` | OS 키체인 랩퍼 |
| i18n | `react-i18next` | I18N-001 공유 |

### 6.8 TDD 진입 순서

1. **RED #1** — `TestOnboardingFlow_FirstLaunch_StartsStep1` → AC-OB-001
2. **RED #2** — `TestStep3Identity_EmptyName_Rejected` → AC-OB-005
3. **RED #3** — `TestStep4_Skip_AppliesDefaults` → AC-OB-006
4. **RED #4** — `TestStep7_APIKey_StoredInKeychain` → AC-OB-007
5. **RED #5** — `TestStep8_GDPR_RequiresExplicitConsent` → AC-OB-008
6. **RED #6** — `TestComplete_ActivatesRegionSkills` → AC-OB-009
7. **RED #7** — `TestDraftResume_AfterQuit` → AC-OB-011
8. **RED #8** — `TestStep3_NameInjection_Rejected` → AC-OB-015
9. **RED #9** — `TestLocaleConflict_ForcesChoice` → AC-OB-004
10. **GREEN** — 최소 구현
11. **REFACTOR** — step validator를 선언적 테이블로
12. **E2E (Playwright)** — 5분 이내 전체 플로우 완주

### 6.9 TRUST 5 매핑

| 차원 | 달성 방법 |
|-----|---------|
| **T**ested | 15 unit tests (step validation), 5 integration tests (flow state machine), 1 E2E(Playwright full flow), 3 OS matrix |
| **R**eadable | 각 Step 컴포넌트 분리(React) + Go validator 선언적 테이블, i18n 모든 문자열 |
| **U**nified | shadcn/ui 일관, React state = zustand 단일 소스 |
| **S**ecured | API key 키체인 전용 저장(REQ-OB-007), 입력 사니타이징(REQ-OB-017), GDPR 명시적 동의 강제(REQ-OB-008) |
| **T**rackable | 각 step 소요 시간 익명 로그(opt-in), 완료율 추적, `onboarding-draft.yaml`로 상태 복원 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | **SPEC-GOOSE-DESKTOP-001** | 호스트 환경 (Tauri v2 + React) |
| 선행 SPEC | **SPEC-GOOSE-LOCALE-001** | Detect() 결과 + override API + CulturalContext |
| 선행 SPEC | **SPEC-GOOSE-I18N-001** | UI 번역 + RTL + 국가별 consent 문구 |
| 선행 SPEC | **SPEC-GOOSE-CONFIG-001** | 최종 UserProfile 저장소 |
| 동시 | SPEC-GOOSE-REGION-SKILLS-001 | 완료 시 자동 활성화 호출 |
| 동시 | SPEC-GOOSE-IDENTITY-001 (최소) | Initial node seed (이름 + 관심사) |
| 후속 SPEC | SPEC-GOOSE-SCHEDULER-001 | DailyPattern 소비 |
| 후속 SPEC | SPEC-GOOSE-RITUAL-001 | RitualPreferences 소비 |
| 후속 SPEC | SPEC-GOOSE-CREDPOOL-001 | LLMProviderChoice + API key |
| 후속 SPEC | SPEC-GOOSE-BRIDGE-001 | Mobile 페어링 QR 호출 |
| 외부 | `tauri-plugin-keyring` | OS 키체인 |
| 외부 | `framer-motion` 11.x | 애니메이션 |
| 외부 | `zod` 3.x + `react-hook-form` 7.x | 폼 검증 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 8단계가 너무 길다고 느껴 중도 이탈 | 고 | 고 | Skip 적극 강조, 5분 내 완료 보장, 각 단계 평균 30초 목표 |
| R2 | GDPR 명시적 동의가 미준수 시 법적 리스크 | 중 | 고 | 외부 법률 검토 + LOCALE-001 legal_flags로 조건부 분기 + audit 로그 |
| R3 | LLM provider 스킵 후 사용자가 영영 미설정 | 중 | 중 | 메인 UI 상단 persistent banner + Preferences 진입 단축 |
| R4 | API key validation이 provider별로 다름 | 중 | 중 | regex + "Test connection" 버튼 (실제 API 호출 테스트, 1 req 소비) |
| R5 | 중도 이탈 시 draft 파일 손상 | 낮 | 중 | atomic write (`rename` 기반) + JSON schema 검증 |
| R6 | 애니메이션이 저사양 PC에서 끊김 | 중 | 낮 | `prefers-reduced-motion` 존중 + 애니메이션 스킵 버튼 |
| R7 | 이름 입력에 이모지/특수문자 | 중 | 낮 | Unicode Name 규격 허용, XSS만 차단 |
| R8 | RTL 언어(ar)에서 Step 진행 방향 혼란 | 중 | 중 | Back/Next 버튼의 시각적 방향을 RTL 자동 대응 (logical properties) |
| R9 | Mobile 페어링 제안이 사용자에게 부담 | 중 | 낮 | Optional, "나중에" 버튼 큰 표시 |
| R10 | 온보딩 반복 버그 (완료 후에도 다시 실행) | 낮 | 고 | `onboarding_completed: true` + timestamp 검증, Force-reset은 Preferences에서만 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/branding.md` §5.1 First Day with GOOSE (Hatching)
- `.moai/project/branding.md` §3 다국어 페르소나
- `.moai/project/adaptation.md` §2.1 명시적 Persona
- `.moai/project/adaptation.md` §11 일상 리추얼 커스터마이징
- `.moai/specs/SPEC-GOOSE-DESKTOP-001/spec.md` — 호스트
- `.moai/specs/SPEC-GOOSE-LOCALE-001/spec.md` — Step 2 데이터 소스
- `.moai/specs/SPEC-GOOSE-I18N-001/spec.md` — UI 언어
- `.moai/specs/SPEC-GOOSE-REGION-SKILLS-001/spec.md` — 완료 시 활성화
- `.moai/specs/SPEC-GOOSE-CONFIG-001/spec.md` — 저장소

### 9.2 외부 참조

- GDPR Article 6, 7 (동의 요건)
- CCPA §1798.120 (do-not-sell)
- PIPA 제15조 (수집 동의)
- WCAG 2.1 Level AA (접근성)
- Material Design: Onboarding patterns
- Apple HIG: First run experiences
- `tauri-plugin-keyring` documentation

### 9.3 부속 문서

- `./research.md` — 온보딩 UX 경쟁사 분석, 5분 목표 검증, GDPR/PIPA/PIPL 문구 매핑, 애니메이션 비용 측정
- `../SPEC-GOOSE-LOCALE-001/spec.md`
- `../SPEC-GOOSE-I18N-001/spec.md`
- `../SPEC-GOOSE-REGION-SKILLS-001/spec.md`
- `../SPEC-GOOSE-DESKTOP-001/spec.md`

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **LLM OAuth 플로우 상세를 구현하지 않는다**(CREDPOOL-001 전담).
- 본 SPEC은 **Identity Graph 완전 구축을 수행하지 않는다**. seed 노드만(IDENTITY-001이 확장).
- 본 SPEC은 **LoRA 훈련을 실행하지 않는다**(LORA-001). 동의 flag만 저장.
- 본 SPEC은 **Mobile 페어링 프로토콜을 구현하지 않는다**(BRIDGE-001/MOBILE-001). QR 호출 진입점만.
- 본 SPEC은 **법률 문구 전문을 작성하지 않는다**. `docs/privacy-policy.md` 링크만. 실제 문구는 외부 법률 검토 후 주입.
- 본 SPEC은 **재온보딩 플로우를 구현하지 않는다**. v1.0+ Preferences 메뉴에서 별도.
- 본 SPEC은 **A/B 테스트를 수행하지 않는다**.
- 본 SPEC은 **민감정보(SSN, biometric, medical, government ID)를 수집하지 않는다**(REQ-OB-016).
- 본 SPEC은 **사용자 입력을 외부 서버로 전송하지 않는다**(REQ-OB-014 + Step 7/8 opt-in 예외).
- 본 SPEC은 **온보딩 완료 후 데이터를 자동 수정하지 않는다**. 수정은 Preferences 별도 화면.
- 본 SPEC은 **CLI 환경에서 온보딩을 강제하지 않는다**. CLI-001은 환경변수 + config.yaml 직접 편집 경로 유지.

---

**End of SPEC-GOOSE-ONBOARDING-001**
