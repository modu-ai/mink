package subagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"
	"go.uber.org/zap"

	"github.com/modu-ai/mink/internal/userpath"
)

// MemdirManager는 3-scope 메모리 디렉토리를 관리한다.
// REQ-SA-003/012/017/021
//
// @MX:ANCHOR: [AUTO] 3-scope memory의 단일 관리자
// @MX:REASON: SPEC-GOOSE-SUBAGENT-001 REQ-SA-003/012/021 — 모든 memdir I/O가 이 타입을 통과
type MemdirManager struct {
	agentType string
	scopes    []MemoryScope
	// baseDirs는 각 scope의 실제 절대경로이다.
	baseDirs map[MemoryScope]string
}

// NewMemdirManager는 MemdirManager를 생성한다.
// projectRoot와 homeDir를 기반으로 3-scope 경로를 구성한다.
// REQ-MINK-UDM-002: homeDir은 userpath.UserHomeE() 경유 .mink, projectRoot는 .mink/ 서브디렉토리.
func NewMemdirManager(agentType string, scopes []MemoryScope, projectRoot, homeDir string) *MemdirManager {
	if len(scopes) == 0 {
		scopes = []MemoryScope{ScopeProject}
	}
	// ScopeUser: homeDir 은 이미 .mink 경로 (userpath.UserHomeE() 결과)
	// ScopeProject/ScopeLocal: projectRoot 기반 .mink/ 서브디렉토리
	minkProject := userpath.ProjectLocal(projectRoot)
	baseDirs := map[MemoryScope]string{
		ScopeUser:    filepath.Join(homeDir, "agent-memory", agentType),
		ScopeProject: filepath.Join(minkProject, "agent-memory", agentType),
		ScopeLocal:   filepath.Join(minkProject, "agent-memory-local", agentType),
	}
	return &MemdirManager{
		agentType: agentType,
		scopes:    scopes,
		baseDirs:  baseDirs,
	}
}

// BuildMemoryPrompt는 3-scope 우선순위(local→project→user)에 따라
// memdir.jsonl 항목을 system prompt 문자열로 반환한다.
// REQ-SA-003: 중복 키는 nearest scope 값이 우선한다.
func (m *MemdirManager) BuildMemoryPrompt() (string, error) {
	// 우선순위: local > project > user
	priorityOrder := []MemoryScope{ScopeLocal, ScopeProject, ScopeUser}

	seen := make(map[string]bool)
	var result []MemoryEntry

	for _, scope := range priorityOrder {
		// 이 scope가 enabled인지 확인
		if !m.isScopeEnabled(scope) {
			continue
		}
		dir, ok := m.baseDirs[scope]
		if !ok {
			continue
		}
		entries, err := m.readEntries(dir)
		if err != nil {
			// 디렉토리 없음은 무시
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("buildMemoryPrompt: read %s: %w", scope, err)
		}
		for _, e := range entries {
			if !seen[e.Key] {
				seen[e.Key] = true
				result = append(result, e)
			}
		}
	}

	if len(result) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("## Agent Memory\n\n")
	for _, e := range result {
		valueJSON, _ := json.Marshal(e.Value)
		fmt.Fprintf(&sb, "- [%s] %s: %s\n", e.Category, e.Key, valueJSON)
	}
	return sb.String(), nil
}

// Append는 entry를 해당 scope의 memdir.jsonl에 append한다.
// REQ-SA-012: advisory file lock + O_APPEND|O_SYNC.
// REQ-SA-017: 디렉토리 0700, 파일 0600.
// REQ-SA-021: scope가 enabled인지 검증.
//
// @MX:WARN: [AUTO] flock을 사용한 advisory lock — NFS에서는 ErrMemdirLockUnsupported
// @MX:REASON: REQ-SA-012(b) — peer sub-agent 동시 쓰기 semantics. flock이 지원되지 않으면 corruption 방지를 위해 에러 반환
func (m *MemdirManager) Append(entry MemoryEntry) error {
	scope := entry.Scope
	if scope == "" && len(m.scopes) > 0 {
		scope = m.scopes[0]
	}

	if !m.isScopeEnabled(scope) {
		return fmt.Errorf("%w: scope=%q", ErrScopeNotEnabled, scope)
	}

	dir, ok := m.baseDirs[scope]
	if !ok {
		return fmt.Errorf("subagent: no base dir for scope %q", scope)
	}

	// REQ-SA-017: 디렉토리 0700으로 생성
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("memdir: mkdir %s: %w", dir, err)
	}
	// 기존 디렉토리 권한 확인 및 로그 (변경하지 않음)
	if info, err := os.Stat(dir); err == nil {
		if info.Mode().Perm() != 0o700 {
			logWarn("memdir: directory has wider permissions than 0700 (sysadmin's responsibility)",
				zap.String("dir", dir),
				zap.String("perm", info.Mode().Perm().String()),
			)
		}
	}

	memdirPath := filepath.Join(dir, "memdir.jsonl")

	// REQ-SA-012(b): advisory file lock
	fl := flock.New(memdirPath + ".lock")
	locked, err := fl.TryLock()
	if err != nil {
		// flock 자체 에러 (NFS 등)
		return fmt.Errorf("%w: %v", ErrMemdirLockUnsupported, err)
	}
	if !locked {
		// 다른 프로세스가 lock 중 — 대기
		if err2 := fl.Lock(); err2 != nil {
			return fmt.Errorf("%w: %v", ErrMemdirLockUnsupported, err2)
		}
	}
	defer fl.Unlock() //nolint:errcheck

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("memdir: marshal: %w", err)
	}
	data = append(data, '\n')

	// REQ-SA-012(a): O_APPEND|O_SYNC + full-line atomic append
	f, err := os.OpenFile(memdirPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_SYNC, 0o600)
	if err != nil {
		return fmt.Errorf("memdir: open %s: %w", memdirPath, err)
	}
	defer f.Close()

	if _, err = f.Write(data); err != nil {
		return fmt.Errorf("memdir: write: %w", err)
	}
	return nil
}

// Query는 predicate를 만족하는 entry 목록을 반환한다.
func (m *MemdirManager) Query(predicate func(MemoryEntry) bool) ([]MemoryEntry, error) {
	var result []MemoryEntry
	for _, scope := range m.scopes {
		dir, ok := m.baseDirs[scope]
		if !ok {
			continue
		}
		entries, err := m.readEntries(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if predicate(e) {
				result = append(result, e)
			}
		}
	}
	return result, nil
}

// isScopeEnabled는 scope가 enabled 목록에 있는지 확인한다.
func (m *MemdirManager) isScopeEnabled(scope MemoryScope) bool {
	for _, s := range m.scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// readEntries는 디렉토리의 memdir.jsonl에서 모든 entry를 읽는다.
func (m *MemdirManager) readEntries(dir string) ([]MemoryEntry, error) {
	memdirPath := filepath.Join(dir, "memdir.jsonl")
	data, err := os.ReadFile(memdirPath)
	if err != nil {
		return nil, err
	}

	var entries []MemoryEntry
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e MemoryEntry
		if err2 := json.Unmarshal([]byte(line), &e); err2 != nil {
			return nil, fmt.Errorf("memdir: parse entry: %w", err2)
		}
		entries = append(entries, e)
	}
	return entries, nil
}
