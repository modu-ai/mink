---
id: SPEC-MINK-CROSSPLAT-001
version: 0.2.0+amendment-v0.2
spec: SPEC-MINK-CROSSPLAT-001
---

# Implementation Plan — SPEC-MINK-CROSSPLAT-001

## Overview

Universal Cross-Platform Installer + Model Distribution의 구현 계획.

> **[AMENDED by amendment-v0.2 — 2026-05-15]**: curl-single + WSL-only 정책 적용. M1.A (Homebrew tap + winget + nfpms + AUR + scoop) 와 M3 (install.ps1) 는 전면 OUT scope 전환. M6/M7 은 curl/WSL2 경로로 축소. M1/M2/M4/M5 머지본 (#189/#194/#195) 영향 없음. 상세 amendment-v0.2.md §4 참조.

---

## Milestones

### M1: goreleaser 설정 + CI 파이프라인 (Priority: High) — 완료 (PR #189)

**목표**: 6플랫폼 크로스 컴파일 + GitHub Release 자동 업로드

**작업 항목**:
1. `.goreleaser.yaml` 작성
   - 6개 타겟 플랫폼 빌드 설정
   - checksums + SBOM(syft) 생성
   - ~~Homebrew tap 자동 업데이트 설정~~ **[SUPERSEDED by amendment-v0.2 §4.1 — 2026-05-15]**
   - ~~Debian/RPM 패키지 설정~~ **[SUPERSEDED by amendment-v0.2 §4.1 — 2026-05-15]**
2. `.github/workflows/release.yml` 작성
   - tag push 트리거
   - goreleaser 실행
   - 설치 스크립트 CDN 업로드
3. ~~Homebrew tap 저장소 생성 (`ai-mink/homebrew-tap`)~~ **[SUPERSEDED by amendment-v0.2 §4.1 — 2026-05-15: M1.A 전면 OUT scope]**
4. 로컬 빌드 검증 (`goreleaser release --snapshot --clean`)

**산출물**:
- `.goreleaser.yaml`
- `.github/workflows/release.yml`
- ~~Homebrew tap 저장소~~ **[SUPERSEDED by amendment-v0.2 — 2026-05-15]**

**검증**: `goreleaser check` 통과, dry-run으로 6개 바이너리 생성 확인

---

### M1.A: 추가 패키지 매니저 매니페스트 — **SUPERSEDED by amendment-v0.2 (2026-05-15)**

> **본 milestone 은 amendment-v0.2 §4.1 에 따라 전면 OUT scope 로 전환되었다.**
>
> 원래 범위: Homebrew tap 저장소 (`ai-mink/homebrew-tap`) + winget pipeline (`manifests/a/ai-mink/mink/`) + nfpms .deb/.rpm + AUR PKGBUILD + scoop bucket.
>
> 폐지 사유: hermes-agent 공식 권장 (curl-single) + 2026 WSL2 dev 표준 + 유지보수 ROI. 상세 amendment-v0.2.md §2.
>
> 0.1.0 이후 재도입 여부는 amendment-v0.2 §6.2 에 따라 별도 결정.

---

### M2: Unix Shell 설치 스크립트 (Priority: High)

**목표**: `curl -fsSL https://mink.ai/install | sh` 작동

**작업 항목**:
1. `scripts/install.sh` 작성
   - OS/ARCH 감지 함수
   - GitHub Release API에서 최신 버전 조회
   - 바이너리 다운로드 + SHA256 검증
   - 설치 경로 생성 (`~/.local/bin`)
   - PATH 설정 (shell profile 감지)
2. 플랫폼 감지 로직
   - macOS: `uname -s` + `uname -m`
   - Linux: `uname -s` + `uname -m` (+ musl libc 감지)
   - Windows: Git Bash/WSL 감지
3. 에러 처리
   - 미지원 플랫폼 안내
   - 다운로드 실패 재시도 (3회)
   - 체크섬 불일치 경고

**산출물**:
- `scripts/install.sh`

**검증**: macOS, Ubuntu, Windows Git Bash에서 각각 테스트

---

### M3: PowerShell 설치 스크립트 — **SUPERSEDED by amendment-v0.2 (2026-05-15)**

> **본 milestone 은 amendment-v0.2 §4.1 에 따라 전면 OUT scope 로 전환되었다.**
>
> 원래 범위: `scripts/install.ps1` 작성 (PowerShell 5.1/7 지원, SHA256 검증, PATH 환경변수 업데이트) + winget 매니페스트 템플릿 (`manifests/a/ai-mink/mink/`).
>
> 폐지 사유: Windows = WSL2 only 정책. hermes-agent 공식 권장 ("Native Windows = early beta, WSL2 recommended") + 2026 WSL2 dev 표준 + install.ps1 + Pester 매트릭스 유지보수 비용 제거 (~150k effort).
>
> Windows 사용자는 amendment-v0.2 적용 후 다음 경로로 설치한다:
>
> ```sh
> wsl bash -c "curl -fsSL https://mink.ai/install | sh"
> ```
>
> 0.1.0 이후 install.ps1 재도입 여부는 amendment-v0.2 §6.2 에 따라 별도 결정.

---

### M4: Ollama 자동 설치 + 서비스 시작 (Priority: High)

**목표**: Ollama 미설치 시 자동 설치 + 실행 확인

**작업 항목**:
1. Ollama 감지 함수 (install.sh + install.ps1 공통 로직)
   - `ollama --version` 확인
   - 버전 파싱 (최소 요구 버전 비교)
2. 플랫폼별 Ollama 설치
   - macOS: brew 기반 + 직접 다운로드 fallback
   - Linux: 공식 설치 스크립트
   - Windows: winget + 직접 다운로드 안내
3. 서비스 시작 로직
   - 백그라운드 실행 (`ollama serve &` / `Start-Process`)
   - 30초 타임아웃 + 헬스체크 (`ollama list`)
4. 실패 시 graceful degradation
   - 바이너리 설치는 계속 진행
   - 수동 설치 안내 메시지 출력

**산출물**:
- install.sh 내 Ollama 함수
- install.ps1 내 Ollama 함수

**검증**: Ollama 미설치 환경에서 전체 설치 플로우 테스트

---

### M5: 모델 자동 선택 + 다운로드 (Priority: High)

**목표**: RAM 기반 모델 선택 + `ollama pull` + 진행률 표시

**작업 항목**:
1. RAM 감지 함수
   - Linux: `/proc/meminfo` 파싱
   - macOS: `sysctl -n hw.memsize`
   - Windows: WMI/CIM 쿼리
2. 모델 선택 로직
   - RAM 범주별 모델 매핑 (REQ-CP-011 표)
   - 선택 결과 사용자 표시 + 확인
3. `ollama pull` 실행
   - 진행률 파싱 (Ollama stdout)
   - 에러 처리 (디스크 공간 부족, 네트워크 오류)
   - 재개(resume) 동작 검증
4. 다운로드 완료 확인
   - `ollama list`에서 모델 존재 확인

**산출물**:
- install.sh 내 모델 선택/다운로드 함수
- install.ps1 내 모델 선택/다운로드 함수

**검증**: 다양한 RAM 크기(4GB, 8GB, 16GB, 64GB)에서 올바른 모델 선택 확인

---

### M6: CLI 도구 감지 + 설정 기록 (Priority: Medium) — install.sh 측 완료 (M2 PR #194 포함)

> **[REDUCED by amendment-v0.2 §4.2 — 2026-05-15]**: install.ps1 측 (PowerShell 경로) 은 amendment-v0.2 로 제거됨. install.sh 측만 유지.

**목표**: claude/gemini/codex 감지 + config.yaml 기록 (Unix/WSL2)

**작업 항목**:
1. CLI 도구 스캔 함수
   - `which`/`command -v` (Unix/WSL2)
   - `claude`, `gemini`, `codex` 각각 확인
   - 버전 정보 수집 (선택적)
2. `.mink/config.yaml` 생성
   - `delegation.available_tools` 배열 기록
   - 감지된 모델 정보 기록
   - 감지된 Ollama 버전 기록
3. 설정 파일 디렉토리 생성
   - `~/.mink/` 생성 (존재하지 않을 때)

**산출물**:
- install.sh 내 CLI 감지 함수 (M2 PR #194 에 포함, 사실상 완료)
- ~~install.ps1 내 CLI 감지 함수~~ **[SUPERSEDED by amendment-v0.2 §4.1 — 2026-05-15]**
- `.mink/config.yaml` 초기 템플릿

**검증**: 다양한 도구 설치 상태에서 올바른 config 생성 (M2 PR #194 의 bats 테스트로 검증 완료)

---

### M7: 통합 테스트 + 문서 (Priority: Medium) — 축소

> **[REDUCED by amendment-v0.2 §4.2 — 2026-05-15]**: curl + WSL2 시나리오만 유지. brew/winget/nfpms E2E 시나리오 제거.

**목표**: curl + WSL2 설치 플로우 E2E 검증 + 설치 문서

**작업 항목**:
1. CI 매트릭스 테스트
   - macOS runner (arm64)
   - Ubuntu runner (amd64)
   - (선택) Windows runner + WSL2 (`wsl-ubuntu` action) — WSL2 시나리오 검증
   - 각 환경에서 curl-single 진입 full install 플로우 실행
2. 에지 케이스 테스트
   - 미지원 플랫폼 (FreeBSD)
   - Ollama 설치 실패
   - 모델 다운로드 중단
   - CLI 도구 전부 없음
   - (선택) install.sh 가 native Windows 쉘 (MINGW/CYGWIN/MSYS) 에서 WSL2 안내 메시지 표시 + exit 1 (amendment-v0.2 §5.1)
3. 설치 문서 작성
   - README.md 설치 섹션 업데이트 (curl + WSL2 안내)
   - Windows = WSL2 가이드 (amendment-v0.2 §5.2)
   - 트러블슈팅 가이드

**산출물**:
- `.github/workflows/install-test.yml` (M2 PR #194 에 ubuntu + macos 기본 매트릭스 포함, WSL2 매트릭스는 잔여)
- 업데이트된 README.md
- `docs/install-guide.md`

**검증**: CI 에서 curl + WSL2 시나리오 전체 설치 성공. 기존 "3개 플랫폼 native" 시나리오 (M3 + winget + brew) 는 SUPERSEDED.

---

## Technical Approach

### 아키텍처 결정

1. **설치 스크립트는 순수 쉘/PowerShell** — Go가 아닌 네이티브 스크립트. 설치 전 Go 바이너리가 없으므로.
2. **설치 스크립트는 단일 파일** — 의존성 없이 `curl | sh`로 실행 가능.
3. **모든 다운로드는 HTTPS** — GitHub Releases API + checksum 검증.
4. **점진적 설치** — 각 단계 실패해도 다음 단계 진행 (Ollama 실패 시 바이너리만 설치).

### 파일 구조 (amendment-v0.2 적용 후)

```
scripts/
  install.sh              # Unix/macOS/WSL2 installer (curl-single 진입)

.goreleaser.yaml          # Cross-compilation config (brews/scoop/nfpms/aurs 섹션 없음)

.github/
  workflows/
    release.yml            # Tag-triggered release
    install-test.yml       # Install script E2E test (ubuntu + macos + WSL2)
```

> **[AMENDED by amendment-v0.2 §4.1 — 2026-05-15]**: `scripts/install.ps1` (M3) 와 `manifests/a/ai-mink/mink/` (M1.A winget) 디렉토리는 OUT scope 으로 전환되어 본 파일 구조에서 제거되었다.

### 위험 완화

- **R1 (보안)**: SHA256 체크섬 필수 검증, v0.2에서 코드 서명 도입
- **R4 (macOS Gatekeeper)**: README에 `xattr -cr` 가이드 포함
- **R5 (Windows SmartScreen)**: winget 경유 설치 권장, 직접 다운로드 시 가이드

---

## Dependencies (amendment-v0.2 적용 후)

| 마일스톤 | 선행 | 비고 |
|---------|------|------|
| M1 | 없음 | 독립적으로 시작 가능. **완료 (PR #189)** |
| M1.A | — | **SUPERSEDED by amendment-v0.2 §4.1**. Homebrew tap / winget / nfpms / scoop / AUR 전면 OUT scope |
| M2 | M1 | 다운로드할 바이너리가 Release에 있어야 함. **완료 (PR #194)** |
| M3 | — | **SUPERSEDED by amendment-v0.2 §4.1**. install.ps1 전면 OUT scope |
| M4 | 없음 | 스크립트 내 함수. **완료 (PR #195, Unix 측)** |
| M5 | M4 | Ollama 설치 후 모델 다운로드. **완료 (PR #195, Unix 측)** |
| M6 | 없음 | install.sh 측은 M2 PR #194 에 포함, **사실상 완료**. install.ps1 측은 amendment-v0.2 로 OUT scope |
| M7 | M2, M4, M5 | curl + WSL2 시나리오 통합 테스트. M3 + M1.A 종속성 제거 |

---

**End of Implementation Plan**
