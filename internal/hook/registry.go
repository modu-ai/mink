package hook

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

// HookRegistry는 HookEvent → []HookBinding의 atomic 스냅샷 기반 저장소이다.
// REQ-HK-001, REQ-HK-002, REQ-HK-003
//
// @MX:ANCHOR: [AUTO] Hook 등록/조회의 단일 진입점 — atomic.Pointer 기반 스냅샷 스왑
// @MX:REASON: SPEC-GOOSE-HOOK-001 REQ-HK-003 — fan_in >= 3 (dispatchers, tests, plugin loader)
type HookRegistry struct {
	// current는 lock-free 읽기를 위해 atomic.Pointer로 보호된다.
	current atomic.Pointer[map[HookEvent][]HookBinding]
	// mu는 ClearThenRegister / Register의 쓰기 경쟁을 보호한다.
	mu     sync.Mutex
	logger *zap.Logger

	// skillsConsumer는 DispatchFileChanged 완료 후 호출되는 외부 consumer이다.
	// nil-safe: 등록 전까지 호출 생략.
	// D11 resolution
	consumerMu        sync.RWMutex
	skillsConsumer    SkillsFileChangedConsumer
	workspaceResolver WorkspaceRootResolver // SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-002 step 8

	// loader는 PluginHookLoader 참조이다.
	// REQ-HK-013: loader.IsLoading() == true면 Register 거부.
	loader PluginHookLoader
}

// RegistryOption은 HookRegistry 생성 옵션이다.
type RegistryOption func(*HookRegistry)

// WithLogger는 로거를 설정한다.
func WithLogger(l *zap.Logger) RegistryOption {
	return func(r *HookRegistry) { r.logger = l }
}

// WithPluginLoader는 PluginHookLoader를 설정한다.
// REQ-HK-013
func WithPluginLoader(l PluginHookLoader) RegistryOption {
	return func(r *HookRegistry) { r.loader = l }
}

// NewHookRegistry는 새 HookRegistry를 생성한다.
func NewHookRegistry(opts ...RegistryOption) *HookRegistry {
	r := &HookRegistry{
		logger: zap.NewNop(),
	}
	empty := make(map[HookEvent][]HookBinding)
	r.current.Store(&empty)
	for _, o := range opts {
		o(r)
	}
	return r
}

