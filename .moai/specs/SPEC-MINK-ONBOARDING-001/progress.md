## SPEC-MINK-ONBOARDING-001 Progress

- **Status**: 🟡 draft — Phase 1 backend 100% + Phase 2A CLI happy path 완료, Phase 2B (Skip/Back/Resume) / 3 (Web) / 4 (E2E) 잔여
- **Last update**: 2026-05-16
- **Spec version**: v0.3.2 (Phase 1C canonical path policy 명문화)
- **Doc commits**: SPEC 생성 (`a2b3551`) → MINK rebrand (`81d9fa4`) → amendment-v0.3 본문 병합 → v0.3.2 §6.0 canonical path policy
- **Implementation commits**: 8 (Phase 1A PR #200 / Phase 1B PR #201 / Phase 1C-paths PR #202 / Phase 1C-keyring PR #203 / Phase 1E PR #204 / Phase 1F PR #205 / Phase 1D PR #206 / Phase 2A PR #207)

## 문서 진척 (Phase 0)

| 단계 | 상태 | 비고 |
|------|------|------|
| SPEC 초안 (v0.1.0) | 🟢 완료 | `a2b3551` (선행 SPEC-GOOSE-ONBOARDING-001) |
| v0.2 Amendment (Desktop Tauri → CLI + Web UI 5-Step) | 🟢 완료 | 선행 SPEC v0.2.0 |
| MINK rebrand (v0.3.0) | 🟢 완료 | `81d9fa4` (PR #180) |
| amendment-v0.3 본문 병합 (v0.3.1) | 🟢 완료 | 본 PR — 5-Step → 7-Step 확장, REQ-OB-021~027 + AC-OB-021~027 신설, CROSSPLAT-001 의존성 추가 |

## Implementation 잔여 (분할 PR 권장)

| Sub-scope | 상태 | 예상 규모 | 비고 |
|-----------|------|---------|------|
| **Phase 1A**: Backend state machine 골격 (types + flow) | 🟢 완료 (PR #200) | 782 LOC (3 files, 12 tests) | `internal/onboarding/{types.go, flow.go, flow_test.go}` — OnboardingData 7-step 타입 + OnboardingFlow state machine (StartFlow/SubmitStep/SkipStep/Back/Complete) + 5 sentinel errors. File I/O / keyring 미포함. |
| **Phase 1B**: Validators (`validators.go`) | 🟢 완료 (PR #201) | 912 LOC (2 files, 46 sub-tests) | `internal/onboarding/{validators.go (338), validators_test.go (574)}`. 5 validator (Persona name / Provider API key / GDPR consent / Honorific level / Sensitive field whitelist) + 10 sentinel errors. flow.go 변경 없음 (wiring 은 후속 phase). |
| **Phase 1C-paths**: paths.go + progress.go (file I/O) + spec.md §6.0 Canonical Path Policy | 🟢 완료 (PR #202) | 989 LOC (4 files, 56 sub-tests + spec.md +43) | `internal/onboarding/{paths.go (152), progress.go (204), paths_test.go (168), progress_test.go (218)}`. GlobalConfigDir/ProjectConfigDir/DraftPath/SecurityEventsPath resolver + Draft struct + SaveDraft (atomic) + LogSecurityEvent (NDJSON). spec.md v0.3.1 → v0.3.2 + §6.0 신설 + AC-OB-007/012/018/027 정정 (~/.mink vs ./.mink canonical 결정). |
| **Phase 1C-keyring**: keyring.go + zalando/go-keyring | 🟢 완료 (PR #203) | 552 LOC (2 files, 15 sub-tests) | `internal/onboarding/{keyring.go (259), keyring_test.go (293)}`. KeyringClient interface + SystemKeyring (zalando wrap) + InMemoryKeyring (RWMutex map, concurrent-safe) + 3 high-level helpers (SetProviderAPIKey / GetProviderAPIKey / DeleteProviderAPIKey, prefix `mink.provider.{name}.api_key`, service `"mink"`). 5 sentinel errors + zalando ErrNotFound → ErrKeyNotFound 번역. 신규 의존성 zalando/go-keyring v0.2.8 (direct). |
| **Phase 1D**: Model Setup + CLI Tools (`model_setup.go` + `cli_detection.go`) | 🟢 완료 (PR #206) | 1334 LOC (4 files, 26 TestXxx) | `internal/onboarding/{model_setup.go (512) + model_setup_test.go (485) + cli_detection.go (94) + cli_detection_test.go (243)}`. DetectOllama (PATH+HTTP /api/tags probe) + DetectMINKModel (HTTP 우선, daemon-alive 시그널로 exec fallback 차단 — fix-forward 1회) + DetectRAM (runtime.GOOS 분기: /proc/meminfo, sysctl, wmic, override hook) + RecommendModel (REQ-CP-011 매핑 pure 함수) + PullModel (ollama pull + stdout 진행률 파싱: phase/layer/bytes/percent). DetectCLITools (claude/gemini/codex PATH probe + --version SemVer 추출) + ParseToolVersion (regex). 5+1 sentinel errors. exec.CommandContext + httpGet + execLookPath + detectRAMOverride + versionExecTimeout 5종 indirection 으로 hermetic. TestHelperProcess 패턴으로 fake binary. gopls stringsseq modernize 사전 적용. fix-forward 1회 (HTTP success+no-match 가 잘못 exec fallback 진입). |
| **Phase 1E**: Completion (`completion.go`) | 🟢 완료 (PR #204) | 1026 LOC (2 files, 19 Test 함수) | `internal/onboarding/{completion.go (377), completion_test.go (650)}`. WriteCompletionConfig (global half: model+delegation+providers / project half: persona+messenger+consent) + WriteOnboardingCompleted (RFC3339 idempotent marker) + CompletionOptions (DryRun + path overrides + Now 주입). install.sh merge 는 `map[string]any` round-trip 으로 unrelated key 보존. 5 sentinel errors + AuthMethodEnv → "env" / keyring 성공 → "keyring" / ErrKeyNotFound·nil-client → "none" 분기. ProviderUnset → providers 섹션 생략. 시크릿 disk 미기록 (defense-in-depth). 1차 push CLEAN. |
| **Phase 1F**: validators + keyring + completion wiring (flow.go 확장) | 🟢 완료 (PR #205) | 657 LOC (2 files, 20 신규 Test 함수, 12 Phase 1A 테스트 보존) | `internal/onboarding/{flow.go +151/-11, flow_test.go +517}`. functional options 패턴 (FlowOption / WithKeyring / WithCompletionOptions) 으로 StartFlow variadic 확장 (backward compat). ProviderStepInput{Choice, APIKey} wrapper — secret 이 OnboardingData 에 잔류 안 함. step 4 ValidatePersonaName + ValidateHonorificLevel (empty 허용), step 5 ValidateProviderAPIKey + AuthMethodAPIKey/Env/nil-keyring 3-way 분기로 SetProviderAPIKey 호출, step 7 ValidateGDPRConsent. SkipStep(7) GDPR 차단 (AC-OB-014). CompleteAndPersist() 신설 (Complete + WriteCompletionConfig + WriteOnboardingCompleted) + ErrPersistFailed / ErrMarkerFailed 신규 sentinel 2종. Complete() 기존 동작 무변경. 1차 push CLEAN (3번째 연속). |
| **Phase 2A**: CLI `mink init` 7-step TUI happy path | 🟢 완료 (PR #207) | 1063 LOC (3 신규 파일 + rootcmd +1, 10 TestXxx) | `internal/cli/commands/init.go (62)` + `internal/cli/install/tui.go (721)` + `tui_test.go (263)` + `rootcmd.go +1`. charmbracelet/huh v1.0.0 + 의존 추가. TTY 가드 + 7-step huh form (KR locale 하드코딩, Ollama 3-way 분기, MultiSelect tools, Persona Validate hook, EchoModePassword API key, Consent 4 confirm) + CompleteAndPersist + ErrWizardCancelled. 1회복 후 admin-bypass merge (CI insights timing regression 본 PR 무관 입증). Skip/Back/Resume/LOCALE wiring 은 Phase 2B 이월. |
| **Phase 2B**: Skip + Back + --resume + LOCALE wiring | ⏸️ | 250-400 LOC | flow.go 의 SkipStep + Back + Phase 1C-paths 의 draft 복구 활용. LOCALE-001 Detect() 통합. |
| **Phase 2C**: 통합 테스트 + polish | ⏸️ | 150-300 LOC | charmbracelet/x/exp/teatest 기반 전체 form 통합 테스트. 색상 테마 + ollama pull 진행률 UI. |
| **Phase 3**: Web UI 7-step Wizard | ⏸️ | 800-1200 LOC | `internal/server/install/` + `web/install/` — React 19 + shadcn/ui + Vite + `/install` route + progress bar + fetch → Go server |
| **Phase 4**: E2E (Playwright + CLI speedrun) | ⏸️ | 300-500 LOC | `e2e/install-wizard-speedrun.spec.ts` + `scripts/cli-install-speedrun.sh` + `.github/workflows/install-wizard-e2e.yml` — AC-OB-016 Web 4분 / CLI 3분 검증 |

## 의존성 상태

- ✅ SPEC-MINK-CROSSPLAT-001 (v0.2.0+§5.1) — **완료 (2026-05-15)**. install.sh 가 Ollama/모델/CLI 도구 감지 결과를 `./.mink/config.yaml` 에 기록.
- ⏸️ SPEC-MINK-CONFIG-001 — 선행 (작성 필요 또는 검토)
- ⏸️ SPEC-MINK-LOCALE-001 / I18N-001 / REGION-SKILLS-001 — 선행 (LOCALE/I18N 은 draft, REGION-SKILLS 는 draft)
- ⏸️ SPEC-GOOSE-LLM-001 (동시) — provider key 관리

## 운영 노트

본 SPEC 은 대형 (43 KB → 47 KB after amendment merge). implementation 진입 시 분할 PR 전략 권장. 후속 세션에서 위 7개 sub-scope 중 우선순위 결정 (예: Backend state machine 먼저 → CLI → Web UI 순). amendment-v0.3 의 §10.3 step number semantic 변경 (Step 2/3/4/5 → Step 4/5/6/7) 은 본 병합에서 일부 AC 만 갱신됨 — implementation 시점에 잔여 AC 의 step 참조 정합성 재검토 필요.

---
Last Updated: 2026-05-16 (Phase 1C-keyring KeyringClient + System/InMemory + 3 helpers — internal/onboarding/{keyring,keyring_test}.go 552 LOC, 15 sub-tests, 누적 129 tests. zalando/go-keyring v0.2.8 direct dep 추가)
