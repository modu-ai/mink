package anthropic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ClaudeCredentials는 ~/.claude/.credentials.json 파일의 구조이다.
type ClaudeCredentials struct {
	// AccessToken은 현재 OAuth access token이다.
	AccessToken string `json:"access_token"`
	// RefreshToken은 OAuth refresh token이다.
	RefreshToken string `json:"refresh_token"`
	// ExpiresAt은 access token 만료 시각이다.
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	// ClientID는 OAuth client ID이다.
	ClientID string `json:"client_id,omitempty"`
}

// DefaultClaudeCredentialPath는 기본 credentials 파일 경로이다.
func DefaultClaudeCredentialPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", ".credentials.json")
}

// ReadClaudeCredentials는 credentials 파일을 읽어 ClaudeCredentials를 반환한다.
func ReadClaudeCredentials(path string) (*ClaudeCredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("token_sync: 파일 읽기 실패 %q: %w", path, err)
	}

	var creds ClaudeCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("token_sync: JSON 파싱 실패: %w", err)
	}
	return &creds, nil
}

// AtomicWriteClaudeCredentials는 credentials를 임시 파일에 쓴 후 atomic rename한다.
// 파일 권한은 0600이다.
func AtomicWriteClaudeCredentials(path string, creds *ClaudeCredentials) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("token_sync: 직렬화 실패: %w", err)
	}

	return writeFileAtomic(path, data)
}

// MarshalJSON은 테스트에서 JSON을 편리하게 생성하기 위한 도우미 함수이다.
func MarshalJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// readFile은 파일을 읽어 바이트 슬라이스로 반환한다.
// oauth.go에서 공유로 사용된다.
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// writeFileAtomic은 임시 파일에 쓰고 대상 파일로 rename한다.
// 권한은 0600이다.
func writeFileAtomic(path string, data []byte) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("token_sync: 임시 파일 쓰기 실패: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("token_sync: atomic rename 실패: %w", err)
	}
	return nil
}
