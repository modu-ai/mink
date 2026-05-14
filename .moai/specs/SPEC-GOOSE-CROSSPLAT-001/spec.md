---
id: SPEC-GOOSE-CROSSPLAT-001
version: 0.1.1
status: draft
created_at: 2026-04-29
updated_at: 2026-05-14
author: manager-spec
priority: P0
issue_number: null
phase: 1
size: 대(L)
lifecycle: spec-anchored
labels: [installer, cross-platform, ollama, goreleaser, distribution, model-download, post-brand-rename]
---

# SPEC-GOOSE-CROSSPLAT-001 — Universal Cross-Platform Installer + Model Distribution

> **POST-BRAND-RENAME NOTICE (2026-05-14)**: 본 SPEC 은 SPEC-MINK-BRAND-RENAME-001 (commit f0f02e4, 2026-05-13) 이전에 작성된 draft 이다. 본문 곳곳에 GOOSE / AI.GOOSE 명칭이 남아 있으며, 후속 implementation 진입 시 다음 중 하나로 처리해야 한다.
>
> 1. **MINK 로 rebrand** — id `SPEC-MINK-CROSSPLAT-001` 신설, 본 SPEC 은 status=superseded
> 2. **본문 내 MINK 치환** — id 유지, 본문 GOOSE → MINK 치환 (BRAND-RENAME-001 의 binary rename 정책과 align)
>
> 후속 implementation 진입 직전에 결정. 본 marker 가 추가되기 전까지 본 SPEC 은 "draft, awaiting brand-rename decision" 상태이다.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-29 | 초안 작성. Phase 1 배포 인프라: 범용 설치 스크립트, Ollama 자동 설치, Gemma 4 RL 모델 자동 선택/다운로드, goreleaser 다중 플랫폼 빌드, 패키지 매니저 배포. | manager-spec |
| 0.1.1 | 2026-05-14 | POST-BRAND-RENAME marker 추가. BRAND-RENAME-001 (commit f0f02e4) 이후 GOOSE prefix draft 의 후속 처리 (rebrand vs 본문 치환) 미결정. labels 에 `post-brand-rename` 추가. | manager-spec |

---

## 1. 개요 (Overview)

AI.GOOSE를 **macOS, Linux, Windows** 세 플랫폼에서 **단일 명령어로 설치**할 수 있는 범용 인스톨러와, 훈련된 Gemma 4 RL 모델의 자동 선택 및 다운로드 시스템을 정의한다.

사용자 경험 목표:

```bash
# macOS / Linux / Windows (Git Bash / WSL)
curl -fsSL https://goose.ai/install | sh

# Windows Native (PowerShell)
irm https://goose.ai/install.ps1 | iex
```

위 한 줄 실행으로 다음이 모두 자동 완료된다:

1. OS/CPU 아키텍처 감지
2. AI.GOOSE 바이너리 다운로드 및 설치
3. Ollama 미설치 시 자동 설치 + 서비스 시작
4. 시스템 RAM 기반 적절한 Gemma 4 모델 자동 선택
5. `ollama pull` 모델 다운로드 (진행률 표시)
6. 선택적 CLI 도구 감지 (claude, gemini, codex) — 위임 라우팅용

### 1.1 핵심 지표

| 지표 | 목표 |
|------|------|
| 설치 명령어 수 | 1줄 |
| 수동 개입 | 0 (모든 단계 자동, 선택적 확인만) |
| 지원 플랫폼 | 6 (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64, windows/arm64) |
| 설치 소요 시간 (빠른 네트워크) | 5분 이하 (모델 다운로드 포함) |
| 설치 소요 시간 (바이너리만) | 30초 이하 |

---

## 2. 배경 (Background)

### 2.1 왜 범용 인스톨러가 필요한가

AI.GOOSE는 개발자뿐 아니라 비개발자(가족, 친지)도 사용할 수 있는 컴패니언 AI이다. 설치 장벽을 최소화하기 위해:

