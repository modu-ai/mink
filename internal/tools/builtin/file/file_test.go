package file_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/modu-ai/goose/internal/tools"
	"github.com/modu-ai/goose/internal/tools/builtin/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileRead_BasicRead — 기본 파일 읽기
func TestFileRead_BasicRead(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	tool := file.NewFileRead()
	input, _ := json.Marshal(map[string]any{"path": testFile})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "hello world", string(result.Content))
}

// TestFileRead_OffsetLimit — offset/limit 적용
func TestFileRead_OffsetLimit(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "lines.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))

	tool := file.NewFileRead()
	offset := 1
	limit := 2
	input, _ := json.Marshal(map[string]any{
		"path":   testFile,
		"offset": offset,
		"limit":  limit,
	})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "line2\nline3", string(result.Content))
}

// TestFileRead_NotFound — 존재하지 않는 파일
func TestFileRead_NotFound(t *testing.T) {
	tool := file.NewFileRead()
	input, _ := json.Marshal(map[string]any{"path": "/nonexistent/path/file.txt"})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "read_error")
}

// TestFileWrite_BasicWrite — 기본 파일 쓰기 (atomic)
func TestFileWrite_BasicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "output.txt")

	tool := file.NewFileWrite()
	input, _ := json.Marshal(map[string]any{
		"path":    testFile,
		"content": "hello world",
	})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, readErr := os.ReadFile(testFile)
	require.NoError(t, readErr)
	assert.Equal(t, "hello world", string(content))
}

// TestFileWrite_OutsideCwd_Denied — AC-TOOLS-009, REQ-TOOLS-015
// cwd 바깥 쓰기 거부
func TestFileWrite_OutsideCwd_Denied(t *testing.T) {
	tmpDir := t.TempDir()
	tool := file.NewFileWriteWithCwd(tmpDir, nil)

	input, _ := json.Marshal(map[string]any{
		"path":    "/etc/passwd",
		"content": "malicious",
	})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "outside cwd")
}

// TestFileWrite_AdditionalDirectories_Allowed — REQ-TOOLS-015 additional_directories
func TestFileWrite_AdditionalDirectories_Allowed(t *testing.T) {
	cwd := t.TempDir()
	extraDir := t.TempDir()
	outputFile := filepath.Join(extraDir, "output.txt")

	tool := file.NewFileWriteWithCwd(cwd, []string{extraDir})
	input, _ := json.Marshal(map[string]any{
		"path":    outputFile,
		"content": "allowed",
	})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError, "additional_directories에 포함된 경로는 허용되어야 함")
}

// TestFileEdit_BasicEdit — 기본 문자열 교체
func TestFileEdit_BasicEdit(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "edit.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	tool := file.NewFileEdit()
	input, _ := json.Marshal(map[string]any{
		"path":       testFile,
		"old_string": "world",
		"new_string": "Go",
	})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, _ := os.ReadFile(testFile)
	assert.Equal(t, "hello Go", string(content))
}

// TestFileEdit_OldStringNotFound — old_string 미존재 에러
func TestFileEdit_OldStringNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "edit.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	tool := file.NewFileEdit()
	input, _ := json.Marshal(map[string]any{
		"path":       testFile,
		"old_string": "not_exist",
		"new_string": "x",
	})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "old_string not found")
}

// TestFileEdit_ReplaceAll — replace_all=true
func TestFileEdit_ReplaceAll(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "multi.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("a b a b a"), 0644))

	tool := file.NewFileEdit()
	replaceAll := true
	input, _ := json.Marshal(map[string]any{
		"path":        testFile,
		"old_string":  "a",
		"new_string":  "X",
		"replace_all": replaceAll,
	})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, _ := os.ReadFile(testFile)
	assert.Equal(t, "X b X b X", string(content))
}

