---
id: SPEC-MINK-CROSSPLAT-001
version: 0.2.0+amendment-v0.2
spec: SPEC-MINK-CROSSPLAT-001
---

# Acceptance Criteria — SPEC-MINK-CROSSPLAT-001

> **[AMENDED by amendment-v0.2 — 2026-05-15]**: curl-single + WSL-only 정책 적용. install.ps1 / winget / Homebrew tap / .deb-.rpm 시나리오는 SUPERSEDED. Windows 시나리오는 WSL2 bash 로 재정의. 상세 amendment-v0.2.md §3 / §4.

## Definition of Done

본 SPEC의 구현이 완료된 것으로 간주되려면 아래의 모든 필수 기준을 충족해야 한다.

### 필수 기준 (Must Pass)

- [x] `curl -fsSL https://mink.ai/install | sh`가 macOS, Linux 에서 정상 동작
- [x] **(amendment-v0.2)** Windows + WSL2 환경에서 `wsl bash -c "curl -fsSL https://mink.ai/install | sh"` 실행 시 install.sh 정상 동작 + `mink --version` 응답
- [ ] ~~`irm https://mink.ai/install.ps1 | iex`가 Windows PowerShell에서 정상 동작~~ **[SUPERSEDED by amendment-v0.2 §3 — 2026-05-15]**
- [ ] ~~`winget install ai-mink.mink`가 Windows에서 정상 동작~~ **[SUPERSEDED by amendment-v0.2 §3 — 2026-05-15]**
- [x] OS + CPU 아키텍처 자동 감지 (6개 타겟 플랫폼)
- [x] GitHub Release에서 올바른 바이너리 자동 다운로드 + SHA256 검증
- [x] Ollama 미설치 시 자동 설치 (macOS/Linux/WSL2)
- [x] Ollama 서비스 시작 후 30초 이내 응답 확인
- [x] RAM 기반 모델 자동 선택 (4가지 RAM 범주)
- [x] `ollama pull` 모델 다운로드 + 진행률 표시
- [x] 모델 다운로드 재개(resume) 지원
- [x] CLI 도구(claude, gemini, codex) 감지 + config 기록
- [x] CLI 도구 미설치 시 설치 차단 없음
- [x] goreleaser 6플랫폼 빌드 + checksums + SBOM 생성
- [ ] ~~Homebrew tap (`brew install ai-mink/tap/mink`) 동작~~ **[SUPERSEDED by amendment-v0.2 §3 — 2026-05-15]**
- [x] 미지원 플랫폼에서 명확한 에러 메시지 + exit code 1
- [x] Ollama 설치 실패 시 바이너리 설치 계속 진행
- [x] 시스템 전역 설정 미수정 (사용자 프로필만 수정)

---

## Given/When/Then Scenarios

> **TODO (amendment-v0.2 §6.1 — 2026-05-15)**: 본 섹션의 시나리오 1 / 3-WSL2 / 5 등에 등장하는 Ollama 모델 namespace `ai-mink/gemma4-e2b-rl-v1`, `ai-mink/gemma4-e4b-rl-v1:{q4_k_m,q5_k_m,q8_0}` 는 GitHub org (`ai-mink/*` repo) 와 별개의 Ollama Hub account namespace (`ollama.com/library/ai-mink/...`) 이다. 두 namespace 의 통일 여부는 SPEC-GOOSE-GEMMA4-001 (모델 배포 SPEC) 의 별도 결정 사항. 본 amendment-v0.2 의 범위 밖.

### Scenario 1: Fresh macOS Install (Full Flow)

**Given** macOS arm64 환경, Ollama 미설치, RAM 16 GB, `claude` 설치됨

**When** `curl -fsSL https://mink.ai/install | sh` 실행

**Then**:
1. "Detected: macOS arm64" 출력
2. MINK 바이너리 다운로드 (SHA256 검증 통과)
3. `~/.local/bin/mink` 설치 + PATH 추가
4. "Ollama not found. Installing..." → `brew install ollama` 실행
5. Ollama 서비스 시작 + 헬스체크 통과
6. "System RAM: 16 GB → Model: ai-mink/gemma4-e4b-rl-v1:q5_k_m (~4 GB)" 출력
7. `ollama pull` 진행률 표시 → 완료
8. "Detected CLI tools: claude" 출력
9. `~/.mink/config.yaml` 생성 (모델 + CLI 도구 정보)
10. "MINK installed successfully! Run 'mink init' to get started." 출력

