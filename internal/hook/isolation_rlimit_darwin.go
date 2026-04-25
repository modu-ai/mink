//go:build darwin || freebsd || netbsd || openbsd

package hook

import (
	"os/exec"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

var rlimitWarnOnce sync.Once

// applyRlimitIfSupported는 darwin/freebsd에서 Rlimit 배열 없이 Setpgid만 설정한다.
// REQ-HK-021 c: darwin은 syscall.SysProcAttr.Rlimit 배열 미지원.
// WARN 1회 emit (per session).
func applyRlimitIfSupported(cmd *exec.Cmd, timeout time.Duration, logger *zap.Logger) {
	_ = timeout
	rlimitWarnOnce.Do(func() {
		if logger != nil {
			logger.Warn("rlimit array not supported on this OS; RLIMIT_AS/NOFILE/CPU not applied",
				zap.String("platform", "darwin"),
			)
		}
	})
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
