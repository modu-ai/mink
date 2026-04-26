package plugin

import (
	"fmt"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

// PluginRegistry는 로드된 플러그인 인스턴스를 관리하는 atomic 스냅샷 기반 저장소이다.
// REQ-PL-002: 이름 유일성 보장.
// REQ-PL-003: ClearThenRegister atomic swap.
//
// @MX:ANCHOR: [AUTO] PluginRegistry — 모든 플러그인 인스턴스 관리의 단일 진입점
// @MX:REASON: REQ-PL-002/003 — atomic.Pointer 기반 lock-free 읽기, 쓰기는 mu 보호. fan_in >= 3
// @MX:SPEC: REQ-PL-002, REQ-PL-003
type PluginRegistry struct {
	// current는 lock-free 읽기를 위해 atomic.Pointer로 보호된다.
	current atomic.Pointer[map[PluginID]*PluginInstance]
	// mu는 쓰기 경쟁을 보호한다.
	mu     sync.Mutex
	logger *zap.Logger
}

// NewPluginRegistry는 빈 PluginRegistry를 생성한다.
func NewPluginRegistry(logger *zap.Logger) *PluginRegistry {
	if logger == nil {
		logger = zap.NewNop()
	}
	r := &PluginRegistry{logger: logger}
	empty := make(map[PluginID]*PluginInstance)
	r.current.Store(&empty)
	return r
}

// registerInstance는 새 플러그인 인스턴스를 등록한다.
// REQ-PL-002: 동일 이름이 이미 등록된 경우 ErrDuplicatePluginName 반환.
func (r *PluginRegistry) registerInstance(inst *PluginInstance) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	current := *r.current.Load()
	if _, exists := current[inst.ID]; exists {
		return ErrDuplicatePluginName{Name: string(inst.ID)}
	}

	next := copySnapshot(current)
	next[inst.ID] = inst
	r.current.Store(&next)
	return nil
}

// ClearThenRegister는 현재 스냅샷을 원자적으로 새 스냅샷으로 교체한다.
// REQ-PL-003: atomic swap — 동시 독자는 이전 또는 새 스냅샷만 관찰한다.
//
// @MX:WARN: [AUTO] sync.Mutex + atomic.Pointer 혼용 — Lock 없이 Store 단독 사용 금지
// @MX:REASON: ClearThenRegister는 mu를 잡고 Store해야 partial update가 방지됨. atomic.Pointer는 읽기 전용으로 사용
func (r *PluginRegistry) ClearThenRegister(snapshot map[PluginID]*PluginInstance) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	next := copySnapshot(snapshot)
	r.current.Store(&next)
	return nil
}

// List는 현재 등록된 모든 플러그인 인스턴스를 반환한다.
// lock-free 읽기 (atomic.Pointer).
func (r *PluginRegistry) List() []*PluginInstance {
	snap := *r.current.Load()
	result := make([]*PluginInstance, 0, len(snap))
	for _, inst := range snap {
		result = append(result, inst)
	}
	return result
}

// Unload는 지정된 ID의 플러그인을 언로드한다.
func (r *PluginRegistry) Unload(id PluginID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	current := *r.current.Load()
	if _, exists := current[id]; !exists {
		return fmt.Errorf("plugin %q not found in registry", id)
	}

	next := copySnapshot(current)
	delete(next, id)
	r.current.Store(&next)
	return nil
}

// copySnapshot은 snapshot의 shallow copy를 반환한다.
func copySnapshot(src map[PluginID]*PluginInstance) map[PluginID]*PluginInstance {
	dst := make(map[PluginID]*PluginInstance, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