- **한 줄 설치**: `curl | sh` / `irm | iex` / `winget install`
- **의존성 자동 해결**: Ollama 미설치 시 자동 설치
- **모델 자동 선택**: 사용자가 양자화 레벨을 이해할 필요 없이 RAM 기반 자동 선택
- **3대 OS 지원**: macOS, Linux, Windows 모두 동등한 경험

### 2.2 왜 goreleaser인가

- Go 생태계 표준 크로스 컴파일 도구
- 6개 타겟 플랫폼 단일 설정
- Homebrew tap, Debian/RPM 패키지 자동 생성
- checksums, SBOM 내장 지원
- GitHub Actions 통합

### 2.3 왜 Ollama 레지스트리인가

- Gemma 4 RL 모델이 Ollama 형식으로 배포됨
- `ollama pull`은 중단 시 재개(resume)를 네이티브 지원
- 진행률(progress) 표시를 API로 제공
- 모델 버전 관리가 레지스트리에서 자동 처리됨

### 2.4 범위 경계

- **IN**: 설치 스크립트(sh/ps1), goreleaser 설정, Ollama 자동 설치, 모델 자동 선택/다운로드, CLI 도구 감지, Homebrew tap, winget 매니페스트, Debian/RPM 패키지
- **OUT**: 자동 업데이트 메커니즘, GUI 설치 마법사(ONBOARDING-001 Web UI), Docker 이미지, macOS .pkg/.dmg 설치 관리자, 모델 미세조정/RL 훈련, 코드 서명/공증(macOS notarization)

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. **Unix Shell 설치 스크립트** — `install.sh`:
   - `curl -fsSL https://goose.ai/install | sh` 진입점
   - macOS, Linux, Windows (Git Bash / WSL) 감지 및 지원
   - OS + CPU 아키텍처 자동 감지
   - GitHub Release에서 정확한 바이너리 다운로드
   - 설치 경로: `~/.local/bin` (Linux/macOS), `%USERPROFILE%\bin` (Windows)
   - PATH 설정 자동화 (`.bashrc`, `.zshrc`, `.profile` 감지)
   - SHA256 체크섬 검증

2. **PowerShell 설치 스크립트** — `install.ps1`:
   - `irm https://goose.ai/install.ps1 | iex` 진입점
   - Windows 네이티브 PowerShell 5.1+ / PowerShell 7 지원
   - Windows Defender SmartScreen 우회 가이드 (문서만)

3. **winget 매니페스트**:
   - `winget install ai-goose.goose` 진입점
   - `manifests/a/ai-goose/goose/` 디렉토리 구조
   - 버전별 YAML 매니페스트 자동 업데이트 (CI)

4. **Ollama 자동 설치**:
   - macOS: `brew install ollama` → 실패 시 직접 다운로드
   - Linux: `curl -fsSL https://ollama.com/install.sh | sh`
   - Windows: `winget install Ollama.Ollama` → 실패 시 직접 다운로드 안내
   - 설치 후 서비스 자동 시작 (`ollama serve` 백그라운드)
   - 실행 확인: `ollama list` 명령으로 응답 확인

5. **모델 자동 선택 + 다운로드**:
   - 시스템 RAM 감지 (OS별 API / `/proc/meminfo`)
   - RAM 기반 모델 선택 로직 (REQ-CP-011 표)
   - `ollama pull {model}` 실행 + 진행률 표시
   - 다운로드 완료 후 `ollama list`로 모델 가용성 확인

6. **CLI 도구 감지**:
   - PATH에서 `claude`, `gemini`, `codex` 존재 여부 확인
   - 감지 결과를 `~/.goose/config.yaml`에 기록
   - 미감지 도구는 경고 없이 스킵 (설치 차단 안 함)

7. **goreleaser 설정** — `.goreleaser.yaml`:
   - 6개 타겟: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64, windows/arm64
   - 산출물: 바이너리, SHA256 체크섬, SBOM (syft)
   - Homebrew tap: `ai-goose/tap/goose`
   - Debian 패키지 (.deb)
   - RPM 패키지 (.rpm)
   - scoopa bucket (Windows)

8. **Homebrew tap** — `ai-goose/homebrew-tap`:
   - `brew install ai-goose/tap/goose` 진입점
   - goreleaser가 자동 업데이트

