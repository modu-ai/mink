// Package provider는 LLM provider 인터페이스와 구현을 담는다.
// SPEC-GOOSE-ADAPTER-001 M0 T-006
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SecretStore는 credential secret(access_token 등)을 읽고 쓰는 인터페이스이다.
// CREDPOOL의 Zero-Knowledge 설계와 연동된다: pool은 KeyringID만 보유하고,
// SecretStore가 실제 토큰을 조회한다.
type SecretStore interface {
	// Resolve는 keyringID에 해당하는 access_token을 반환한다.
	Resolve(ctx context.Context, keyringID string) (string, error)
	// WriteBack은 갱신된 access_token을 keyringID에 해당하는 저장소에 기록한다.
	WriteBack(ctx context.Context, keyringID, secret string) error
}

// FileSecretStore는 파일 기반 SecretStore MVP 구현이다.
// ~/.goose/credentials/{keyringID}.json 형식의 파일에서 access_token을 읽고 쓴다.
type FileSecretStore struct {
	// BaseDir은 credential 파일이 저장되는 디렉터리이다.
	BaseDir string
}

// NewFileSecretStore는 주어진 baseDir을 사용하는 FileSecretStore를 생성한다.
func NewFileSecretStore(baseDir string) *FileSecretStore {
	return &FileSecretStore{BaseDir: baseDir}
}

// credentialFile은 keyringID를 파일 경로로 변환한다.
// 경로 순회(path traversal) 공격을 방지한다.
func (s *FileSecretStore) credentialFile(keyringID string) (string, error) {
	// ".." 포함 여부 검사
	if strings.Contains(keyringID, "..") || strings.Contains(keyringID, "/") || strings.Contains(keyringID, string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid keyringID: %q", keyringID)
	}
	return filepath.Join(s.BaseDir, keyringID+".json"), nil
}

// CredentialFile은 keyringID를 파일 경로로 변환하는 exported wrapper이다.
// 같은 패키지 외부에서 경로 순회 방어 로직을 재사용할 때 사용한다.
func (s *FileSecretStore) CredentialFile(keyringID string) (string, error) {
	return s.credentialFile(keyringID)
}

// credentialPayload는 credential 파일의 JSON 구조이다.
type credentialPayload struct {
	AccessToken string `json:"access_token"`
}

// Resolve는 keyringID.json 파일에서 access_token을 읽어 반환한다.
func (s *FileSecretStore) Resolve(_ context.Context, keyringID string) (string, error) {
	path, err := s.credentialFile(keyringID)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("secret: %w", err)
	}

	var payload credentialPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("secret: JSON 파싱 실패: %w", err)
	}

	return payload.AccessToken, nil
}

// WriteBack은 keyringID.json 파일에 access_token을 원자적으로 기록한다.
// temp 파일에 쓴 뒤 rename하여 atomic write를 보장한다.
// 파일 권한은 0600이다.
func (s *FileSecretStore) WriteBack(_ context.Context, keyringID, secret string) error {
	path, err := s.credentialFile(keyringID)
	if err != nil {
		return err
	}

	// 기존 파일이 있으면 기존 데이터를 읽어 merge한다.
	existing := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing)
	}
	existing["access_token"] = secret

	raw, err := json.Marshal(existing)
	if err != nil {
		return fmt.Errorf("secret: JSON 직렬화 실패: %w", err)
	}

	// atomic write: temp 파일에 쓰고 rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0600); err != nil {
		return fmt.Errorf("secret: 임시 파일 쓰기 실패: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("secret: atomic rename 실패: %w", err)
	}
	return nil
}

// MemorySecretStore는 테스트용 in-memory SecretStore 구현이다.
type MemorySecretStore struct {
	secrets map[string]string
}

// NewMemorySecretStore는 주어진 secrets 맵으로 MemorySecretStore를 생성한다.
func NewMemorySecretStore(secrets map[string]string) *MemorySecretStore {
	m := make(map[string]string, len(secrets))
	for k, v := range secrets {
		m[k] = v
	}
	return &MemorySecretStore{secrets: m}
}

// Resolve는 keyringID에 해당하는 시크릿을 반환한다.
func (s *MemorySecretStore) Resolve(_ context.Context, keyringID string) (string, error) {
	v, ok := s.secrets[keyringID]
	if !ok {
		return "", fmt.Errorf("secret: keyringID %q not found", keyringID)
	}
	return v, nil
}

// WriteBack은 keyringID에 해당하는 시크릿을 메모리에 기록한다.
func (s *MemorySecretStore) WriteBack(_ context.Context, keyringID, secret string) error {
	s.secrets[keyringID] = secret
	return nil
}
