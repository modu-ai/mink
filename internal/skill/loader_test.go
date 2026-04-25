package skill

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestLoadSkillsDir_Minimal는 최소 유효 SKILL.md가 있는 디렉토리에서
// 정상적으로 레지스트리를 구성하는지 검증한다.
// AC-SK-001, REQ-SK-005
func TestLoadSkillsDir_Minimal(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "hello")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: hello
description: "say hi"
---

Hello world
`), 0644))

	logger := zap.NewNop()
	reg, errs := LoadSkillsDir(dir, WithLogger(logger))
	require.NotNil(t, reg)
	assert.Empty(t, errs)

	def, ok := reg.Get("hello")
	require.True(t, ok)
	assert.Equal(t, "hello", def.ID)
	assert.Equal(t, EffortL1, def.Effort)
	assert.Equal(t, TriggerInline, def.Trigger)
	assert.False(t, def.IsRemote)
}

// TestLoadSkillsDir_SymlinkEscape는 root 외부를 가리키는 symlink가
// 안전하게 처리되는지 검증한다.
// RED #9 — AC-SK-008, REQ-SK-015
func TestLoadSkillsDir_SymlinkEscape(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root 환경에서는 symlink escape가 의미 없음")
	}

	dir := t.TempDir()

	// 정상 skill
	goodDir := filepath.Join(dir, "good")
	require.NoError(t, os.MkdirAll(goodDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(goodDir, "SKILL.md"), []byte(`---
name: good
description: "good skill"
---

Good skill body
`), 0644))

	// symlink escape: /etc/passwd 또는 외부 파일을 가리키는 symlink
	evilDir := filepath.Join(dir, "evil")
	require.NoError(t, os.MkdirAll(evilDir, 0755))

	// 외부 파일 생성 (root 외부)
	outsideFile := filepath.Join(t.TempDir(), "outside.md")
	require.NoError(t, os.WriteFile(outsideFile, []byte(`---
name: outside
description: "outside skill"
---

Outside
`), 0644))

	symlinkPath := filepath.Join(evilDir, "SKILL.md")
	require.NoError(t, os.Symlink(outsideFile, symlinkPath))

	logger := zap.NewNop()
	reg, errs := LoadSkillsDir(dir, WithLogger(logger))
	require.NotNil(t, reg)

	// evil skill은 등록되지 않아야 함
	_, evilOk := reg.Get("outside")
	assert.False(t, evilOk, "symlink escape skill은 레지스트리에 등록되지 않아야 한다")

	// good skill은 정상 등록
	goodDef, goodOk := reg.Get("good")
	assert.True(t, goodOk, "정상 skill은 레지스트리에 등록되어야 한다")
	assert.NotNil(t, goodDef)

	// error slice에 ErrSymlinkEscape 포함
	var foundEscape bool
	for _, e := range errs {
		var escErr ErrSymlinkEscape
		if isSymlinkEscapeErr(e, &escErr) {
			foundEscape = true
			break
		}
	}
	assert.True(t, foundEscape, "error slice에 ErrSymlinkEscape가 포함되어야 한다: %v", errs)
}

// TestLoadSkillsDir_DuplicateID는 중복 ID 처리가 partial-success 방식으로
// 동작하는지 검증한다.
// RED #10 — AC-SK-009, REQ-SK-004, REQ-SK-005
func TestLoadSkillsDir_DuplicateID(t *testing.T) {
	dir := t.TempDir()

	// dup_a: 먼저 등록될 skill (같은 name "same")
	dupADir := filepath.Join(dir, "a_first")
	require.NoError(t, os.MkdirAll(dupADir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dupADir, "SKILL.md"), []byte(`---
name: same
description: "first duplicate"
---

First
`), 0644))

	// dup_b: 나중에 등록될 skill (같은 name "same")
	dupBDir := filepath.Join(dir, "b_second")
	require.NoError(t, os.MkdirAll(dupBDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dupBDir, "SKILL.md"), []byte(`---
name: same
description: "second duplicate"
---

Second
`), 0644))

	// unique: 유니크한 skill
	uniqueDir := filepath.Join(dir, "c_unique")
	require.NoError(t, os.MkdirAll(uniqueDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(uniqueDir, "SKILL.md"), []byte(`---
name: unique
description: "unique skill"
---

Unique
`), 0644))

	logger := zap.NewNop()
	reg, errs := LoadSkillsDir(dir, WithLogger(logger))
	require.NotNil(t, reg)

	// (a) 레지스트리에 "same"은 1개만
	sameDef, sameOk := reg.Get("same")
	assert.True(t, sameOk)
	assert.NotNil(t, sameDef)
	// 첫 번째 등록 항목 보존 (description으로 구분)
	assert.Equal(t, "first duplicate", sameDef.Frontmatter.Description,
		"첫 번째 등록된 skill이 보존되어야 한다")

	// (b) error slice에 ErrDuplicateSkillID 포함
	var foundDup bool
	for _, e := range errs {
		var dupErr ErrDuplicateSkillID
		if isDuplicateSkillIDErr(e, &dupErr) {
			foundDup = true
			break
		}
	}
	assert.True(t, foundDup, "error slice에 ErrDuplicateSkillID가 포함되어야 한다: %v", errs)

	// (c) unique skill은 정상 등록
	uniqueDef, uniqueOk := reg.Get("unique")
	assert.True(t, uniqueOk)
	assert.NotNil(t, uniqueDef)
}

// TestLoadSkillsDir_MultipleSkills는 여러 skill이 모두 로드되는지 검증한다.
func TestLoadSkillsDir_MultipleSkills(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"alpha", "beta", "gamma"} {
		skillDir := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(skillDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: \""+name+" skill\"\n---\n\n"+name+" body\n"), 0644))
	}

	logger := zap.NewNop()
	reg, errs := LoadSkillsDir(dir, WithLogger(logger))
	require.NotNil(t, reg)
	assert.Empty(t, errs)

	for _, name := range []string{"alpha", "beta", "gamma"} {
		def, ok := reg.Get(name)
		assert.True(t, ok, "%s skill이 등록되어야 한다", name)
		assert.NotNil(t, def)
	}
}
