---
id: SPEC-MINK-ONBOARDING-001
version: 0.3.0
amendment_of: v0.2.0
status: draft
created_at: 2026-04-29
author: manager-spec
priority: critical
---

# Amendment v0.3 — Model Download + CLI Detection Steps

> **본 문서는 SPEC-MINK-ONBOARDING-001 v0.2.0에 대한 Amendment이다.**
> v0.2.0 본문은 그대로 유지되며, 본 Amendment는 v0.3.0으로 승격 시 본문에 병합된다.
> v0.3 Amendment의 핵심 변경: 5-Step → **7-Step** 확장 (Model Setup + CLI Tools Detection 추가).

---

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.3.0 | 2026-04-29 | Amendment v0.3 작성. SPEC-GOOSE-CROSSPLAT-001과의 통합을 위해 Model Setup(Step 2)과 CLI Tools Detection(Step 3)을 추가하여 5-Step → 7-Step으로 확장. 기존 Step 2~5는 Step 4~7로 이동. 번호 재배치 없이 REQ-OB-021~027을 추가. | manager-spec |

---

## 1. Amendment 개요

### 1.1 변경 배경

SPEC-GOOSE-CROSSPLAT-001이 CLI 설치 스크립트 수준에서 Ollama 자동 설치 + 모델 다운로드 + CLI 도구 감지를 담당한다. 그러나 설치 스크립트는 "자동 감지 + 자동 설치"에 집중하고, **사용자가 선택을 조정하거나 설정을 세밀하게 제어**할 수 있는 UI는 온보딩 마법사가 담당해야 한다.

따라서:

- **Model Setup 단계**: 설치 스크립트가 이미 Ollama를 설치하고 모델을 다운로드했다면 감지만 수행. 설치 스크립트를 거치지 않은 경우(예: 소스 빌드, 수동 설치) 온보딩에서 Ollama 설치 안내 + 모델 선택 제공.
- **CLI Tools Detection 단계**: 설치 스크립트가 이미 감지했다면 결과 표시 + 편집 가능. 그렇지 않다면 온보딩에서 감지 수행.

### 1.2 변경 요약

| 항목 | v0.2.0 | v0.3.0 (본 Amendment) |
|------|--------|----------------------|
| 총 단계 수 | 5 | **7** |
| Step 1 | Welcome + Locale | Welcome + Locale (변경 없음) |
| Step 2 | Persona | **Model Setup (NEW)** |
| Step 3 | Provider | **CLI Tools Detection (NEW)** |
| Step 4 | Messenger Channel | Persona (기존 Step 2) |
| Step 5 | Privacy & Consent | Provider (기존 Step 3) |
| Step 6 | — | Messenger Channel (기존 Step 4) |
| Step 7 | — | Privacy & Consent (기존 Step 5) |

---

## 2. 신규 요구사항

### 2.1 EARS 요구사항

**REQ-OB-021 [Event-Driven]** — **When** the user reaches the "Model Setup" step (new Step 2), the onboarding flow **shall** detect whether Ollama is installed and whether a MINK model is available, and:
- (a) **If** Ollama is installed and a model is available, display the detected model name and size, and allow the user to proceed or change the model. "Already available" **shall** be determined by querying `ollama list` (or `GET /api/tags`) and checking for any model whose name starts with `ai-mink/`.
- (b) **If** Ollama is installed but no model is available, detect system RAM (per REQ-CP-010 logic), recommend the appropriate model (per REQ-CP-011 logic), and execute `ollama pull {model}` with progress display.
- (c) **If** Ollama is not installed, display OS-specific installation instructions and a "Re-check" button (CLI: `r` key) that re-runs the Ollama detection after the user completes manual installation.

**REQ-OB-022 [Ubiquitous]** — The Model Setup step **shall** display model download progress with an estimated remaining time derived from download speed and remaining bytes, using Ollama's stdout parsing for real-time progress.

