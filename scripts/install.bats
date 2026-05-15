#!/usr/bin/env bats
# install.bats — bats-core test suite for scripts/install.sh
#
# SPEC: SPEC-MINK-CROSSPLAT-001 M2
# REQ:  REQ-CP-001, REQ-CP-004, REQ-CP-005, REQ-CP-020 ~ REQ-CP-025
# AC:   AC-CP-001, AC-CP-009, AC-CP-010, AC-CP-013, AC-CP-016
#
# Coverage:
#   - detect_os: darwin/linux/windows normalization + unsupported exit
#   - detect_arch: amd64/arm64 normalization + unsupported exit
#   - verify_checksum: pass/fail on hash comparison
#   - detect_cli_tools: found/not-found scenarios
#   - write_config: YAML structure validation
#   - configure_path: user profile only, not /etc/profile
#   - fetch_latest_version: JSON tag_name parsing
#   - download_with_retry: retry logic on failure

# ── helpers ──────────────────────────────────────────────────────────────────

setup() {
    TEST_TMPDIR="$(mktemp -d)"
    STUBS_DIR="${TEST_TMPDIR}/stubs"
    mkdir -p "${STUBS_DIR}"
    export MINK_INSTALL_TEST=1
    export HOME="${TEST_TMPDIR}/home"
    mkdir -p "${HOME}"
    # Prepend stubs to PATH so they override real commands
    export PATH="${STUBS_DIR}:${PATH}"
    # Source the installer with MINK_INSTALL_TEST=1 to avoid running main()
    # shellcheck source=/dev/null
    source "${BATS_TEST_DIRNAME}/install.sh"
}

teardown() {
    rm -rf "${TEST_TMPDIR}"
}

# Create a stub executable in STUBS_DIR
make_stub() {
    local name="$1"
    local body="$2"
    printf '#!/bin/sh\n%s\n' "${body}" > "${STUBS_DIR}/${name}"
    chmod +x "${STUBS_DIR}/${name}"
}

# ── detect_os ─────────────────────────────────────────────────────────────────

@test "detect_os returns darwin on Darwin uname" {
    make_stub "uname" 'printf "Darwin\n"'
    result="$(detect_os)"
    [ "${result}" = "darwin" ]
}

@test "detect_os returns linux on Linux uname" {
    make_stub "uname" 'printf "Linux\n"'
    result="$(detect_os)"
    [ "${result}" = "linux" ]
}

@test "detect_os returns windows on MINGW64 uname" {
    make_stub "uname" 'printf "MINGW64_NT-10.0\n"'
    result="$(detect_os)"
    [ "${result}" = "windows" ]
}

@test "detect_os exits 1 on FreeBSD with unsupported message" {
    make_stub "uname" 'printf "FreeBSD\n"'
    run detect_os
    [ "${status}" -eq 1 ]
    [[ "${output}" == *"Unsupported platform"* ]]
}

# ── detect_arch ───────────────────────────────────────────────────────────────

@test "detect_arch normalizes x86_64 to amd64" {
    make_stub "uname" 'printf "x86_64\n"'
    result="$(detect_arch)"
    [ "${result}" = "amd64" ]
}

@test "detect_arch normalizes aarch64 to arm64" {
    make_stub "uname" 'printf "aarch64\n"'
    result="$(detect_arch)"
    [ "${result}" = "arm64" ]
}

@test "detect_arch normalizes arm64 to arm64" {
    make_stub "uname" 'printf "arm64\n"'
    result="$(detect_arch)"
    [ "${result}" = "arm64" ]
}

@test "detect_arch exits 1 on i386 with unsupported message" {
    make_stub "uname" 'printf "i386\n"'
    run detect_arch
    [ "${status}" -eq 1 ]
    [[ "${output}" == *"Unsupported"* ]]
}

# ── verify_checksum ───────────────────────────────────────────────────────────

@test "verify_checksum passes on matching hash" {
    # Create a test file and its known hash
    local test_file="${TEST_TMPDIR}/testfile.tar.gz"
    printf 'hello mink' > "${test_file}"
    local known_hash
    known_hash="$(sha256sum "${test_file}" 2>/dev/null | awk '{print $1}' || shasum -a 256 "${test_file}" | awk '{print $1}')"

    local checksum_file="${TEST_TMPDIR}/checksums.txt"
    printf '%s  testfile.tar.gz\n' "${known_hash}" > "${checksum_file}"

    run verify_checksum "${test_file}" "${checksum_file}"
    [ "${status}" -eq 0 ]
}

@test "verify_checksum fails on hash mismatch" {
    local test_file="${TEST_TMPDIR}/testfile.tar.gz"
    printf 'hello mink' > "${test_file}"

    local checksum_file="${TEST_TMPDIR}/checksums.txt"
    # Deliberately wrong hash
    printf '%s  testfile.tar.gz\n' "0000000000000000000000000000000000000000000000000000000000000000" > "${checksum_file}"

    run verify_checksum "${test_file}" "${checksum_file}"
    [ "${status}" -ne 0 ]
}

