//go:build linux

package hook

import (
	"math"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// rlimitWarnOnce keeps non-fatal warnings (e.g. Prlimit failures) bounded to
// one log per process — the same pattern the darwin/freebsd stub uses for the
// "rlimit not supported" message.
var rlimitWarnOnce sync.Once

// Resource limits applied to every hook subprocess on Linux.
// REQ-HK-021 c — see SPEC-GOOSE-HOOK-001.
//
//   - RLIMIT_AS      512 MiB virtual address space cap.
//   - RLIMIT_NOFILE  256 file descriptors.
//   - RLIMIT_CPU     ceil(timeout seconds) + 1, with a hard floor of 2 s so
//     short timeouts (e.g. 500 ms) still leave the kernel one second of CPU
//     wall-clock headroom before SIGXCPU fires.
const (
	hookRlimitAS         uint64 = 512 * 1024 * 1024
	hookRlimitNOFILE     uint64 = 256
	hookRlimitCPUFloorSc uint64 = 2
)

// applyRlimitIfSupported configures Setpgid on the SysProcAttr and arranges
// resource limits to be applied via Prlimit immediately after Start completes.
//
// Linux does not expose a portable PreExec hook through os/exec, so the
// canonical pattern is to set the resource limits on the child PID right after
// Start returns: Prlimit takes effect on the running process and bounds any
// further allocations / open file descriptors / CPU time consumed by the hook.
//
// The companion startSubprocess wrapper (see startWithRlimits below) is what
// actually invokes Prlimit; this function just records the desired limits on
// the cmd via SysProcAttr + a sidecar map so startWithRlimits can read them.
//
// REQ-HK-021 c — addresses #40.
func applyRlimitIfSupported(cmd *exec.Cmd, timeout time.Duration, _ *zap.Logger) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	// Override the default exec.CommandContext cancel (which only signals the
	// leader PID): when ctx fires, kill the entire process group so descendants
	// like a shell-spawned `sleep` don't keep the inherited stdout pipe open and
	// hang cmd.Wait(). WaitDelay is the backstop if the kernel is slow to
	// deliver the signal.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Negative PID targets the process group (Setpgid above made the child
		// the leader of its own group, so PGID == child PID).
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		return nil
	}
	cmd.WaitDelay = 2 * time.Second
	rlimitsByCmd.store(cmd, computeRlimits(timeout))
}

// computeRlimits derives the per-hook rlimit set for a given timeout.
// Exposed (unexported) for testing.
func computeRlimits(timeout time.Duration) hookRlimits {
	var cpuSec uint64 = hookRlimitCPUFloorSc
	if timeout > 0 {
		// ceil(timeout / 1s) + 1, clamped to at least the floor.
		seconds := uint64(math.Ceil(timeout.Seconds())) + 1
		if seconds > cpuSec {
			cpuSec = seconds
		}
	}
	return hookRlimits{
		as:     hookRlimitAS,
		nofile: hookRlimitNOFILE,
		cpu:    cpuSec,
	}
}

// hookRlimits is the rlimit triple applied to a single hook subprocess.
type hookRlimits struct {
	as     uint64
	nofile uint64
	cpu    uint64
}

// rlimitsByCmd is a concurrency-safe sidecar that links an *exec.Cmd to its
// desired rlimit set. Entries are removed by startWithRlimits once Prlimit has
// been applied (or skipped on error).
//
// A package-level map is used because os/exec gives us no per-cmd extension
// point that survives the Start boundary on Linux.
var rlimitsByCmd = newRlimitRegistry()

type rlimitRegistry struct {
	mu sync.Mutex
	m  map[*exec.Cmd]hookRlimits
}

func newRlimitRegistry() *rlimitRegistry {
	return &rlimitRegistry{m: make(map[*exec.Cmd]hookRlimits)}
}

func (r *rlimitRegistry) store(cmd *exec.Cmd, rl hookRlimits) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[cmd] = rl
}

func (r *rlimitRegistry) take(cmd *exec.Cmd) (hookRlimits, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rl, ok := r.m[cmd]
	if ok {
		delete(r.m, cmd)
	}
	return rl, ok
}

// startSubprocess starts the cmd and applies any rlimits registered for it via
// applyRlimitIfSupported. The Linux kernel honours Prlimit on a running
// process, so calling it immediately after Start is sufficient to bound
// subsequent address-space, FD, and CPU-time usage.
//
// On any Prlimit error, a single one-shot WARN is emitted and the subprocess
// is left running with the parent's inherited limits — matching the previous
// stub's "best effort" semantics so a kernel quirk cannot block hook
// execution outright.
func startSubprocess(cmd *exec.Cmd, logger *zap.Logger) error {
	if err := cmd.Start(); err != nil {
		// Drop any orphan registry entry; cmd never reached a running state.
		_, _ = rlimitsByCmd.take(cmd)
		return err
	}
	rl, ok := rlimitsByCmd.take(cmd)
	if !ok || cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	for _, lim := range []struct {
		resource int
		value    uint64
		name     string
	}{
		{unix.RLIMIT_AS, rl.as, "RLIMIT_AS"},
		{unix.RLIMIT_NOFILE, rl.nofile, "RLIMIT_NOFILE"},
		{unix.RLIMIT_CPU, rl.cpu, "RLIMIT_CPU"},
	} {
		r := &unix.Rlimit{Cur: lim.value, Max: lim.value}
		if err := unix.Prlimit(pid, lim.resource, r, nil); err != nil {
			rlimitWarnOnce.Do(func() {
				if logger != nil {
					logger.Warn("hook rlimit Prlimit failed; subprocess running with inherited limits",
						zap.String("resource", lim.name),
						zap.Int("pid", pid),
						zap.Uint64("value", lim.value),
						zap.Error(err),
					)
				}
			})
		}
	}
	return nil
}
