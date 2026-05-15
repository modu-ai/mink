## SPEC-MINK-CROSSPLAT-001 Progress

- **Status**: 🟡 PARTIAL — M1 + M2 + M4 + M5 + M6 (sh) 완료, M1.A + M3 = SUPERSEDED by amendment-v0.2, M7 잔여
- **Last update**: 2026-05-15
- **Milestones completed**: M1 (PR #189), M2 (PR #194), M4+M5 (PR #195), M6 install.sh (M2 PR 에 포함)
- **Amendment applied**: amendment-v0.2 (curl-single + WSL-only, 2026-05-15)

## 마일스톤 진척

| M | 제목 | 상태 | 비고 |
|---|------|------|------|
| M1 | goreleaser + release workflow | 🟢 완료 | PR #189 (`260e032`), 6플랫폼 cross-compile + checksums + SBOM |
| M1.A | Homebrew tap + winget + nfpms + AUR + scoop | ⏸️ SUPERSEDED (amendment-v0.2) | 2026-05-15 amendment-v0.2 §4.1 에 따라 전면 OUT scope. curl-single 진입 정책. 0.1.0 이후 재도입 여부는 별도 결정 |
| M2 | Unix shell installer (install.sh) | 🟢 완료 | PR #194 (`8f35aec`). POSIX-compliant, 17 bats 테스트, CI workflow 포함 |
| M3 | PowerShell installer (install.ps1) | ⏸️ SUPERSEDED (amendment-v0.2) | 2026-05-15 amendment-v0.2 §4.1 에 따라 전면 OUT scope. Windows = WSL2 only 정책. 0.1.0 이후 재도입 여부는 별도 결정 |
| M4 | Ollama 자동 설치 + 서비스 시작 | 🟢 완료 (Unix/WSL2) | PR #195 (`948fdfc`). install.sh 측 완료. install.ps1 측은 amendment-v0.2 로 OUT scope |
| M5 | 모델 자동 선택 + 다운로드 | 🟢 완료 (Unix/WSL2) | PR #195 (`948fdfc`). RAM 감지 + ollama pull + verify. install.ps1 측은 amendment-v0.2 로 OUT scope |
| M6 | CLI 도구 감지 + 설정 기록 | 🟢 완료 (install.sh 측) | M2 PR #194 에 포함 (claude/gemini/codex 감지 + ~/.mink/config.yaml 기록). install.ps1 측은 amendment-v0.2 로 OUT scope |
| M7 | 통합 테스트 + 문서 (curl + WSL2) | ⏸️ 잔여 | install-test.yml 의 unit 매트릭스는 M2 에 포함. WSL2 매트릭스 + end-to-end (실 GitHub Release 다운로드) 는 v0.1.0 태그 이후. amendment-v0.2 §4.2 로 축소 |

## REQ/AC 충족 현황

### M4+M5 PR 으로 GREEN (Unix 부분)

- REQ-CP-006: Ollama 설치 여부 감지 (`detect_ollama` — `command -v ollama` + `ollama --version`)
- REQ-CP-007: macOS Ollama 자동 설치 (Homebrew → `brew install ollama`, 미설치 시 graceful)
- REQ-CP-008: Linux Ollama 자동 설치 (`curl -fsSL https://ollama.com/install.sh | sh`, 3회 재시도)
- REQ-CP-009: Ollama 서비스 시작 (`start_ollama_service` — macOS Ollama.app + ollama serve, Linux ollama serve &)
- REQ-CP-010: 서비스 응답 대기 (`wait_for_ollama` — 30초 타임아웃, 1초 간격)
- REQ-CP-011: RAM 기반 모델 자동 선택 (`select_model`: <8GB→e2b, 8-15GB→q4_k_m, 16-31GB→q5_k_m, 32+GB→q8_0)
- REQ-CP-012: 모델 다운로드 (`pull_model` — `ollama pull` 진행률 실시간 표시)
- REQ-CP-013: 모델 검증 (`verify_model` — `ollama list | grep` 기반)
- REQ-CP-014: RAM 감지 (`detect_ram_gb` — Linux `/proc/meminfo`, macOS `sysctl hw.memsize`)
- REQ-CP-026: Ollama/모델 설치 실패 graceful degradation (어떤 단계도 바이너리 설치 차단 안 함)

- AC-CP-004: Ollama 이미 설치된 환경에서 재설치 skip (idempotent — `detect_ollama` guard)
- AC-CP-005: Ollama 미설치 → 자동 설치 시도 + 실패 시 graceful warning
- AC-CP-006: Ollama 서비스 시작 후 30초 내 응답 → 모델 다운로드 진입
- AC-CP-007: RAM 16 GB 환경 → `ai-mink/gemma4-e4b-rl-v1:q5_k_m` 선택
- AC-CP-008: Ollama 서비스 30초 내 미응답 → warning + skip (exit 0 유지)
- AC-CP-015: `verify_model` 실패 시 warning + exit 0 유지 (바이너리 설치 성공)

### M2 PR #194 으로 GREEN (9 REQ + 5 AC)

- REQ-CP-001: Unix shell installation command (`curl -fsSL ... | sh`)
- REQ-CP-004: OS + CPU architecture 감지 (`uname -s` / `uname -m` 정규화)
- REQ-CP-005: GitHub Releases API → 올바른 binary 다운로드
- REQ-CP-020: CLI 도구 감지 (claude/gemini/codex, `command -v` 스캔)
- REQ-CP-021: `~/.mink/config.yaml` 의 `delegation.available_tools` 기록
- REQ-CP-022: CLI 도구 미설치 시 설치 차단 없음 (local mode)
- REQ-CP-023: HTTPS 전용 + SHA256 checksum 검증
- REQ-CP-024: 사용자 프로필만 수정 (`~/.bashrc` / `~/.zshrc` / `~/.profile`), `/etc/profile` 미수정
- REQ-CP-025: 미지원 플랫폼 명확한 에러 메시지 + exit code 1

- AC-CP-001: Unix 설치 스크립트 동작 (mocked GitHub API)
- AC-CP-009: 1 of 3 CLI 도구 감지 (claude 만 존재)
- AC-CP-010: 0 CLI 도구 (warning/error 없음, local mode)
- AC-CP-013: 미지원 플랫폼 거부 (FreeBSD 시뮬레이션, exit 1)
- AC-CP-016: 사용자 프로필만 수정 (`/etc/profile` mtime 변화 없음)

### M2 PR 산출물

- `scripts/install.sh` (444 LOC → M4+M5 추가 후 609 LOC, POSIX-compliant `#!/bin/sh`, dash/bash/zsh 호환)
- `scripts/install.bats` (236 LOC → M4+M5 추가 후 383 LOC, bats-core 32 테스트)
- `.github/workflows/install-test.yml` (48 LOC, ubuntu-latest + macos-latest)

### M1 PR #189 으로 산출 (구현 완료, E2E 검증 보류)

- REQ-CP-015, REQ-CP-016, REQ-CP-017: goreleaser 6플랫폼 cross-compile + `checksums.txt` (SHA256) + SBOM(SPDX) — `.goreleaser.yaml` + `.github/workflows/release.yml` 에 구현 완료. 실제 artifact 생성/업로드는 v0.1.0 tag push 시점부터, 최종 E2E 검증은 M7 통합 테스트에서.
- AC-CP-011 (goreleaser 6플랫폼 빌드), AC-CP-014 (`.deb`/`.rpm` 패키지 생성): 같은 시점에 검증.

### SUPERSEDED REQ/AC (amendment-v0.2 — 2026-05-15)

다음 REQ / AC 는 amendment-v0.2 §3 에 따라 SUPERSEDED 마킹되었다. spec.md 본문에서 항목은 유지하되 traceability 마커가 적용됨:

- REQ-CP-002 (PowerShell installation command) — SUPERSEDED
- REQ-CP-003 (winget install) — SUPERSEDED
- REQ-CP-018 (Homebrew tap) — SUPERSEDED
- REQ-CP-019 (Debian/RPM via nfpms) — SUPERSEDED
- AC-CP-003 (winget 설치) — SUPERSEDED
- AC-CP-012 (Homebrew install) — SUPERSEDED
- AC-CP-014 (.deb/.rpm 패키지 생성) — SUPERSEDED

AC-CP-002 는 WSL2 bash 시나리오로 재정의되었다.

### 잔여 REQ/AC (M7)

- M7 통합 테스트 + 문서 작성 (curl + WSL2 시나리오) — amendment-v0.2 §4.2 로 축소
- (선택) install.sh 의 non-WSL Windows 감지 (MINGW/CYGWIN/MSYS) + 친절한 거부 메시지 — amendment-v0.2 §5.1, 별도 PR 가능

## 운영 노트

본 SPEC은 milestone 별 분할 PR 전략으로 점진적 종결. amendment-v0.2 (2026-05-15) 적용으로 M1.A + M3 가 전면 OUT scope 전환되어 잔여 작업이 M7 (curl + WSL2 E2E + 문서) 로 축소되었다. paste-ready prompt 잔여 (M7 + 선택적 install.sh non-WSL 가드) 는 hand-off 메모리에 적재되어 후속 세션에서 진입 가능.

---
Last Updated: 2026-05-15 (amendment-v0.2 — curl-single + WSL-only 정책 적용. M1.A + M3 SUPERSEDED)
