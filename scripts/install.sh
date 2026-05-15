#!/bin/sh
# install.sh — MINK Unix installer
#
# SPEC: SPEC-MINK-CROSSPLAT-001 M2 + M4 + M5
# REQ:  REQ-CP-001, REQ-CP-004, REQ-CP-005, REQ-CP-006, REQ-CP-007, REQ-CP-008,
#       REQ-CP-009, REQ-CP-010, REQ-CP-011, REQ-CP-012, REQ-CP-013, REQ-CP-014,
#       REQ-CP-020 ~ REQ-CP-026
# AC:   AC-CP-001, AC-CP-004, AC-CP-005, AC-CP-006, AC-CP-007, AC-CP-008,
#       AC-CP-009, AC-CP-010, AC-CP-013, AC-CP-015, AC-CP-016
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
# Fails fast if the response is empty, the tag_name field is missing, or the
# parsed value does not look like a valid version tag (v<digits>...).
fetch_latest_version() {
    _api_url="https://api.github.com/repos/${MINK_REPO}/releases/latest"
    _response="$(curl -fsSL "${_api_url}")"

    if [ -z "${_response}" ]; then
        log_error "GitHub API returned an empty response for ${_api_url}"
        exit 1
    fi

    # Extract "tag_name":"v0.1.0" → v0.1.0
    # Works with both `"tag_name": "v0.1.0"` and `"tag_name":"v0.1.0"` formats.
    # If the sed pattern does not match, the substitution leaves the input
    # unchanged — detect that case by comparing length with the raw response.
    _tag="$(printf '%s' "${_response}" \
        | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
        | head -n 1)"

    if [ -z "${_tag}" ]; then
        log_error "Failed to parse tag_name from GitHub Releases API."
        log_error "API URL: ${_api_url}"
        log_error "Check that the repository has at least one published release."
        exit 1
    fi

    # Validate tag format: must start with 'v' followed by a digit.
    # This catches cases where unrelated JSON content slipped through the sed.
    case "${_tag}" in
        v[0-9]*) : ;;
        *)
            log_error "Parsed tag_name does not look like a version: ${_tag}"
            log_error "Expected format: v<major>.<minor>.<patch> (e.g. v0.1.0)"
            exit 1
            ;;
    esac

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

# ── M4: Ollama runtime management ─────────────────────────────────────────────

# detect_ollama: check whether Ollama is installed and responding.
# Prints "installed <version>" to stdout when found.
# Returns 1 (exit non-zero) when Ollama is not installed or not responding.
detect_ollama() {
    if ! command -v ollama >/dev/null 2>&1; then
        return 1
    fi
    _ver="$(ollama --version 2>/dev/null || true)"
    printf 'installed %s' "${_ver}"
}

# install_ollama: install Ollama using the platform-appropriate method.
# macOS: prefers Homebrew; falls back to manual download guidance.
# Linux: uses the official install.sh script (3 curl retries).
# Other platforms: graceful failure (log_warn, return 1 — never exit 1).
install_ollama() {
    _ollama_os="$(detect_os 2>/dev/null || true)"
    case "${_ollama_os}" in
        darwin)
            if command -v brew >/dev/null 2>&1; then
                log_info "Installing Ollama via Homebrew..."
                if brew install ollama; then
                    return 0
                fi
                log_warn "Homebrew install of Ollama failed. Install manually: https://ollama.com/download"
                return 1
            else
                log_warn "Homebrew not found. Install Ollama manually: https://ollama.com/download"
                return 1
            fi
            ;;
        linux)
            log_info "Installing Ollama via official installer..."
            _attempt=0
            _delay=1
            while [ "${_attempt}" -lt 3 ]; do
                _attempt=$((_attempt + 1))
                log_info "Ollama install attempt ${_attempt}/3..."
                if curl -fsSL https://ollama.com/install.sh | sh; then
                    return 0
                fi
                if [ "${_attempt}" -lt 3 ]; then
                    log_warn "Ollama installer failed, retrying in ${_delay}s..."
                    sleep "${_delay}"
                    _delay=$((_delay * 2))
                fi
            done
            log_warn "Ollama installation failed. Install manually: https://ollama.com"
            return 1
            ;;
        *)
            log_warn "Ollama auto-install not supported on this platform."
            log_warn "Install manually: https://ollama.com"
            return 1
            ;;
    esac
}

# start_ollama_service: start the Ollama background service.
# macOS: attempts Ollama.app launch first, falls back to `ollama serve`.
# Linux: starts `ollama serve` in background (disowned).
# Idempotent: does nothing if service is already running.
start_ollama_service() {
    # Check if service is already responding — idempotent path
    if ollama list >/dev/null 2>&1; then
        log_info "Ollama service already running"
        return 0
    fi

    _svc_os="$(detect_os 2>/dev/null || true)"
    case "${_svc_os}" in
        darwin)
            # Try the Ollama.app bundle first (silent if absent)
            if command -v open >/dev/null 2>&1; then
                open -a Ollama 2>/dev/null || true
            fi
            # Always also attempt `ollama serve` as fallback
            if command -v ollama >/dev/null 2>&1; then
                ollama serve >/dev/null 2>&1 &
                disown 2>/dev/null || true
            fi
            ;;
        *)
            # Linux and others: serve in background
            if command -v ollama >/dev/null 2>&1; then
                ollama serve >/dev/null 2>&1 &
                disown 2>/dev/null || true
            fi
            ;;
    esac
}

