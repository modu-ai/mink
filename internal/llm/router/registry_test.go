// Package router_test는 router 패키지의 외부 테스트를 포함한다.
package router_test

import (
	"sort"
	"testing"

	"github.com/modu-ai/goose/internal/llm/router"
)

// TestRegistry_DefaultRegistry_HasAtLeastFifteenProviders는 DefaultRegistry가
// 15개 이상의 provider를 포함하는지 검증한다. REQ-ROUTER-003.
func TestRegistry_DefaultRegistry_HasAtLeastFifteenProviders(t *testing.T) {
	t.Parallel()

	reg := router.DefaultRegistry()
	providers := reg.List()

	const minProviders = 15
	if len(providers) < minProviders {
		t.Errorf("DefaultRegistry provider 수=%d, want >= %d", len(providers), minProviders)
	}
}

// TestRegistry_DefaultRegistry_AdapterReadyProviders는 SPEC-002 완료 후 adapter-ready provider가
// 정확히 15종인지 검증한다. REQ-ROUTER-003, REQ-ADP2-005, AC-ADP2-016.
func TestRegistry_DefaultRegistry_AdapterReadyProviders(t *testing.T) {
	t.Parallel()

	reg := router.DefaultRegistry()
	providers := reg.List()

	// SPEC-002 완료 후 15 provider 전부 AdapterReady=true
	expectedAdapterReady := map[string]bool{
		// SPEC-001 6종
		"anthropic": false,
		"openai":    false,
		"google":    false,
		"xai":       false,
		"deepseek":  false,
		"ollama":    false,
		// SPEC-002 9종
		"glm":        false,
		"groq":       false,
		"openrouter": false,
		"together":   false,
		"fireworks":  false,
		"cerebras":   false,
		"mistral":    false,
		"qwen":       false,
		"kimi":       false,
	}

	adapterReadyCount := 0
	for _, p := range providers {
		if p.AdapterReady {
			adapterReadyCount++
			if _, ok := expectedAdapterReady[p.Name]; ok {
				expectedAdapterReady[p.Name] = true
			}
		}
	}

	if adapterReadyCount != 15 {
		t.Errorf("AdapterReady provider 수=%d, want 15 (SPEC-002 완료 후)", adapterReadyCount)
	}

	for name, found := range expectedAdapterReady {
		if !found {
			t.Errorf("예상 AdapterReady provider %q가 등록되지 않음", name)
		}
	}
}

// TestRegistry_DefaultRegistry_RequiredFields는 모든 등록된 provider가
// 필수 필드를 가지는지 검증한다.
func TestRegistry_DefaultRegistry_RequiredFields(t *testing.T) {
	t.Parallel()

	reg := router.DefaultRegistry()
	providers := reg.List()

	for _, p := range providers {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()

			if p.Name == "" {
				t.Error("Name이 비어 있음")
			}
			if p.DisplayName == "" {
				t.Errorf("provider %q: DisplayName이 비어 있음", p.Name)
			}
			if p.AuthType == "" {
				t.Errorf("provider %q: AuthType이 비어 있음", p.Name)
			}
			// ollama는 local이므로 base URL이 있어야 함, custom은 없어도 됨
			if p.Name != "custom" && p.DefaultBaseURL == "" {
				t.Errorf("provider %q: DefaultBaseURL이 비어 있음 (custom이 아닌 경우)", p.Name)
			}
		})
	}
}

// TestRegistry_Register_Duplicate_ReturnsError는 동일 이름의 provider를
// 중복 등록할 때 에러를 반환하는지 검증한다.
func TestRegistry_Register_Duplicate_ReturnsError(t *testing.T) {
	t.Parallel()

	reg := router.NewRegistry()

	meta := &router.ProviderMeta{
		Name:           "test-provider",
		DisplayName:    "Test Provider",
		DefaultBaseURL: "https://api.test.com/v1",
		AuthType:       "api_key",
	}

	if err := reg.Register(meta); err != nil {
		t.Fatalf("첫 번째 Register 실패: %v", err)
	}

	if err := reg.Register(meta); err == nil {
		t.Error("중복 Register: 에러 없이 성공 — 에러 기대")
	}
}

