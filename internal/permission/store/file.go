package store

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/modu-ai/goose/internal/permission"
)

// grantsFile은 grants.json의 직렬화 스키마다.
// REQ-PE-017: schema_version 필드를 필수로 포함한다.
type grantsFile struct {
	SchemaVersion int         `json:"schema_version"`
	Grants        []grantJSON `json:"grants"`
}

// grantJSON은 Grant의 JSON 직렬화 표현이다.
// Capability는 문자열로 직렬화한다.
type grantJSON struct {
	ID          string     `json:"id"`
	SubjectID   string     `json:"subject_id"`
	SubjectType string     `json:"subject_type"`
	Capability  string     `json:"capability"`
	Scope       string     `json:"scope"`
	GrantedAt   time.Time  `json:"granted_at"`
	GrantedBy   string     `json:"granted_by"`
	ExpiresAt   *time.Time `json:"expires_at"`
	Revoked     bool       `json:"revoked"`
	RevokedAt   *time.Time `json:"revoked_at"`
}

func toGrantJSON(g permission.Grant) grantJSON {
	return grantJSON{
		ID:          g.ID,
		SubjectID:   g.SubjectID,
		SubjectType: string(g.SubjectType),
		Capability:  g.Capability.String(),
		Scope:       g.Scope,
		GrantedAt:   g.GrantedAt,
		GrantedBy:   g.GrantedBy,
		ExpiresAt:   g.ExpiresAt,
		Revoked:     g.Revoked,
		RevokedAt:   g.RevokedAt,
	}
}

func fromGrantJSON(j grantJSON) (permission.Grant, error) {
	cap, ok := permission.CapabilityFromString(j.Capability)
	if !ok {
		return permission.Grant{}, fmt.Errorf("unknown capability %q in grant %s", j.Capability, j.ID)
	}
	return permission.Grant{
		ID:          j.ID,
		SubjectID:   j.SubjectID,
		SubjectType: permission.SubjectType(j.SubjectType),
		Capability:  cap,
		Scope:       j.Scope,
		GrantedAt:   j.GrantedAt,
		GrantedBy:   j.GrantedBy,
		ExpiresAt:   j.ExpiresAt,
		Revoked:     j.Revoked,
		RevokedAt:   j.RevokedAt,
	}, nil
}

// FileStore는 파일 기반 Store 구현이다.
// atomic write (temp + fsync + rename) + file mode 0600 보장.
// REQ-PE-004, REQ-PE-014, REQ-PE-017
type FileStore struct {
	path   string
	logger *zap.Logger

	mu    sync.RWMutex
	index []permission.Grant
	ready bool
	// failed는 Store.Open 실패 사유이다. (비어있으면 not-called)
	failed string
}

// NewFileStore는 지정 경로의 FileStore를 생성한다.
// path가 빈 문자열이면 기본 경로(~/.goose/permissions/grants.json)를 사용한다.
func NewFileStore(path string, logger *zap.Logger) (*FileStore, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home dir: %w", err)
		}
		path = filepath.Join(home, ".goose", "permissions", "grants.json")
	}
	return &FileStore{
		path:   path,
		logger: logger,
	}, nil
}

// Open은 store를 초기화한다.
// 파일이 없으면 새로 생성한다. 권한·schema 버전 불일치 시 에러를 반환한다.
// REQ-PE-004, REQ-PE-017
func (f *FileStore) Open() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 부모 디렉토리 생성 (mode 0700)
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		f.failed = fmt.Sprintf("mkdir %s: %v", dir, err)
		return fmt.Errorf("create permission dir: %w", err)
	}

	// 파일이 없으면 빈 store로 초기화
	if _, err := os.Stat(f.path); os.IsNotExist(err) {
		f.index = []permission.Grant{}
		f.ready = true
		return f.writeUnlocked()
	}

	// 파일 권한 검사 (REQ-PE-004)
	info, err := os.Stat(f.path)
	if err != nil {
		f.failed = fmt.Sprintf("stat %s: %v", f.path, err)
		return fmt.Errorf("stat grant file: %w", err)
	}
	mode := info.Mode().Perm()
	if mode > 0o600 {
		f.logger.Warn("permission grant file mode exceeds 0600",
			zap.String("path", f.path),
			zap.String("mode", fmt.Sprintf("%04o", mode)),
		)
		f.failed = fmt.Sprintf("insecure permissions %04o on %s", mode, f.path)
		return permission.ErrStoreFilePermissions{Path: f.path, Mode: uint32(mode)}
	}

	// 파일 읽기 및 schema 버전 검사 (REQ-PE-017)
	data, err := os.ReadFile(f.path)
	if err != nil {
		f.failed = fmt.Sprintf("read %s: %v", f.path, err)
		return fmt.Errorf("read grant file: %w", err)
	}

	var gf grantsFile
	if err := json.Unmarshal(data, &gf); err != nil {
		f.failed = fmt.Sprintf("parse %s: %v", f.path, err)
		return fmt.Errorf("parse grant file: %w", err)
	}

	if gf.SchemaVersion != CurrentSchemaVersion {
		f.logger.Warn("grant store schema version mismatch",
			zap.String("path", f.path),
			zap.Int("file_version", gf.SchemaVersion),
			zap.Int("expected_version", CurrentSchemaVersion),
		)
		f.failed = fmt.Sprintf("schema version mismatch: got %d expected %d", gf.SchemaVersion, CurrentSchemaVersion)
		return permission.ErrIncompatibleStoreVersion{
			Path:     f.path,
			Got:      gf.SchemaVersion,
			Expected: CurrentSchemaVersion,
		}
	}

	// grant 파싱
	grants := make([]permission.Grant, 0, len(gf.Grants))
	for _, gj := range gf.Grants {
		g, err := fromGrantJSON(gj)
		if err != nil {
			f.logger.Warn("skip invalid grant entry", zap.Error(err))
			continue
		}
		grants = append(grants, g)
	}
	f.index = grants
	f.ready = true
	return nil
}

