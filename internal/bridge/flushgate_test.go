// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-010
// AC: AC-BR-010
// M4-T4, M4-T5 — flush-gate watermark behavior.

package bridge

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestFlushGate_InitiallyNotStalled(t *testing.T) {
	t.Parallel()
	g := newFlushGate()
	if g.Stalled("s") {
		t.Fatal("fresh gate must not be stalled")
	}
	// Wait must return immediately when not stalled.
	if err := g.Wait(context.Background(), "s"); err != nil {
		t.Fatalf("Wait on idle gate must not error: %v", err)
	}
}

func TestFlushGate_StallsAtByteHighWatermark(t *testing.T) {
	t.Parallel()
	g := newFlushGate()
	g.ObserveWrite("s", HighWatermarkBytes-1)
	if g.Stalled("s") {
		t.Fatal("must not stall just below high watermark")
	}
	g.ObserveWrite("s", 10)
	if !g.Stalled("s") {
		t.Fatal("must stall once high watermark is crossed")
	}
	if got := g.Stalls(); got != 1 {
		t.Fatalf("stall counter expected 1, got %d", got)
	}
}

func TestFlushGate_StallsAtFrameHighWatermark(t *testing.T) {
	t.Parallel()
	g := newFlushGate()
	for range HighWatermarkFrames - 1 {
		g.ObserveWrite("s", 1)
	}
	if g.Stalled("s") {
		t.Fatal("must not stall below frame high watermark")
	}
	g.ObserveWrite("s", 1)
	if !g.Stalled("s") {
		t.Fatal("must stall when frame high watermark is reached")
	}
}

func TestFlushGate_DrainsBelowBothLowWatermarks(t *testing.T) {
	t.Parallel()
	g := newFlushGate()
	// Push above both high watermarks.
	for range HighWatermarkFrames + 1 {
		g.ObserveWrite("s", 8*1024) // 64*8KB = 512KB > 256KB
	}
	if !g.Stalled("s") {
		t.Fatal("setup expected stall")
	}

	// Drain frames first, but bytes still high → still stalled.
	for range HighWatermarkFrames - LowWatermarkFrames {
		g.ObserveDrain("s", 1)
	}
	if !g.Stalled("s") {
		t.Fatal("must remain stalled while bytes still above low watermark")
	}

	// Drain enough bytes to cross the low byte watermark.
	for {
		g.ObserveDrain("s", 8*1024)
		if !g.Stalled("s") {
			break
		}
	}
	if g.Stalled("s") {
		t.Fatal("must drain once both watermarks are below low")
	}
}

func TestFlushGate_WaitUnblocksOnDrain(t *testing.T) {
	t.Parallel()
	g := newFlushGate()
	g.ObserveWrite("s", HighWatermarkBytes+1)
	if !g.Stalled("s") {
		t.Fatal("setup expected stall")
	}

	done := make(chan error, 1)
	go func() {
		done <- g.Wait(context.Background(), "s")
	}()

	// Wait briefly to ensure the goroutine parks.
	time.Sleep(20 * time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("Wait returned prematurely: %v", err)
	default:
	}

	// Drain entire queue → low watermark satisfied.
	g.ObserveDrain("s", HighWatermarkBytes+1)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Wait returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait did not unblock after drain")
	}
}

func TestFlushGate_WaitRespectsContextCancel(t *testing.T) {
	t.Parallel()
	g := newFlushGate()
	g.ObserveWrite("s", HighWatermarkBytes+1)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- g.Wait(ctx, "s")
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Wait must return ctx.Err on cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("Wait did not honor context cancellation")
	}
}

func TestFlushGate_StallsCounterIncrementsOnReentry(t *testing.T) {
	t.Parallel()
	g := newFlushGate()
	// First stall → drain → stall again should be 2 stalls.
	g.ObserveWrite("s", HighWatermarkBytes+1)
	g.ObserveDrain("s", HighWatermarkBytes+1)
	g.ObserveWrite("s", HighWatermarkBytes+1)
	if got := g.Stalls(); got != 2 {
		t.Fatalf("expected 2 stalls, got %d", got)
	}
}

func TestFlushGate_DropReleasesParkedWaiters(t *testing.T) {
	t.Parallel()
	g := newFlushGate()
	g.ObserveWrite("s", HighWatermarkBytes+1)
	done := make(chan error, 1)
	go func() {
		done <- g.Wait(context.Background(), "s")
	}()
	time.Sleep(20 * time.Millisecond)
	g.Drop("s")
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Drop did not release parked Wait")
	}
}

func TestFlushGate_PerSessionIsolation(t *testing.T) {
	t.Parallel()
	g := newFlushGate()
	g.ObserveWrite("a", HighWatermarkBytes+1)
	if !g.Stalled("a") {
		t.Fatal("session a must stall")
	}
	if g.Stalled("b") {
		t.Fatal("session b must not stall")
	}
}

func TestFlushGate_ConcurrentObserveRaceFree(t *testing.T) {
	t.Parallel()
	g := newFlushGate()
	const n = 1000
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range n {
			g.ObserveWrite("s", 1024)
		}
	}()
	go func() {
		defer wg.Done()
		for range n {
			g.ObserveDrain("s", 1024)
		}
	}()
	wg.Wait()
	// Net should be zero; permitting tiny clamps from the floor in either path.
	g.ObserveDrain("s", 0)
}
