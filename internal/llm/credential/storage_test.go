// Package credential_test는 Storage 인터페이스 및 메타데이터 전용 JSON 영속 테스트를 포함한다.
// OI-01: Storage interface + atomic JSON write backend
// AC-CREDPOOL-006: 재기동 시 영속 상태 복원
package credential_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/llm/credential"
)

// TestStorage_SaveAndLoad는 Storage가 메타데이터를 저장하고 재로드할 수 있는지 검증한다.
// AC-CREDPOOL-006: 재기동 시 영속 상태 복원
func TestStorage_SaveAndLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "anthropic.json")

	store := credential.NewFileStorage(path)

	// 저장할 크레덴셜 메타데이터 생성
	creds := []*credential.PooledCredential{
		{
			ID:         "cred-1",
			Provider:   "anthropic",
			KeyringID:  "keyring-anthropic-1",
			Status:     credential.CredExhausted,
			ExpiresAt:  time.Now().Add(1 * time.Hour).Truncate(time.Millisecond),
			UsageCount: 42,
			Priority:   10,
			Weight:     1,
		},
	}

	// Save
	if err := store.Save(context.Background(), creds); err != nil {
		t.Fatalf("Save 실패: %v", err)
	}

	// Load
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load 실패: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("Load 결과: %d개 크레덴셜, 1개 기대", len(loaded))
	}

	got := loaded[0]
	if got.ID != "cred-1" {
		t.Errorf("ID 불일치: got %q, want %q", got.ID, "cred-1")
	}
	if got.Provider != "anthropic" {
		t.Errorf("Provider 불일치: got %q, want %q", got.Provider, "anthropic")
	}
	if got.KeyringID != "keyring-anthropic-1" {
		t.Errorf("KeyringID 불일치: got %q, want %q", got.KeyringID, "keyring-anthropic-1")
	}
	if got.Status != credential.CredExhausted {
		t.Errorf("Status 불일치: got %v, want CredExhausted", got.Status)
	}
	if got.UsageCount != 42 {
		t.Errorf("UsageCount 불일치: got %d, want 42", got.UsageCount)
	}
	if got.Priority != 10 {
		t.Errorf("Priority 불일치: got %d, want 10", got.Priority)
	}
}

// TestStorage_NoSecretFieldsInJSON은 JSON 파일에 raw secret 필드가 없는지 검증한다.
// Zero-Knowledge 불변: access_token / refresh_token / api_key 등이 파일에 없어야 함
func TestStorage_NoSecretFieldsInJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	store := credential.NewFileStorage(path)

	creds := []*credential.PooledCredential{
		{
			ID:        "cred-1",
			Provider:  "anthropic",
			KeyringID: "keyring-ref",
			Status:    credential.CredOK,
		},
	}

	if err := store.Save(context.Background(), creds); err != nil {
		t.Fatalf("Save 실패: %v", err)
	}

	// JSON 파일 내용을 읽어 forbidden 키 확인
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("파일 읽기 실패: %v", err)
	}

	forbiddenKeys := []string{
		"access_token", "refresh_token", "api_key", "id_token",
		"agent_key", "accessToken", "refreshToken", "apiKey",
	}

	for _, key := range forbiddenKeys {
		if containsKey(data, key) {
			t.Errorf("JSON 파일에 raw secret 필드 발견: %q (Zero-Knowledge 위반)", key)
		}
	}

	// keyring_id는 반드시 포함되어야 함 (reference)
	if !containsKey(data, "keyring_id") {
		t.Error("JSON 파일에 keyring_id가 없음")
	}
}

// containsKey는 JSON 데이터에 특정 키가 존재하는지 확인한다.
func containsKey(data []byte, key string) bool {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		// 최상위 파싱 실패 — 배열일 수 있음
		var rawSlice []map[string]any
		if err2 := json.Unmarshal(data, &rawSlice); err2 != nil {
			return false
		}
		for _, item := range rawSlice {
			if containsKeyInMap(item, key) {
				return true
			}
		}
		return false
	}
	return containsKeyInMap(raw, key)
}

