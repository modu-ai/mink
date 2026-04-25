package skill

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestLoadRemoteSkill_HTTP는 HTTP로 원격 SKILL.md를 로드하는지 검증한다.
// RED #11 — AC-SK-010, REQ-SK-012, REQ-SK-022d
func TestLoadRemoteSkill_HTTP(t *testing.T) {
	// 테스트 서버 설정
	remoteContent := `---
name: remote-skill
description: "remote skill via HTTP"
shell:
  executable: "/bin/sh"
---

Remote skill body with ${CLAUDE_SKILL_DIR}
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(remoteContent))
	}))
	defer server.Close()

	logger := zap.NewNop()
	def, err := LoadRemoteSkill(server.URL+"/skills/remote.md", logger)
	require.NoError(t, err)
	require.NotNil(t, def)

	// ID에 _canonical_ 접두사
	assert.True(t, len(def.ID) > 0)
	assert.Contains(t, def.ID, "_canonical_",
		"remote skill ID는 _canonical_ 접두사를 포함해야 한다")

	// IsRemote == true
	assert.True(t, def.IsRemote)

	// 동일한 parse-time no-exec 제약 적용
	// shell.executable은 리터럴 보존, 실행되지 않음
	require.NotNil(t, def.Frontmatter.Shell)
	assert.Equal(t, "/bin/sh", def.Frontmatter.Shell.Executable,
		"remote skill의 shell.executable도 리터럴로 보존되어야 한다")
}

// TestLoadRemoteSkill_HTTPError는 HTTP 에러 시 적절히 처리되는지 검증한다.
func TestLoadRemoteSkill_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	logger := zap.NewNop()
	def, err := LoadRemoteSkill(server.URL+"/notfound", logger)
	assert.Error(t, err)
	assert.Nil(t, def)
}

// TestLoadRemoteSkill_InvalidURI는 잘못된 URI 처리를 검증한다.
func TestLoadRemoteSkill_InvalidURI(t *testing.T) {
	logger := zap.NewNop()
	def, err := LoadRemoteSkill("not-a-valid-uri", logger)
	assert.Error(t, err)
	assert.Nil(t, def)
}
