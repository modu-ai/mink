//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd

package hook

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

var rlimitWarnOnce sync.Once

// applySysProcAttr는 rlimit 미지원 OS에서 WARN 로그만 출력한다.
// REQ-HK-021 c
func applySysProcAttr(cmd *exec.Cmd, timeout time.Duration, logger *zap.Logger) {
	_ = timeout
	rlimitWarnOnce.Do(func() {
		if logger != nil {
			logger.Warn("rlimit not supported on this OS; subprocess resource limits not applied")
		}
	})
}

// buildCmd는 context + shell + command로 *exec.Cmd를 생성한다.
func buildCmd(ctx context.Context, shell, command string) *exec.Cmd {
	return exec.CommandContext(ctx, shell, "-c", command)
}

// exitCodeOf는 cmd.Wait() 에러에서 exit code를 추출한다.
func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

// scrubEnv는 parent environment에서 deny-list 변수를 제거한 slice를 반환한다.
func scrubEnv(env []string) []string {
	result := make([]string, 0, len(env))
	for _, kv := range env {
		key, _, _ := strings.Cut(kv, "=")
		if isDenyListed(key) {
			continue
		}
		result = append(result, kv)
	}
	return result
}

func isDenyListed(key string) bool {
	upper := strings.ToUpper(key)
	if upper == "ANTHROPIC_API_KEY" || upper == "OPENAI_API_KEY" {
		return true
	}
	if strings.HasPrefix(upper, "GOOSE_AUTH_") {
		return true
	}
	lowerKey := strings.ToLower(key)
	for _, pat := range denyPatterns {
		if strings.Contains(lowerKey, pat) {
			return true
		}
	}
	return false
}

var denyPatterns = []string{
	"token",
	"secret",
	"password",
	"apikey",
	"api_key",
}

func markExtraFDsCloseOnExec(logger *zap.Logger) {
	_ = logger
	// 미지원 OS에서는 no-op
}

// startSubprocess on unsupported OSes simply delegates to cmd.Start(); no
// rlimit application is available. Defined here to keep the cross-platform
// startSubprocess contract.
func startSubprocess(cmd *exec.Cmd, _ *zap.Logger) error {
	return cmd.Start()
}