9. **CI/CD 파이프라인** — `.github/workflows/release.yml`:
   - 태그 push 시 goreleaser 트리거
   - 설치 스크립트를 `goose.ai` 도메인에 배포
   - winget PR 자동 생성

### 3.2 OUT OF SCOPE

- **자동 업데이트**: `goose update` 명령은 후속 SPEC
- **GUI 설치 마법사**: ONBOARDING-001 Web UI에서 담당
- **Docker 이미지**: 컨테이너 배포는 후속 SPEC
- **macOS .pkg/.dmg**: macOS 패키지 설치 관리자는 v1.0+ 검토
- **코드 서명/공증**: macOS notarization, Windows Authenticode는 v0.2+ (비용 발생)
- **모델 미세조정**: TRAIN-001에서 담당
- **모델 레지스트리 호스팅**: Ollama 레지스트리(or Hugging Face)에 배포하는 것은 별도 작업
- **프록시/방화벽 대응**: `HTTP_PROXY`/`HTTPS_PROXY` 환경변수 인식은 기본 지원하나 상세 SOCKS5/NTLM 프록시는 후속
- **ARM Windows 네이티브 검증**: windows/arm64 빌드는 제공하나 QA는 x86_64 우선

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-CP-001 [Ubiquitous]** — The system **shall** provide a Unix shell installation command `curl -fsSL https://goose.ai/install | sh` that works on macOS, Linux, and Windows (Git Bash / WSL) without prerequisite software beyond a POSIX-compliant shell and `curl`.

**REQ-CP-002 [Ubiquitous]** — The system **shall** provide a PowerShell installation command `irm https://goose.ai/install.ps1 | iex` for Windows native environments (PowerShell 5.1+ and PowerShell 7). The PowerShell script **shall** handle restrictive execution policies by using `Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass` internally (process-scoped only, no system-wide change); if the execution policy cannot be bypassed, the script **shall** print a manual instruction suggesting `powershell -ExecutionPolicy Bypass -File install.ps1`.

**REQ-CP-003 [Ubiquitous]** — The system **shall** provide `winget install ai-goose.goose` as a Windows package manager installation option.

**REQ-CP-004 [Ubiquitous]** — The install script **shall** detect the operating system (macOS, Linux, Windows) and CPU architecture (x86_64, arm64) and report both values to the user before proceeding with download.

**REQ-CP-005 [Ubiquitous]** — The install script **shall** download the correct AI.GOOSE binary matching the detected OS and CPU architecture from the GitHub Releases API.

**REQ-CP-015 [Ubiquitous]** — AI.GOOSE **shall** be distributed as a single statically-linked Go binary per platform with no runtime dependencies beyond the OS kernel and libc (or musl for linux/arm64).

**REQ-CP-016 [Ubiquitous]** — goreleaser **shall** be configured to build for six target platforms: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64, windows/arm64.

**REQ-CP-017 [Ubiquitous]** — Release artifacts **shall** include: the binary for each platform, SHA256 checksums file, and SBOM (Software Bill of Materials) in SPDX format.

**REQ-CP-020 [Ubiquitous]** — The install script **shall** detect available CLI tools (`claude`, `gemini`, `codex`) by scanning the system PATH and report detected tools to the user.

**REQ-CP-021 [Ubiquitous]** — Detected CLI tools **shall** be recorded in `.goose/config.yaml` under a `delegation.available_tools` key for use by the routing system.

**REQ-CP-022 [Ubiquitous]** — Missing CLI tools **shall not** block installation; local mode operates independently without any external CLI tools.

### 4.2 Event-Driven

**REQ-CP-006 [Event-Driven]** — **When** the install script executes, it **shall** detect whether Ollama is installed by checking `which ollama` (Unix) or `Get-Command ollama` (PowerShell) and verifying `ollama --version` returns a valid response.

**REQ-CP-007 [Event-Driven]** — **When** Ollama is not installed, the script **shall** auto-install it using the platform-appropriate method:
- macOS: `brew install ollama` (with Homebrew), or direct `.zip` download from `ollama.com/download` (without Homebrew)
- Linux: `curl -fsSL https://ollama.com/install.sh | sh`
- Windows: `winget install Ollama.Ollama` (with winget), or guide user to download from `ollama.com/download`