**REQ-OB-023 [Event-Driven]** — **When** the user reaches the "CLI Tools" step (new Step 3), the onboarding flow **shall** scan the system PATH for `claude`, `gemini`, and `codex` CLI tools, display detected tools with their version numbers, and configure delegation routing rules in `~/.mink/config.yaml`.

**REQ-OB-024 [Ubiquitous]** — The onboarding flow **shall** be a 7-step sequence: Step 1 (Welcome + Locale) → Step 2 (Model Setup) → Step 3 (CLI Tools) → Step 4 (Persona) → Step 5 (Provider) → Step 6 (Messenger Channel) → Step 7 (Privacy & Consent).

**REQ-OB-025 [Optional]** — **Where** the user wishes to skip Model Setup or CLI Tools Detection steps, the onboarding flow **shall** accept a Skip action and apply auto-detected defaults:
- Model Setup Skip: Use the model recommended by RAM detection, or "no model" if Ollama is unavailable.
- CLI Tools Skip: Use the tools already recorded by the install script, or empty list if not recorded.

**REQ-OB-026 [Event-Driven]** — **When** the Model Setup step detects that Ollama is not installed, the step **shall** guide installation per OS:
- macOS: Display "Run in terminal: `brew install ollama`" or provide direct download link.
- Linux: Display "Run in terminal: `curl -fsSL https://ollama.com/install.sh | sh`".
- Windows: Display "Download from https://ollama.com/download or run: `winget install Ollama.Ollama`".

**REQ-OB-027 [Event-Driven]** — **When** the onboarding completes (Step 7 Submit), the backend **shall** auto-generate `~/.mink/config.yaml` incorporating all collected settings including: model selection (from Step 2), CLI tool availability (from Step 3), persona (from Step 4), provider configuration (from Step 5), messenger channel (from Step 6), and consent flags (from Step 7).

---

## 3. 갱신된 플로우 다이어그램

```
Step 1: Welcome + Locale                    (EXISTING, unchanged)
  ├── Detect country/language/timezone via LOCALE-001
  ├── User override if desired
  └── Apply locale + reload I18N bundles

Step 2: Model Setup                         (NEW — REQ-OB-021, 022, 025, 026)
  ├── Detect Ollama installation
  │   ├── Ollama + model available → display model info, allow change
  │   ├── Ollama installed, no model → detect RAM → recommend model → download
  │   └── Ollama not installed → OS-specific install guide + re-check
  ├── Model download progress (with estimated time)
  └── Skip → auto-detected defaults (REQ-OB-025)

Step 3: CLI Tools Detection                 (NEW — REQ-OB-023, 025)
  ├── Scan PATH for claude, gemini, codex
  ├── Display detected tools + versions
  ├── Configure delegation routing rules
  └── Skip → use install script results or empty

Step 4: Persona                             (EXISTING — was Step 2)
  ├── Name (required), honorific level, pronouns
  ├── soul.md body (template prefill)
  └── Skip → default persona

Step 5: Provider                            (EXISTING — was Step 3)
  ├── LLM provider selection
  ├── API key / OAuth (OS keyring)
  └── Skip → "unset" (local mode if Ollama available)

Step 6: Messenger Channel                   (EXISTING — was Step 4)
  ├── First channel selection
  └── Skip → local_terminal

Step 7: Privacy & Consent                   (EXISTING — was Step 5)
  ├── Consent checkboxes
  ├── GDPR explicit consent (if EU)
  └── Required (cannot fully skip in GDPR regions)
```

---

## 4. 갱신된 공통 수집 대상 (7-Step)

| Step | 데이터 | 소비 SPEC |
|------|-------|----------|
| 1. Welcome + Locale | country/language/timezone | LOCALE-001, I18N-001, REGION-SKILLS-001 |
| 2. Model Setup | ollama_status, selected_model, model_size | LLM-001, CROSSPLAT-001 |
| 3. CLI Tools | detected_tools[], delegation_rules | ROUTER-001, SUBAGENT-001 |
| 4. Persona | name, honorific_level, soul.md | IDENTITY-001, ADAPTER-001 |
| 5. Provider | llm_provider, api key (OS keyring) | CREDPOOL-001, ROUTER-001 |
| 6. Messenger Channel | first channel | MESSENGER-*, BRIDGE-001 |
| 7. Privacy & Consent | consent flags | MEMORY-001, LORA-001 |

