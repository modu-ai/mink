#!/usr/bin/env bash
# scripts/cli-install-speedrun.sh — CLI onboarding speedrun harness.
#
# Builds the mink binary (or reuses $MINK_BIN if set), runs
# `mink init --yes --dry-run --persona-name "SpeedrunTester"`, measures
# total elapsed time, and asserts the result is within the 3-minute SLA.
#
# Exit codes:
#   0  — PASS (SLA met, exit 0 from mink)
#   1  — FAIL (SLA exceeded or mink exited non-zero)
#
# Environment overrides:
#   MINK_BIN   — absolute path to a pre-built mink binary; skips go build
#   MINK_HOME  — override MINK home directory (defaults to a temp dir)
#   SPEEDRUN_SLA_SECONDS — SLA in seconds (default: 180)
#
# SPEC: SPEC-MINK-ONBOARDING-001 Phase 4 (AC-OB-016)
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
SLA_SECONDS="${SPEEDRUN_SLA_SECONDS:-180}"
PERSONA_NAME="SpeedrunTester"
PASS_SYMBOL="PASS"
FAIL_SYMBOL="FAIL"

# Use a temporary directory for MINK_HOME to avoid polluting the real home.
TEMP_DIR="$(mktemp -d)"
export MINK_HOME="${MINK_HOME:-${TEMP_DIR}/mink-home}"
export MINK_PROJECT_DIR="${TEMP_DIR}/mink-project"
mkdir -p "${MINK_HOME}" "${MINK_PROJECT_DIR}"

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------
cleanup() {
    rm -rf "${TEMP_DIR}"
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Step 1: resolve or build the mink binary
# ---------------------------------------------------------------------------
if [[ -n "${MINK_BIN:-}" ]]; then
    echo "Using pre-built binary: ${MINK_BIN}"
    MINK="${MINK_BIN}"
else
    MINK="${TEMP_DIR}/mink-speedrun"
    echo "Building mink binary -> ${MINK}"
    # Determine repo root (two levels up from this script).
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
    go build -o "${MINK}" "${REPO_ROOT}/cmd/mink" 2>&1
    echo "Build complete."
fi

if [[ ! -x "${MINK}" ]]; then
    echo "ERROR: mink binary not found or not executable: ${MINK}" >&2
    exit 1
fi

# ---------------------------------------------------------------------------
# Step 2: run the speedrun
# ---------------------------------------------------------------------------
echo ""
echo "Running CLI speedrun: mink init --yes --dry-run --persona-name \"${PERSONA_NAME}\""
echo "SLA: ${SLA_SECONDS}s"
echo ""

START_EPOCH="$(date +%s)"

# Capture output; propagate exit code separately.
SPEEDRUN_OUTPUT="$("${MINK}" init --yes --dry-run --persona-name "${PERSONA_NAME}" 2>&1)" || MINK_EXIT=$?
MINK_EXIT="${MINK_EXIT:-0}"

END_EPOCH="$(date +%s)"
ELAPSED=$(( END_EPOCH - START_EPOCH ))

# ---------------------------------------------------------------------------
# Step 3: display output
# ---------------------------------------------------------------------------
echo "${SPEEDRUN_OUTPUT}"
echo ""

# ---------------------------------------------------------------------------
# Step 4: verify exit code
# ---------------------------------------------------------------------------
if [[ "${MINK_EXIT}" -ne 0 ]]; then
    echo "FAIL: mink init --yes exited with code ${MINK_EXIT}" >&2
    exit 1
fi

# ---------------------------------------------------------------------------
# Step 5: verify dry-run artefacts were NOT left on disk
# ---------------------------------------------------------------------------
# In DryRun mode, no files should be written to MINK_HOME.
if find "${MINK_HOME}" -type f | grep -q .; then
    echo "WARN: unexpected files found in MINK_HOME (dry-run should not write):"
    find "${MINK_HOME}" -type f
fi

# ---------------------------------------------------------------------------
# Step 6: assert SLA
# ---------------------------------------------------------------------------
if [[ "${ELAPSED}" -le "${SLA_SECONDS}" ]]; then
    STATUS="${PASS_SYMBOL}"
else
    STATUS="${FAIL_SYMBOL}"
fi

printf "\nCLI speedrun: %ds / %ds SLA — %s\n" "${ELAPSED}" "${SLA_SECONDS}" "${STATUS}"

if [[ "${STATUS}" == "${FAIL_SYMBOL}" ]]; then
    echo "ERROR: CLI speedrun exceeded ${SLA_SECONDS}s SLA (actual: ${ELAPSED}s)" >&2
    exit 1
fi

exit 0
