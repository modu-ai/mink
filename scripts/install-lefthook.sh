#!/usr/bin/env bash
# install-lefthook.sh — lefthook 1회 설치 + git hook 등록 편의 스크립트.
#
# 1인 개발 CI/CD 가이드 §3(D) 의 local pre-flight 패턴 (lefthook + Makefile).
#
# 사용:
#   bash scripts/install-lefthook.sh
#
# 동작:
#   1. lefthook 바이너리가 PATH 에 없으면 brew/apt 로 설치 시도
#   2. lefthook.yml 발견 시 `lefthook install` 실행
#   3. .git/hooks/pre-push 에 lefthook hook 등록 확인
#
# 긴급 우회 (hotfix 등):
#   LEFTHOOK=0 git push

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

if [ ! -f "lefthook.yml" ]; then
  echo "ERROR: lefthook.yml not found in repo root ($REPO_ROOT)" >&2
  exit 1
fi

# ---- Step 1: ensure lefthook binary ----------------------------------------
if ! command -v lefthook >/dev/null 2>&1; then
  echo "lefthook not found in PATH — attempting install..."
  case "$(uname -s)" in
    Darwin)
      if command -v brew >/dev/null 2>&1; then
        brew install lefthook
      else
        echo "ERROR: Homebrew not found. Install lefthook manually: https://lefthook.dev/installation/" >&2
        exit 1
      fi
      ;;
    Linux)
      if command -v apt-get >/dev/null 2>&1; then
        # debian/ubuntu: snap or go install — apt 패키지 없을 수 있음
        if command -v snap >/dev/null 2>&1; then
          sudo snap install lefthook
        else
          echo "Linux: snap/apt 미지원 — go install 로 시도"
          go install github.com/evilmartians/lefthook@latest
        fi
      else
        echo "ERROR: apt-get not found. Install lefthook manually: https://lefthook.dev/installation/" >&2
        exit 1
      fi
      ;;
    MINGW* | MSYS* | CYGWIN*)
      echo "Windows: scoop 또는 winget 권장"
      echo "  scoop install lefthook"
      echo "  또는 go install github.com/evilmartians/lefthook@latest"
      exit 1
      ;;
    *)
      echo "ERROR: 지원되지 않는 OS ($(uname -s)). 수동 설치: https://lefthook.dev/installation/" >&2
      exit 1
      ;;
  esac
fi

# ---- Step 2: install git hooks ---------------------------------------------
echo ""
echo "Installing lefthook git hooks..."
lefthook install

# ---- Step 3: verify --------------------------------------------------------
if [ -x ".git/hooks/pre-push" ] && grep -q "lefthook" ".git/hooks/pre-push" 2>/dev/null; then
  echo ""
  echo "✓ lefthook installed successfully"
  echo "  pre-push hook → make ci-local (fmt + vet + test -race + brand-lint)"
  echo ""
  echo "Emergency bypass: LEFTHOOK=0 git push"
else
  echo "WARN: .git/hooks/pre-push 가 lefthook 으로 설정되지 않았을 수 있음." >&2
  echo "  수동 확인: cat .git/hooks/pre-push" >&2
fi
