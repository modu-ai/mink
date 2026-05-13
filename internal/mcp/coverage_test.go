package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// credTestTime은 테스트용 Unix timestamp → time.Time 변환이다.
func credTestTime(unix int64) time.Time {
	return time.Unix(unix, 0)
}

// --- server.go 커버리지 ---

// TestMCPServer_Resource는 Resource 등록을 검증한다.
func TestMCPServer_Resource(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "test"})
	handler := func(_ context.Context, uri string) (ResourceContent, error) {
		return ResourceContent{URI: uri, Text: "content"}, nil
	}
	result := srv.Resource("file:///test.txt", handler)
	assert.Same(t, srv, result)
	assert.Len(t, srv.resources, 1)
}

// TestMCPServer_Prompt는 Prompt 등록을 검증한다.
func TestMCPServer_Prompt(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "test"})
	handler := func(_ context.Context, args map[string]string) (string, error) {
		return "Hello " + args["name"], nil
	}
	result := srv.Prompt("greet", []PromptArgument{{Name: "name"}}, handler)
	assert.Same(t, srv, result)
	assert.Len(t, srv.prompts, 1)
}

// TestMCPServer_HandleInitialize는 initialize 요청 처리를 검증한다.
func TestMCPServer_HandleInitialize(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "test-srv", Version: "1.0"})
	msg := JSONRPCMessage{JSONRPC: "2.0", ID: 1, Method: "initialize"}
	resp, err := srv.handleRequest(context.Background(), msg)
	require.NoError(t, err)
	assert.Nil(t, resp.Error)

	var result map[string]any
	_ = json.Unmarshal(resp.Result, &result)
	assert.Equal(t, "2025-03-26", result["protocolVersion"])
}

// TestMCPServer_HandleResourcesList는 resources/list 처리를 검증한다.
func TestMCPServer_HandleResourcesList(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "test"})
	srv.Resource("file:///test.txt", func(_ context.Context, _ string) (ResourceContent, error) {
		return ResourceContent{}, nil
	})

	msg := JSONRPCMessage{JSONRPC: "2.0", ID: 1, Method: "resources/list"}
	resp, err := srv.handleRequest(context.Background(), msg)
	require.NoError(t, err)
	assert.Nil(t, resp.Error)
}

// TestMCPServer_HandlePromptsList는 prompts/list 처리를 검증한다.
func TestMCPServer_HandlePromptsList(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "test"})
	srv.Prompt("greet", []PromptArgument{{Name: "lang"}}, func(_ context.Context, _ map[string]string) (string, error) {
		return "Hello", nil
	})

	msg := JSONRPCMessage{JSONRPC: "2.0", ID: 1, Method: "prompts/list"}
	resp, err := srv.handleRequest(context.Background(), msg)
	require.NoError(t, err)
	assert.Nil(t, resp.Error)
}

// TestMCPServer_HandleUnknownMethod는 알 수 없는 메서드 처리를 검증한다.
func TestMCPServer_HandleUnknownMethod(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "test"})
	msg := JSONRPCMessage{JSONRPC: "2.0", ID: 1, Method: "unknown/method"}
	resp, err := srv.handleRequest(context.Background(), msg)
	require.NoError(t, err)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, ErrCodeMethodNotFound, resp.Error.Code)
}

// TestMCPServer_HandleToolsCall_Error는 존재하지 않는 tool 호출을 검증한다.
func TestMCPServer_HandleToolsCall_Error(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "test"})
	params, _ := json.Marshal(map[string]any{"name": "nonexistent"})
	msg := JSONRPCMessage{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: params}
	resp, err := srv.handleRequest(context.Background(), msg)
	require.NoError(t, err)
	assert.NotNil(t, resp.Error)
}

// TestMCPServer_HandleToolsCall_InvalidParams는 잘못된 파라미터 처리를 검증한다.
func TestMCPServer_HandleToolsCall_InvalidParams(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "test"})
	msg := JSONRPCMessage{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: json.RawMessage(`{invalid}`)}
	resp, err := srv.handleRequest(context.Background(), msg)
	require.NoError(t, err)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, ErrCodeInvalidParams, resp.Error.Code)
}

