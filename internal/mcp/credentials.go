package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

// credentialsDir은 credential 파일 저장 디렉토리이다.
// REQ-MCP-003: ~/.goose/mcp-credentials/{server-id}.json
const credentialsDir = ".goose/mcp-credentials"

// credentialFileMode은 credential 파일에 요구되는 최대 파일 mode이다.
// REQ-MCP-003: 0600 초과 시 거부
const credentialFileMode = os.FileMode(0600)

// credentialData는 파일에 저장되는 credential 데이터이다.
type credentialData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at_unix"`
	Scope        string `json:"scope"`
}

// credentialPath는 서버 ID에 대응하는 credential 파일 경로를 반환한다.
func credentialPath(serverID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, credentialsDir, serverID+".json"), nil
}

// SaveCredential은 token을 credential 파일에 저장한다.
// REQ-MCP-003: file mode 0600
func SaveCredential(serverID string, ts *TokenSet) error {
	path, err := credentialPath(serverID)
	if err != nil {
		return err
	}

	// 디렉토리 생성
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}

	data := credentialData{
		AccessToken:  ts.AccessToken,
		RefreshToken: ts.RefreshToken,
		Scope:        ts.Scope,
	}
	if !ts.ExpiresAt.IsZero() {
		data.ExpiresAt = ts.ExpiresAt.Unix()
	}

	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal credential: %w", err)
	}

	// REQ-MCP-003: 0600 mode로 파일 생성
	return os.WriteFile(path, b, credentialFileMode)
}

// LoadCredential은 credential 파일에서 token을 로드한다.
// REQ-MCP-003: file mode 0600 초과 시 ErrCredentialFilePermissions
func LoadCredential(serverID string, logger *zap.Logger) (*TokenSet, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	path, err := credentialPath(serverID)
	if err != nil {
		return nil, err
	}

	// 파일 존재 여부 확인
	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil // 파일 없음 = credential 없음
	}
	if err != nil {
		return nil, fmt.Errorf("stat credential file: %w", err)
	}

	// REQ-MCP-003: file mode 검증
	if fi.Mode()&0777 > credentialFileMode {
		logger.Warn("credential file mode exceeds 0600",
			zap.String("path", path),
			zap.String("mode", fmt.Sprintf("%04o", fi.Mode()&0777)),
		)
		return nil, fmt.Errorf("%w: path=%s mode=%04o", ErrCredentialFilePermissions, path, fi.Mode()&0777)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read credential file: %w", err)
	}

	var data credentialData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("parse credential file: %w", err)
	}

	ts := &TokenSet{
		AccessToken:  data.AccessToken,
		RefreshToken: data.RefreshToken,
		Scope:        data.Scope,
	}
	if data.ExpiresAt > 0 {
		ts.ExpiresAt = time.Unix(data.ExpiresAt, 0)
	}

	return ts, nil
}

// DeleteCredential은 credential 파일을 삭제한다.
func DeleteCredential(serverID string) error {
	path, err := credentialPath(serverID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete credential: %w", err)
	}
	return nil
}
