package provider

import (
	"fmt"
	"sort"
	"sync"
)

// ProviderRegistry는 Provider 인스턴스의 레지스트리이다.
// router.ProviderRegistry(메타)와 별도의 인스턴스 레지스트리이다.
// NewLLMCall은 이 레지스트리를 참조하여 provider 인스턴스를 조회한다.
//
// @MX:ANCHOR: [AUTO] ProviderRegistry — provider 인스턴스 조회의 단일 진입점
// @MX:REASON: NewLLMCall, 모든 adapter 등록 코드가 이 레지스트리를 통과함
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry는 빈 ProviderRegistry를 생성한다.
func NewRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]Provider),
	}
}

// Register는 provider를 레지스트리에 등록한다.
// 같은 이름의 provider가 이미 등록된 경우 에러를 반환한다.
func (r *ProviderRegistry) Register(p Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("registry: provider %q already registered", name)
	}
	r.providers[name] = p
	return nil
}

// Get은 이름으로 provider를 조회한다.
// 등록되지 않은 경우 (nil, false)를 반환한다.
func (r *ProviderRegistry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[name]
	return p, ok
}

// Names는 등록된 provider 이름을 알파벳 순으로 반환한다.
func (r *ProviderRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Len은 등록된 provider 수를 반환한다.
func (r *ProviderRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers)
}