---

## 5. 갱신된 OnboardingData 타입 (Go)

```go
// Amendment: OnboardingData 확장
// v0.2.0에 Step 1~5가 있었으나, v0.3.0에서 Step 2,3이 추가되어 7-Step이 됨

type OnboardingData struct {
    Locale    LocaleChoice       // Step 1
    Model     ModelSetup         // Step 2 (NEW)
    CLITools  CLIToolsDetection  // Step 3 (NEW)
    Persona   PersonaProfile     // Step 4 (was Step 2)
    Provider  ProviderChoice     // Step 5 (was Step 3)
    Messenger MessengerChannel   // Step 6 (was Step 4)
    Consent   ConsentFlags       // Step 7 (was Step 5)
}

// NEW: Model Setup data
type ModelSetup struct {
    OllamaInstalled bool   // detected
    DetectedModel   string // e.g., "ai-mink/gemma4-e4b-rl-v1:q5_k_m"
    SelectedModel   string // user choice (defaults to detected)
    ModelSizeBytes  int64  // estimated download size
    RAMBytes        int64  // detected system RAM
}

// NEW: CLI Tools Detection data
type CLIToolsDetection struct {
    DetectedTools []CLITool // detected from PATH
}

type CLITool struct {
    Name    string // "claude" | "gemini" | "codex"
    Version string // parsed from --version output
    Path    string // full path to binary
}
```

---

## 6. 갱신된 패키지 레이아웃

기존 v0.2.0 패키지 레이아웃에 다음 파일이 추가됨:

```
internal/onboarding/
├── ... (기존 파일 유지)
├── model_setup.go          # Step 2: Ollama detection + model selection (NEW)
├── model_setup_test.go     # Step 2 tests (NEW)
├── cli_detection.go        # Step 3: CLI tool scanning (NEW)
└── cli_detection_test.go   # Step 3 tests (NEW)

web/install/src/steps/
├── ... (기존 파일 유지, 번호 재지정)
├── Step2ModelSetup.tsx     # NEW
├── Step3CLITools.tsx       # NEW
├── Step4Persona.tsx        # RENAMED from Step2Persona.tsx
├── Step5Provider.tsx       # RENAMED from Step3Provider.tsx
├── Step6Messenger.tsx      # RENAMED from Step4Messenger.tsx
└── Step7Privacy.tsx        # RENAMED from Step5Privacy.tsx
```

---

## 7. 신규 수용 조건 (Test Scenarios)

### AC-OB-021 — Model Setup: Ollama + 모델 이미 설치됨 (verifies REQ-OB-021)

- **Given** Ollama 설치됨, `ai-mink/gemma4-e4b-rl-v1:q5_k_m` 모델 존재
- **When** Step 2 (Model Setup) 표시
- **Then** "Ollama: installed" + "Model: gemma4-e4b-rl-v1 Q5_K_M (~4 GB)" 표시, "Use this model" / "Change model" 선택지 제공

### AC-OB-022 — Model Setup: 모델 다운로드 진행률 (verifies REQ-OB-022)

- **Given** Ollama 설치됨, 모델 미설치, RAM 16 GB
- **When** Step 2에서 추천 모델 "gemma4-e4b-rl-v1 Q5_K_M" 다운로드 시작
- **Then** 진행률 바(CLI: 텍스트 percentage / Web UI: progress bar) + "Downloading... 45% (~1.8 GB / ~4 GB), ETA ~2 min" 표시
- **Then** 다운로드 완료 후 "Model ready: gemma4-e4b-rl-v1 Q5_K_M (~4 GB)" 메시지 표시, "Next" 버튼 활성화