// TestRegistry_Get_Unregistered_ReturnsFalse는 미등록 provider를 조회할 때
// false를 반환하는지 검증한다.
func TestRegistry_Get_Unregistered_ReturnsFalse(t *testing.T) {
	t.Parallel()

	reg := router.NewRegistry()

	_, found := reg.Get("nonexistent")
	if found {
		t.Error("미등록 provider Get: found=true, want false")
	}
}

// TestRegistry_Get_Registered_ReturnsProvider는 등록된 provider를 조회할 때
// 올바른 metadata를 반환하는지 검증한다.
func TestRegistry_Get_Registered_ReturnsProvider(t *testing.T) {
	t.Parallel()

	reg := router.NewRegistry()

	meta := &router.ProviderMeta{
		Name:           "my-provider",
		DisplayName:    "My Provider",
		DefaultBaseURL: "https://api.myprovider.com/v1",
		AuthType:       "api_key",
		SupportsStream: true,
		AdapterReady:   true,
	}

	if err := reg.Register(meta); err != nil {
		t.Fatalf("Register 실패: %v", err)
	}

	got, found := reg.Get("my-provider")
	if !found {
		t.Fatal("등록된 provider를 Get으로 찾을 수 없음")
	}
	if got.Name != meta.Name {
		t.Errorf("Name=%q, want %q", got.Name, meta.Name)
	}
	if got.DefaultBaseURL != meta.DefaultBaseURL {
		t.Errorf("DefaultBaseURL=%q, want %q", got.DefaultBaseURL, meta.DefaultBaseURL)
	}
	if got.AdapterReady != meta.AdapterReady {
		t.Errorf("AdapterReady=%v, want %v", got.AdapterReady, meta.AdapterReady)
	}
}

// TestRegistry_List_ReturnsDeterministicOrder는 List()가 결정적 순서를
// 반환하는지 검증한다 (멱등성).
func TestRegistry_List_ReturnsDeterministicOrder(t *testing.T) {
	t.Parallel()

	reg := router.DefaultRegistry()

	list1 := reg.List()
	list2 := reg.List()

	if len(list1) != len(list2) {
		t.Fatalf("List() 길이 불일치: %d vs %d", len(list1), len(list2))
	}

	names1 := make([]string, len(list1))
	names2 := make([]string, len(list2))
	for i, p := range list1 {
		names1[i] = p.Name
	}
	for i, p := range list2 {
		names2[i] = p.Name
	}

	// 두 호출 모두 동일한 순서여야 함
	for i := range names1 {
		if names1[i] != names2[i] {
			t.Errorf("List()[%d] 순서 불일치: %q vs %q", i, names1[i], names2[i])
		}
	}
}

// TestRegistry_Register_InvalidAuthType_ReturnsError는 유효하지 않은 AuthType으로
// 등록할 때 에러를 반환하는지 검증한다.
func TestRegistry_Register_InvalidAuthType_ReturnsError(t *testing.T) {
	t.Parallel()

	reg := router.NewRegistry()

	meta := &router.ProviderMeta{
		Name:           "bad-provider",
		DisplayName:    "Bad Provider",
		DefaultBaseURL: "https://api.bad.com/v1",
		AuthType:       "invalid_auth", // 유효하지 않은 auth type
	}

	if err := reg.Register(meta); err == nil {
		t.Error("유효하지 않은 AuthType으로 Register: 에러 없이 성공 — 에러 기대")
	}
}

// TestRegistry_Register_EmptyName_ReturnsError는 Name이 비어 있을 때
// 에러를 반환하는지 검증한다.
func TestRegistry_Register_EmptyName_ReturnsError(t *testing.T) {
	t.Parallel()

	reg := router.NewRegistry()

	meta := &router.ProviderMeta{
		Name:           "",
		DisplayName:    "No Name Provider",
		DefaultBaseURL: "https://api.test.com/v1",
		AuthType:       "api_key",
	}

	if err := reg.Register(meta); err == nil {
		t.Error("빈 Name으로 Register: 에러 없이 성공 — 에러 기대")
	}
}

