package subagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemdir_ScopeResolutionOrder는 3-scope 우선순위가 local→project→user임을 검증한다.
// AC-SA-004: REQ-SA-003
func TestMemdir_ScopeResolutionOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	proj := root

	// 3개 scope에 동일 키가 다른 값으로 존재
	mgr := &MemdirManager{
		agentType: "researcher",
		scopes:    []MemoryScope{ScopeLocal, ScopeProject, ScopeUser},
		baseDirs: map[MemoryScope]string{
			ScopeUser:    filepath.Join(home, ".goose", "agent-memory", "researcher"),
			ScopeProject: filepath.Join(proj, ".goose", "agent-memory", "researcher"),
			ScopeLocal:   filepath.Join(proj, ".goose", "agent-memory-local", "researcher"),
		},
	}

	// 각 scope 디렉토리 생성 및 memdir.jsonl 기록
	userEntry := MemoryEntry{ID: "u1", Timestamp: time.Now(), Category: "pref", Key: "user.preference", Value: "U", Scope: ScopeUser}
	projEntry := MemoryEntry{ID: "p1", Timestamp: time.Now(), Category: "pref", Key: "user.preference", Value: "P", Scope: ScopeProject}
	localEntry := MemoryEntry{ID: "l1", Timestamp: time.Now(), Category: "pref", Key: "user.preference", Value: "L", Scope: ScopeLocal}

	// 별도 키도 각 scope에 추가
	userOnly := MemoryEntry{ID: "u2", Timestamp: time.Now(), Category: "info", Key: "user.only", Value: "userval", Scope: ScopeUser}
	projOnly := MemoryEntry{ID: "p2", Timestamp: time.Now(), Category: "info", Key: "proj.only", Value: "projval", Scope: ScopeProject}
	localOnly := MemoryEntry{ID: "l2", Timestamp: time.Now(), Category: "info", Key: "local.only", Value: "localval", Scope: ScopeLocal}

	require.NoError(t, writeMemdirEntry(mgr.baseDirs[ScopeUser], userEntry))
	require.NoError(t, writeMemdirEntry(mgr.baseDirs[ScopeUser], userOnly))
	require.NoError(t, writeMemdirEntry(mgr.baseDirs[ScopeProject], projEntry))
	require.NoError(t, writeMemdirEntry(mgr.baseDirs[ScopeProject], projOnly))
	require.NoError(t, writeMemdirEntry(mgr.baseDirs[ScopeLocal], localEntry))
	require.NoError(t, writeMemdirEntry(mgr.baseDirs[ScopeLocal], localOnly))

	prompt, err := mgr.BuildMemoryPrompt()
	require.NoError(t, err)

	// local이 우선 (nearest wins)
	assert.Contains(t, prompt, `"L"`, "local scope value must win")
	assert.NotContains(t, prompt, `"P"`, "project scope value must not appear for duplicate key")
	assert.NotContains(t, prompt, `"U"`, "user scope value must not appear for duplicate key")

	// 각 scope 고유 키는 모두 포함
	assert.Contains(t, prompt, "user.only")
	assert.Contains(t, prompt, "proj.only")
	assert.Contains(t, prompt, "local.only")
}

// TestMemdir_Append_SingleWriter는 단일 writer에서 O_APPEND|O_SYNC로
// memdir.jsonl에 entry를 append함을 검증한다. (REQ-SA-012a)
func TestMemdir_Append_SingleWriter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".goose", "agent-memory", "researcher")

	mgr := &MemdirManager{
		agentType: "researcher",
		scopes:    []MemoryScope{ScopeProject},
		baseDirs: map[MemoryScope]string{
			ScopeProject: dir,
		},
	}

	for i := 0; i < 5; i++ {
		entry := MemoryEntry{
			ID:        fmt.Sprintf("e%d", i),
			Timestamp: time.Now(),
			Category:  "test",
			Key:       fmt.Sprintf("key%d", i),
			Value:     i,
			Scope:     ScopeProject,
		}
		require.NoError(t, mgr.Append(entry))
	}

	// 5개 entry가 모두 완전한 JSON line으로 파싱 가능해야 함
	entries, err := readMemdirEntries(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 5)
}

