//go:build ignore

// fixture_server는 테스트용 최소 MCP 서버이다.
// 이 파일은 `go build -o fixture-server .` 로 빌드하여 테스트에서 subprocess로 사용한다.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RpcError       `json:"error,omitempty"`
}

type RpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func writeResponse(resp Response) {
	b, _ := json.Marshal(resp)
	fmt.Println(string(b))
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}

		switch req.Method {
		case "initialize":
			result, _ := json.Marshal(map[string]any{
				"protocolVersion": "2025-03-26",
				"capabilities": map[string]any{
					"tools":   map[string]any{},
					"prompts": map[string]any{},
				},
				"serverInfo": map[string]string{"name": "fixture-server", "version": "0.1.0"},
			})
			writeResponse(Response{JSONRPC: "2.0", ID: req.ID, Result: result})

		case "notifications/initialized":
			// 알림은 응답 없음

		case "tools/list":
			result, _ := json.Marshal(map[string]any{
				"tools": []map[string]any{
					{"name": "search", "description": "Search tool", "inputSchema": map[string]any{"type": "object"}},
					{"name": "fetch", "description": "Fetch tool", "inputSchema": map[string]any{"type": "object"}},
				},
			})
			writeResponse(Response{JSONRPC: "2.0", ID: req.ID, Result: result})

		case "tools/call":
			var params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			json.Unmarshal(req.Params, &params)
			result, _ := json.Marshal(map[string]any{
				"content": []map[string]any{{"type": "text", "text": "echo:" + params.Name}},
				"isError": false,
			})
			writeResponse(Response{JSONRPC: "2.0", ID: req.ID, Result: result})

		case "prompts/list":
			result, _ := json.Marshal(map[string]any{
				"prompts": []map[string]any{
					{"name": "greet", "description": "Greeting prompt", "arguments": []map[string]any{{"name": "lang", "required": false}}},
				},
			})
			writeResponse(Response{JSONRPC: "2.0", ID: req.ID, Result: result})

		default:
			writeResponse(Response{
				JSONRPC: "2.0", ID: req.ID,
				Error: &RpcError{Code: -32601, Message: "method not found: " + req.Method},
			})
		}
	}
}