---

### Scenario 2: Fresh Ubuntu Install (Ollama Install Failure)

**Given** Ubuntu 22.04 amd64, Ollama 미설치, RAM 8 GB, 네트워크 제한으로 Ollama 설치 실패

**When** `curl -fsSL https://mink.ai/install | sh` 실행

**Then**:
1. MINK 바이너리 정상 설치
2. Ollama 설치 시도 → 실패
3. "Warning: Ollama installation failed. Install manually: https://ollama.com" 출력
4. 모델 다운로드 스킵
5. "MINK binary installed. Configure Ollama and run 'mink init'." 출력
6. exit code 0

---

### Scenario 3: Windows PowerShell Install **[SUPERSEDED by amendment-v0.2 §3 — 2026-05-15]**

> 본 시나리오는 amendment-v0.2 의 Windows = WSL2 only 정책에 따라 SUPERSEDED. 새로운 Windows 시나리오는 아래 Scenario 3-WSL2 참조.

**Given** Windows 11, PowerShell 7, Ollama 이미 설치됨, RAM 32 GB

**When** `irm https://mink.ai/install.ps1 | iex` 실행

**Then**:
1. "Detected: Windows amd64" 출력
2. mink.exe 다운로드 + SHA256 검증
3. `%USERPROFILE%\bin\mink.exe` 설치
4. "Ollama already installed" 감지 → 설치 스킵
5. "System RAM: 32 GB → Model: ai-mink/gemma4-e4b-rl-v1:q8_0 (~5 GB)" 출력
6. `ollama pull` 진행률 표시 → 완료
7. "No external CLI tools detected" 출력
8. `.mink\config.yaml` 생성

---

### Scenario 3-WSL2: Windows + WSL2 Install (amendment-v0.2, redefines AC-CP-002)

> **[ADDED by amendment-v0.2 §3 — 2026-05-15]**: 기존 Scenario 3 (PowerShell native) 의 대체 시나리오. Windows = WSL2 only 정책.

**Given** Windows 11 + WSL2 (Ubuntu 22.04) 환경, WSL2 내부에 Ollama 이미 설치됨, WSL2 가시 RAM 16 GB, `curl` 사용 가능

**When** Windows 호스트에서 `wsl bash -c "curl -fsSL https://mink.ai/install | sh"` 실행

**Then**:
1. WSL2 의 install.sh 가 "Detected: Linux amd64" 출력 (WSL2 는 Linux 로 감지)
2. `mink_linux_amd64` 바이너리 다운로드 + SHA256 검증
3. WSL2 의 `~/.local/bin/mink` 설치 + PATH 추가 (`.bashrc` / `.zshrc`)
4. "Ollama already installed" 감지 → 설치 스킵
5. "System RAM: 16 GB → Model: ai-mink/gemma4-e4b-rl-v1:q5_k_m (~4 GB)" 출력
   (Ollama 모델 namespace 는 amendment-v0.2 §6.1 의 별도 결정 사항)
6. `ollama pull` 진행률 표시 → 완료
7. CLI 도구 감지 단계 (claude/gemini/codex)
8. WSL2 의 `~/.mink/config.yaml` 생성
9. `wsl bash -c "mink --version"` 응답 확인

---

### Scenario 4: Model Download Resume

**Given** 이전 설치에서 모델 다운로드 50%에서 중단, Ollama 설치됨

**When** 설치 스크립트 재실행

**Then**:
1. MINK 바이너리 이미 설치됨 감지 → 재설치 확인
2. Ollama 이미 설치됨 감지 → 스킵
3. 모델 다운로드 재개 (처음부터가 아닌 부분 다운로드부터)
4. `ollama pull` 출력에 "pulling" 진행 표시
5. 완료 후 `ollama list`에 모델 표시

---

### Scenario 5: Low RAM System

**Given** Linux amd64, RAM 4 GB

**When** 설치 스크립트의 모델 선택 단계