// TestRegistry_DefaultRegistry_SpecificProviders는 요구하는 특정 provider들이
// 모두 등록되어 있는지 검증한다.
func TestRegistry_DefaultRegistry_SpecificProviders(t *testing.T) {
	t.Parallel()

	reg := router.DefaultRegistry()

	requiredProviders := []string{
		// SPEC-001 adapter-ready (6종)
		"anthropic", "openai", "google", "xai", "deepseek", "ollama",
		// SPEC-001 metadata-only (항상 존재)
		"nous", "minimax", "cohere",
		// SPEC-002 신규/업데이트 (9종)
		"glm", "groq", "openrouter", "together", "fireworks", "cerebras", "mistral", "qwen", "kimi",
	}

	for _, name := range requiredProviders {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, found := reg.Get(name)
			if !found {
				t.Errorf("필수 provider %q가 DefaultRegistry에 등록되지 않음", name)
			}
		})
	}
}

// TestRegistry_MetadataOnly_NotAdapterReady는 SPEC-002 미구현 provider들이
// AdapterReady=false인지 검증한다 (nous, minimax, cohere은 본 SPEC 대상 외).
func TestRegistry_MetadataOnly_NotAdapterReady(t *testing.T) {
	t.Parallel()

	reg := router.DefaultRegistry()

	// SPEC-002 범위 외: nous, minimax, cohere만 metadata-only 유지
	metadataOnlyProviders := []string{
		"nous", "minimax", "cohere",
	}

	for _, name := range metadataOnlyProviders {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p, found := reg.Get(name)
			if !found {
				t.Skipf("provider %q not registered (skip)", name)
			}
			if p.AdapterReady {
				t.Errorf("metadata-only provider %q: AdapterReady=true, want false", name)
			}
		})
	}
}

// TestRegistry_DefaultRegistry_GLMEndpointUpdated는 AC-ADP2-016을 검증한다.
// GLM endpoint가 api.z.ai/api/paas/v4로 업데이트되었는지 확인.
func TestRegistry_DefaultRegistry_GLMEndpointUpdated(t *testing.T) {
	t.Parallel()

	reg := router.DefaultRegistry()
	glm, found := reg.Get("glm")
	if !found {
		t.Fatal("glm provider가 등록되지 않음")
	}

	const expectedURL = "https://api.z.ai/api/paas/v4"
	if glm.DefaultBaseURL != expectedURL {
		t.Errorf("GLM BaseURL=%q, want %q (REQ-ADP2-022: Z.ai endpoint 이전)", glm.DefaultBaseURL, expectedURL)
	}
	if !glm.AdapterReady {
		t.Error("GLM AdapterReady=false, want true (SPEC-002 완료)")
	}
}

// TestRegistry_DefaultRegistry_NewProviders는 SPEC-002 신규 provider들이 등록되었는지 검증한다.
func TestRegistry_DefaultRegistry_NewProviders(t *testing.T) {
	t.Parallel()

	reg := router.DefaultRegistry()

	newProviders := []string{"together", "fireworks", "cerebras"}
	for _, name := range newProviders {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p, found := reg.Get(name)
			if !found {
				t.Fatalf("SPEC-002 신규 provider %q가 등록되지 않음", name)
			}
			if !p.AdapterReady {
				t.Errorf("provider %q: AdapterReady=false, want true", name)
			}
		})
	}
}

// TestRegistry_List_IsSorted는 List()가 이름 알파벳 순으로 정렬되어 있는지 검증한다.
func TestRegistry_List_IsSorted(t *testing.T) {
	t.Parallel()

	reg := router.DefaultRegistry()
	providers := reg.List()

	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name
	}

	sorted := make([]string, len(names))
	copy(sorted, names)
	sort.Strings(sorted)

	for i := range names {
		if names[i] != sorted[i] {
			t.Errorf("List() 순서 오류: [%d]=%q, want %q (정렬 기대)",
				i, names[i], sorted[i])
		}
	}
}
