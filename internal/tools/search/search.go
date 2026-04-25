// Package search는 deferred loading을 통한 tool 탐색 기능을 제공한다.
// SPEC-GOOSE-TOOLS-001 §3.1 #2
package search

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modu-ai/goose/internal/tools"
	"github.com/modu-ai/goose/internal/tools/mcp"
)

const fetchTimeout = 5 * time.Second

// Filter는 Search.List의 필터 옵션이다.
type Filter struct {
	// IncludeDeferred가 true이면 아직 activate되지 않은 tool도 포함한다.
	IncludeDeferred bool
}

// manifestFetcher는 MCP stub tool이 구현하는 manifest fetch 인터페이스이다.
// Search.Activate에서 type assertion으로 사용된다.
type manifestFetcher interface {
	FetchManifest(ctx context.Context) (mcp.ToolManifest, error)
}

// Search는 Deferred Loading 기반 tool 탐색을 제공한다.
// REQ-TOOLS-007, REQ-TOOLS-008, REQ-TOOLS-009
type Search struct {
	registry *tools.Registry
	// cache는 "serverID/toolName" → mcp.ToolManifest 매핑이다.
	cache sync.Map
}

// New는 새 Search를 생성한다.
func New(registry *tools.Registry) *Search {
	return &Search{registry: registry}
}

// List는 필터를 적용한 tool descriptor 목록을 반환한다.
func (s *Search) List(ctx context.Context, filter Filter) []tools.ToolDescriptor {
	names := s.registry.ListNames()
	result := make([]tools.ToolDescriptor, 0, len(names))

	for _, name := range names {
		entry, ok := s.registry.ResolveEntry(name)
		if !ok {
			continue
		}
		result = append(result, entry.Descriptor())
	}
	return result
}

// Activate는 MCP-backed tool의 manifest를 fetch하고 캐시한다.
// REQ-TOOLS-008: 5초 이내 완료 또는 ErrMCPTimeout.
func (s *Search) Activate(ctx context.Context, name string) error {
	entry, ok := s.registry.ResolveEntry(name)
	if !ok {
		return fmt.Errorf("tool not found: %s", name)
	}

	desc := entry.Descriptor()
	if desc.Source != tools.SourceMCP {
		return nil // built-in은 activation 불필요
	}

	// 이미 캐시됨 확인
	serverID := desc.ServerID
	toolName := extractToolName(name, serverID)
	cacheKey := serverID + "/" + toolName

	if _, cached := s.cache.Load(cacheKey); cached {
		return nil
	}

	// mcpStubTool이 manifestFetcher를 구현하는지 확인
	tool := entry.Tool()
	fetcher, ok := tool.(manifestFetcher)
	if !ok {
		return fmt.Errorf("tool %s does not support manifest fetch", name)
	}

	// 타임아웃 컨텍스트
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	type fetchResult struct {
		manifest mcp.ToolManifest
		err      error
	}
	ch := make(chan fetchResult, 1)

	go func() {
		manifest, err := fetcher.FetchManifest(fetchCtx)
		ch <- fetchResult{manifest, err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			if fetchCtx.Err() != nil {
				return tools.ErrMCPTimeout
			}
			return r.err
		}
		s.cache.Store(cacheKey, r.manifest)
		return nil
	case <-fetchCtx.Done():
		return tools.ErrMCPTimeout
	}
}

// InvalidateCache는 serverID에 속하는 캐시를 제거한다.
// REQ-TOOLS-009: MCP 재연결 시 호출
func (s *Search) InvalidateCache(serverID string) {
	prefix := serverID + "/"
	s.cache.Range(func(key, _ any) bool {
		k := key.(string)
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			s.cache.Delete(k)
		}
		return true
	})
}

// Cache는 테스트용으로 내부 캐시 참조를 반환한다.
func (s *Search) Cache() *sync.Map {
	return &s.cache
}

// extractToolName은 canonical MCP 이름에서 tool 이름만 추출한다.
func extractToolName(canonicalName, serverID string) string {
	prefix := "mcp__" + serverID + "__"
	if len(canonicalName) > len(prefix) {
		return canonicalName[len(prefix):]
	}
	return ""
}