**Then**:
1. "System RAM: 4 GB" 감지
2. "Selected model: ai-mink/gemma4-e2b-rl-v1 (2B, Q4_K_M, ~1.5 GB)" 출력
3. `ollama pull ai-mink/gemma4-e2b-rl-v1` 실행
4. 다운로드 완료 후 `ollama list`에 `ai-mink/gemma4-e2b-rl-v1` 표시

---

### Scenario 6: Unsupported Platform

**Given** FreeBSD amd64

**When** `curl -fsSL https://mink.ai/install | sh` 실행

**Then**:
1. "Error: Unsupported platform: FreeBSD (amd64)" 출력
2. "Supported platforms: macOS (amd64, arm64), Linux (amd64, arm64), Windows (amd64, arm64)" 출력
3. exit code 1
4. 파일 다운로드 없음

---

### Scenario 7: Homebrew Install (macOS) **[SUPERSEDED by amendment-v0.2 §3 — 2026-05-15]**

> 본 시나리오는 amendment-v0.2 의 curl-single 정책에 따라 SUPERSEDED. macOS 사용자는 `curl -fsSL https://mink.ai/install | sh` 사용 (Scenario 1 참조).

**Given** macOS, Homebrew 설치됨

**When** `brew install ai-mink/tap/mink` 실행

**Then**:
1. mink 바이너리 다운로드 + 설치
2. `mink --version` 정상 응답
3. `which mink` → Homebrew 셀러 경로

---

### Scenario 8: goreleaser Release (CI) (amendment-v0.2 적용)

**Given** main 브랜치에 `v0.1.0` 태그 push

**When** GitHub Actions release workflow 실행

**Then**:
1. 6개 플랫폼 바이너리 생성 (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64, windows/arm64)
2. `checksums.txt` 생성 (SHA256)
3. SBOM 파일 생성 (SPDX JSON)
4. GitHub Release 생성 + 모든 산출물 업로드
5. ~~Homebrew tap 자동 업데이트~~ **[SUPERSEDED by amendment-v0.2 §3 — 2026-05-15]**
6. ~~Debian `.deb` + RPM `.rpm` 패키지 생성~~ **[SUPERSEDED by amendment-v0.2 §3 — 2026-05-15]**

---

### Scenario 9: CLI Tools Partial Detection

**Given** PATH에 `claude` + `codex` 존재, `gemini` 부재

**When** 설치 스크립트 CLI 감지 단계

**Then**:
1. "Detected CLI tools: claude, codex" 출력
2. "Not found: gemini" 미표시 (경고 없음)
3. `~/.mink/config.yaml` 내용:
   ```yaml
   delegation:
     available_tools:
       - claude
       - codex
   ```

---

### Scenario 10: System Integrity Check

**Given** Linux, 설치 스크립트 실행 완료

**When** 설치 후 시스템 상태 확인

**Then**:
1. `/etc/profile` 수정 없음
2. `~/.bashrc` 또는 `~/.zshrc`에 PATH 추가됨 (사용자 동의 후)
3. `~/.local/bin/mink` 존재 + 실행 권한
4. `~/.mink/config.yaml` 존재
5. 홈 디렉토리 외부에 생성된 파일 없음

---

## Quality Gates

### TRUST 5 검증

| 차원 | 기준 | 검증 방법 |
|-----|------|----------|
| **T**ested | 모든 AC에 대응하는 자동화 테스트 존재 | CI 매트릭스 (macOS/Linux/Windows) |
| **R**eadable | 스크립트에 명확한 주석 + 진행 상태 메시지 | 코드 리뷰 |
| **U**nified | install.sh와 install.ps1의 논리적 구조 일치 | 구조 비교 검토 |
| **S**ecured | HTTPS 전용 + SHA256 검증 + 시스템 설정 보호 | 보안 검토 + AC-10 |
| **T**rackable | 각 설치 단계 로깅 + 에러 코드 표준화 | 설치 로그 검증 |

### 성능 기준

| 지표 | 목표 | 측정 방법 |
|------|------|----------|
| 바이너리 설치 시간 | 30초 이하 | CI 타이밍 |
| Ollama 설치 시간 | 2분 이하 | CI 타이밍 |
| 모델 다운로드 (3 GB) | 3분 이하 (빠른 네트워크) | 진행률 로그 |
| 전체 설치 시간 | 5분 이하 (빠른 네트워크) | E2E 타이밍 |

---

**End of Acceptance Criteria**
