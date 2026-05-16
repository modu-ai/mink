// Package onboarding — model_setup.go provides runtime detection helpers for
// Ollama installation, MINK model presence, system RAM, and model pull progress.
// All detection functions are pure I/O operations with no side effects on
// onboarding state; callers (Phase 2 TUI / Phase 3 Web) decide when to invoke them.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2
// REQ: REQ-OB-021, REQ-CP-010, REQ-CP-011
package onboarding

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// -------------------------------------------------------------------------
// Sentinel errors
// -------------------------------------------------------------------------

var (
	// ErrOllamaProbeFailed is returned when the ollama binary probe fails unexpectedly.
	ErrOllamaProbeFailed = errors.New("model_setup: ollama probe failed")
	// ErrModelDetection is returned when MINK model detection fails via all available methods.
	ErrModelDetection = errors.New("model_setup: MINK model detection failed")
	// ErrRAMDetection is returned when the platform-specific RAM probe fails.
	ErrRAMDetection = errors.New("model_setup: RAM detection failed")
	// ErrPullFailed is returned when `ollama pull` exits with a non-zero status.
	ErrPullFailed = errors.New("model_setup: ollama pull failed")
	// ErrUnsupportedOS is returned when the current GOOS has no RAM probe implementation.
	ErrUnsupportedOS = errors.New("model_setup: unsupported GOOS")
)

// -------------------------------------------------------------------------
// Exec and HTTP indirection for testability
// -------------------------------------------------------------------------

// execCommandContext is exec.CommandContext, indirected for test injection.
// @MX:WARN: [AUTO] Global mutable state — tests must restore via t.Cleanup.
// @MX:REASON: Package-level indirection is required for the TestHelperProcess pattern;
// any race between parallel tests and this variable would corrupt test results.
var execCommandContext = exec.CommandContext

// httpGet issues an HTTP GET with the provided context, indirected for test injection.
var httpGet = func(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

// -------------------------------------------------------------------------
// RAM detection: platform dispatch table
// -------------------------------------------------------------------------

// ramProbes maps GOOS to platform-specific RAM detection functions.
var ramProbes = map[string]func(context.Context) (int64, error){
	"linux":   detectRAMLinux,
	"darwin":  detectRAMDarwin,
	"windows": detectRAMWindows,
}

// detectRAMOverride is used by tests to substitute the platform-dispatched probe.
// When non-nil, DetectRAM calls this function instead of dispatching via ramProbes.
var detectRAMOverride func(context.Context) (int64, error)

// -------------------------------------------------------------------------
// Public types
// -------------------------------------------------------------------------

// OllamaStatus is the result of DetectOllama.
type OllamaStatus struct {
	Installed   bool   // true if `ollama` binary is in PATH
	BinaryPath  string // absolute path returned by exec.LookPath (empty if not installed)
	DaemonAlive bool   // true if HTTP GET /api/tags responds (best-effort, false on timeout)
}

// DetectedModel is the result of DetectMINKModel.
type DetectedModel struct {
	Name      string // first model name starting with "ai-mink/" (empty if none)
	SizeBytes int64  // model size in bytes (best-effort, 0 if unavailable)
}

// ProgressUpdate is emitted on the progress channel during PullModel.
type ProgressUpdate struct {
	Phase       string  // "pulling manifest" | "downloading layer" | "verifying" | "writing manifest" | "success"
	Layer       string  // short layer digest, e.g., "sha256:a1b2c3..." (may be empty)
	BytesTotal  int64   // total bytes for the current phase (0 if unknown)
	BytesDone   int64   // bytes downloaded so far (0 if unknown)
	PercentDone float64 // 0..100, derived from BytesDone/BytesTotal (-1 if unknown)
	Raw         string  // raw stdout line, retained for debugging
}

// -------------------------------------------------------------------------
// DetectOllama
// -------------------------------------------------------------------------

// DetectOllama probes the system PATH for the ollama binary and queries the daemon HTTP endpoint.
// Returns OllamaStatus with the probe results. HTTP probe uses a 2 second timeout.
// Returns nil error on every code path — absence of ollama is normal, not exceptional.
//
// @MX:ANCHOR: [AUTO] Called by DetectMINKModel and potentially by flow.go step 2 — fan_in >= 2.
// @MX:REASON: OllamaStatus.Installed gates all downstream model detection paths.
func DetectOllama(ctx context.Context) (OllamaStatus, error) {
	path, err := execLookPath("ollama")
	if err != nil {
		// Not installed — normal path, not an error.
		return OllamaStatus{Installed: false}, nil
	}

	status := OllamaStatus{
		Installed:  true,
		BinaryPath: path,
	}

	// Probe the daemon with a 2-second HTTP timeout.
	httpCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := httpGet(httpCtx, "http://localhost:11434/api/tags")
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			status.DaemonAlive = true
		}
	}
	// HTTP failure (daemon down) is not an error — just DaemonAlive=false.

	return status, nil
}