// Register는 event/matcher/handler 트리플을 registry에 등록한다.
// REQ-HK-002: FIFO 순서 보장.
// REQ-HK-013: PluginHookLoader.IsLoading == true면 ErrRegistryLocked 반환.
func (r *HookRegistry) Register(event HookEvent, matcher string, h HookHandler) error {
	if r.loader != nil && r.loader.IsLoading() {
		return ErrRegistryLocked
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 현재 스냅샷의 deep copy를 기반으로 새 스냅샷을 생성한다.
	old := *r.current.Load()
	next := deepCopySnapshot(old)
	next[event] = append(next[event], HookBinding{
		Event:   event,
		Matcher: matcher,
		Handler: h,
		Source:  "inline",
	})
	r.current.Store(&next)
	return nil
}

// ClearThenRegister는 기존 스냅샷을 원자적으로 새 스냅샷으로 교체한다.
// REQ-HK-003: atomic swap — 동시 독자는 이전 또는 새 스냅샷만 관찰한다.
// REQ-HK-018: swap 중 핸들러 호출 없음.
func (r *HookRegistry) ClearThenRegister(snapshot map[HookEvent][]HookBinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	next := deepCopySnapshot(snapshot)
	r.current.Store(&next)
	return nil
}

// Handlers는 event에 등록된 핸들러 중 binding의 matcher에 매치되는 것을 FIFO 순서로 반환한다.
// REQ-HK-002: FIFO 순서 보장.
// REQ-HK-020: binding.Matcher로 매칭 (glob 또는 regex: 접두사).
// lock-free 읽기 (atomic.Pointer).
func (r *HookRegistry) Handlers(event HookEvent, input HookInput) []HookHandler {
	snap := *r.current.Load()
	bindings := snap[event]
	// 방어적 복사 (R3 리스크 대응)
	result := make([]HookHandler, 0, len(bindings))
	for _, b := range bindings {
		if bindingMatches(b, input) {
			result = append(result, b.Handler)
		}
	}
	return result
}

// bindingMatches는 binding의 Matcher를 input에 대해 평가한다.
// REQ-HK-020: registry binding의 Matcher 기준으로 매칭.
func bindingMatches(b HookBinding, input HookInput) bool {
	matcher := b.Matcher
	var target string
	switch input.HookEvent {
	case EvPreToolUse, EvPostToolUse, EvPostToolUseFailure:
		if input.Tool != nil {
			target = input.Tool.Name
		}
	case EvFileChanged:
		// FileChanged는 경로 중 하나라도 매치되면 true
		for _, p := range input.ChangedPaths {
			if matcherMatches(matcher, p) {
				return true
			}
		}
		return false
	default:
		target = string(input.HookEvent)
	}
	return matcherMatches(matcher, target)
}

// HandlerBindings는 event에 등록된 모든 바인딩을 반환한다 (매처 미적용).
// 내부 테스트 및 디버그용.
func (r *HookRegistry) HandlerBindings(event HookEvent) []HookBinding {
	snap := *r.current.Load()
	bindings := snap[event]
	result := make([]HookBinding, len(bindings))
	copy(result, bindings)
	return result
}

// SetSkillsFileChangedConsumer는 FileChanged 이벤트 후 호출할 외부 consumer를 등록한다.
// REQ-HK-008 / D11 resolution.
// nil 인자는 ErrInvalidConsumer를 반환한다 (SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-008).
// thread-safe.
func (r *HookRegistry) SetSkillsFileChangedConsumer(fn SkillsFileChangedConsumer) error {
	if fn == nil {
		return ErrInvalidConsumer
	}
	r.consumerMu.Lock()
	defer r.consumerMu.Unlock()
	r.skillsConsumer = fn
	return nil
}

// SetWorkspaceRootResolver는 shell hook subprocess의 CWD 결정에 사용할
// WorkspaceRootResolver를 등록한다.
// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-002 step 8.
// nil resolver는 ErrInvalidConsumer를 반환한다 (REQ-WIRE-008).
// thread-safe.
func (r *HookRegistry) SetWorkspaceRootResolver(resolver WorkspaceRootResolver) error {
	if resolver == nil {
		return ErrInvalidConsumer
	}
	r.consumerMu.Lock()
	defer r.consumerMu.Unlock()
	r.workspaceResolver = resolver
	return nil
}

// WorkspaceResolver는 현재 등록된 WorkspaceRootResolver를 반환한다.
func (r *HookRegistry) WorkspaceResolver() WorkspaceRootResolver {
	r.consumerMu.RLock()
	defer r.consumerMu.RUnlock()
	return r.workspaceResolver
}

// SkillsConsumer는 현재 등록된 SkillsFileChangedConsumer를 반환한다.
func (r *HookRegistry) SkillsConsumer() SkillsFileChangedConsumer {
	r.consumerMu.RLock()
	defer r.consumerMu.RUnlock()
	return r.skillsConsumer
}

// deepCopySnapshot은 snapshot의 deep copy를 반환한다.
// REQ-HK-003 / ClearThenRegister 원자성 보장.
func deepCopySnapshot(src map[HookEvent][]HookBinding) map[HookEvent][]HookBinding {
	dst := make(map[HookEvent][]HookBinding, len(src))
	for ev, bindings := range src {
		cp := make([]HookBinding, len(bindings))
		copy(cp, bindings)
		dst[ev] = cp
	}
	return dst
}

// matcherMatches는 matcher 문자열과 target 문자열의 매치 여부를 반환한다.
// REQ-HK-020: "regex:" 접두사이면 regexp.Regexp, 그 외는 filepath.Match (glob).
func matcherMatches(matcher, target string) bool {
	if matcher == "" || matcher == "*" {
		return true
	}
	if strings.HasPrefix(matcher, "regex:") {
		pattern := strings.TrimPrefix(matcher, "regex:")
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(target)
	}
	// glob (filepath.Match 기반)
	matched, err := filepath.Match(matcher, target)
	if err != nil {
		return false
	}
	return matched
}
