#!/usr/bin/env bash
# check-brand.sh — AI.GOOSE brand-lint 검증 스크립트
#
# SPEC: SPEC-GOOSE-BRAND-RENAME-001
# REQ:  REQ-BR-006, REQ-BR-008, REQ-BR-012, REQ-BR-019
# AC:   AC-BR-003, AC-BR-004, AC-BR-010
#
# 알고리즘 (§7.5 brand-lint 검증 알고리즘):
# 1. 검사 대상 .md 파일 목록 수집
# 2. 코드 영역(백틱 인용, fenced code block) 제거
# 3. ## HISTORY 섹션 제거 (immutable history 보존)
# 4. 위반 패턴 검출: goose 프로젝트, Goose 프로젝트, GOOSE-AGENT(brand 위치), goose project 등
# 5. SPEC ID 형태(SPEC-GOOSE-*)는 위반에서 제외
# 6. .go / .proto / .sum / .mod 등 코드 파일은 검사 제외
# 7. exit 0 (위반 없음) 또는 exit 1 (위반 있음, 파일:라인:패턴 출력)
#
# 사용법:
#   bash scripts/check-brand.sh
#   bash scripts/check-brand.sh path/to/file.md [path/to/another.md ...]

set -euo pipefail

VIOLATIONS=0
VIOLATION_LINES=()

# ==============================================================================
# 검사 대상 파일 목록
# ==============================================================================
if [ "$#" -gt 0 ]; then
  # 인자로 지정된 파일만 검사
  FILES=()
  for arg in "$@"; do
    FILES+=("$arg")
  done
else
  # 기본 검사 대상 (spec §7.5 §1)
  # SPEC-GOOSE-BRAND-RENAME-001 자기 SPEC은 제외:
  #   - spec.md / research.md가 brand-lint 위반 패턴 자체를 설명(인용)하므로
  #     false positive가 발생함. 이 SPEC의 내용은 brand 규범의 원본 정의이므로 제외.
  while IFS= read -r f; do
    FILES+=("$f")
  done < <(
    find . \
      -name "*.md" \
      ! -path "./.git/*" \
      ! -path "./vendor/*" \
      ! -path "./.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/*" \
      ! -name "*.sum" \
      | sort
  )
fi

# ==============================================================================
# 파일별 검사
# ==============================================================================
for filepath in "${FILES[@]}"; do
  if [ ! -f "$filepath" ]; then
    continue
  fi

  # Python을 사용하여 정교한 마크다운 파싱 수행
  result=$(python3 - "$filepath" <<'PYEOF'
import sys
import re

filepath = sys.argv[1]

with open(filepath, encoding='utf-8', errors='replace') as f:
    content = f.read()

lines = content.split('\n')

violations = []

# 상태 추적
in_fenced_code = False
fenced_fence = ''
in_history_section = False

for lineno, line in enumerate(lines, 1):
    stripped = line.strip()

    # ── fenced code block 진입/탈출 ──────────────────────────────────────────
    fence_match = re.match(r'^(`{3,}|~{3,})', stripped)
    if fence_match:
        fence = fence_match.group(1)
        if not in_fenced_code:
            in_fenced_code = True
            fenced_fence = fence
        elif fence.startswith(fenced_fence):
            in_fenced_code = False
            fenced_fence = ''
        continue

    if in_fenced_code:
        continue  # fenced code block 내부 → 건너뜀

    # ── ## HISTORY 섹션 추적 ─────────────────────────────────────────────────
    if re.match(r'^##\s+HISTORY\b', stripped):
        in_history_section = True
        continue
    if re.match(r'^##\s+', stripped) and in_history_section:
        in_history_section = False
    if in_history_section:
        continue  # HISTORY 섹션 내부 → 건너뜀

    # ── inline code 제거 (백틱 인용) ─────────────────────────────────────────
    # inline code span을 임시 플레이스홀더로 치환하여 패턴 검사에서 제외
    line_without_code = re.sub(r'`[^`]*`', 'CODE_SPAN', line)

    # ── SPEC ID 형태 제거 (SPEC-GOOSE-* 는 SPEC ID이므로 보존) ─────────────
    line_cleaned = re.sub(r'SPEC-GOOSE-[A-Z0-9_-]+', 'SPEC_ID_REF', line_without_code)

    # ── 위반 패턴 검출 ────────────────────────────────────────────────────────
    # 검출 대상 (spec §7.5 §3):
    #   - goose 프로젝트 (대소문자 무관)
    #   - goose project / Goose project (영문)
    #   - GOOSE-AGENT (백틱 외부, brand 위치) — SPEC ID 형태 제외 후 검사
    patterns = [
        (r'(?i)goose\s+프로젝트',    'goose 프로젝트 (brand 위치)'),
        (r'(?i)goose\s+project\b',   'goose project (brand 위치)'),
        (r'\bGOOSE-AGENT\b',         'GOOSE-AGENT (brand 위치, 백틱 외부)'),
    ]

    for pattern, label in patterns:
        if re.search(pattern, line_cleaned):
            violations.append(f'{filepath}:{lineno}: [{label}] {line.rstrip()}')

for v in violations:
    print(v)
PYEOF
  )

  if [ -n "$result" ]; then
    while IFS= read -r vline; do
      VIOLATION_LINES+=("$vline")
      VIOLATIONS=$((VIOLATIONS + 1))
    done <<< "$result"
  fi
done

# ==============================================================================
# 결과 출력
# ==============================================================================
if [ "${VIOLATIONS}" -gt 0 ]; then
  echo "brand-lint: ${VIOLATIONS} violation(s) found" >&2
  echo "" >&2
  for vline in "${VIOLATION_LINES[@]}"; do
    echo "  ${vline}" >&2
  done
  echo "" >&2
  echo "Fix: Replace brand violations with 'AI.GOOSE'." >&2
  echo "  OK: AI.GOOSE는 Daily Companion AI입니다." >&2
  echo "  OK: AI.GOOSE runs the \`goose CLI\`." >&2
  echo "  NG: GOOSE 프로젝트, GOOSE-AGENT, Goose project" >&2
  echo "" >&2
  echo "See .moai/project/brand/style-guide.md for brand notation rules." >&2
  exit 1
else
  echo "brand-lint: OK — 0 violations found."
  exit 0
fi