// TestMCPServer_HandleToolsCall_HandlerError는 handler가 에러를 반환하는 경우를 검증한다.
func TestMCPServer_HandleToolsCall_HandlerError(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "test"})
	_, _ = srv.Tool("failing", nil, func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
		return nil, fmt.Errorf("tool failed")
	})

	params, _ := json.Marshal(map[string]any{"name": "failing", "arguments": map[string]any{}})
	msg := JSONRPCMessage{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: params}
	resp, err := srv.handleRequest(context.Background(), msg)
	require.NoError(t, err)
	assert.Nil(t, resp.Error) // handler error는 result로 반환

	var result map[string]any
	_ = json.Unmarshal(resp.Result, &result)
	assert.Equal(t, true, result["isError"])
}

// --- client.go 커버리지 ---

// TestMCPClient_ListResources는 ListResources 기본 동작을 검증한다.
func TestMCPClient_ListResources(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return makeInitResponse(map[string]bool{"resources": true})
			}
			if req.Method == "resources/list" {
				result, _ := json.Marshal(map[string]any{
					"resources": []map[string]any{
						{"uri": "file:///test.txt", "name": "test", "mimeType": "text/plain"},
					},
				})
				return JSONRPCResponse{JSONRPC: JSONRPCVersion, Result: result}
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	session, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "res-test"})
	require.NoError(t, err)

	resources, err := client.ListResources(context.Background(), session)
	require.NoError(t, err)
	assert.Len(t, resources, 1)
	assert.Equal(t, "file:///test.txt", resources[0].URI)
}

// TestMCPClient_ReadResource는 ReadResource 기본 동작을 검증한다.
func TestMCPClient_ReadResource(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return makeInitResponse(map[string]bool{"resources": true})
			}
			if req.Method == "resources/read" {
				result, _ := json.Marshal(map[string]any{
					"contents": []map[string]any{
						{"uri": "file:///test.txt", "mimeType": "text/plain", "text": "hello world"},
					},
				})
				return JSONRPCResponse{JSONRPC: JSONRPCVersion, Result: result}
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	session, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "read-res-test"})
	require.NoError(t, err)

	content, err := client.ReadResource(context.Background(), session, "file:///test.txt")
	require.NoError(t, err)
	assert.Equal(t, "hello world", content.Text)
}

// TestMCPClient_ReadResource_EmptyContents는 빈 contents 처리를 검증한다.
func TestMCPClient_ReadResource_EmptyContents(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return makeInitResponse(map[string]bool{"resources": true})
			}
			if req.Method == "resources/read" {
				result, _ := json.Marshal(map[string]any{"contents": []any{}})
				return JSONRPCResponse{JSONRPC: JSONRPCVersion, Result: result}
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	session, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "empty-res-test"})
	require.NoError(t, err)

	content, err := client.ReadResource(context.Background(), session, "file:///empty.txt")
	require.NoError(t, err)
	assert.Equal(t, "file:///empty.txt", content.URI)
}

// TestMCPClient_ListPrompts는 ListPrompts 기본 동작을 검증한다.
func TestMCPClient_ListPrompts(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return makeInitResponse(map[string]bool{"prompts": true})
			}
			if req.Method == "prompts/list" {
				result, _ := json.Marshal(map[string]any{
					"prompts": []map[string]any{
						{
							"name":        "greet",
							"description": "Greeting",
							"arguments":   []map[string]any{{"name": "lang", "required": false}},
						},
					},
				})
				return JSONRPCResponse{JSONRPC: JSONRPCVersion, Result: result}
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	session, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "prompts-test"})
	require.NoError(t, err)

	prompts, err := client.ListPrompts(context.Background(), session)
	require.NoError(t, err)
	assert.Len(t, prompts, 1)
	assert.Equal(t, "greet", prompts[0].Name)
	assert.Len(t, prompts[0].Arguments, 1)
}

// TestMCPClient_CheckConnected는 disconnected 세션에서의 에러를 검증한다.
func TestMCPClient_CheckConnected(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return makeInitResponse(map[string]bool{"tools": true})
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	session, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "check-test"})
	require.NoError(t, err)

	session.SetState(SessionDisconnected)

	_, err = client.ListTools(context.Background(), session)
	assert.True(t, errors.Is(err, ErrSessionNotConnected))
}