**REQ-CP-008 [Event-Driven]** — **When** Ollama installation completes, the script **shall** start the Ollama service in the background:
- macOS: `open -a Ollama` (launches GUI app) or `ollama serve &`
- Linux: `ollama serve &` (systemd user service if available)
- Windows: Start Ollama from Start Menu or `Start-Process ollama`

**REQ-CP-009 [Event-Driven]** — **When** the Ollama service start command is issued, the script **shall** verify Ollama is running by executing `ollama list` and confirming a successful response (empty list is acceptable) within a 30-second timeout.

**REQ-CP-012 [Event-Driven]** — **When** the appropriate model is determined (per REQ-CP-011), the script **shall** execute `ollama pull {selected_model}` and display download progress to the user in real-time.

**REQ-CP-014 [Event-Driven]** — **When** model download completes, the script **shall** verify the model is available by executing `ollama list` and confirming the selected model appears in the output.

### 4.3 State-Driven

**REQ-CP-010 [State-Driven]** — **While** the install script is determining the appropriate model, it **shall** detect total system RAM using platform-specific methods:
- Linux: `/proc/meminfo` `MemTotal` field
- macOS: `sysctl -n hw.memsize`
- Windows (PowerShell): `(Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory`

**REQ-CP-011 [State-Driven]** — **While** selecting the model to download, the script **shall** choose based on detected system RAM:
- < 8 GB: `ai-goose/gemma4-e2b-rl-v1` (~1.5 GB, 2B parameter, Q4_K_M)
- 8-16 GB: `ai-goose/gemma4-e4b-rl-v1:q4_k_m` (~3 GB, 4B parameter, Q4_K_M)
- 16-32 GB: `ai-goose/gemma4-e4b-rl-v1:q5_k_m` (~4 GB, 4B parameter, Q5_K_M)
- 32 GB+: `ai-goose/gemma4-e4b-rl-v1:q8_0` (~5 GB, 4B parameter, Q8_0)

**REQ-CP-013 [State-Driven]** — **While** model download is in progress and interrupted (network failure, user Ctrl+C), the download **shall** support resume when the script is re-executed, utilizing Ollama's native resume capability (`ollama pull` resumes partial layers).

### 4.4 Unwanted Behavior

**REQ-CP-023 [Unwanted]** — The install script **shall not** execute arbitrary remote code beyond the explicitly downloaded AI.GOOSE binary and Ollama installer; all download URLs **shall** use HTTPS and **shall** verify checksums before execution.

**REQ-CP-024 [Unwanted]** — The install script **shall not** modify system-level configuration (e.g., `/etc/profile`, Windows Registry) without explicit user consent; PATH modifications **shall** target user-level shell profiles only.

**REQ-CP-025 [Unwanted]** — **If** the detected platform is unsupported (e.g., BSD, Solaris, non-x86/non-ARM CPU), the script **shall** display a clear error message listing supported platforms and exit with code 1, rather than attempting an incompatible download.

**REQ-CP-026 [Unwanted]** — **If** Ollama auto-installation fails (network error, permission denied), the script **shall not** abort the entire installation; it **shall** install the AI.GOOSE binary successfully and display instructions for manual Ollama setup.

### 4.5 Optional

**REQ-CP-018 [Optional]** — **Where** a Homebrew tap is maintained at `ai-goose/tap/goose`, the system **shall** support `brew install ai-goose/tap/goose` as an alternative installation method on macOS and Linux.

**REQ-CP-019 [Optional]** — **Where** Debian and RPM package generation is configured in goreleaser, the system **shall** produce `.deb` and `.rpm` artifacts for Linux distribution.

---

## 5. 수용 조건 (Acceptance Criteria)

### AC-CP-001 — Unix 설치 스크립트 동작 (verifies REQ-CP-001, REQ-CP-004, REQ-CP-005)
- **Given** macOS/arm64 환경에서 `curl` 사용 가능
- **When** `curl -fsSL https://goose.ai/install | sh` 실행
- **Then** OS="macOS", ARCH="arm64" 감지 후 `goose_darwin_arm64` 바이너리 다운로드, `~/.local/bin/goose`에 설치, SHA256 체크섬 검증 통과

