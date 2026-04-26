//go:build linux

package hook

import (
	"os/exec"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

var rlimitWarnOnce sync.Once

// applyRlimitIfSupported is a Linux stub matching the darwin/freebsd variant.
//
// REQ-HK-021 c (RLIMIT_AS / RLIMIT_NOFILE / RLIMIT_CPU enforcement) is currently
// not implemented on Linux: the previous code referenced a hypothetical
// syscall.SysProcAttr.Rlimit field that does not exist in the Go standard
// library, causing a build failure when CI compiles for linux.
//
// As an interim measure (introduced 2026-04-27 to unblock the first
// protection-enabled PR) this variant only sets Setpgid and emits a one-time
// WARN log noting that rlimit enforcement is absent. The proper Linux
// implementation should arrive via a follow-up HOOK-001 PR that uses
// golang.org/x/sys/unix.Setrlimit inside a PreExec callback (or fork+execve
// with explicit setrlimit syscalls in the child), tracked as TODO below.
//
// TODO(SPEC-GOOSE-HOOK-001 REQ-HK-021 c): implement actual Linux rlimit
//
//	enforcement via x/sys/unix.Setrlimit (PreExec or fork+execve pattern).
//	Until then, Linux executions match the darwin variant: Setpgid only.
func applyRlimitIfSupported(cmd *exec.Cmd, timeout time.Duration, logger *zap.Logger) {
	_ = timeout
	rlimitWarnOnce.Do(func() {
		if logger != nil {
			logger.Warn("rlimit enforcement not implemented on linux; RLIMIT_AS/NOFILE/CPU not applied",
				zap.String("platform", "linux"),
				zap.String("spec", "SPEC-GOOSE-HOOK-001 REQ-HK-021 c"),
				zap.String("status", "interim_stub_pending_xsys_unix_implementation"),
			)
		}
	})
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