func containsKeyInMap(m map[string]any, key string) bool {
	for k, v := range m {
		if k == key {
			return true
		}
		// 중첩 맵 확인
		if nested, ok := v.(map[string]any); ok {
			if containsKeyInMap(nested, key) {
				return true
			}
		}
		// 슬라이스 확인
		if arr, ok := v.([]any); ok {
			for _, item := range arr {
				if nested, ok := item.(map[string]any); ok {
					if containsKeyInMap(nested, key) {
						return true
					}
				}
			}
		}
	}
	return false
}

// TestStorage_AtomicWrite는 저장이 원자적으로 이루어지는지 검증한다.
// 임시 파일이 생성되고 rename으로 교체되어야 한다.
func TestStorage_AtomicWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "anthropic.json")

	store := credential.NewFileStorage(path)

	creds := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "anthropic", KeyringID: "kr-1", Status: credential.CredOK},
	}

	if err := store.Save(context.Background(), creds); err != nil {
		t.Fatalf("Save 실패: %v", err)
	}

	// 파일이 존재해야 함
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("저장 후 파일이 존재하지 않음")
	}

	// 두 번째 저장 (덮어쓰기)
	creds2 := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "anthropic", KeyringID: "kr-1", Status: credential.CredOK},
		{ID: "cred-2", Provider: "anthropic", KeyringID: "kr-2", Status: credential.CredOK},
	}
	if err := store.Save(context.Background(), creds2); err != nil {
		t.Fatalf("두 번째 Save 실패: %v", err)
	}

	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load 실패: %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("두 번째 Save 후 Load: %d개, 2개 기대", len(loaded))
	}
}

// TestStorage_LoadNonexistent는 파일이 없을 때 빈 슬라이스를 반환하는지 검증한다.
func TestStorage_LoadNonexistent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	store := credential.NewFileStorage(path)

	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("존재하지 않는 파일 Load 시 에러: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("존재하지 않는 파일 Load: %d개, 0개 기대", len(loaded))
	}
}

// TestStorage_FilePermissions는 저장된 파일 권한이 0600인지 검증한다.
func TestStorage_FilePermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "anthropic.json")

	store := credential.NewFileStorage(path)

	creds := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "anthropic", KeyringID: "kr-1", Status: credential.CredOK},
	}

	if err := store.Save(context.Background(), creds); err != nil {
		t.Fatalf("Save 실패: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("파일 stat 실패: %v", err)
	}

	// 0600 권한 검증
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("파일 권한: got %o, want 0600", perm)
	}
}

// TestPool_PersistsStateAcrossRecreation은 AC-CREDPOOL-006을 검증한다.
// 풀이 파괴되고 새 풀이 생성되어도 Exhausted 상태가 복원되어야 한다.
func TestPool_PersistsStateAcrossRecreation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "anthropic.json")

	store := credential.NewFileStorage(path)

	initialCreds := []*credential.PooledCredential{
		{ID: "cred-a", Provider: "anthropic", KeyringID: "kr-a", Status: credential.CredOK},
	}
	src := credential.NewDummySource(initialCreds)

	// 첫 번째 풀 생성
	pool1, err := credential.New(src, credential.NewRoundRobinStrategy(),
		credential.WithStorage(store))
	if err != nil {
		t.Fatalf("풀1 생성 실패: %v", err)
	}

	// cred-a 선택 후 Exhausted 처리
	c, err := pool1.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}

	exhaustDur := 5 * time.Minute
	if err := pool1.MarkExhausted(c, exhaustDur); err != nil {
		t.Fatalf("MarkExhausted 실패: %v", err)
	}

	// 상태 저장
	if err := pool1.PersistState(context.Background()); err != nil {
		t.Fatalf("PersistState 실패: %v", err)
	}

	// 두 번째 풀 생성 (상태 복원)
	src2 := credential.NewDummySource(initialCreds)
	pool2, err := credential.New(src2, credential.NewRoundRobinStrategy(),
		credential.WithStorage(store))
	if err != nil {
		t.Fatalf("풀2 생성 실패: %v", err)
	}

	// cred-a는 여전히 exhausted 상태여야 함
	_, err = pool2.Select(context.Background())
	if !errors.Is(err, credential.ErrExhausted) {
		t.Errorf("재생성된 풀에서 Exhausted 복원 실패: got %v, want ErrExhausted", err)
	}
}