### AC-CP-002 — PowerShell 설치 스크립트 동작 (verifies REQ-CP-002)
- **Given** Windows 11, PowerShell 5.1 환경
- **When** `irm https://goose.ai/install.ps1 | iex` 실행
- **Then** OS="Windows", ARCH="amd64" 감지 후 `goose_windows_amd64.exe` 다운로드, `%USERPROFILE%\bin\goose.exe`에 설치

### AC-CP-003 — winget 설치 (verifies REQ-CP-003)
- **Given** Windows 11, winget 설치됨
- **When** `winget install ai-goose.goose` 실행
- **Then** 최신 버전 goose 설치, `goose --version` 응답 확인

### AC-CP-004 — Ollama 자동 설치 (verifies REQ-CP-006, REQ-CP-007, REQ-CP-008, REQ-CP-009)
- **Given** Ubuntu 22.04, Ollama 미설치 상태
- **When** 설치 스크립트 실행
- **Then** `curl -fsSL https://ollama.com/install.sh | sh`로 Ollama 설치, `ollama serve &` 백그라운드 실행, `ollama list` 30초 이내 성공 응답

### AC-CP-005 — Ollama 이미 설치됨 (verifies REQ-CP-006)
- **Given** macOS, Ollama 이미 설치됨 (`ollama --version` 응답)
- **When** 설치 스크립트 실행
- **Then** Ollama 설치 단계 스킵, 서비스 실행 확인만 수행

### AC-CP-006 — RAM 기반 모델 자동 선택 (verifies REQ-CP-010, REQ-CP-011)
- **Given** 시스템 RAM = 12 GB
- **When** 모델 선택 단계
- **Then** `ai-goose/gemma4-e4b-rl-v1:q4_k_m` (8-16 GB 범주) 선택, 사용자에게 "Model: gemma4-e4b-rl-v1 (Q4_K_M, ~3 GB)" 표시

### AC-CP-007 — 모델 다운로드 + 진행률 (verifies REQ-CP-012, REQ-CP-014)
- **Given** Ollama 실행 중, 모델 미다운로드
- **When** `ollama pull ai-goose/gemma4-e4b-rl-v1:q4_k_m` 실행
- **Then** 실시간 진행률 표시 (pulling... 45%), 완료 후 `ollama list`에 모델 표시

### AC-CP-008 — 다운로드 재개 (verifies REQ-CP-013)
- **Given** 모델 다운로드 50%에서 네트워크 중단 후 재연결
- **When** 설치 스크립트 재실행
- **Then** `ollama pull`이 처음부터 재시작하지 않고 부분 레이어부터 재개, 총 다운로드량 감소

### AC-CP-009 — CLI 도구 감지 (verifies REQ-CP-020, REQ-CP-021, REQ-CP-022)
- **Given** PATH에 `claude` 존재, `gemini`/`codex` 부재
- **When** 설치 스크립트의 CLI 도구 감지 단계
- **Then** "Detected: claude" 표시, `~/.goose/config.yaml`에 `delegation.available_tools: [claude]` 기록, 설치 계속 진행

### AC-CP-010 — CLI 도구 전부 없음 (verifies REQ-CP-022)
- **Given** PATH에 claude, gemini, codex 모두 부재
- **When** 설치 스크립트의 CLI 도구 감지 단계
- **Then** "No external CLI tools detected (local mode)" 표시, 설치 성공, 경고/에러 없음

### AC-CP-011 — goreleaser 6플랫폼 빌드 (verifies REQ-CP-015, REQ-CP-016, REQ-CP-017)
- **Given** `git tag v0.1.0` push
- **When** GitHub Actions release workflow 실행
- **Then** 6개 플랫폼 바이너리 생성, `checksums.txt` 생성, SBOM(spdx) 생성, GitHub Release에 업로드

