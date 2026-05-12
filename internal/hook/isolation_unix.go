//go:build linux || darwin || freebsd || netbsd || openbsd

package hook

import (
	"context"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// buildCmd는 context + shell + command로 *exec.Cmd를 생성한다.
// REQ-HK-016: no sudo, exec.Command(shell, "-c", command) 고정.
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
// REQ-HK-021 a / §6.11.1
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

// isDenyListed는 환경변수 이름이 deny-list에 해당하는지 반환한다.
// §6.11.1: 정규식 + 명시적 이름 두 규칙의 합집합.
func isDenyListed(key string) bool {
	upper := strings.ToUpper(key)
	// 명시적 이름
	if upper == "ANTHROPIC_API_KEY" || upper == "OPENAI_API_KEY" {
		return true
	}
	// MINK_AUTH_* / GOOSE_AUTH_* prefix glob (SPEC-MINK-ENV-MIGRATE-001 §5)
	if strings.HasPrefix(upper, "MINK_AUTH_") || strings.HasPrefix(upper, "GOOSE_AUTH_") {
		return true
	}
	// case-insensitive 패턴: token, secret, password, apikey, api_key
	lowerKey := strings.ToLower(key)
	for _, pat := range denyPatterns {
		if strings.Contains(lowerKey, pat) {
			return true
		}
	}
	return false
}

// denyPatterns는 §6.11.1의 패턴 목록이다.
var denyPatterns = []string{
	"token",
	"secret",
	"password",
	"apikey",
	"api_key",
}

// markExtraFDsCloseOnExec는 현재 프로세스의 추가 FD에 close-on-exec를 마킹한다.
// REQ-HK-021 d — best-effort
func markExtraFDsCloseOnExec(logger *zap.Logger) {
	_ = logger
	for fd := 3; fd < 256; fd++ {
		_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_SETFD, syscall.FD_CLOEXEC)
		_ = errno // FD가 없으면 에러 — 정상
	}
}

// applySysProcAttr는 플랫폼별 rlimit + close-on-exec를 적용한다.
// REQ-HK-021 c/d
// darwin/freebsd: Rlimit 배열 미지원 → WARN 1회 emit하고 Setpgid만 설정.
// linux: 별도 isolation_linux.go에서 override (빌드 태그 우선순위).
func applySysProcAttr(cmd *exec.Cmd, timeout time.Duration, logger *zap.Logger) {
	// darwin/freebsd/netbsd/openbsd: Rlimit 배열 미지원
	// Setpgid만 설정하고 WARN 로그
	applyRlimitIfSupported(cmd, timeout, logger)
	markExtraFDsCloseOnExec(logger)
}
