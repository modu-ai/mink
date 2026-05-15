#!/bin/sh
# install.sh — MINK Unix installer
#
# SPEC: SPEC-MINK-CROSSPLAT-001 M2
# REQ:  REQ-CP-001, REQ-CP-004, REQ-CP-005, REQ-CP-020 ~ REQ-CP-025
# AC:   AC-CP-001, AC-CP-009, AC-CP-010, AC-CP-013, AC-CP-016
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/modu-ai/mink/main/scripts/install.sh | sh
#   MINK_INSTALL_TEST=1 . scripts/install.sh   # source-only for bats testing
#
# Supported platforms:
#   OS:   darwin, linux, windows (Git Bash / MSYS2 / Cygwin)
#   Arch: amd64 (x86_64), arm64 (aarch64)
#
# All inline comments are in English per .moai/config/sections/language.yaml.

set -eu

# ── Global configuration ──────────────────────────────────────────────────────

MINK_REPO="modu-ai/mink"
INSTALL_DIR="${HOME}/.local/bin"
CONFIG_DIR="${HOME}/.mink"
DOWNLOAD_RETRIES=3

# ── Logging helpers ───────────────────────────────────────────────────────────

log_info() {
    printf '==> %s\n' "$1"
}

log_warn() {
    printf 'Warning: %s\n' "$1" >&2
}

log_error() {
    printf 'Error: %s\n' "$1" >&2
}

# ── Platform detection ────────────────────────────────────────────────────────

# detect_os: normalize uname -s output to "darwin" | "linux" | "windows"
# Prints the normalized OS name to stdout; exits 1 on unsupported platform.
detect_os() {
    _raw_os="$(uname -s)"
    case "${_raw_os}" in
        Darwin)
            printf 'darwin'
            ;;
        Linux)
            printf 'linux'
            ;;
        MINGW*|MSYS*|CYGWIN*)
            printf 'windows'
            ;;
        *)
            log_error "Unsupported platform: ${_raw_os}"
            log_error "Supported platforms: darwin, linux, windows (MINGW/MSYS/Cygwin)"
            return 1
            ;;
    esac
}

# detect_arch: normalize uname -m output to "amd64" | "arm64"
# Prints the normalized architecture to stdout; exits 1 on unsupported arch.
detect_arch() {
    _raw_arch="$(uname -m)"
    case "${_raw_arch}" in
        x86_64|amd64)
            printf 'amd64'
            ;;
        arm64|aarch64)
            printf 'arm64'
            ;;
        *)
            log_error "Unsupported CPU architecture: ${_raw_arch}"
            log_error "Supported architectures: amd64 (x86_64), arm64 (aarch64)"
            return 1
            ;;
    esac
}

# ── Release lookup ────────────────────────────────────────────────────────────

# fetch_latest_version: query GitHub Releases API and parse tag_name without jq.
# Uses sed/awk to extract the tag_name field from the JSON response.
# Prints the tag string (e.g. "v0.1.0") to stdout.
fetch_latest_version() {
    _api_url="https://api.github.com/repos/${MINK_REPO}/releases/latest"
    _response="$(curl -fsSL "${_api_url}")"
    # Extract "tag_name":"v0.1.0" → v0.1.0
    # Works with both `"tag_name": "v0.1.0"` and `"tag_name":"v0.1.0"` formats
    _tag="$(printf '%s' "${_response}" \
        | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')"
    printf '%s' "${_tag}"
}

# ── Download ──────────────────────────────────────────────────────────────────

# download_with_retry: download a URL to a destination file.
# Retries up to DOWNLOAD_RETRIES times with exponential backoff.
# Arguments: $1 = URL, $2 = destination file path
download_with_retry() {
    _url="$1"
    _dest="$2"
    _attempt=0
    _delay=1

    while [ "${_attempt}" -lt "${DOWNLOAD_RETRIES}" ]; do
        _attempt=$((_attempt + 1))
        log_info "Downloading (attempt ${_attempt}/${DOWNLOAD_RETRIES}): ${_url}"
        if curl -fsSL -o "${_dest}" "${_url}"; then
            return 0
        fi
        if [ "${_attempt}" -lt "${DOWNLOAD_RETRIES}" ]; then
            log_warn "Download failed, retrying in ${_delay}s..."
            sleep "${_delay}"
            _delay=$((_delay * 2))
        fi
    done

    log_error "Download failed after ${DOWNLOAD_RETRIES} attempts: ${_url}"
    return 1
}

# ── Checksum verification ─────────────────────────────────────────────────────

