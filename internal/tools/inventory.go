package tools

import (
	"context"
	"encoding/json"
)

// InventoryFilter는 Inventory.ForModel 필터 옵션이다.
type InventoryFilter struct {
	// CoordinatorMode가 true이면 ScopeLeaderOnly tool을 제외한다.
	// REQ-TOOLS-012
	CoordinatorMode bool
}

// Inventory는 Registry에서 tool descriptor 목록을 생성한다.
type Inventory struct {
	registry *Registry
}

// NewInventory는 새 Inventory를 생성한다.
func NewInventory(r *Registry) *Inventory {
	return &Inventory{registry: r}
}

// ForModel은 필터를 적용한 tool descriptor 목록을 반환한다.
// REQ-TOOLS-005: alphabetical sort, deterministic output.
// REQ-TOOLS-012: CoordinatorMode=true 시 ScopeLeaderOnly 제외.
func (inv *Inventory) ForModel(ctx context.Context, filter InventoryFilter) []ToolDescriptor {
	names := inv.registry.ListNames()
	result := make([]ToolDescriptor, 0, len(names))

	for _, name := range names {
		entry, ok := inv.registry.ResolveEntry(name)
		if !ok {
			continue
		}
		desc := entry.descriptor

		// CoordinatorMode 필터
		if filter.CoordinatorMode && desc.Scope == ScopeLeaderOnly {
			continue
		}

		result = append(result, desc)
	}
	return result
}

// ForModelJSON은 ForModel 결과를 JSON 직렬화한다.
// REQ-TOOLS-005: 동일 tool 집합에서 바이트 동일 출력 보장.
func (inv *Inventory) ForModelJSON(ctx context.Context, filter InventoryFilter) (json.RawMessage, error) {
	descriptors := inv.ForModel(ctx, filter)
	return json.Marshal(descriptors)
}
