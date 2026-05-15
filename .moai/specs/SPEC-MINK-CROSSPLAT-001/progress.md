## SPEC-MINK-CROSSPLAT-001 Progress

- **Status**: 🟡 PARTIAL — M1 + M2 완료, M3/M4/M5/M7 미완
- **Last update**: 2026-05-15
- **Milestones completed**: M1 (PR #189), M2 (현재 PR)

## 마일스톤 진척

| M | 제목 | 상태 | 비고 |
|---|------|------|------|
| M1 | goreleaser + release workflow | 🟢 완료 | PR #189 (`260e032`), 6플랫폼 cross-compile + checksums + SBOM |
| M2 | Unix shell installer (install.sh) | 🟢 완료 | 현재 PR. POSIX-compliant, 17 bats 테스트, CI workflow 포함 |
| M3 | PowerShell installer (install.ps1) | ⏸️ 미착수 | M1 산출물 사용. winget 매니페스트 포함 |
| M4 | Ollama 자동 설치 + 서비스 시작 | ⏸️ 미착수 | install.sh + install.ps1 양쪽에 함수 추가 |
| M5 | 모델 자동 선택 + 다운로드 | ⏸️ 미착수 | M4 의존. RAM 감지 + ollama pull 진행률 |
| M6 | CLI 도구 감지 + 설정 기록 | 🟡 부분 완료 | install.sh 측 (claude/gemini/codex 감지 + ~/.mink/config.yaml 기록) 은 M2 PR 에 포함. install.ps1 측은 M3 와 함께 진행 |
| M7 | 통합 테스트 + 문서 | ⏸️ 미착수 | install-test.yml 의 unit 매트릭스는 M2 에 포함. end-to-end (실 GitHub Release 다운로드) 는 v0.1.0 태그 이후 |

## REQ/AC 충족 현황

### M2 PR 으로 GREEN (9 REQ + 5 AC)

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

- `scripts/install.sh` (410 LOC, POSIX-compliant `#!/bin/sh`, dash/bash/zsh 호환)
- `scripts/install.bats` (234 LOC, bats-core 17 테스트)
- `.github/workflows/install-test.yml` (48 LOC, ubuntu-latest + macos-latest)

### M1 PR #189 으로 산출 (구현 완료, E2E 검증 보류)

- REQ-CP-015, REQ-CP-016, REQ-CP-017: goreleaser 6플랫폼 cross-compile + `checksums.txt` (SHA256) + SBOM(SPDX) — `.goreleaser.yaml` + `.github/workflows/release.yml` 에 구현 완료. 실제 artifact 생성/업로드는 v0.1.0 tag push 시점부터, 최종 E2E 검증은 M7 통합 테스트에서.
- AC-CP-011 (goreleaser 6플랫폼 빌드), AC-CP-014 (`.deb`/`.rpm` 패키지 생성): 같은 시점에 검증.

### 잔여 REQ/AC (M3-M7)

- REQ-CP-002, REQ-CP-003: PowerShell + winget (M3)
- REQ-CP-006~009: Ollama 자동 설치 + 서비스 시작 (M4)
- REQ-CP-010~014: 모델 자동 선택 + 다운로드 + 재개 (M5)
- REQ-CP-018, REQ-CP-019: Homebrew tap, `.deb`/`.rpm` (M1.A — 외부 repo 작업 후)
- REQ-CP-026: Ollama 설치 실패 graceful degradation (M4)

## 운영 노트

본 SPEC은 milestone 별 분할 PR 전략으로 점진적 종결. paste-ready prompt 4종 (M1.A / M3 / M4+M5 / M7) 은 hand-off 메모리에 적재되어 후속 세션에서 진입 가능.

---
Last Updated: 2026-05-15 (M2 milestone PR)