// -------------------------------------------------------------------------
// DetectMINKModel
// -------------------------------------------------------------------------

// ollamaTagsResponse is the JSON shape returned by GET /api/tags.
type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	} `json:"models"`
}

// DetectMINKModel queries the Ollama daemon for installed models and returns the first
// model whose name starts with "ai-mink/". Tries HTTP GET /api/tags first; on HTTP failure
// (daemon not running, network error), falls back to `ollama list` stdout parsing.
// Returns empty DetectedModel{} (Name="", SizeBytes=0) and nil error when no ai-mink/ model
// is found via either method. Returns wrapped ErrModelDetection only when both methods fail
// AND the failure is unrecoverable (e.g., malformed JSON from a running daemon).
func DetectMINKModel(ctx context.Context, status OllamaStatus) (DetectedModel, error) {
	if !status.Installed {
		// No ollama binary — nothing to probe.
		return DetectedModel{}, nil
	}

	// Attempt HTTP probe first (more deterministic than stdout parsing).
	if model, ok, err := detectMINKModelHTTP(ctx); err != nil {
		// HTTP returned data but it was malformed — unrecoverable.
		return DetectedModel{}, fmt.Errorf("%w: HTTP response parse error: %v", ErrModelDetection, err)
	} else if ok {
		return model, nil
	}

	// HTTP was unavailable (daemon down) — fall back to exec.
	model, err := detectMINKModelExec(ctx)
	if err != nil {
		return DetectedModel{}, fmt.Errorf("%w: exec fallback failed: %v", ErrModelDetection, err)
	}
	return model, nil
}

// detectMINKModelHTTP tries the Ollama HTTP API.
// Returns (model, true, nil) when the daemon responded with a parseable list AND
// a MINK model was found.
// Returns ({}, true, nil) when the daemon responded with a parseable list but NO
// MINK model is present — the daemon-alive signal suppresses the exec fallback.
// Returns ({}, false, nil) when the daemon is unreachable or returns a non-200
// status — the caller falls back to exec.
// Returns ({}, false, err) only on JSON parse failure from a live daemon (200 OK
// with malformed body) — an unrecoverable condition.
func detectMINKModelHTTP(ctx context.Context) (DetectedModel, bool, error) {
	httpCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := httpGet(httpCtx, "http://localhost:11434/api/tags")
	if err != nil {
		// Daemon not running or unreachable — not an error condition.
		return DetectedModel{}, false, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return DetectedModel{}, false, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return DetectedModel{}, false, err
	}

	var tags ollamaTagsResponse
	if err := json.Unmarshal(body, &tags); err != nil {
		return DetectedModel{}, false, err
	}

	for _, m := range tags.Models {
		if strings.HasPrefix(m.Name, "ai-mink/") {
			return DetectedModel{Name: m.Name, SizeBytes: m.Size}, true, nil
		}
	}
	// Daemon responded but no MINK model is installed. Return ok=true so the
	// caller does NOT fall back to exec — the daemon is authoritative here.
	return DetectedModel{}, true, nil
}

// detectMINKModelExec runs `ollama list` and parses stdout for ai-mink/ models.
// Returns an error only when exec fails (non-zero exit code).
func detectMINKModelExec(ctx context.Context) (DetectedModel, error) {
	cmd := execCommandContext(ctx, "ollama", "list")
	out, err := cmd.Output()
	if err != nil {
		return DetectedModel{}, err
	}

	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if i == 0 {
			// Skip header row (NAME\tID\tSIZE\tMODIFIED).
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// First field is the model name; fields are tab- or space-separated.
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if strings.HasPrefix(name, "ai-mink/") {
			// SizeBytes is best-effort from exec (no reliable byte count in list output).
			return DetectedModel{Name: name, SizeBytes: 0}, nil
		}
	}
	// No MINK model found — not an error.
	return DetectedModel{}, nil
}

// -------------------------------------------------------------------------
// DetectRAM
// -------------------------------------------------------------------------

