## SPEC-MINK-ONBOARDING-001 Progress

- **Status**: 🟡 draft — amendment-v0.3 본문 병합 완료, implementation 미진입
- **Last update**: 2026-05-15
- **Spec version**: v0.3.1 (amendment-v0.3 본문 병합 후)
- **Doc commits**: SPEC 생성 (`a2b3551`) → MINK rebrand (`81d9fa4`) → amendment-v0.3 본문 병합 (본 PR)
- **Implementation commits**: 0

## 문서 진척 (Phase 0)

| 단계 | 상태 | 비고 |
|------|------|------|
| SPEC 초안 (v0.1.0) | 🟢 완료 | `a2b3551` (선행 SPEC-GOOSE-ONBOARDING-001) |
| v0.2 Amendment (Desktop Tauri → CLI + Web UI 5-Step) | 🟢 완료 | 선행 SPEC v0.2.0 |
| MINK rebrand (v0.3.0) | 🟢 완료 | `81d9fa4` (PR #180) |
| amendment-v0.3 본문 병합 (v0.3.1) | 🟢 완료 | 본 PR — 5-Step → 7-Step 확장, REQ-OB-021~027 + AC-OB-021~027 신설, CROSSPLAT-001 의존성 추가 |

## Implementation 잔여 (분할 PR 권장)

| Sub-scope | 예상 규모 | 비고 |
|-----------|---------|------|
| Backend state machine (`internal/onboarding`) | 200-300 LOC + unit test | OnboardingData 7-step 타입 + step transitions + persist to `./.mink/`. CLI / Web UI 가 공유. |
| CLI `mink init` 7-step TUI (`cmd/mink/cmd/init.go`) | 600-800 LOC | charmbracelet/huh 의존 추가 + 7-step TUI + `--resume` + tty echo off + OS keyring 통합 |
| Web UI 7-step Wizard (`web/install/`) | 800-1200 LOC | React + shadcn/ui + `/install` route + progress bar + fetch → Go server → OS keyring |
| Model Setup 통합 (Step 2 — CROSSPLAT-001 의존) | 100-150 LOC | install.sh 가 이미 처리한 경우 detected 통과 + 미처리 시 `ollama pull` 호출 |
| CLI Tools 감지 (Step 3 — CROSSPLAT-001 의존) | 50-100 LOC | claude/gemini/codex 감지 결과를 onboarding 이 읽기 |
| LOCALE/I18N/REGION-SKILLS 초기화 호출 (Step 1) | 100-150 LOC | LOCALE-001 / I18N-001 / REGION-SKILLS-001 의존 |
| Privacy & Consent (Step 7) | 50-100 LOC | GDPR/PIPA/CCPA/LGPD/PIPL/FZ-152 country flags + consent.yaml 기록 |

## 의존성 상태

- ✅ SPEC-MINK-CROSSPLAT-001 (v0.2.0+§5.1) — **완료 (2026-05-15)**. install.sh 가 Ollama/모델/CLI 도구 감지 결과를 `./.mink/config.yaml` 에 기록.
- ⏸️ SPEC-MINK-CONFIG-001 — 선행 (작성 필요 또는 검토)
- ⏸️ SPEC-MINK-LOCALE-001 / I18N-001 / REGION-SKILLS-001 — 선행 (LOCALE/I18N 은 draft, REGION-SKILLS 는 draft)
- ⏸️ SPEC-GOOSE-LLM-001 (동시) — provider key 관리

## 운영 노트

본 SPEC 은 대형 (43 KB → 47 KB after amendment merge). implementation 진입 시 분할 PR 전략 권장. 후속 세션에서 위 7개 sub-scope 중 우선순위 결정 (예: Backend state machine 먼저 → CLI → Web UI 순). amendment-v0.3 의 §10.3 step number semantic 변경 (Step 2/3/4/5 → Step 4/5/6/7) 은 본 병합에서 일부 AC 만 갱신됨 — implementation 시점에 잔여 AC 의 step 참조 정합성 재검토 필요.

---
Last Updated: 2026-05-15 (amendment-v0.3 본문 병합 — v0.3.1)