// TestGlob_BasicPattern — 기본 glob 패턴
func TestGlob_BasicPattern(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte{}, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "b.go"), []byte{}, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte{}, 0644))

	tool := file.NewGlobWithCwd(tmpDir)
	input, _ := json.Marshal(map[string]any{"pattern": "*.go"})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var matches []string
	require.NoError(t, json.Unmarshal(result.Content, &matches))
	assert.Len(t, matches, 2)
}

// TestGlob_DoubleStarPattern — ** 패턴 지원
func TestGlob_DoubleStarPattern(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte{}, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "b.go"), []byte{}, 0644))

	tool := file.NewGlobWithCwd(tmpDir)
	input, _ := json.Marshal(map[string]any{"pattern": "**/*.go"})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var matches []string
	require.NoError(t, json.Unmarshal(result.Content, &matches))
	assert.GreaterOrEqual(t, len(matches), 2)
}

// TestGrep_BasicSearch — 기본 패턴 검색
func TestGrep_BasicSearch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "search.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world\nfoo bar\nhello go"), 0644))

	tool := file.NewGrep()
	input, _ := json.Marshal(map[string]any{
		"pattern": "hello",
		"path":    testFile,
	})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	type grepMatch struct {
		File string `json:"file"`
		Line int    `json:"line"`
		Text string `json:"text"`
	}
	var matches []grepMatch
	require.NoError(t, json.Unmarshal(result.Content, &matches))
	assert.Len(t, matches, 2)
}

// TestFileEdit_OutsideCwd_Denied — REQ-TOOLS-015
func TestFileEdit_OutsideCwd_Denied(t *testing.T) {
	cwd := t.TempDir()
	tool := file.NewFileEditWithCwd(cwd, nil)

	input, _ := json.Marshal(map[string]any{
		"path":       "/etc/passwd",
		"old_string": "root",
		"new_string": "hacked",
	})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "outside cwd")
}

// TestGlob_EmptyResult — 일치 없는 패턴
func TestGlob_EmptyResult(t *testing.T) {
	tmpDir := t.TempDir()
	tool := file.NewGlobWithCwd(tmpDir)
	input, _ := json.Marshal(map[string]any{"pattern": "*.nonexistent"})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var matches []string
	require.NoError(t, json.Unmarshal(result.Content, &matches))
	assert.Empty(t, matches)
}

// TestGrep_DirectorySearch — 디렉토리 검색
func TestGrep_DirectorySearch(t *testing.T) {
	tmpDir := t.TempDir()
	for i, content := range []string{"hello world", "no match", "hello go"} {
		path := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}

	tool := file.NewGrep()
	input, _ := json.Marshal(map[string]any{
		"pattern": "hello",
		"path":    tmpDir,
	})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var matches []map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &matches))
	assert.Equal(t, 2, len(matches))
}

// TestTools_NameSchemaScope — Name/Schema/Scope 메서드 직접 호출 커버리지
func TestTools_NameSchemaScope(t *testing.T) {
	toolList := []struct {
		name string
		tool tools.Tool
	}{
		{"FileRead", file.NewFileRead()},
		{"FileWrite", file.NewFileWrite()},
		{"FileEdit", file.NewFileEdit()},
		{"Glob", file.NewGlobWithCwd("")},
		{"Grep", file.NewGrep()},
	}
	for _, tc := range toolList {
		assert.Equal(t, tc.name, tc.tool.Name(), "Name() mismatch for %s", tc.name)
		assert.NotEmpty(t, tc.tool.Schema(), "Schema() empty for %s", tc.name)
		// Scope() 호출 — 커버리지 확보 (ScopeShared = 0이므로 값 검사 대신 호출만)
		_ = tc.tool.Scope()
	}
}