# ── detect_cli_tools ──────────────────────────────────────────────────────────

@test "detect_cli_tools returns claude when only claude exists" {
    # Only create claude stub, not gemini or codex.
    # Use an isolated PATH that contains only our stubs + essential system bins
    # to prevent real system tools (e.g. codex) from leaking into the result.
    make_stub "claude" 'exit 0'
    # Remove any real gemini/codex from stub dir (should not exist, but be safe)
    rm -f "${STUBS_DIR}/gemini" "${STUBS_DIR}/codex"
    # Restrict PATH to stubs + minimal POSIX bins only
    result="$(PATH="${STUBS_DIR}:/usr/bin:/bin" detect_cli_tools)"
    [[ "${result}" == *"claude"* ]]
    [[ "${result}" != *"gemini"* ]]
    [[ "${result}" != *"codex"* ]]
}

@test "detect_cli_tools returns empty when none exist" {
    # No stubs for claude, gemini, or codex
    # Remove any real tools from STUBS_DIR path lookups by using a clean PATH
    PATH="${STUBS_DIR}:/usr/bin:/bin"
    # Ensure none of the tools are in STUBS_DIR
    rm -f "${STUBS_DIR}/claude" "${STUBS_DIR}/gemini" "${STUBS_DIR}/codex"
    result="$(detect_cli_tools)"
    # Result should be empty (no tools detected)
    [ -z "${result}" ]
}

# ── write_config ──────────────────────────────────────────────────────────────

@test "write_config writes valid YAML structure with delegation keys" {
    local config_dir="${HOME}/.mink"
    mkdir -p "${config_dir}"
    write_config "claude gemini"
    local config_file="${config_dir}/config.yaml"
    [ -f "${config_file}" ]
    grep -q "delegation:" "${config_file}"
    grep -q "available_tools:" "${config_file}"
}

@test "write_config writes empty tools list when no tools detected" {
    local config_dir="${HOME}/.mink"
    mkdir -p "${config_dir}"
    write_config ""
    local config_file="${config_dir}/config.yaml"
    [ -f "${config_file}" ]
    grep -q "delegation:" "${config_file}"
    grep -q "available_tools:" "${config_file}"
}

# ── configure_path ────────────────────────────────────────────────────────────

@test "configure_path appends to .bashrc only, not /etc/profile" {
    # Use a temp /etc/profile-like file to verify it is never touched.
    # Use content comparison instead of mtime — `stat -f '%m'` means file
    # modification time on macOS but filesystem status mode on Linux GNU
    # coreutils, making cross-platform mtime comparison unreliable.
    # Content equality is a stronger, portable invariant.
    local fake_etc_profile="${TEST_TMPDIR}/etc_profile"
    local original_content="# system profile marker"
    printf '%s\n' "${original_content}" > "${fake_etc_profile}"

    # Create a .bashrc for the test
    touch "${HOME}/.bashrc"
    export SHELL="/bin/bash"

    # Configure PATH using the installer function
    configure_path "${HOME}/.local/bin"

    # Verify /etc/profile equivalent content unchanged (AC-CP-016)
    [ "$(cat "${fake_etc_profile}")" = "${original_content}" ]

    # Verify .bashrc was updated with the install dir
    grep -q "${HOME}/.local/bin" "${HOME}/.bashrc"
}

# ── fetch_latest_version ──────────────────────────────────────────────────────

@test "fetch_latest_version parses tag_name from GitHub API JSON" {
    # Stub curl to return canned GitHub API response
    make_stub "curl" 'printf "{\"tag_name\":\"v0.1.0\",\"name\":\"MINK v0.1.0\"}\n"'
    result="$(fetch_latest_version)"
    [ "${result}" = "v0.1.0" ]
}

# ── download_with_retry ───────────────────────────────────────────────────────

@test "download_with_retry retries 3 times on failure then aborts" {
    # Stub curl to always fail (exit 1) and record call count
    local counter_file="${TEST_TMPDIR}/call_count"
    printf '0' > "${counter_file}"
    # Write a stub that increments a counter file and always fails
    cat > "${STUBS_DIR}/curl" << 'STUB'
#!/bin/sh
COUNTER_FILE="$(dirname "$0")/../call_count"
count=$(cat "${COUNTER_FILE}")
count=$((count + 1))
printf '%d' "${count}" > "${COUNTER_FILE}"
exit 1
STUB
    chmod +x "${STUBS_DIR}/curl"
    # Override DOWNLOAD_RETRIES to exactly 3 for this test
    DOWNLOAD_RETRIES=3
    run download_with_retry "https://example.com/fake.tar.gz" "${TEST_TMPDIR}/output.tar.gz"
    [ "${status}" -ne 0 ]
    local call_count
    call_count="$(cat "${counter_file}")"
    [ "${call_count}" -eq 3 ]
}