// DetectRAM returns total system RAM in bytes, using platform-specific methods:
//   - linux:   /proc/meminfo MemTotal (kB → bytes)
//   - darwin:  `sysctl -n hw.memsize` (bytes)
//   - windows: `wmic ComputerSystem get TotalPhysicalMemory /value` (bytes)
//
// Returns wrapped ErrRAMDetection for unsupported GOOS or when the platform-specific
// probe fails (file not readable, exec returns non-zero, output unparseable).
func DetectRAM(ctx context.Context) (int64, error) {
	if detectRAMOverride != nil {
		return detectRAMOverride(ctx)
	}

	probe, ok := ramProbes[runtime.GOOS]
	if !ok {
		return 0, fmt.Errorf("%w: GOOS=%s", ErrUnsupportedOS, runtime.GOOS)
	}
	return probe(ctx)
}

// detectRAMLinux reads /proc/meminfo and parses the MemTotal line.
func detectRAMLinux(_ context.Context) (int64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, fmt.Errorf("%w: cannot read /proc/meminfo: %v", ErrRAMDetection, err)
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		// Format: "MemTotal:     16384000 kB"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			break
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			break
		}
		return kb * 1024, nil
	}
	return 0, fmt.Errorf("%w: MemTotal not found in /proc/meminfo", ErrRAMDetection)
}

// detectRAMDarwin executes `sysctl -n hw.memsize` and parses the output as bytes.
func detectRAMDarwin(ctx context.Context) (int64, error) {
	cmd := execCommandContext(ctx, "sysctl", "-n", "hw.memsize")
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("%w: sysctl failed: %v", ErrRAMDetection, err)
	}
	val := strings.TrimSpace(string(out))
	bytes, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: cannot parse sysctl output %q: %v", ErrRAMDetection, val, err)
	}
	return bytes, nil
}

// detectRAMWindows executes `wmic ComputerSystem get TotalPhysicalMemory /value` and parses the result.
func detectRAMWindows(ctx context.Context) (int64, error) {
	cmd := execCommandContext(ctx, "wmic", "ComputerSystem", "get", "TotalPhysicalMemory", "/value")
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("%w: wmic failed: %v", ErrRAMDetection, err)
	}

	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "TotalPhysicalMemory=") {
			continue
		}
		val := strings.TrimPrefix(line, "TotalPhysicalMemory=")
		val = strings.TrimSpace(val)
		bytes, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: cannot parse wmic output %q: %v", ErrRAMDetection, val, err)
		}
		return bytes, nil
	}
	return 0, fmt.Errorf("%w: TotalPhysicalMemory not found in wmic output", ErrRAMDetection)
}

// -------------------------------------------------------------------------
// RecommendModel
// -------------------------------------------------------------------------

// Model name constants per REQ-CP-011 / install.sh select_model.
const (
	modelE2B   = "ai-mink/gemma4-e2b-rl-v1"
	modelE4BQ4 = "ai-mink/gemma4-e4b-rl-v1:q4_k_m"
	modelE4BQ5 = "ai-mink/gemma4-e4b-rl-v1:q5_k_m"
	modelE4BQ8 = "ai-mink/gemma4-e4b-rl-v1:q8_0"
	gib        = int64(1024 * 1024 * 1024)
)

// RecommendModel returns the canonical MINK model name for a given system RAM in bytes,
// per the REQ-CP-011 mapping table. Boundaries:
//
//	ramBytes < 8 GiB        → "ai-mink/gemma4-e2b-rl-v1"
//	8 GiB <= ram < 16 GiB   → "ai-mink/gemma4-e4b-rl-v1:q4_k_m"
//	16 GiB <= ram < 32 GiB  → "ai-mink/gemma4-e4b-rl-v1:q5_k_m"
//	ramBytes >= 32 GiB      → "ai-mink/gemma4-e4b-rl-v1:q8_0"
//
// "GiB" boundaries use 1024^3 (the install.sh /proc/meminfo math uses /1048576 of kB,
// which equals /1024^3 bytes — see comment in install.sh detect_ram). This is a pure
// function and never returns an error.
func RecommendModel(ramBytes int64) string {
	switch {
	case ramBytes < 8*gib:
		return modelE2B
	case ramBytes < 16*gib:
		return modelE4BQ4
	case ramBytes < 32*gib:
		return modelE4BQ5
	default:
		return modelE4BQ8
	}
}

// -------------------------------------------------------------------------
// PullModel
// -------------------------------------------------------------------------

