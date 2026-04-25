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

// applyRlimitIfSupported는 Linux에서 rlimit을 설정한다.
// REQ-HK-021 c: RLIMIT_AS, RLIMIT_NOFILE, RLIMIT_CPU
func applyRlimitIfSupported(cmd *exec.Cmd, timeout time.Duration, logger *zap.Logger) {
	cpuSec := uint64(defaultShellTimeout + 5)
	if timeout > 0 {
		cpuSec = uint64(timeout.Seconds()) + 5
	}

	const (
		rlimitAS     = uint64(1 << 30) // 1 GiB 가상 메모리
		rlimitNOFILE = uint64(128)     // 파일 디스크립터 수
	)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Rlimit: []syscall.Rlimit{
			{Type: syscall.RLIMIT_AS, Cur: rlimitAS, Max: rlimitAS},
			{Type: syscall.RLIMIT_NOFILE, Cur: rlimitNOFILE, Max: rlimitNOFILE},
			{Type: syscall.RLIMIT_CPU, Cur: cpuSec, Max: cpuSec},
		},
	}
	_ = logger
}
