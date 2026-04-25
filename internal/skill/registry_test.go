package skill

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestCanInvoke_DisableModelInvocation는 CanInvoke 3-branch actor matrix를 검증한다.
// RED #8 — AC-SK-007, REQ-SK-009
func TestCanInvoke_DisableModelInvocation(t *testing.T) {
	trueVal := true
	falseVal := false

	logger := zap.NewNop()
	reg := NewSkillRegistry(logger)

	reg.replaceInternal(map[string]*SkillDefinition{
		"skill-a": {
			ID: "skill-a",
			Frontmatter: SkillFrontmatter{
				DisableModelInvocation: true,
			},
		},
		"skill-b": {
			ID: "skill-b",
			Frontmatter: SkillFrontmatter{
				DisableModelInvocation: false,
			},
		},
		"skill-c": {
			ID: "skill-c",
			// DisableModelInvocation 기본값(false)
		},
		"skill-d": {
			ID: "skill-d",
			Frontmatter: SkillFrontmatter{
				UserInvocable: &trueVal,
			},
		},
		"skill-e": {
			ID: "skill-e",
			Frontmatter: SkillFrontmatter{
				UserInvocable: &falseVal,
			},
		},
	})

	tests := []struct {
		skillID  string
		actor    string
		expected bool
		desc     string
	}{
		// skill-a: disable-model-invocation=true
		{"skill-a", "model", false, "A: model은 false"},
		{"skill-a", "user", true, "A: user는 true"},
		{"skill-a", "hook", true, "A: hook는 true"},
		{"skill-a", "plugin", false, "A: unknown actor는 false (default-deny)"},

		// skill-b: disable-model-invocation=false (명시)
		{"skill-b", "model", true, "B: model은 true"},
		{"skill-b", "user", true, "B: user는 true"},
		{"skill-b", "hook", true, "B: hook는 true"},
		{"skill-b", "plugin", false, "B: unknown actor는 false"},

		// skill-c: disable-model-invocation 미지정 (기본 false)
		{"skill-c", "model", true, "C: model은 true"},
		{"skill-c", "user", true, "C: user는 true"},
		{"skill-c", "hook", true, "C: hook는 true"},
		{"skill-c", "plugin", false, "C: unknown actor는 false"},
		{"skill-c", "", false, "C: 빈 actor는 false"},

		// skill-d: user-invocable=true
		{"skill-d", "user", true, "D: user-invocable=true → user는 true"},

		// skill-e: user-invocable=false
		{"skill-e", "user", false, "E: user-invocable=false → user는 false"},

		// 존재하지 않는 skill
		{"nonexistent", "user", false, "존재하지 않는 skill은 false"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := reg.CanInvoke(tt.skillID, tt.actor)
			assert.Equal(t, tt.expected, result, tt.desc)
		})
	}
}

// TestRegistry_Replace_Atomic는 Replace가 atomic swap으로 race condition 없이
// 동작하는지 검증한다.
// AC-SK-014, REQ-SK-016
func TestRegistry_Replace_Atomic(t *testing.T) {
	logger := zap.NewNop()
	reg := NewSkillRegistry(logger)

	defV1 := &SkillDefinition{
		ID:          "skill-a",
		Frontmatter: SkillFrontmatter{Name: "skill-a-v1"},
	}
	reg.replaceInternal(map[string]*SkillDefinition{
		"skill-a": defV1,
	})

	defV2 := &SkillDefinition{
		ID:          "skill-a",
		Frontmatter: SkillFrontmatter{Name: "skill-a-v2"},
	}
	defB := &SkillDefinition{
		ID:          "skill-b",
		Frontmatter: SkillFrontmatter{Name: "skill-b"},
	}

	newMap := map[string]*SkillDefinition{
		"skill-a": defV2,
		"skill-b": defB,
	}

	var wg sync.WaitGroup
	const readers = 100

	// 100개의 리더 goroutine이 동시에 Get을 호출
	results := make([]string, readers)
	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func(idx int) {
			defer wg.Done()
			if def, ok := reg.Get("skill-a"); ok {
				results[idx] = def.Frontmatter.Name
			}
		}(i)
	}

	// Replace 실행
	reg.Replace(newMap)
	wg.Wait()

	// 각 리더는 v1 또는 v2만 관측해야 함 (혼합 상태 없음)
	for i, name := range results {
		if name != "" {
			assert.True(t, name == "skill-a-v1" || name == "skill-a-v2",
				"reader %d: 혼합 상태가 관측되지 않아야 한다, got: %s", i, name)
		}
	}

	// Replace 완료 후 상태 확인
	def, ok := reg.Get("skill-a")
	require.True(t, ok)
	assert.Equal(t, "skill-a-v2", def.Frontmatter.Name)

	defBResult, okB := reg.Get("skill-b")
	require.True(t, okB)
	assert.Equal(t, "skill-b", defBResult.Frontmatter.Name)

	_, nonexist := reg.Get("nonexistent")
	assert.False(t, nonexist)

	// in-place mutation 없음 확인: defV1 원본은 변경되지 않아야 함
	assert.Equal(t, "skill-a-v1", defV1.Frontmatter.Name,
		"Replace 전 SkillDefinition은 in-place로 변경되지 않아야 한다")
}

// TestRegistry_Get_NotFound는 없는 skill 조회 시 (nil, false)를 반환하는지 검증한다.
func TestRegistry_Get_NotFound(t *testing.T) {
	logger := zap.NewNop()
	reg := NewSkillRegistry(logger)

	def, ok := reg.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, def)
}

// TestRegistry_Replace_CopiesMap는 Replace가 외부 map을 deep copy하여
// caller mutation으로부터 격리되는지 검증한다.
// REQ-SK-016
func TestRegistry_Replace_CopiesMap(t *testing.T) {
	logger := zap.NewNop()
	reg := NewSkillRegistry(logger)

	originalDef := &SkillDefinition{
		ID:          "test",
		Frontmatter: SkillFrontmatter{Name: "original"},
	}
	externalMap := map[string]*SkillDefinition{
		"test": originalDef,
	}

	reg.Replace(externalMap)

	// 외부 map을 변경해도 레지스트리에 영향이 없어야 함
	externalMap["test"] = &SkillDefinition{
		ID:          "test",
		Frontmatter: SkillFrontmatter{Name: "mutated"},
	}

	def, ok := reg.Get("test")
	require.True(t, ok)
	assert.Equal(t, "original", def.Frontmatter.Name,
		"외부 map 변경이 레지스트리에 영향을 주지 않아야 한다")
}
