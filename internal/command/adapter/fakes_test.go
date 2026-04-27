package adapter

import (
	"context"
	"sync"

	"github.com/modu-ai/goose/internal/command"
)

// fakeLoopController is a test double for LoopController.
// It records method calls and can be configured to return errors.
type fakeLoopController struct {
	mu              sync.Mutex
	clearCount      int
	compactRequests []int
	modelChanges    []command.ModelInfo
	snapshotVal     LoopSnapshot
	nextErr         error
}

func (f *fakeLoopController) RequestClear(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.nextErr != nil {
		return f.nextErr
	}
	f.clearCount++
	return nil
}

func (f *fakeLoopController) RequestReactiveCompact(_ context.Context, target int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.nextErr != nil {
		return f.nextErr
	}
	f.compactRequests = append(f.compactRequests, target)
	return nil
}

func (f *fakeLoopController) RequestModelChange(_ context.Context, info command.ModelInfo) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.nextErr != nil {
		return f.nextErr
	}
	f.modelChanges = append(f.modelChanges, info)
	return nil
}

func (f *fakeLoopController) Snapshot() LoopSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snapshotVal
}

// getClearCount returns the number of RequestClear calls (thread-safe).
func (f *fakeLoopController) getClearCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.clearCount
}

// getCompactRequests returns a copy of the captured compact request targets.
func (f *fakeLoopController) getCompactRequests() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]int, len(f.compactRequests))
	copy(cp, f.compactRequests)
	return cp
}

// getModelChanges returns a copy of the captured model change infos.
func (f *fakeLoopController) getModelChanges() []command.ModelInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]command.ModelInfo, len(f.modelChanges))
	copy(cp, f.modelChanges)
	return cp
}

// fakeWarnLogger records Warn calls for test assertions.
type fakeWarnLogger struct {
	mu        sync.Mutex
	warnCount int
	lastMsg   string
	lastArgs  []any
}

func (l *fakeWarnLogger) Warn(msg string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warnCount++
	l.lastMsg = msg
	l.lastArgs = fields
}

func (l *fakeWarnLogger) getWarnCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.warnCount
}

func (l *fakeWarnLogger) getLastArgs() []any {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lastArgs
}