### AC-OB-023 — Model Setup: Ollama 미설치 안내 (verifies REQ-OB-026)

- **Given** macOS, Ollama 미설치
- **When** Step 2 표시
- **Then** "Ollama is required for local AI. Install it:" + "brew install ollama" 명령어 표시 + "Re-check" 버튼. Re-check 후 Ollama 감지 시 모델 선택 단계로 전환

### AC-OB-024 — CLI Tools: 부분 감지 (verifies REQ-OB-023)

- **Given** PATH에 `claude` (v1.2.3) + `codex` (v0.5.0) 존재, `gemini` 부재
- **When** Step 3 (CLI Tools) 표시
- **Then** "claude v1.2.3 (detected)" + "codex v0.5.0 (detected)" + "gemini (not found)" 표시, 위임 규칙 기본값 제안 (사용자 편집 가능)

### AC-OB-025 — 7-Step 진행 표시 (verifies REQ-OB-024)

- **Given** 온보딩 진행 중
- **When** Step 4 (Persona) 도착
- **Then** CLI는 "[4/7]" 텍스트 표시, Web UI는 progress bar 57% 위치 (round(4/7 * 100) = 57%, 반올림)

### AC-OB-026 — Model Setup Skip (verifies REQ-OB-025)

- **Given** Step 2 (Model Setup)
- **When** Skip 실행
- **Then** RAM 감지 결과 기반 추천 모델 자동 선택 (또는 Ollama 미설치 시 "no model") + Step 3으로 진행

### AC-OB-027 — 완료 시 config.yaml 통합 생성 (verifies REQ-OB-027)

- **Given** 전체 7-Step 완료
- **When** Step 7 Submit
- **Then** `~/.mink/config.yaml`에 다음 항목 모두 포함:
  ```yaml
  model:
    selected: "ai-mink/gemma4-e4b-rl-v1:q5_k_m"
    provider: "ollama"
  delegation:
    available_tools:
      - name: claude
        version: "1.2.3"
      - name: codex
        version: "0.5.0"
  persona:
    name: "User"
    honorific_level: "formal"
  providers:
    anthropic:
      api_key_source: keyring
  messenger:
    type: local_terminal
  consent:
    conversation_storage_local: true
    lora_training: false
    telemetry: false
    crash_reporting: false
  ```

---

## 8. 갱신된 의존성

| 타입 | 대상 | 설명 | v0.2.0 대비 변경 |
|-----|------|------|----------------|
| 신규 | **SPEC-GOOSE-CROSSPLAT-001** | 설치 스크립트에서 감지한 모델/CLI 도구 결과를 온보딩이 읽음 | NEW |
| 유지 | SPEC-GOOSE-LOCALE-001 | Step 1 | 변경 없음 |
| 유지 | SPEC-GOOSE-I18N-001 | UI 언어 | 변경 없음 |
| 유지 | SPEC-GOOSE-CONFIG-001 | 최종 config 저장 | 변경 없음 |
| 유지 | SPEC-GOOSE-LLM-001 | Ollama 어댑터 | 변경 없음 |
| 유지 | SPEC-GOOSE-REGION-SKILLS-001 | 완료 시 활성화 | 변경 없음 |

---

## 9. CROSSPLAT-001과의 상호작용

### 9.1 설치 스크립트 → 온보딩 데이터 전달

설치 스크립트(CROSSPLAT-001)가 성공적으로 실행된 경우, 온보딩(ONBOARDING-001)은 그 결과를 활용한다:

| 설치 스크립트가 기록한 데이터 | 온보딩에서의 사용 |
|---------------------------|-----------------|
| `~/.mink/config.yaml`의 모델 정보 | Step 2에서 "detected"로 표시, 재다운로드 불필요 |
| `~/.mink/config.yaml`의 CLI 도구 | Step 3에서 "detected"로 표시, 재스캔 불필요 |
| 설치된 Ollama | Step 2에서 "installed" 확인 |