### AC-CP-012 — Homebrew 설치 (verifies REQ-CP-018)
- **Given** macOS, Homebrew 설치됨
- **When** `brew install ai-goose/tap/goose` 실행
- **Then** goose 바이너리 설치, `goose --version` 응답

### AC-CP-013 — 미지원 플랫폼 거부 (verifies REQ-CP-025)
- **Given** FreeBSD 환경
- **When** 설치 스크립트 실행

### AC-CP-013 — 미지원 플랫폼 거부 (verifies REQ-CP-025)
- **Given** FreeBSD 환경
- **When** 설치 스크립트 실행
- **Then** "Unsupported platform: FreeBSD. Supported: macOS, Linux, Windows" 에러 메시지, exit code 1

### AC-CP-014 — .deb/.rpm 패키지 생성 (verifies REQ-CP-019)
- **Given** goreleaser config에 `nfpms` 섹션이 `.deb` + `.rpm` 형식으로 설정됨
- **When** GitHub Actions release workflow 실행
- **Then** `goose_0.1.0_linux_amd64.deb` 및 `goose_0.1.0_linux_amd64.rpm` 아티팩트가 생성되어 GitHub Release에 업로드됨

### AC-CP-015 — Ollama 설치 실패 시 계속 진행 (verifies REQ-CP-026)
- **Given** Linux, Ollama 설치 실패 (네트워크 오류)
- **When** 설치 스크립트 실행
- **Then** AI.GOOSE 바이너리는 정상 설치, "Ollama installation failed. Please install manually: https://ollama.com" 안내 표시, exit code 0

### AC-CP-016 — 시스템 설정 미수정 (verifies REQ-CP-024)
- **Given** Linux, 설치 스크립트 실행
- **When** PATH 설정 단계
- **Then** `~/.bashrc` 또는 `~/.zshrc`만 수정, `/etc/profile` 미수정, 수정 전 사용자 동의 표시

---

## 6. 테스트 시나리오

### 6.1 통합 테스트 (CI)

| 시나리오 | 환경 | 검증 |
|---------|------|------|
| Fresh install (macOS arm64) | GitHub Actions macOS runner | 바이너리 다운로드 + 체크섬 + PATH |
| Fresh install (Ubuntu amd64) | GitHub Actions Ubuntu runner | 바이너리 + Ollama 자동 설치 + 모델 |
| Fresh install (Windows) | GitHub Actions Windows runner | PowerShell 스크립트 + 바이너리 |
| Resume after interrupt | Docker (network simulation) | 모델 다운로드 50% 차단 후 재개 |
| Unsupported platform | FreeBSD VM | exit code 1 + 에러 메시지 |

### 6.2 단위 테스트