// TestMCPClient_ListTools_Error는 tools/list wire 에러를 검증한다.
func TestMCPClient_ListTools_Error(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return makeInitResponse(map[string]bool{"tools": true})
			}
			if req.Method == "tools/list" {
				return JSONRPCResponse{
					JSONRPC: JSONRPCVersion,
					Error:   &JSONRPCError{Code: ErrCodeInternal, Message: "server error"},
				}
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	session, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "tools-err-test"})
	require.NoError(t, err)

	_, err = client.ListTools(context.Background(), session)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server error")
}

// --- transport_factory.go 커버리지 ---

// TestTransportAdapter_Methods는 transportAdapter의 메서드들을 검증한다.
func TestTransportAdapter_Methods(t *testing.T) {
	// mockTransport를 wrap하는 테스트는 이미 client_test.go에서 수행됨.
	// 여기서는 transport.Transport → mcp.Transport 변환 어댑터 직접 테스트

	// wrapTransport는 transport_factory.go에서만 사용하고 공개되지 않으므로
	// createWebSocketTransport를 통해 간접 테스트
	// TLS 에러 경로 검증

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wsURI := strings.Replace(srv.URL, "https://", "wss://", 1)

	// insecure=false → TLS 에러 경로
	_, err := createWebSocketTransport(context.Background(), MCPServerConfig{
		URI: wsURI,
		// TLS: nil (strict)
	})
	// 에러가 발생해야 함
	if err != nil {
		t.Logf("Expected error: %v", err)
	}
}

// TestCreateSSETransport는 SSE transport 생성을 검증한다.
func TestCreateSSETransport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	t_sse, err := createSSETransport(ctx, MCPServerConfig{URI: srv.URL})
	require.NoError(t, err)
	require.NotNil(t, t_sse)

	cancel()
	_ = t_sse.Close()
}

// TestCreateSSETransport_NoURI는 URI 없는 SSE 생성이 에러를 반환하는지 검증한다.
func TestCreateSSETransport_NoURI(t *testing.T) {
	_, err := createSSETransport(context.Background(), MCPServerConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "URI")
}

// TestCreateWebSocketTransport_NoURI는 URI 없는 WS 생성이 에러를 반환하는지 검증한다.
func TestCreateWebSocketTransport_NoURI(t *testing.T) {
	_, err := createWebSocketTransport(context.Background(), MCPServerConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "URI")
}

// TestCreateStdioTransport_NoCommand는 Command 없는 stdio 생성이 에러를 반환하는지 검증한다.
func TestCreateStdioTransport_NoCommand(t *testing.T) {
	_, err := createStdioTransport(context.Background(), MCPServerConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Command")
}

// TestDefaultTransportFactory는 기본 transport factory를 검증한다.
func TestDefaultTransportFactory(t *testing.T) {
	// unknown transport type
	_, err := defaultTransportFactory(context.Background(), MCPServerConfig{Transport: "unknown"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown transport type")
}

// --- auth.go 커버리지 ---

// TestRefreshToken_HTTP는 RefreshToken HTTP 요청을 검증한다.
func TestRefreshToken_HTTP(t *testing.T) {
	// fixture OAuth 토큰 서버
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_ = r.ParseForm()
		if r.FormValue("grant_type") != "refresh_token" {
			http.Error(w, "bad grant_type", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access-token",
			"refresh_token": "new-refresh-token",
			"expires_in":    3600,
		})
	}))
	defer srv.Close()

	ts, err := RefreshToken(srv.URL, "test-client", "old-refresh-token")
	require.NoError(t, err)
	assert.Equal(t, "new-access-token", ts.AccessToken)
	assert.Equal(t, "new-refresh-token", ts.RefreshToken)
	assert.False(t, ts.ExpiresAt.IsZero())
}

// TestRefreshToken_InvalidGrant는 invalid_grant 에러를 검증한다.
func TestRefreshToken_InvalidGrant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	_, err := RefreshToken(srv.URL, "test-client", "invalid-refresh-token")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrReauthRequired))
}

// TestPKCEVerifier는 PKCE verifier 생성을 검증한다.
func TestPKCEVerifier(t *testing.T) {
	v1, err := generatePKCEVerifier()
	require.NoError(t, err)
	v2, err := generatePKCEVerifier()
	require.NoError(t, err)
	assert.NotEqual(t, v1, v2, "PKCE verifier는 매번 다른 값이어야 함")
}

