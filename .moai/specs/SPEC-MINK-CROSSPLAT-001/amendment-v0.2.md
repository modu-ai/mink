---
id: SPEC-MINK-CROSSPLAT-001
amendment: v0.2.0
status: proposed
supersedes_milestones: [M1.A, M3]
created_at: 2026-05-15
author: manager-spec
parent_spec: SPEC-MINK-CROSSPLAT-001
parent_version: 0.2.0
---

# Amendment v0.2 — curl-single + WSL-only Distribution Policy

> 본 amendment 는 SPEC-MINK-CROSSPLAT-001 의 배포 채널을 **curl 단일 진입점 + Windows = WSL2 only** 로 축소한다. M1.A (Homebrew tap + winget + nfpms + AUR + scoop) 와 M3 (install.ps1) 는 전면 OUT-of-scope 로 전환된다. 본 정책 변경은 hermes-agent 공식 권장 + 2026 WSL2 dev 표준 + 유지보수 ROI 에 근거한다.

---

## §1 Header

| 필드 | 값 |
|------|-----|
| Amendment ID | v0.2.0 |
| Parent SPEC | SPEC-MINK-CROSSPLAT-001 |
| Parent Version | 0.2.0 |
| Status | proposed |
| Created | 2026-05-15 |
| Author | manager-spec |
| Type | scope-reduction (subtractive amendment) |
| Supersedes Milestones | M1.A (Homebrew tap + winget + nfpms + AUR + scoop), M3 (install.ps1 + winget manifests) |
| Impact on merged PRs | 없음 (#189 M1, #194 M2, #195 M4+M5 모두 curl 경로 — 영향 없음) |
| Docs-only | YES (코드/테스트/스크립트 변경 없음) |

본 amendment 적용 시 SPEC-MINK-CROSSPLAT-001 의 배포 채널은 다음 두 가지로 축소된다:

1. **curl 단일 진입점** (Unix shell installer `install.sh` via `curl -fsSL https://mink.ai/install | sh`) — macOS / Linux / WSL2 공통.
2. **GitHub Release direct binary download** — `install.sh` 가 내부적으로 사용하며, 사용자가 수동 다운로드도 가능.

---

## §2 결정 근거 (Rationale)

### §2.1 hermes-agent 공식 권장 (Primary Authority)

NousResearch 의 hermes-agent (v0.13.0, 2026-05-07 릴리스) 는 동등한 로컬 LLM 컴패니언 카테고리의 선행 사례이다. 본 amendment 는 다음 두 가지 공식 입장에 기반한다:

> "We recommend `curl -fsSL https://hermes.nousresearch.ai/install.sh | bash` as the canonical installation path on macOS and Linux. This single entry point is battle-tested across our beta cohort." — hermes-agent Installation Guide v0.13.0

> "For Windows users, the most battle-tested setup today is to run the Linux/macOS one-liner inside WSL2. Native Windows support remains in early beta and is not recommended for production." — hermes-agent Platform Support Matrix v0.13.0

근거 인용 의의:
- 동급 카테고리의 reference distribution 이 curl-single 을 canonical 로 채택.
- Native Windows (PowerShell + irm/iex) 를 "early beta, not recommended" 로 명시적 격하.
- WSL2 가 Windows 환경의 권장 deployment surface.

### §2.2 2026 WSL2 Dev 표준 (Secondary Authority)

2026 년 현재 WSL2 는 Windows 환경의 사실상 표준 dev surface 이다:

- Microsoft 가 2026 년 WSL2 의 GPU passthrough / systemd / GUI app / network forwarding 등 성능·안정성 강화를 지속 진행.
- Major dev toolchains (Docker Desktop, VS Code Remote-WSL, ML/AI 컨테이너 워크플로우) 가 WSL2 를 1급 타겟으로 우선 지원.
- `wsl --install` 한 줄로 모든 Windows 11 이상 환경에 즉시 설치 가능 (Windows 10 21H2+ 도 지원).
- Microsoft Learn `https://learn.microsoft.com/en-us/windows/wsl/install` 가 공식 진입점.

### §2.3 MINK 의존성의 Linux-first 성격

MINK 의 핵심 의존성은 모두 Linux/Unix 환경을 기준으로 설계되어 있다:

- **Ollama**: Linux/macOS 가 1급, Windows 는 별도 빌드. `curl -fsSL https://ollama.com/install.sh | sh` 가 표준.
- **GNU tools** (`coreutils`, `grep`, `sed`, `awk`): WSL2 에서 native, Windows native 에서는 별도 설치 필요.
- **SHA256 검증** (`sha256sum`): WSL2 에서 native, PowerShell 에서는 `Get-FileHash` 로 별도 코드 경로 필요.
- **`curl`**: Windows 10+ 부터 내장이나 동작 차이 존재, WSL2 에서 표준 동작.

### §2.4 유지보수 ROI (Decisive Factor)

M1.A + M3 유지 시 예상 비용:

- **install.ps1 작성 + Pester 테스트 매트릭스**: Windows runner CI 비용 + PowerShell 7/5.1 양쪽 호환성 유지 + Windows-specific edge case (Defender SmartScreen, ExecutionPolicy, UAC) 대응. 추정 ~150k token 의 구현·유지보수 effort.
- **winget manifest pipeline**: `manifests/a/ai-mink/mink/` YAML 자동 생성·PR 제출. Microsoft winget repo 의 PR review 사이클 대기 추가.
- **Homebrew tap repo** (`ai-mink/homebrew-tap`): 별도 GitHub repo 유지, goreleaser brews 섹션의 PAT/SSH key 관리.
- **nfpms (.deb/.rpm)**: 패키지 매니페스트 + Debian/RPM 컨벤션 (post-install scripts, dependency 명시) 유지보수.
- **scoop bucket**: 추가 Windows 패키지 매니저 매니페스트.

curl-single + WSL-only 로 전환 시 제거 가능:
- install.ps1 + Pester (~150k effort).
- winget pipeline + Microsoft winget repo PR cycle.
- Homebrew tap repo + goreleaser brews 섹션.
- nfpms .deb/.rpm + Debian/RPM 컨벤션.
- scoop bucket.

남는 경로:
- `install.sh` (M2 PR #194 완료) — POSIX-compliant, bash/dash/zsh + WSL2 검증 완료.
- GitHub Release direct binary (M1 PR #189 완료).

ROI 판정: 유지보수 비용 대비 사용자 도달 범위 차이 미미 (WSL2 가 Windows dev 표준이므로 native Windows 경로의 marginal user 만 손실). curl-single 정책이 우월.

### §2.5 영향 받지 않는 머지본

다음 PR 의 산출물은 본 amendment 와 **충돌하지 않으며 그대로 유효**하다:

| PR | Milestone | 영향 |
|----|----------|------|
| #189 | M1 (goreleaser + release.yml) | 영향 없음. 6플랫폼 cross-compile + checksums + SBOM 산출은 curl-single 정책에서도 유효 (install.sh 가 GitHub Release 에서 binary 다운로드). |
| #194 | M2 (install.sh) | 영향 없음. POSIX-compliant install.sh + bats 테스트 + install-test.yml 모두 그대로 유효. |
| #195 | M4 + M5 (Ollama 자동 설치 + RAM 기반 모델 선택, Unix 측) | 영향 없음. install.sh 측 Ollama 자동 설치 + 모델 선택은 그대로 유효 (WSL2 에서도 동일 코드 경로). |

본 amendment 는 **순수 docs-only** 이며 산출 코드/테스트/스크립트 변경 없음.

---

## §3 영향 REQ / AC 목록

본 amendment 는 다음 REQ / AC 를 SUPERSEDE 또는 재정의한다. 실제 spec.md 의 REQ/AC 항목은 삭제하지 않고 `**[SUPERSEDED by amendment-v0.2 §3 — 2026-05-15]**` 마킹으로 traceability 를 보존한다.

### §3.1 SUPERSEDED (전면 폐지)

| 식별자 | 본문 요약 | SUPERSEDED 사유 |
|--------|----------|-----------------|
| REQ-CP-002 | PowerShell installation command (`irm ... \| iex`) for Windows native PowerShell 5.1+/7 | install.ps1 (M3) 전면 OUT scope. Windows = WSL2 only. |
| REQ-CP-003 | `winget install ai-mink.mink` Windows package manager 경로 | winget pipeline (M1.A) 전면 OUT scope. |
| REQ-CP-018 | Homebrew tap (`brew install ai-mink/tap/mink`) on macOS/Linux | Homebrew tap (M1.A) 전면 OUT scope. curl-single 진입. |
| REQ-CP-019 | Debian/RPM package generation via goreleaser nfpms | nfpms (M1.A) 전면 OUT scope. curl-single 진입. |
| AC-CP-003 | `winget install ai-mink.mink` 동작 검증 | REQ-CP-003 SUPERSEDED 에 종속. |
| AC-CP-012 | `brew install ai-mink/tap/mink` 동작 검증 | REQ-CP-018 SUPERSEDED 에 종속. |
| AC-CP-014 | `.deb` + `.rpm` 패키지 생성 검증 | REQ-CP-019 SUPERSEDED 에 종속. |

### §3.2 재정의 (Redefined)

| 식별자 | 기존 본문 | 재정의 본문 |
|--------|----------|-----------|
| AC-CP-002 | "Windows 11, PowerShell 5.1 환경에서 `irm ... \| iex` 실행 → install.ps1 동작" | "Windows 11 + WSL2 환경에서 `wsl bash -c \"curl -fsSL https://mink.ai/install \| sh\"` 실행 시 install.sh 가 정상 동작하며 `mink --version` 이 응답한다 (verifies REQ-CP-001 on WSL2 surface)." |

### §3.3 영향 없음 (그대로 유효)

다음 REQ / AC 는 본 amendment 영향 없이 그대로 유효하다:

- REQ-CP-001 (curl Unix shell installer) — **본 amendment 의 primary surface**.
- REQ-CP-004 (OS + ARCH 감지) — install.sh 가 WSL2 를 Linux 로 감지.
- REQ-CP-005 (GitHub Releases binary 다운로드) — 그대로 유효.
- REQ-CP-006~014 (Ollama 자동 설치 + 모델 선택 + 다운로드) — install.sh 측 모두 유효, WSL2 에서도 동일 코드 경로.
- REQ-CP-015~017 (goreleaser 6플랫폼 + checksums + SBOM) — 그대로 유효 (install.sh 가 소비하는 source).
- REQ-CP-020~022 (CLI 도구 감지) — 그대로 유효.
- REQ-CP-023~026 (Unwanted behavior 가드) — 그대로 유효.
- AC-CP-001, AC-CP-004~011, AC-CP-013, AC-CP-015~016 — 그대로 유효.

---

## §4 Milestone 변경

### §4.1 SUPERSEDED milestones

| Milestone | 기존 범위 | 본 amendment 후 |
|-----------|----------|----------------|
| **M1.A** (Homebrew tap + winget + nfpms + AUR + scoop) | goreleaser brews + winget pipeline + nfpms .deb/.rpm + AUR PKGBUILD + scoop bucket | **SUPERSEDED — 전면 OUT scope**. goreleaser 의 brews/scoop/nfpms/aurs 섹션 자체를 추가하지 않으며, `manifests/` 디렉토리 / `ai-mink/homebrew-tap` repo / Microsoft winget repo PR 도 작성하지 않는다. |
| **M3** (install.ps1 + winget manifests) | PowerShell 5.1/7 installer + `manifests/a/ai-mink/mink/` 매니페스트 템플릿 + Windows 전용 edge case 대응 | **SUPERSEDED — 전면 OUT scope**. `scripts/install.ps1` 작성하지 않으며 Pester 테스트 매트릭스 도입하지 않는다. |

### §4.2 축소 milestones

| Milestone | 기존 범위 | 본 amendment 후 |
|-----------|----------|----------------|
| **M6** (CLI 도구 감지 + 설정 기록) | install.sh + install.ps1 양쪽에서 CLI 감지 (`claude` / `gemini` / `codex`) | **install.sh 측만 유지**. install.ps1 측 (PowerShell 경로) 은 본 amendment 로 제거. install.sh 측은 M2 PR #194 에 이미 포함되어 사실상 완료. |
| **M7** (통합 테스트 + 문서) | macOS + Linux + Windows native 3개 매트릭스 + brew/winget/nfpms E2E | **curl + WSL2 시나리오로 축소**. CI 매트릭스 = ubuntu-latest + macos-latest + (선택적) Windows + WSL2 (`wsl-ubuntu` action). brew/winget/nfpms E2E 시나리오 제거. 문서는 README 의 install 섹션 + WSL2 안내. |

### §4.3 영향 없는 milestones

| Milestone | 상태 |
|-----------|------|
| M1 (goreleaser + release.yml, PR #189) | 그대로 유효. 6플랫폼 cross-compile + checksums + SBOM 유지. `goreleaser.yaml` 의 brews/scoop/nfpms/aurs 섹션이 **없음**을 확인 후 유지 (현재 머지본도 이 섹션 없음). |
| M2 (install.sh, PR #194) | 그대로 유효. POSIX install.sh + bats 32 테스트 + install-test.yml 유지. |
| M4 + M5 (Ollama 자동 설치 + 모델 선택, PR #195) | 그대로 유효. install.sh 측 모두 유효. |

### §4.4 잔여 milestone roadmap

본 amendment 적용 후 SPEC-MINK-CROSSPLAT-001 의 잔여 작업:

1. **M6 (install.sh 측 CLI 감지)**: 이미 M2 PR #194 에 포함되어 사실상 완료. 추가 작업 없음.
2. **M7 (curl + WSL2 E2E + 문서)**: 잔여. 별도 PR 에서 진행 가능.
3. (Optional) **install.sh non-WSL Windows 감지**: §5 구현 노트 참조. 별도 PR 가능.

---

## §5 구현 노트

본 amendment 는 docs-only 이지만 후속 PR 에서 다음 구현이 권장된다:

### §5.1 install.sh non-WSL Windows 감지 (선택, 별도 PR)

`install.sh` 가 native Windows 환경 (Git Bash / MinGW / Cygwin / MSYS2 등 WSL2 가 아닌 Windows 쉘) 에서 실행될 때 친절한 거부 메시지를 표시하는 것이 권장된다.

감지 방법 (POSIX `uname -a` 기반):

```sh
case "$(uname -a)" in
  *MINGW*|*CYGWIN*|*MSYS*)
    printf '%s\n' "MINK requires WSL2 on Windows."
    printf '%s\n' "Native Windows shells (Git Bash, MinGW, Cygwin, MSYS) are not supported."
    printf '%s\n' "Please install WSL2 first:"
    printf '%s\n' "  wsl --install"
    printf '%s\n' "See: https://learn.microsoft.com/en-us/windows/wsl/install"
    exit 1
    ;;
esac
```

검증 방법 (bats):

```bash
@test "install.sh on MINGW (Git Bash) prints WSL2 requirement and exits 1" {
  UNAME_MOCK_OUTPUT="MINGW64_NT-10.0 ..." run install.sh
  [ "$status" -eq 1 ]
  [[ "$output" == *"MINK requires WSL2 on Windows"* ]]
  [[ "$output" == *"wsl --install"* ]]
}
```

**Note**: 본 amendment 는 이 구현을 강제하지 않는다 (별도 PR 가능). install.sh 가 native Windows 환경에서 단순히 `Unsupported platform` 으로 거부해도 무방하다 — 그 경우 REQ-CP-025 의 기본 동작이 이미 적용된다.

### §5.2 WSL2 가이드 문서 (M7 와 함께)

README install 섹션에 다음 안내 추가:

```markdown
### Windows

MINK on Windows requires WSL2. After installing WSL2 (`wsl --install`),
open an Ubuntu shell and run the standard Unix installer:

```sh
wsl bash -c "curl -fsSL https://mink.ai/install | sh"
```

For WSL2 installation, see Microsoft's official guide:
https://learn.microsoft.com/en-us/windows/wsl/install
```

### §5.3 .goreleaser.yaml 현재 상태 (참고)

`.goreleaser.yaml` (M1 PR #189) 의 현재 머지본은 brews / scoop / nfpms / aurs 섹션을 **포함하지 않는다**. 따라서 본 amendment 의 §4.1 SUPERSEDED 정책과 자연스럽게 일치한다. 추가 삭제 작업 없음.

---

## §6 본 Amendment 의 OUT-of-scope

본 amendment 는 다음 항목을 **다루지 않는다**. 별도 결정 또는 별도 SPEC 으로 처리한다:

### §6.1 Ollama 모델 namespace `ai-mink/gemma4-*`

spec.md 의 REQ-CP-011 (RAM 기반 모델 선택 표), AC-CP-006 (RAM 16 GB → q5_k_m 검증), Acceptance Scenario 1/3/5 등에서 사용되는 Ollama 모델 namespace `ai-mink/gemma4-e2b-rl-v1`, `ai-mink/gemma4-e4b-rl-v1:{q4_k_m,q5_k_m,q8_0}` 의 명명 정책은 본 amendment 의 범위 밖이다.

- **본 amendment 의 범위**: GitHub org 명명 (`ai-mink/*` repo, GitHub Releases) 및 그것에 의존하는 배포 채널 (Homebrew/winget/nfpms).
- **본 amendment 가 다루지 않는 범위**: Ollama Hub account 명명 (`ai-mink` namespace on `ollama.com/library/`). GitHub org 명과 Ollama Hub account 명을 동일하게 유지할지 별도 namespace 로 분리할지는 SPEC-GOOSE-GEMMA4-001 (모델 배포 SPEC) 의 별도 결정 사항이다.

이에 따라 spec.md / acceptance.md 의 `ai-mink/gemma4-*` 참조는 **그대로 유지**되며, 본 amendment §6.1 의 TODO 코멘트로 traceability 만 추가한다.

### §6.2 0.1.0 이후 배포 채널 확장

본 amendment 는 0.1.0 이전 (현재) 시점의 배포 정책을 정의한다. 0.1.0 이후 시점에:

- Homebrew tap / winget / nfpms 재도입 여부
- macOS .pkg/.dmg 또는 Windows MSI 등 GUI installer 도입 여부
- Docker 이미지 배포 (SPEC-GOOSE-DOCKER-001)
- 자동 업데이트 (SPEC-GOOSE-AUTOUPDATE-001)

등은 0.1.0 release 시점에 재검토한다.

### §6.3 코드 서명 / 공증

macOS notarization, Windows Authenticode 코드 서명은 본 amendment 영향 밖이며 SPEC-MINK-CROSSPLAT-001 §3.2 OUT-of-scope 가 그대로 적용된다 (v0.2+ 검토).

---

## §7 변경 적용 절차

본 amendment 적용 후 즉시:

1. spec.md §3.1 (IN SCOPE) 에서 install.ps1 / winget / Homebrew tap / nfpms 항목 → §3.2 (OUT SCOPE) 로 이동.
2. spec.md §4 (Requirements) 의 REQ-CP-002, REQ-CP-003, REQ-CP-018, REQ-CP-019 각 항목 끝에 `**[SUPERSEDED by amendment-v0.2 §3 — 2026-05-15]**` 마킹.
3. spec.md §5 (Acceptance Criteria) 의 AC-CP-002 본문을 WSL2 시나리오로 재정의, AC-CP-003 / AC-CP-012 / AC-CP-014 끝에 SUPERSEDED 마킹.
4. spec.md HISTORY 표에 amendment-v0.2 항목 추가.
5. plan.md 의 M1.A 작업 + M3 섹션 삭제, SUPERSEDED 노트로 대체.
6. acceptance.md 의 winget / Homebrew / brew install / .deb-.rpm 시나리오 SUPERSEDED 마킹, WSL2 시나리오 추가.
7. progress.md milestone 상태 표 갱신 (M1.A / M3 → ⏸️ SUPERSEDED).

상세 변경 내역은 본 amendment 와 함께 단일 commit 으로 적용된다 (docs-only).

---

## §8 References

- hermes-agent Installation Guide v0.13.0 (2026-05-07): canonical `curl ... | bash` recommendation.
- hermes-agent Platform Support Matrix v0.13.0: "Native Windows = early beta, WSL2 recommended" 입장.
- Microsoft WSL2 Install Guide: https://learn.microsoft.com/en-us/windows/wsl/install
- SPEC-MINK-CROSSPLAT-001 v0.2.0 spec.md / plan.md / acceptance.md / progress.md (rebrand 본).
- SPEC-MINK-BRAND-RENAME-001 (commit f0f02e4, 2026-05-13): GOOSE → MINK rebrand 정책.
- PR #189 (M1, `260e032`): goreleaser + release workflow 머지본.
- PR #194 (M2, `8f35aec`): Unix install.sh 머지본.
- PR #195 (M4+M5, `948fdfc`): Ollama 자동 설치 + RAM 기반 모델 선택 머지본.

---

**End of Amendment v0.2 — curl-single + WSL-only Distribution Policy**