# verify_checksum: verify SHA-256 checksum of a downloaded file.
# Uses sha256sum (Linux) with fallback to shasum -a 256 (macOS).
# Arguments: $1 = file to verify, $2 = checksums.txt file path
# Exits non-zero if verification fails.
verify_checksum() {
    _file="$1"
    _checksums="$2"
    _filename="$(basename "${_file}")"

    # Extract the expected hash for this specific filename from checksums.txt
    _expected="$(grep "[[:space:]]${_filename}$" "${_checksums}" | awk '{print $1}')"
    if [ -z "${_expected}" ]; then
        log_error "No checksum entry found for ${_filename} in ${_checksums}"
        return 1
    fi

    # Compute actual hash; prefer sha256sum (Linux/Git Bash), fall back to shasum (macOS)
    if command -v sha256sum >/dev/null 2>&1; then
        _actual="$(sha256sum "${_file}" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
        _actual="$(shasum -a 256 "${_file}" | awk '{print $1}')"
    else
        log_error "No SHA-256 tool found (sha256sum or shasum required)"
        return 1
    fi

    if [ "${_actual}" = "${_expected}" ]; then
        log_info "Checksum verified: ${_filename}"
        return 0
    else
        log_error "Checksum mismatch for ${_filename}"
        log_error "  Expected: ${_expected}"
        log_error "  Actual:   ${_actual}"
        return 1
    fi
}

# ── Binary installation ───────────────────────────────────────────────────────

# install_binary: extract archive, set permissions, copy binary to INSTALL_DIR.
# Arguments: $1 = archive file path, $2 = target install directory
install_binary() {
    _archive="$1"
    _target_dir="$2"
    _work_dir="${TEST_TMPDIR:-/tmp}/mink_extract_$$"

    mkdir -p "${_work_dir}"
    mkdir -p "${_target_dir}"

    # Extract archive (tar.gz for Linux/macOS, zip for Windows)
    case "${_archive}" in
        *.zip)
            unzip -q "${_archive}" -d "${_work_dir}"
            ;;
        *.tar.gz|*.tgz)
            tar -xzf "${_archive}" -C "${_work_dir}"
            ;;
        *)
            log_error "Unrecognized archive format: ${_archive}"
            rm -rf "${_work_dir}"
            return 1
            ;;
    esac

    # Install mink binary
    if [ -f "${_work_dir}/mink" ]; then
        chmod +x "${_work_dir}/mink"
        cp "${_work_dir}/mink" "${_target_dir}/mink"
        log_info "Installed mink to ${_target_dir}/mink"
    else
        log_error "mink binary not found in archive"
        rm -rf "${_work_dir}"
        return 1
    fi

    # Install minkd daemon binary if present
    if [ -f "${_work_dir}/minkd" ]; then
        chmod +x "${_work_dir}/minkd"
        cp "${_work_dir}/minkd" "${_target_dir}/minkd"
        log_info "Installed minkd to ${_target_dir}/minkd"
    fi

    rm -rf "${_work_dir}"
}

# ── Shell profile detection ───────────────────────────────────────────────────

# detect_shell_profile: determine the appropriate user shell profile file.
# Checks $SHELL to select .bashrc / .zshrc, defaults to .profile.
# Never returns /etc/profile or system-wide paths (REQ-CP-024).
detect_shell_profile() {
    case "${SHELL:-}" in
        */zsh)
            printf '%s/.zshrc' "${HOME}"
            ;;
        */bash)
            printf '%s/.bashrc' "${HOME}"
            ;;
        *)
            # Generic POSIX fallback — user profile only
            printf '%s/.profile' "${HOME}"
            ;;
    esac
}

# configure_path: idempotently add INSTALL_DIR to the user's PATH in their
# shell profile. Uses a marker comment to prevent duplicate entries.
# Arguments: $1 = directory to add to PATH
# NEVER modifies /etc/profile, /etc/environment, or any system-wide file.
configure_path() {
    _dir="$1"
    _profile="$(detect_shell_profile)"
    _marker="# Added by MINK installer"

    # Create profile file if it does not exist
    if [ ! -f "${_profile}" ]; then
        touch "${_profile}"
    fi

    # Idempotent: skip if the marker is already present
    if grep -qF "${_marker}" "${_profile}" 2>/dev/null; then
        log_info "PATH already configured in ${_profile}"
        return 0
    fi

    # Append PATH configuration with idempotency marker
    printf '\n%s\nexport PATH="%s:${PATH}"\n' "${_marker}" "${_dir}" >> "${_profile}"
    log_info "Added ${_dir} to PATH in ${_profile}"
}

# ── CLI tool detection ────────────────────────────────────────────────────────

# detect_cli_tools: scan PATH for known external CLI tools used by MINK.
# Returns a space-separated list of found tool names, or empty string.
# Missing tools do NOT block installation (REQ-CP-022).
# Uses `command -v` (POSIX) not `which`.
detect_cli_tools() {
    _found=""
    for _tool in claude gemini codex; do
        if command -v "${_tool}" >/dev/null 2>&1; then
            if [ -z "${_found}" ]; then
                _found="${_tool}"
            else
                _found="${_found} ${_tool}"
            fi
        fi
    done
    printf '%s' "${_found}"
}