# wait_for_ollama: poll until Ollama service responds or timeout is reached.
# Polls every 1 second for up to 30 seconds.
# Returns 0 on success, 1 on timeout.
wait_for_ollama() {
    _retries=30
    _count=0
    while [ "${_count}" -lt "${_retries}" ]; do
        if ollama list >/dev/null 2>&1; then
            return 0
        fi
        _count=$((_count + 1))
        sleep 1
    done
    return 1
}

# ── M5: RAM detection and model selection ──────────────────────────────────────

# detect_ram_gb: detect total system RAM in gigabytes (integer, floored).
# Linux: reads /proc/meminfo (MemTotal in kB → GB).
# macOS: uses sysctl hw.memsize (bytes → GB).
# Returns 0 with GB count on stdout. Returns 1 and prints "0" on failure.
detect_ram_gb() {
    if [ -f /proc/meminfo ]; then
        # Linux: MemTotal is in kB; divide by 1024*1024 = 1048576 to get GB
        _gb="$(awk '/^MemTotal:/ {print int($2/1048576)}' /proc/meminfo 2>/dev/null)"
        if [ -n "${_gb}" ] && [ "${_gb}" -gt 0 ] 2>/dev/null; then
            printf '%s' "${_gb}"
            return 0
        fi
    fi

    _hw_memsize="$(sysctl -n hw.memsize 2>/dev/null || true)"
    if [ -n "${_hw_memsize}" ] && [ "${_hw_memsize}" -gt 0 ] 2>/dev/null; then
        _gb="$(printf '%s' "${_hw_memsize}" | awk '{print int($1/1073741824)}')"
        printf '%s' "${_gb}"
        return 0
    fi

    printf '0'
    return 1
}

# select_model: select the appropriate Ollama model based on RAM (GB).
# Mapping per REQ-CP-011:
#   < 8 GB  → ai-mink/gemma4-e2b-rl-v1
#   8-15 GB → ai-mink/gemma4-e4b-rl-v1:q4_k_m
#   16-31 GB → ai-mink/gemma4-e4b-rl-v1:q5_k_m
#   32+ GB  → ai-mink/gemma4-e4b-rl-v1:q8_0
# Arguments: $1 = RAM in GB (integer)
# Prints model name to stdout.
select_model() {
    _ram="$1"
    if [ "${_ram}" -lt 8 ] 2>/dev/null; then
        printf 'ai-mink/gemma4-e2b-rl-v1'
    elif [ "${_ram}" -lt 16 ] 2>/dev/null; then
        printf 'ai-mink/gemma4-e4b-rl-v1:q4_k_m'
    elif [ "${_ram}" -lt 32 ] 2>/dev/null; then
        printf 'ai-mink/gemma4-e4b-rl-v1:q5_k_m'
    else
        printf 'ai-mink/gemma4-e4b-rl-v1:q8_0'
    fi
}

# pull_model: download an Ollama model, streaming progress to the terminal.
# Arguments: $1 = full model name (e.g. ai-mink/gemma4-e4b-rl-v1:q4_k_m)
# Returns 0 on success, non-zero on failure.
pull_model() {
    _model="$1"
    ollama pull "${_model}"
}

# verify_model: confirm that a model appears in `ollama list`.
# Uses the base model name without the :tag suffix for matching.
# Arguments: $1 = full model name (may include :tag)
# Returns 0 if found, 1 if not found.
verify_model() {
    _model="$1"
    # Extract the name portion before the colon (strip :tag)
    _model_short="$(printf '%s' "${_model}" | cut -d: -f1)"
    ollama list 2>/dev/null | grep -qF "${_model_short}"
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

    # Download to temp directory. Register cleanup trap immediately so partial
    # downloads or intermediate failures (verify_checksum, install_binary, etc.)
    # do not leak temp artifacts. The trap uses single quotes so _tmp_dir is
    # expanded at trap-fire time, after the value is fully set.
    _tmp_dir="${TMPDIR:-/tmp}/mink_install_$$"
    mkdir -p "${_tmp_dir}"
    # shellcheck disable=SC2064
    trap "rm -rf '${_tmp_dir}'" EXIT INT HUP TERM

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

    # Cleanup runs automatically via the EXIT trap registered earlier.

    # ── Ollama auto-install + model setup (M4 + M5) ───────────────────────────
    # All failures below are graceful per REQ-CP-026.
    # Binary installation is always successful regardless of Ollama/model status.
    log_info ""
    log_info "Setting up Ollama runtime..."

    _ollama_ok=0
    if detect_ollama >/dev/null 2>&1; then
        log_info "Ollama already installed (skipping install)"
        _ollama_ok=1
    else
        log_info "Ollama not found. Installing..."
        if install_ollama; then
            _ollama_ok=1
        else
            log_warn "Ollama installation failed. Skipping model setup."
            log_warn "Install manually: https://ollama.com"
        fi
    fi

    if [ "${_ollama_ok}" = "1" ]; then
        log_info "Starting Ollama service..."
        start_ollama_service
        if wait_for_ollama; then
            log_info "Ollama service responding"
            _ram_gb="$(detect_ram_gb 2>/dev/null || printf '0')"
            if [ "${_ram_gb}" -gt 0 ] 2>/dev/null; then
                _model="$(select_model "${_ram_gb}")"
                log_info "System RAM: ${_ram_gb} GB -> Model: ${_model}"
                if pull_model "${_model}"; then
                    if verify_model "${_model}"; then
                        log_info "Model ready: ${_model}"
                    else
                        log_warn "Model verification failed (ollama list missing entry)"
                    fi
                else
                    log_warn "Model download failed. Pull manually: ollama pull ${_model}"
                fi
            else
                log_warn "RAM detection failed. Skipping model auto-selection."
            fi
        else
            log_warn "Ollama service did not respond within 30s. Start manually."
        fi
    fi

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