// TestMemdir_DirectoryPermission은 생성된 memdir 디렉토리 권한이 0700임을 검증한다.
// AC-SA-012: REQ-SA-017
func TestMemdir_DirectoryPermission(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".goose", "agent-memory", "researcher")

	mgr := &MemdirManager{
		agentType: "researcher",
		scopes:    []MemoryScope{ScopeProject},
		baseDirs: map[MemoryScope]string{
			ScopeProject: dir,
		},
	}

	entry := MemoryEntry{
		ID:        "e1",
		Timestamp: time.Now(),
		Category:  "test",
		Key:       "k1",
		Value:     "v1",
		Scope:     ScopeProject,
	}
	require.NoError(t, mgr.Append(entry))

	info, err := os.Stat(dir)
	require.NoError(t, err)
	perm := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0o700), perm, "memdir must have 0700 permissions")

	// 파일 권한도 0600이어야 함
	memdirFile := filepath.Join(dir, "memdir.jsonl")
	fileInfo, err := os.Stat(memdirFile)
	require.NoError(t, err)
	filePerm := fileInfo.Mode().Perm()
	assert.Equal(t, os.FileMode(0o600), filePerm, "memdir.jsonl must have 0600 permissions")
}

// TestMemdir_PeerConcurrentWrite는 동일 scope를 공유하는 peer sub-agent가
// 동시에 Append할 때 torn line이 없음을 검증한다. (AC-SA-014, REQ-SA-012b)
func TestMemdir_PeerConcurrentWrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".goose", "agent-memory", "researcher")

	// peer A와 peer B가 동일 dir 공유
	mgrA := &MemdirManager{
		agentType: "researcher",
		scopes:    []MemoryScope{ScopeProject},
		baseDirs:  map[MemoryScope]string{ScopeProject: dir},
	}
	mgrB := &MemdirManager{
		agentType: "researcher",
		scopes:    []MemoryScope{ScopeProject},
		baseDirs:  map[MemoryScope]string{ScopeProject: dir},
	}

	const n = 100
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			entry := MemoryEntry{
				ID:        fmt.Sprintf("a%d", i),
				Timestamp: time.Now(),
				Category:  "test",
				Key:       fmt.Sprintf("a_key%d", i),
				Value:     i,
				Scope:     ScopeProject,
			}
			_ = mgrA.Append(entry)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			entry := MemoryEntry{
				ID:        fmt.Sprintf("b%d", i),
				Timestamp: time.Now(),
				Category:  "test",
				Key:       fmt.Sprintf("b_key%d", i),
				Value:     i,
				Scope:     ScopeProject,
			}
			_ = mgrB.Append(entry)
		}
	}()

	wg.Wait()

	// 200개 entry 모두 완전한 JSON line으로 파싱 성공해야 함 (torn line 0)
	entries, err := readMemdirEntries(dir)
	require.NoError(t, err)
	assert.Equal(t, 2*n, len(entries), "all entries must be parseable")
}

// TestMemdir_ScopeNotEnabled는 허용되지 않은 scope에 Append 시
// ErrScopeNotEnabled를 반환함을 검증한다. (AC-SA-019)
func TestMemdir_ScopeNotEnabled(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".goose", "agent-memory", "researcher")

	// project scope만 enabled
	mgr := &MemdirManager{
		agentType: "researcher",
		scopes:    []MemoryScope{ScopeProject},
		baseDirs: map[MemoryScope]string{
			ScopeProject: dir,
		},
	}

	// user scope (미허용)로 Append 시도
	entry := MemoryEntry{
		ID:       "e1",
		Category: "fact",
		Key:      "k1",
		Value:    "v1",
		Scope:    ScopeUser, // not in scopes
	}
	err := mgr.Append(entry)
	assert.ErrorIs(t, err, ErrScopeNotEnabled)
}

// readMemdirEntries는 memdir.jsonl 파일에서 모든 entry를 읽는 헬퍼이다.
func readMemdirEntries(dir string) ([]MemoryEntry, error) {
	data, err := os.ReadFile(filepath.Join(dir, "memdir.jsonl"))
	if err != nil {
		return nil, err
	}
	var entries []MemoryEntry
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var e MemoryEntry
		if err2 := json.Unmarshal(line, &e); err2 != nil {
			return nil, fmt.Errorf("torn line: %w (content: %q)", err2, line)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// splitLines는 byte slice를 줄 단위로 분리한다.
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	for len(data) > 0 {
		nl := -1
		for i, b := range data {
			if b == '\n' {
				nl = i
				break
			}
		}
		if nl < 0 {
			lines = append(lines, data)
			break
		}
		lines = append(lines, data[:nl])
		data = data[nl+1:]
	}
	return lines
}

// writeMemdirEntry는 테스트 헬퍼로 단일 entry를 memdir.jsonl에 직접 쓴다.
func writeMemdirEntry(dir string, entry MemoryEntry) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.OpenFile(filepath.Join(dir, "memdir.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