### 9.2 설치 스크립트를 거치지 않은 경우

소스 빌드, 수동 설치 등으로 설치 스크립트를 건너뛴 경우:

- Step 2: Ollama 감지부터 시작 (설치 스크립트 결과 없음)
- Step 3: CLI 도구 PATH 스캔부터 시작
- 모든 단계가 정상 동작 (fallback 동작)

---

## 10. 기존 REQ/AC에 대한 영향

### 10.1 번호 보존 원칙 유지

v0.2.0의 REQ-OB-001~019 및 AC-OB-001~020은 **번호와 내용을 그대로 보존**한다. 본 Amendment는 다음만 추가한다:

- REQ-OB-021~027 (신규 요구사항)
- AC-OB-021~027 (신규 수용 조건)

### 10.2 기존 REQ 수정 사항

| REQ | 변경 | 이유 |
|-----|------|------|
| REQ-OB-001 | "5 steps" → "7 steps" | 단계 확장 |
| REQ-OB-002 | Back/Skip/Next 동작은 동일, Step 1에서 Back 비활성은 유지 | 변경 없음 |
| REQ-OB-005 | "start the onboarding flow at Step 1" 유지 | 변경 없음 |
| REQ-OB-009 | "(a) persist all collected data" → 7개 Step 데이터 포함 | 데이터 확장 |
| REQ-OB-011 | draft 파일에 7개 Step 데이터 포함 | 데이터 확장 |

### 10.3 기존 AC 수정 사항

| AC | 변경 | 이유 |
|----|------|------|
| AC-OB-001 | 변경 없음 | Step 1은 동일 |
| AC-OB-002 | "[2/5]" → "[2/7]", progress bar 비율 변경 | 단계 확장 |
| AC-OB-016 | "≤ 3분" → "≤ 4분" (Web UI), "≤ 2분" → "≤ 3분" (CLI) | 단계 추가로 시간 증가 |

---

## 11. 리스크 추가

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|--------|-------|------|------|
| R13 | 7-Step이 5-Step보다 이탈률 증가 | 중 | 중 | Step 2,3은 Skip 가능 + 설치 스크립트가 이미 처리한 경우 "detected, press Next" 즉시 통과 |
| R14 | 모델 다운로드 중 온보딩 블로킹 | 중 | 중 | 비동기 다운로드 + "다운로드 중 다음 단계로" 옵션 (또는 백그라운드 다운로드) |
| R15 | Ollama 설치 안내가 비개발자에게 불친절 | 중 | 중 | "1-click install" 링크 제공 + 설치 완료 후 자동 re-check |

---

## 12. v0.3.0 본문 병합 시 체크리스트

본 Amendment가 승인되어 v0.3.0으로 본문에 병합될 때 수행할 작업:

- [ ] §1.1 공통 수집 대상 표: 5행 → 7행 확장
- [ ] §1.2 두 경로 대비: 진행 표시 "[2/5]" → "[2/7]"
- [ ] §3.1 IN SCOPE: Step 2 (Model Setup) + Step 3 (CLI Tools) 항목 추가
- [ ] §4 EARS 요구사항: REQ-OB-021~027 추가
- [ ] §5 Test Scenarios: AC-OB-021~027 추가, 기존 AC 진행 표시 수정
- [ ] §6.2 핵심 타입: OnboardingData에 Model, CLITools 필드 추가
- [ ] §6.1 패키지 레이아웃: model_setup.go, cli_detection.go 추가
- [ ] §7 의존성: CROSSPLAT-001 추가
- [ ] §8 리스크: R13~R15 추가
- [ ] Exclusions: 변동 없음 (Model Setup, CLI Detection은 이제 IN SCOPE)
- [ ] 목표 소요 시간: "3분 이하" → "4분 이하" (Web UI), "2분 이하" → "3분 이하" (CLI)

---

**End of SPEC-MINK-ONBOARDING-001 Amendment v0.3.0**