// siPrefixBytes maps SI / binary size suffixes to byte multipliers for progress parsing.
var siPrefixBytes = map[string]float64{
	"B":   1,
	"KB":  1e3,
	"MB":  1e6,
	"GB":  1e9,
	"TB":  1e12,
	"KIB": 1024,
	"MIB": 1024 * 1024,
	"GIB": 1024 * 1024 * 1024,
	"TIB": 1024 * 1024 * 1024 * 1024,
}

// byteSizeRe extracts "N.N UNIT / N.N UNIT" from a progress line.
var byteSizeRe = regexp.MustCompile(`(\d+\.?\d*)\s*([KMGT]?i?[Bb])\s*/\s*(\d+\.?\d*)\s*([KMGT]?i?[Bb])`)

// PullModel executes `ollama pull <model>` and streams progress updates on the provided
// channel. Progress channel is closed on completion (success or error). Errors include
// non-zero exit codes wrapped with ErrPullFailed and context cancellation wrapped with
// the context.Context's Err.
// Returns nil on successful download; the caller should drain the channel until closed.
//
// @MX:WARN: [AUTO] Goroutine spawned to pipe stdout — context cancellation must propagate cleanly.
// @MX:REASON: If the goroutine leaks, PullModel will hang waiting for cmd.Wait() indefinitely.
func PullModel(ctx context.Context, modelName string, progress chan<- ProgressUpdate) error {
	cmd := execCommandContext(ctx, "ollama", "pull", modelName)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		close(progress)
		return fmt.Errorf("%w: StdoutPipe: %v", ErrPullFailed, err)
	}

	if err := cmd.Start(); err != nil {
		close(progress)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: Start: %v", ErrPullFailed, err)
	}

	// Scan stdout lines in the current goroutine; cmd.Wait() is called below after the
	// scanner finishes (EOF on the pipe signals process completion).
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		update := parseProgressLine(line)
		progress <- update
	}

	waitErr := cmd.Wait()
	close(progress)

	if waitErr != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %v", ErrPullFailed, waitErr)
	}
	return nil
}

// parseProgressLine converts a single ollama pull stdout line into a ProgressUpdate.
func parseProgressLine(line string) ProgressUpdate {
	u := ProgressUpdate{Raw: line}
	lower := strings.ToLower(strings.TrimSpace(line))

	switch {
	case lower == "pulling manifest" || strings.HasPrefix(lower, "pulling manifest"):
		u.Phase = "pulling manifest"
	case strings.HasPrefix(lower, "verifying"):
		u.Phase = "verifying"
	case lower == "writing manifest" || strings.HasPrefix(lower, "writing manifest"):
		u.Phase = "writing manifest"
	case lower == "success":
		u.Phase = "success"
		u.PercentDone = 100
	case strings.HasPrefix(lower, "pulling "):
		u.Phase = "downloading layer"
		// Extract layer digest: first hex-like token after "pulling ".
		rest := strings.TrimPrefix(strings.TrimSpace(line), "pulling ")
		rest = strings.TrimPrefix(rest, "Pulling ")
		fields := strings.Fields(rest)
		if len(fields) > 0 {
			u.Layer = fields[0]
		}
		// Extract percent: look for N% pattern.
		u.PercentDone = -1
		if m := regexp.MustCompile(`(\d+)%`).FindStringSubmatch(line); m != nil {
			if pct, err := strconv.ParseFloat(m[1], 64); err == nil {
				u.PercentDone = pct
			}
		}
		// Extract byte counts.
		u.BytesDone, u.BytesTotal = parseByteCounts(line)
	default:
		// Unrecognized line — preserve raw, leave other fields zero.
		u.PercentDone = -1
	}

	return u
}

// parseByteCounts extracts done/total byte counts from a progress line.
// Returns 0, 0 on parse failure (best-effort).
func parseByteCounts(line string) (done, total int64) {
	m := byteSizeRe.FindStringSubmatch(line)
	if m == nil {
		return 0, 0
	}
	// m[1]=doneVal, m[2]=doneUnit, m[3]=totalVal, m[4]=totalUnit
	doneVal, err1 := strconv.ParseFloat(m[1], 64)
	totalVal, err2 := strconv.ParseFloat(m[3], 64)
	if err1 != nil || err2 != nil {
		return 0, 0
	}

	doneUnit := strings.ToUpper(m[2])
	totalUnit := strings.ToUpper(m[4])

	doneMult, ok1 := siPrefixBytes[doneUnit]
	totalMult, ok2 := siPrefixBytes[totalUnit]
	if !ok1 {
		doneMult = 1
	}
	if !ok2 {
		totalMult = 1
	}

	return int64(doneVal * doneMult), int64(totalVal * totalMult)
}