func (f *FileStore) checkReady() error {
	if !f.ready {
		if f.failed != "" {
			return permission.ErrStoreNotReady{Reason: f.failed}
		}
		return permission.ErrStoreNotReady{Reason: "Open() not called"}
	}
	return nil
}

func (f *FileStore) Lookup(subjectID string, cap permission.Capability, scope string) (permission.Grant, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	now := time.Now()
	for _, g := range f.index {
		if g.SubjectID == subjectID && g.Capability == cap && g.Scope == scope {
			if g.Revoked || g.IsExpired(now) {
				continue
			}
			return g, true
		}
	}
	return permission.Grant{}, false
}

func (f *FileStore) Save(g permission.Grant) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkReady(); err != nil {
		return err
	}
	f.index = append(f.index, g)
	return f.writeUnlocked()
}

func (f *FileStore) Revoke(subjectID string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkReady(); err != nil {
		return 0, err
	}

	now := time.Now()
	count := 0
	for i := range f.index {
		if f.index[i].SubjectID == subjectID && !f.index[i].Revoked {
			f.index[i].Revoked = true
			f.index[i].RevokedAt = &now
			count++
		}
	}
	if count > 0 {
		if err := f.writeUnlocked(); err != nil {
			return 0, err
		}
	}
	return count, nil
}

func (f *FileStore) List(filter permission.Filter) ([]permission.Grant, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkReady(); err != nil {
		return nil, err
	}

	now := time.Now()
	var result []permission.Grant
	for _, g := range f.index {
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

func (f *FileStore) GC(now time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkReady(); err != nil {
		return 0, err
	}

	var remaining []permission.Grant
	pruned := 0
	for _, g := range f.index {
		if g.IsExpired(now) || g.Revoked {
			pruned++
		} else {
			remaining = append(remaining, g)
		}
	}
	if pruned > 0 {
		f.index = remaining
		if err := f.writeUnlocked(); err != nil {
			return 0, err
		}
	}
	return pruned, nil
}

func (f *FileStore) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ready = false
	return nil
}

// writeUnlocked는 인덱스를 atomic write로 파일에 저장한다.
// 반드시 mu.Lock() 상태에서 호출해야 한다.
// atomic write: temp file → fsync → rename (REQ-PE-014)
func (f *FileStore) writeUnlocked() error {
	gj := make([]grantJSON, len(f.index))
	for i, g := range f.index {
		gj[i] = toGrantJSON(g)
	}
	gf := grantsFile{
		SchemaVersion: CurrentSchemaVersion,
		Grants:        gj,
	}

	data, err := json.MarshalIndent(gf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal grants: %w", err)
	}

	// atomic write: temp → fsync → rename
	dir := filepath.Dir(f.path)
	tmp, err := os.CreateTemp(dir, "grants.json.tmp.*")
	if err != nil {
		return fmt.Errorf("create temp grant file: %w", err)
	}
	tmpName := tmp.Name()

	// temp 파일도 0600으로 설정
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp grant file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp grant file: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("fsync temp grant file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp grant file: %w", err)
	}

	if err := os.Rename(tmpName, f.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp grant file: %w", err)
	}

	// 최종 파일 권한 확인 (rename이 umask를 무시하는지 방어)
	if err := os.Chmod(f.path, fs.FileMode(0o600)); err != nil {
		return fmt.Errorf("chmod final grant file: %w", err)
	}

	return nil
}