# ── Config file generation ────────────────────────────────────────────────────

# write_config: write ~/.mink/config.yaml with detected delegation tools.
# Arguments: $1 = space-separated list of detected tool names (may be empty)
write_config() {
    _tools="$1"
    _config_file="${CONFIG_DIR}/config.yaml"

    mkdir -p "${CONFIG_DIR}"

    # Build YAML tools list (hyphen-prefixed entries per YAML list syntax)
    _tools_yaml=""
    if [ -n "${_tools}" ]; then
        for _t in ${_tools}; do
            _tools_yaml="${_tools_yaml}    - ${_t}
"
        done
    fi

    # Write configuration file using printf to avoid heredoc variable expansion issues
    printf '# MINK configuration\n' > "${_config_file}"
    printf '# Generated by install.sh - edit manually to customize\n' >> "${_config_file}"
    printf 'version: "1"\n' >> "${_config_file}"
    printf '\n' >> "${_config_file}"
    printf 'delegation:\n' >> "${_config_file}"
    printf '  available_tools:\n' >> "${_config_file}"
    if [ -n "${_tools_yaml}" ]; then
        printf '%s' "${_tools_yaml}" >> "${_config_file}"
    fi

    log_info "Configuration written to ${_config_file}"
}

# ── Error helpers ─────────────────────────────────────────────────────────────

# error_unsupported_platform: print a helpful error listing supported platforms.
error_unsupported_platform() {
    log_error "Unsupported platform: $1"
    log_error "MINK supports the following platforms:"
    log_error "  OS:   darwin, linux, windows (MINGW / MSYS2 / Cygwin)"
    log_error "  Arch: amd64 (x86_64), arm64 (aarch64)"
    exit 1
}

# ── Main installer entry point ────────────────────────────────────────────────

main() {
    log_info "MINK installer starting..."

    # Detect platform
    _os="$(detect_os)"
    _arch="$(detect_arch)"
    log_info "Platform detected: ${_os}/${_arch}"

    # Resolve latest release version
    log_info "Querying latest MINK release..."
    _version="$(fetch_latest_version)"
    # Strip leading 'v' for archive name construction
    _ver_num="$(printf '%s' "${_version}" | sed 's/^v//')"
    log_info "Latest version: ${_version}"

    # Determine archive extension
    case "${_os}" in
        windows)
            _ext="zip"
            ;;
        *)
            _ext="tar.gz"
            ;;
    esac

    # Construct download URLs
    _archive_name="mink_${_ver_num}_${_os}_${_arch}.${_ext}"
    _base_url="https://github.com/${MINK_REPO}/releases/download/${_version}"
    _archive_url="${_base_url}/${_archive_name}"
    _checksums_url="${_base_url}/checksums.txt"

    # Download to temp directory
    _tmp_dir="${TMPDIR:-/tmp}/mink_install_$$"
    mkdir -p "${_tmp_dir}"

    _archive_path="${_tmp_dir}/${_archive_name}"
    _checksums_path="${_tmp_dir}/checksums.txt"

    # All downloads over HTTPS only (REQ-CP-023)
    log_info "Downloading checksums..."
    download_with_retry "${_checksums_url}" "${_checksums_path}"

    log_info "Downloading ${_archive_name}..."
    download_with_retry "${_archive_url}" "${_archive_path}"

    # Verify checksum BEFORE extraction (REQ-CP-023)
    log_info "Verifying checksum..."
    verify_checksum "${_archive_path}" "${_checksums_path}"

    # Install binary
    install_binary "${_archive_path}" "${INSTALL_DIR}"

    # Configure PATH in user shell profile (never /etc/profile — REQ-CP-024)
    configure_path "${INSTALL_DIR}"

    # Detect external CLI tools (REQ-CP-020)
    log_info "Scanning for external CLI tools (claude, gemini, codex)..."
    _tools="$(detect_cli_tools)"

    if [ -z "${_tools}" ]; then
        log_info "No external CLI tools detected (local mode)"
    else
        log_info "Detected CLI tools: ${_tools}"
    fi

    # Write config (REQ-CP-021); missing tools do not block install (REQ-CP-022)
    write_config "${_tools}"

    # Cleanup temp files
    rm -rf "${_tmp_dir}"

    log_info ""
    log_info "MINK ${_version} installed successfully!"
    log_info "  Binary:  ${INSTALL_DIR}/mink"
    log_info "  Config:  ${CONFIG_DIR}/config.yaml"
    log_info ""
    log_info "To use mink immediately, run:"
    log_info "  export PATH=\"${INSTALL_DIR}:\${PATH}\""
    log_info "Or start a new shell session."
}

# Guard: when MINK_INSTALL_TEST=1, source-only mode (defines functions, skips main).
# This allows bats tests to import individual functions without triggering installation.
if [ "${MINK_INSTALL_TEST:-0}" != "1" ]; then
    main "$@"
fi