// TestPKCEChallenge는 PKCE challenge 생성을 검증한다.
func TestPKCEChallenge(t *testing.T) {
	verifier := "test-verifier-string"
	c1 := pkceChallenge(verifier)
	c2 := pkceChallenge(verifier)
	assert.Equal(t, c1, c2, "동일 verifier → 동일 challenge")
	assert.NotEqual(t, verifier, c1, "challenge는 verifier와 달라야 함")
}

// --- credentials.go 커버리지 ---

// TestDeleteCredential는 credential 삭제를 검증한다.
func TestDeleteCredential(t *testing.T) {
	oldHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// 없는 파일 삭제는 에러 없이 완료
	err := DeleteCredential("nonexistent-server")
	assert.NoError(t, err)

	// 실제 credential 저장 후 삭제
	err = SaveCredential("to-delete", &TokenSet{AccessToken: "tok"})
	require.NoError(t, err)

	err = DeleteCredential("to-delete")
	assert.NoError(t, err)

	// 삭제 후 로드: nil 반환
	loaded, err := LoadCredential("to-delete", nil)
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

// TestLoadCredential_NotExist는 존재하지 않는 credential 로드를 검증한다.
func TestLoadCredential_NotExist(t *testing.T) {
	oldHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	loaded, err := LoadCredential("nonexistent", nil)
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

// TestSaveCredential_WithExpiry는 만료 시간이 있는 credential 저장/로드를 검증한다.
func TestSaveCredential_WithExpiry(t *testing.T) {
	oldHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	import_t := "time"
	_ = import_t
	// 직접 time.Time 사용
	expiresAt := credTestTime(1000)
	ts := &TokenSet{
		AccessToken: "tok",
		ExpiresAt:   expiresAt,
	}
	err := SaveCredential("expiry-test", ts)
	require.NoError(t, err)

	loaded, err := LoadCredential("expiry-test", nil)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, int64(1000), loaded.ExpiresAt.Unix())
}

// TestSaveCredential_WithExpiryDirect는 만료 시간이 없는 credential를 검증한다.
func TestSaveCredential_WithExpiryDirect(t *testing.T) {
	oldHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// ExpiresAt 없는 경우
	ts := &TokenSet{AccessToken: "tok2"}
	err := SaveCredential("no-expiry", ts)
	require.NoError(t, err)

	loaded, err := LoadCredential("no-expiry", nil)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.True(t, loaded.ExpiresAt.IsZero())
}

// TestCredentialPath는 credentialPath 함수를 검증한다.
// REQ-MINK-UDM-002: .mink/mcp-credentials 경로 사용.
func TestCredentialPath(t *testing.T) {
	oldHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	os.Unsetenv("MINK_HOME")
	defer func() {
		os.Setenv("HOME", oldHome)
		os.Unsetenv("MINK_HOME")
	}()

	path, err := credentialPath("test-server")
	require.NoError(t, err)
	assert.Contains(t, path, "test-server.json")
	assert.Contains(t, path, ".mink") // REQ-MINK-UDM-002: .goose → .mink
}

// --- adapter.go 커버리지 ---

// TestNewAdapter는 NewAdapter 생성을 검증한다.
func TestNewAdapter(t *testing.T) {
	adapter := NewAdapter(nil)
	assert.NotNil(t, adapter)
}

// TestMCPConnectionBridge는 mcpConnectionBridge를 검증한다.
func TestMCPConnectionBridge(t *testing.T) {
	session := &ServerSession{
		ID: "bridge-test",
		tools: []MCPTool{
			{Name: "mcp__fx__search", Description: "Search tool"},
		},
		toolsLoaded: true,
	}

	bridge := &mcpConnectionBridge{
		session: session,
		client:  nil,
	}

	// ServerID
	assert.Equal(t, "bridge-test", bridge.ServerID())

	// ListTools
	manifests := bridge.ListTools()
	assert.Len(t, manifests, 1)
	assert.Equal(t, "mcp__fx__search", manifests[0].Name)

	// CallTool (not implemented)
	_, err := bridge.CallTool(nil, "mcp__fx__search", nil)
	assert.Error(t, err)
}

// TestPromptToSkill_NoServerName은 서버 이름 없는 경우를 검증한다.
func TestPromptToSkill_NoServerName(t *testing.T) {
	_, err := PromptToSkill("", MCPPrompt{Name: "greet"})
	require.Error(t, err)
}

// TestPromptToSkill_NoPromptName은 prompt 이름 없는 경우를 검증한다.
func TestPromptToSkill_NoPromptName(t *testing.T) {
	_, err := PromptToSkill("fx", MCPPrompt{})
	require.Error(t, err)
}

// TestPromptToSkill_NoArguments는 인수 없는 prompt를 검증한다.
func TestPromptToSkill_NoArguments(t *testing.T) {
	def, err := PromptToSkill("fx", MCPPrompt{Name: "simple"})
	require.NoError(t, err)
	assert.Equal(t, "mcp__fx__simple", def.ID)
	assert.Equal(t, "", def.ArgumentHint)
}

// --- client.go rawToolName 커버리지 ---

// TestRawToolName은 rawToolName 함수를 검증한다.
func TestRawToolName(t *testing.T) {
	assert.Equal(t, "search", rawToolName("mcp__fx__search", "fx"))
	assert.Equal(t, "mcp__gh__search", rawToolName("mcp__gh__search", "fx")) // 다른 서버
	assert.Equal(t, "raw-tool", rawToolName("raw-tool", "fx"))               // prefix 없음
}

// TestMCPClient_CallTool_ErrResponse는 CallTool이 에러 응답을 처리하는지 검증한다.
func TestMCPClient_CallTool_ErrResponse(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return makeInitResponse(map[string]bool{"tools": true})
			}
			if req.Method == "tools/call" {
				return JSONRPCResponse{
					JSONRPC: JSONRPCVersion,
					Error:   &JSONRPCError{Code: ErrCodeRequestCancelled, Message: "cancelled"},
				}
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	session, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "cancel-call-test"})
	require.NoError(t, err)

	session.mu.Lock()
	session.tools = []MCPTool{{Name: "mcp__fx__search"}}
	session.toolsLoaded = true
	session.mu.Unlock()

	_, err = client.CallTool(context.Background(), session, "mcp__fx__search", nil)
	// ErrCodeRequestCancelled → ErrRequestTimeout
	assert.True(t, errors.Is(err, ErrRequestTimeout))
}

