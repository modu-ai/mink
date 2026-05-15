## SPEC-MINK-ONBOARDING-001 Progress

- **Status**: 🟡 draft — Phase 1A + 1B + 1C (paths + keyring) 완료, Phase 1D / 1E / 1F / 2 / 3 / 4 잔여
- **Last update**: 2026-05-15
- **Spec version**: v0.3.1 (amendment-v0.3 본문 병합 후)
- **Doc commits**: SPEC 생성 (`a2b3551`) → MINK rebrand (`81d9fa4`) → amendment-v0.3 본문 병합 (본 PR)
- **Implementation commits**: 4 (Phase 1A PR #200 / Phase 1B PR #201 / Phase 1C-paths PR #202 / Phase 1C-keyring PR #203)

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
| **Phase 1D**: Model Setup + CLI Tools (`model_setup.go` + `cli_detection.go`) | ⏸️ | 150-250 LOC | CROSSPLAT-001 의존 (완료) — install.sh 의 `~/.mink/config.yaml` 의 `delegation.available_tools` 읽기. Ollama detection, RAM 감지, ollama pull 호출. claude/gemini/codex `command -v` 스캔. |
| **Phase 1E**: Completion (`completion.go`) | ⏸️ | 100-200 LOC | LOCALE/I18N/REGION-SKILLS 초기화 호출 + UserProfile 빌드 + 후속 SPEC 초기화 + onboarding-completed 기록 |
| **Phase 2**: CLI `mink init` 7-step TUI | ⏸️ | 600-800 LOC | `cmd/mink/cmd/init.go` + `internal/cli/install/tui.go` — charmbracelet/huh 의존 추가 + 7-step TUI + `--resume` + tty echo off |
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
