// Package credential storage.go는 메타데이터 전용 JSON 영속 백엔드를 구현한다.
//
// Zero-Knowledge 원칙: 저장 파일에 raw secret (access_token, refresh_token, api_key 등)이
// 포함되지 않는다. keyring_id만 참조로 저장된다.
// OI-01: Storage interface + atomic JSON write backend
// REQ-CREDPOOL-004: metadata-only persist, atomic write
package credential

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Storage는 크레덴셜 메타데이터를 영속하는 인터페이스이다.
//
// 구현체는 atomic write (temp + rename) 패턴을 사용해야 하며
// raw secret 필드를 절대 저장하지 않아야 한다.
// @MX:ANCHOR: [AUTO] Storage 인터페이스 — 메타데이터 영속 계약
// @MX:REASON: pool 생성자 및 PersistState에서 호출됨 (fan_in >= 3 예상)
type Storage interface {
	// Save는 크레덴셜 메타데이터를 원자적으로 저장한다.
	// raw secret은 저장하지 않는다.
	Save(ctx context.Context, creds []*PooledCredential) error
	// Load는 저장된 크레덴셜 메타데이터를 반환한다.
	// 파일이 없으면 빈 슬라이스를 반환한다.
	Load(ctx context.Context) ([]*PooledCredential, error)
}

// credentialRecord는 JSON 파일에 저장되는 메타데이터 레코드이다.
// raw secret 필드는 포함하지 않는다 (Zero-Knowledge).
type credentialRecord struct {
	ID               string `json:"id"`
	Provider         string `json:"provider"`
	KeyringID        string `json:"keyring_id"`
	Status           int    `json:"status"`
	ExhaustedUntilMs int64  `json:"exhausted_until_ms"`
	LastErrorAtMs    int64  `json:"last_error_at_ms"`
	LastErrorResetMs int64  `json:"last_error_reset_at_ms"`
	ExpiresAtMs      int64  `json:"expires_at_ms"`
	UsageCount       uint64 `json:"usage_count"`
	Priority         int    `json:"priority"`
	Weight           int    `json:"weight"`
}

// storageFile은 JSON 파일의 최상위 구조이다.
type storageFile struct {
	Version int                 `json:"version"`
	Entries []*credentialRecord `json:"entries"`
}

// FileStorage는 파일 기반 Storage 구현체이다.
// atomic write (temp file + rename) 패턴으로 파일 손상을 방지한다.
// 파일 권한은 0600으로 설정된다.
type FileStorage struct {
	path string
}

// NewFileStorage는 지정된 경로에 메타데이터를 저장하는 FileStorage를 생성한다.
func NewFileStorage(path string) *FileStorage {
	return &FileStorage{path: path}
}

// Save는 크레덴셜 메타데이터를 atomic write로 저장한다.
// raw secret 필드는 저장하지 않는다.
func (s *FileStorage) Save(_ context.Context, creds []*PooledCredential) error {
	records := make([]*credentialRecord, 0, len(creds))
	for _, c := range creds {
		rec := &credentialRecord{
			ID:         c.ID,
			Provider:   c.Provider,
			KeyringID:  c.KeyringID,
			Status:     int(c.Status),
			UsageCount: c.UsageCount,
			Priority:   c.Priority,
			Weight:     c.Weight,
		}
		if !c.exhaustedUntil.IsZero() {
			rec.ExhaustedUntilMs = c.exhaustedUntil.UnixMilli()
		}
		if !c.LastErrorAt.IsZero() {
			rec.LastErrorAtMs = c.LastErrorAt.UnixMilli()
		}
		if !c.LastErrorReset.IsZero() {
			rec.LastErrorResetMs = c.LastErrorReset.UnixMilli()
		}
		if !c.ExpiresAt.IsZero() {
			rec.ExpiresAtMs = c.ExpiresAt.UnixMilli()
		}
		records = append(records, rec)
	}

	payload := storageFile{
		Version: 2, // v0.3.0: metadata-only, raw secrets 제거
		Entries: records,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	// atomic write: 임시 파일에 쓰고 rename
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "cred-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// 0600 권한 설정
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// rename은 원자적 교체
	return os.Rename(tmpPath, s.path)
}

// Load는 저장된 크레덴셜 메타데이터를 반환한다.
// 파일이 없으면 빈 슬라이스를 반환한다.
func (s *FileStorage) Load(_ context.Context) ([]*PooledCredential, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var payload storageFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	creds := make([]*PooledCredential, 0, len(payload.Entries))
	for _, rec := range payload.Entries {
		c := &PooledCredential{
			ID:         rec.ID,
			Provider:   rec.Provider,
			KeyringID:  rec.KeyringID,
			Status:     CredStatus(rec.Status),
			UsageCount: rec.UsageCount,
			Priority:   rec.Priority,
			Weight:     rec.Weight,
		}
		if rec.ExhaustedUntilMs > 0 {
			c.exhaustedUntil = time.UnixMilli(rec.ExhaustedUntilMs)
		}
		if rec.LastErrorAtMs > 0 {
			c.LastErrorAt = time.UnixMilli(rec.LastErrorAtMs)
		}
		if rec.LastErrorResetMs > 0 {
			c.LastErrorReset = time.UnixMilli(rec.LastErrorResetMs)
		}
		if rec.ExpiresAtMs > 0 {
			c.ExpiresAt = time.UnixMilli(rec.ExpiresAtMs)
		}
		creds = append(creds, c)
	}
	return creds, nil
}

// WithStorage는 CredentialPool에 Storage를 설정하는 Option이다.
func WithStorage(s Storage) Option {
	return func(p *CredentialPool) {
		p.storage = s
	}
}