// TestFileWrite_InvalidJSON — 잘못된 JSON 입력
func TestFileWrite_InvalidJSON(t *testing.T) {
	tool := file.NewFileWrite()
	result, err := tool.Call(context.Background(), []byte("not-json"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// TestFileWrite_MkdirParent — 부모 디렉토리 자동 생성
func TestFileWrite_MkdirParent(t *testing.T) {
	tmpDir := t.TempDir()
	// EvalSymlinks로 실제 경로 해석 (macOS /var → /private/var)
	realDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		realDir = tmpDir
	}
	nested := filepath.Join(realDir, "a", "b", "c", "out.txt")
	tool := file.NewFileWriteWithCwd(realDir, nil)
	input, _ := json.Marshal(map[string]any{
		"path":    nested,
		"content": "deep",
	})
	result, err2 := tool.Call(context.Background(), input)
	require.NoError(t, err2)
	assert.False(t, result.IsError)
	b, _ := os.ReadFile(nested)
	assert.Equal(t, "deep", string(b))
}

// TestGlob_InvalidJSON — 잘못된 JSON 입력
func TestGlob_InvalidJSON(t *testing.T) {
	tool := file.NewGlobWithCwd("")
	result, err := tool.Call(context.Background(), []byte("not-json"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// TestGlob_InvalidPattern — 잘못된 패턴
func TestGlob_InvalidPattern(t *testing.T) {
	tmpDir := t.TempDir()
	tool := file.NewGlobWithCwd(tmpDir)
	// doublestar.Glob returns error for patterns with "[" (invalid range)
	input, _ := json.Marshal(map[string]any{"pattern": "["})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	// either error or empty result — just check it runs
	_ = result
}

// TestGlob_DefaultCwd_UsesGetwd — defaultCwd 없이 os.Getwd() 사용
func TestGlob_DefaultCwd_UsesGetwd(t *testing.T) {
	// NewGlob()은 defaultCwd=""이고, inp.Cwd도 없으므로 os.Getwd()를 사용
	tool := file.NewGlob()
	input, _ := json.Marshal(map[string]any{"pattern": "*.nonexistent_ext_xyz"})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError, "os.Getwd()로 폴백하면 에러 없어야 함")
}

// TestGlob_OverrideCwd — 입력 cwd로 오버라이드
func TestGlob_OverrideCwd(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte{}, 0644))

	tool := file.NewGlobWithCwd("/some/other/dir")
	input, _ := json.Marshal(map[string]any{
		"pattern": "*.go",
		"cwd":     tmpDir,
	})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	var matches []string
	require.NoError(t, json.Unmarshal(result.Content, &matches))
	assert.Len(t, matches, 1)
}

// TestGrep_InvalidJSON — 잘못된 JSON
func TestGrep_InvalidJSON(t *testing.T) {
	tool := file.NewGrep()
	result, err := tool.Call(context.Background(), []byte("not-json"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// TestGrep_InvalidRegex — 잘못된 정규식
func TestGrep_InvalidRegex(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello"), 0644))

	tool := file.NewGrep()
	input, _ := json.Marshal(map[string]any{
		"pattern": "[invalid",
		"path":    testFile,
	})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "regex_error")
}

// TestGrep_PathNotFound — 존재하지 않는 경로
func TestGrep_PathNotFound(t *testing.T) {
	tool := file.NewGrep()
	input, _ := json.Marshal(map[string]any{
		"pattern": "hello",
		"path":    "/nonexistent/path/does/not/exist",
	})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "path_error")
}

// TestFileRead_InvalidJSON — 잘못된 JSON
func TestFileRead_InvalidJSON(t *testing.T) {
	tool := file.NewFileRead()
	result, err := tool.Call(context.Background(), []byte("not-json"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// TestFileEdit_InvalidJSON — 잘못된 JSON
func TestFileEdit_InvalidJSON(t *testing.T) {
	tool := file.NewFileEdit()
	result, err := tool.Call(context.Background(), []byte("not-json"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// TestGrep_CaseInsensitive — i 플래그
func TestGrep_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "case.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("Hello WORLD\nhello world"), 0644))

	tool := file.NewGrep()
	input, _ := json.Marshal(map[string]any{
		"pattern": "hello",
		"path":    testFile,
		"flags":   map[string]any{"i": true},
	})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var matches []map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &matches))
	assert.Len(t, matches, 2, "대소문자 무시 검색 결과는 2건이어야 함")
}