| 테스트 | 검증 대상 |
|--------|----------|
| `TestDetectPlatform` | OS + ARCH 감지 로직 |
| `TestSelectModel_8GB` | 8 GB → q4_k_m 선택 |
| `TestSelectModel_32GB` | 32 GB → q8_0 선택 |
| `TestDetectCLITools` | PATH 스캔 + config 기록 |
| `TestChecksumVerification` | SHA256 검증 (정상/비정상) |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | **SPEC-GOOSE-CONFIG-001** | `~/.goose/config.yaml` 저장소 (CLI 도구 감지 결과 기록) |
| 선행 SPEC | **SPEC-GOOSE-LLM-001** | Ollama 어댑터 (설치된 Ollama를 백엔드로 사용) |
| 동시 | SPEC-GOOSE-ONBOARDING-001 (v0.3) | 온보딩 마법사에 Model Setup / CLI Tools 단계 추가 |
| 동시 | SPEC-GOOSE-GEMMA4-001 | Gemma 4 RL 모델 배포 (Ollama 레지스트리 업로드) |
| 후속 SPEC | SPEC-GOOSE-AUTOUPDATE-001 (TBD) | 자동 업데이트 메커니즘 |
| 후续 | SPEC-GOOSE-DOCKER-001 (TBD) | Docker 이미지 배포 |
| 외부 | goreleaser/goreleaser | Go 크로스 컴파일 + 릴리스 자동화 |
| 외부 | ollama/ollama | 로컬 LLM 런타임 |
| 외부 | GitHub Actions | CI/CD 파이프라인 |
| 외부 | syft | SBOM 생성 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|--------|-------|------|------|
| R1 | `curl \| sh` 보안 우려 (supply chain attack) | 중 | 고 | HTTPS 강제 + SHA256 체크섬 검증 + 바이너리 서명 (v0.2+ 코드 서명) |
| R2 | Ollama 설치 실패 (방화벽, 권한) | 중 | 중 | 바이너리 설치는 계속 진행 + 수동 설치 안내 표시 (REQ-CP-026) |
| R3 | 모델 다운로드 대역폭 (5 GB 모델) | 중 | 중 | RAM 기반 자동 선택으로 최소 모델 제안 + Ollama native resume + 진행률 표시 |
| R4 | macOS 공증(notarization) 없어 Gatekeeper 차단 | 높 | 중 | 설치 문서에 `xattr -cr goose` 해제 가이드 포함, v0.2에서 공증 도입 |
| R5 | Windows Defender SmartScreen 차단 | 중 | 중 | winget 경유 시 인증 우회, 직접 다운로드 시 "More info > Run anyway" 가이드 |
| R6 | ARM64 Windows 실행 환경 검증 부족 | 중 | 낮 | windows/arm64 빌드 제공하나 QA는 x86_64 우선, 커뮤니티 피드백으로 개선 |
| R7 | Homebrew tap 유지보수 부담 | 낮 | 낮 | goreleaser가 자동 업데이트, 수동 개입 최소화 |
| R8 | 설치 스크립트가 다양한 쉘 환경에서 파손 | 중 | 중 | CI에서 bash/zsh/dash/PowerShell 각각 테스트, POSIX 호환성 검증 |
| R9 | Ollama 버전 호환성 (API 변경) | 낮 | 중 | 최소 Ollama 버전 요구사항 명시, 버전 감지 후 경고 |
| R10 | `goose.ai` 도메인 가용성 | 낮 | 고 | CDN + fallback GitHub Releases URL 이중화 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/product.md` -- "로컬 우선" 정책
- `.moai/project/tech.md` -- Go 바이너리 배포 아키텍처
- `.moai/specs/SPEC-GOOSE-CONFIG-001/spec.md` -- 설정 저장소
- `.moai/specs/SPEC-GOOSE-LLM-001/spec.md` -- LLM Provider + Ollama 어댑터
- `.moai/specs/SPEC-GOOSE-ONBOARDING-001/spec.md` -- 온보딩 마법사 (v0.3 상호연동)

### 9.2 외부 참조

- goreleaser documentation: https://goreleaser.com/
- Ollama installation guide: https://ollama.com/
- Ollama model registry API: https://github.com/ollama/ollama/blob/main/docs/api.md
- winget package manager: https://learn.microsoft.com/en-us/windows/package-manager/
- Homebrew tap: https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap
- SPDX SBOM format: https://spdx.dev/

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **자동 업데이트 메커니즘을 구현하지 않는다** (후속 SPEC-GOOSE-AUTOUPDATE-001).
- 본 SPEC은 **GUI 설치 마법사를 구현하지 않는다** (ONBOARDING-001 Web UI).
- 본 SPEC은 **Docker 이미지를 생성하지 않는다** (후속 SPEC).
- 본 SPEC은 **macOS .pkg/.dmg 설치 관리자를 생성하지 않는다** (v1.0+ 검토).
- 본 SPEC은 **코드 서명/공증을 구현하지 않는다** (macOS notarization, Windows Authenticode, v0.2+).
- 본 SPEC은 **모델 훈련/미세조정을 수행하지 않는다** (TRAIN-001).
- 본 SPEC은 **Ollama 레지스트리에 모델을 업로드하지 않는다** (GEMMA4-001).
- 본 SPEC은 **SOCKS5/NTLM 프록시 상세 대응을 구현하지 않는다** (후속).
- 본 SPEC은 **시스템 전역 설정 수정을 수행하지 않는다** (REQ-CP-024).

---

**End of SPEC-GOOSE-CROSSPLAT-001 v0.1.0**
