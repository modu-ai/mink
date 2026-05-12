package compressor

import (
	"testing"

	"github.com/modu-ai/mink/internal/learning/trajectory"
)

// makeTrajectory is a test helper.
func makeTrajectory(roles ...trajectory.Role) *trajectory.Trajectory {
	entries := make([]trajectory.TrajectoryEntry, len(roles))
	for i, r := range roles {
		entries[i] = trajectory.TrajectoryEntry{From: r, Value: "turn content"}
	}
	return &trajectory.Trajectory{Conversations: entries}
}

// TestProtected_FindIndices verifies AC-COMPRESSOR-003.
// Given: [system, human, gpt, human, tool, human, gpt, ...]
// Expected protected head: indices 0(system), 1(human), 2(gpt), 4(tool).
func TestProtected_FindIndices(t *testing.T) {
	t.Parallel()
	tr := makeTrajectory(
		trajectory.RoleSystem, // 0
		trajectory.RoleHuman,  // 1
		trajectory.RoleGPT,    // 2
		trajectory.RoleHuman,  // 3 — duplicate human, should be skipped
		trajectory.RoleTool,   // 4 — first tool
		trajectory.RoleHuman,  // 5
		trajectory.RoleGPT,    // 6
	)
	// With tailProtectedTurns=0 we isolate head-only behavior.
	protected := findProtectedIndices(tr, 0)

	for _, mustHave := range []int{0, 1, 2, 4} {
		if _, ok := protected[mustHave]; !ok {
			t.Errorf("expected index %d to be protected", mustHave)
		}
	}
	// Index 3 (duplicate human) must NOT be in protected head.
	if _, ok := protected[3]; ok {
		t.Errorf("index 3 (duplicate human) should not be protected in head")
	}
}

// TestProtected_TailFourTurns verifies AC-COMPRESSOR-004.
func TestProtected_TailFourTurns(t *testing.T) {
	t.Parallel()
	roles := make([]trajectory.Role, 20)
	for i := range roles {
		roles[i] = trajectory.RoleGPT
	}
	tr := makeTrajectory(roles...)
	protected := findProtectedIndices(tr, 4)

	for _, mustHave := range []int{16, 17, 18, 19} {
		if _, ok := protected[mustHave]; !ok {
			t.Errorf("expected tail index %d to be protected", mustHave)
		}
	}
}

// TestProtected_NoSystemNoHuman verifies AC-COMPRESSOR-006 (edge: only gpt/tool turns).
func TestProtected_NoSystemNoHuman(t *testing.T) {
	t.Parallel()
	tr := makeTrajectory(
		trajectory.RoleGPT,  // 0
		trajectory.RoleTool, // 1
		trajectory.RoleGPT,  // 2
		trajectory.RoleTool, // 3
	)
	protected := findProtectedIndices(tr, 4)
	// Both gpt and tool first occurrences should be protected.
	if _, ok := protected[0]; !ok {
		t.Error("expected index 0 (first gpt) to be protected")
	}
	if _, ok := protected[1]; !ok {
		t.Error("expected index 1 (first tool) to be protected")
	}
}

// TestProtected_FindCompressibleRegion verifies AC-COMPRESSOR-007.
func TestProtected_FindCompressibleRegion(t *testing.T) {
	t.Parallel()
	// Build: indices 0,1 protected (head), indices 8,9 protected (tail),
	// indices 2-7 compressible.
	protected := map[int]struct{}{
		0: {}, 1: {}, 8: {}, 9: {},
	}
	start, end := findCompressibleRegion(protected, 10)
	if start != 2 {
		t.Errorf("compressStart: got %d, want 2", start)
	}
	if end != 8 {
		t.Errorf("compressEnd: got %d, want 8", end)
	}
}

// TestProtected_AllProtected verifies that when all turns are protected, compressStart >= compressEnd.
func TestProtected_AllProtected(t *testing.T) {
	t.Parallel()
	protected := map[int]struct{}{
		0: {}, 1: {}, 2: {}, 3: {}, 4: {},
	}
	start, end := findCompressibleRegion(protected, 5)
	if start < end {
		t.Errorf("expected no compressible region, got [%d,%d)", start, end)
	}
}
