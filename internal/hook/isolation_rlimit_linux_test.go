//go:build linux

package hook

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// TestComputeRlimits verifies the timeout-to-CPU-rlimit mapping.
// REQ-HK-021 c — covers issue #40.
func TestComputeRlimits(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		timeout   time.Duration
		wantCPU   uint64
		wantAS    uint64
		wantNFile uint64
	}{
		{name: "zero timeout uses floor", timeout: 0, wantCPU: hookRlimitCPUFloorSc},
		{name: "sub-second timeout uses floor", timeout: 500 * time.Millisecond, wantCPU: hookRlimitCPUFloorSc},
		{name: "5s timeout -> 6s CPU", timeout: 5 * time.Second, wantCPU: 6},
		{name: "30s timeout -> 31s CPU", timeout: 30 * time.Second, wantCPU: 31},
		{name: "fractional timeout rounds up", timeout: 1500 * time.Millisecond, wantCPU: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rl := computeRlimits(tc.timeout)
			assert.Equal(t, tc.wantCPU, rl.cpu, "cpu seconds")
			assert.Equal(t, hookRlimitAS, rl.as, "RLIMIT_AS")
			assert.Equal(t, hookRlimitNOFILE, rl.nofile, "RLIMIT_NOFILE")
		})
	}
}

// TestStartSubprocess_AppliesPrlimit confirms that startSubprocess applies the
// registered rlimits to the running child process via Prlimit. We read back
// RLIMIT_NOFILE on the child PID and assert it matches what we registered.
func TestStartSubprocess_AppliesPrlimit(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use a long-ish sleep so we can read /proc before the child exits.
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 2")
	logger := zap.NewNop()
	applyRlimitIfSupported(cmd, 5*time.Second, logger)

	require.NoError(t, startSubprocess(cmd, logger))
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	var got unix.Rlimit
	require.NoError(t, unix.Prlimit(cmd.Process.Pid, unix.RLIMIT_NOFILE, nil, &got))
	assert.Equal(t, hookRlimitNOFILE, got.Cur, "RLIMIT_NOFILE.Cur should match applied limit")
	assert.Equal(t, hookRlimitNOFILE, got.Max, "RLIMIT_NOFILE.Max should match applied limit")
}
