package store

import (
	"sync"
	"time"

	"github.com/modu-ai/goose/internal/permission"
)

// MemoryStore는 테스트용 인메모리 Store 구현이다.
// 파일 I/O 없이 동기적으로 동작한다.
type MemoryStore struct {
	mu     sync.RWMutex
	grants []permission.Grant
	ready  bool
}

// NewMemoryStore는 새 MemoryStore를 생성한다.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (m *MemoryStore) Open() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.grants = []permission.Grant{}
	m.ready = true
	return nil
}

func (m *MemoryStore) Lookup(subjectID string, cap permission.Capability, scope string) (permission.Grant, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	for _, g := range m.grants {
		if g.SubjectID == subjectID && g.Capability == cap && g.Scope == scope {
			if g.Revoked || g.IsExpired(now) {
				continue
			}
			return g, true
		}
	}
	return permission.Grant{}, false
}

func (m *MemoryStore) Save(g permission.Grant) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.grants = append(m.grants, g)
	return nil
}

func (m *MemoryStore) Revoke(subjectID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	count := 0
	for i := range m.grants {
		if m.grants[i].SubjectID == subjectID && !m.grants[i].Revoked {
			m.grants[i].Revoked = true
			m.grants[i].RevokedAt = &now
			count++
		}
	}
	return count, nil
}

func (m *MemoryStore) List(filter permission.Filter) ([]permission.Grant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	var result []permission.Grant
	for _, g := range m.grants {
		if filter.SubjectID != "" && g.SubjectID != filter.SubjectID {
			continue
		}
		if filter.Capability != nil && g.Capability != *filter.Capability {
			continue
		}
		if !filter.IncludeRevoked && g.Revoked {
			continue
		}
		if !filter.IncludeExpired && g.IsExpired(now) {
			continue
		}
		result = append(result, g)
	}
	return result, nil
}

func (m *MemoryStore) GC(now time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var remaining []permission.Grant
	pruned := 0
	for _, g := range m.grants {
		if g.IsExpired(now) || g.Revoked {
			pruned++
		} else {
			remaining = append(remaining, g)
		}
	}
	m.grants = remaining
	return pruned, nil
}

func (m *MemoryStore) Close() error {
	return nil
}

// AllGrants는 테스트용으로 모든 grant를 반환한다 (revoked 포함).
func (m *MemoryStore) AllGrants() []permission.Grant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]permission.Grant, len(m.grants))
	copy(result, m.grants)
	return result
}
