// Package file은 파일 관련 내장 tool을 제공한다.
// SPEC-GOOSE-TOOLS-001 §3.1 #3 file/*
package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/modu-ai/mink/internal/tools"
	"github.com/modu-ai/mink/internal/tools/builtin"
)

func init() {
	builtin.Register(NewFileRead())
}

// fileReadInput은 FileRead tool 입력이다.
type fileReadInput struct {
	Path   string `json:"path"`
	Offset *int   `json:"offset,omitempty"` // line 단위
	Limit  *int   `json:"limit,omitempty"`  // line 단위
}

// fileReadTool은 파일 내용을 읽는 tool이다.
type fileReadTool struct{}

// NewFileRead는 새 FileRead tool을 반환한다.
func NewFileRead() tools.Tool {
	return &fileReadTool{}
}

func (t *fileReadTool) Name() string { return "FileRead" }

func (t *fileReadTool) Schema() json.RawMessage {
	return json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "읽을 파일 경로"
    },
    "offset": {
      "type": "integer",
      "description": "시작 줄 번호 (0-indexed)",
      "minimum": 0
    },
    "limit": {
      "type": "integer",
      "description": "읽을 최대 줄 수",
      "minimum": 1
    }
  },
  "required": ["path"],
  "additionalProperties": false
}`)
}

func (t *fileReadTool) Scope() tools.Scope { return tools.ScopeShared }

func (t *fileReadTool) Call(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	var inp fileReadInput
	if err := json.Unmarshal(input, &inp); err != nil {
		return tools.ToolResult{IsError: true, Content: []byte("invalid input: " + err.Error())}, nil
	}

	content, err := os.ReadFile(inp.Path)
	if err != nil {
		return tools.ToolResult{IsError: true, Content: []byte(fmt.Sprintf("read_error: %v", err))}, nil
	}

	// offset/limit 처리 (line 단위)
	if inp.Offset != nil || inp.Limit != nil {
		lines := strings.Split(string(content), "\n")
		start := 0
		if inp.Offset != nil {
			start = *inp.Offset
			if start >= len(lines) {
				start = len(lines)
			}
		}
		end := len(lines)
		if inp.Limit != nil {
			end = start + *inp.Limit
			if end > len(lines) {
				end = len(lines)
			}
		}
		content = []byte(strings.Join(lines[start:end], "\n"))
	}

	meta := map[string]any{"bytes_read": len(content)}
	if !utf8.Valid(content) {
		meta["encoding"] = "binary"
	}

	return tools.ToolResult{
		Content:  content,
		Metadata: meta,
	}, nil
}