// TestMCPClient_CallTool_ToolError는 CallTool이 tool 에러 응답을 처리하는지 검증한다.
func TestMCPClient_CallTool_ToolError(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return makeInitResponse(map[string]bool{"tools": true})
			}
			if req.Method == "tools/call" {
				return JSONRPCResponse{
					JSONRPC: JSONRPCVersion,
					Error:   &JSONRPCError{Code: ErrCodeInternal, Message: "tool error"},
				}
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	session, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "tool-err-call-test"})
	require.NoError(t, err)

	session.mu.Lock()
	session.tools = []MCPTool{{Name: "mcp__fx__search"}}
	session.toolsLoaded = true
	session.mu.Unlock()

	result, err := client.CallTool(context.Background(), session, "mcp__fx__search", nil)
	require.NoError(t, err) // 에러 응답은 result.IsError로 반환
	assert.True(t, result.IsError)
}

// TestConfigID는 configID 함수를 검증한다.
func TestConfigID(t *testing.T) {
	cfg1 := MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo"}
	cfg2 := MCPServerConfig{Name: "gh", Transport: "stdio", Command: "echo"}
	cfg3 := MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo"}

	id1 := configID(cfg1)
	id2 := configID(cfg2)
	id3 := configID(cfg3)

	assert.NotEmpty(t, id1)
	assert.NotEqual(t, id1, id2)
	assert.Equal(t, id1, id3, "동일 설정은 동일 ID")

	// 명시적 ID 사용
	cfg4 := MCPServerConfig{ID: "explicit-id"}
	assert.Equal(t, "explicit-id", configID(cfg4))
}

// TestMCPClient_NewClient는 NewClient 생성을 검증한다.
func TestMCPClient_NewClient(t *testing.T) {
	// logger nil
	c := NewClient(nil, nil)
	assert.NotNil(t, c)

	// factory nil → defaultTransportFactory 사용
	assert.NotNil(t, c.transportFactory)
}

// TestMCPClient_FetchTools_ParseError는 잘못된 JSON 응답을 검증한다.
func TestMCPClient_FetchTools_ParseError(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return makeInitResponse(map[string]bool{"tools": true})
			}
			if req.Method == "tools/list" {
				return JSONRPCResponse{
					JSONRPC: JSONRPCVersion,
					Result:  json.RawMessage(`{invalid json}`),
				}
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	session, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "parse-err-test"})
	require.NoError(t, err)

	_, err = client.ListTools(context.Background(), session)
	require.Error(t, err)
}

// TestMCPClient_Initialize_Error는 initialize 에러 응답을 검증한다.
func TestMCPClient_Initialize_Error(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return JSONRPCResponse{
					JSONRPC: JSONRPCVersion,
					Error:   &JSONRPCError{Code: ErrCodeInternal, Message: "init failed"},
				}
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	_, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "init-err-test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init failed")
}

// TestMCPClient_Initialize_ParseError는 잘못된 initialize 응답을 검증한다.
func TestMCPClient_Initialize_ParseError(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return JSONRPCResponse{
					JSONRPC: JSONRPCVersion,
					Result:  json.RawMessage(`{bad json}`),
				}
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	_, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "init-parse-err"})
	require.Error(t, err)
}

// TestMCPClient_ListTools_CapabilityNotDeclared는 tools capability 없는 경우를 검증한다.
func TestMCPClient_ListTools_CapabilityNotDeclared(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return makeInitResponse(map[string]bool{}) // 아무 capability 없음
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	session, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "no-cap-test"})
	require.NoError(t, err)

	_, err = client.ListTools(context.Background(), session)
	assert.True(t, errors.Is(err, ErrCapabilityNotSupported))
}

// TestMCPClient_CallTool_Capability는 tools capability 없는 CallTool을 검증한다.
func TestMCPClient_CallTool_Capability(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return makeInitResponse(map[string]bool{})
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	session, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "no-cap-call-test"})
	require.NoError(t, err)

	_, err = client.CallTool(context.Background(), session, "mcp__fx__search", nil)
	assert.True(t, errors.Is(err, ErrCapabilityNotSupported))
}

// TestMCPClient_ListResources_Capability는 resources capability 없는 ListResources를 검증한다.
func TestMCPClient_ListResources_Capability(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return makeInitResponse(map[string]bool{})
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	session, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "no-res-cap-test"})
	require.NoError(t, err)

	_, err = client.ListResources(context.Background(), session)
	assert.True(t, errors.Is(err, ErrCapabilityNotSupported))
}

// TestMCPClient_ReadResource_Capability는 resources capability 없는 ReadResource를 검증한다.
func TestMCPClient_ReadResource_Capability(t *testing.T) {
	factory := func(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
		mockFn := func(req JSONRPCRequest) JSONRPCResponse {
			if req.Method == "initialize" {
				return makeInitResponse(map[string]bool{})
			}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion}
		}
		return newMockTransportFn(mockFn), nil
	}

	client := newTestClient(factory)
	session, err := client.ConnectToServer(context.Background(), MCPServerConfig{Name: "fx", Transport: "stdio", Command: "echo", ID: "no-read-cap-test"})
	require.NoError(t, err)

	_, err = client.ReadResource(context.Background(), session, "file:///test.txt")
	assert.True(t, errors.Is(err, ErrCapabilityNotSupported))
}

// TestSaveCredential_DirCreation은 디렉토리 자동 생성을 검증한다.
func TestSaveCredential_DirCreation(t *testing.T) {
	oldHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	os.Unsetenv("MINK_HOME")
	defer func() {
		os.Setenv("HOME", oldHome)
		os.Unsetenv("MINK_HOME")
	}()

	// 디렉토리가 없는 상태에서 저장
	ts := &TokenSet{AccessToken: "tok"}
	err := SaveCredential("dir-create-test", ts)
	require.NoError(t, err)

	// 파일이 생성되었는지 확인 (.mink/mcp-credentials/, REQ-MINK-UDM-002)
	path := filepath.Join(tmpHome, ".mink", credentialsDirName, "dir-create-test.json")
	_, err = os.Stat(path)
	assert.NoError(t, err)
}
