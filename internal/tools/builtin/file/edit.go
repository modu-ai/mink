package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/modu-ai/goose/internal/tools"
	"github.com/modu-ai/goose/internal/tools/builtin"
)

func init() {
	builtin.Register(NewFileEdit())
}

// fileEditInput은 FileEdit tool 입력이다.
type fileEditInput struct {
	Path       string `json:"path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll *bool  `json:"replace_all,omitempty"`
}

// fileEditTool은 파일의 특정 문자열을 교체하는 tool이다.
type fileEditTool struct {
	cwd                   string
	additionalDirectories []string
}

// NewFileEdit는 새 FileEdit tool을 반환한다.
func NewFileEdit() tools.Tool {
	return &fileEditTool{}
}

// NewFileEditWithCwd는 cwd boundary 제약을 가진 FileEdit tool을 반환한다.
func NewFileEditWithCwd(cwd string, additionalDirs []string) tools.Tool {
	return &fileEditTool{cwd: cwd, additionalDirectories: additionalDirs}
}

func (t *fileEditTool) Name() string { return "FileEdit" }

func (t *fileEditTool) Schema() json.RawMessage {
	return json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "편집할 파일 경로"
    },
    "old_string": {
      "type": "string",
      "description": "교체할 문자열 (정확 일치 필수)"
    },
    "new_string": {
      "type": "string",
      "description": "교체 후 문자열"
    },
    "replace_all": {
      "type": "boolean",
      "description": "모든 일치 항목 교체 여부"
    }
  },
  "required": ["path", "old_string", "new_string"],
  "additionalProperties": false
}`)
}

func (t *fileEditTool) Scope() tools.Scope { return tools.ScopeShared }

func (t *fileEditTool) Call(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	var inp fileEditInput
	if err := json.Unmarshal(input, &inp); err != nil {
		return tools.ToolResult{IsError: true, Content: []byte("invalid input: " + err.Error())}, nil
	}

	// REQ-TOOLS-015: cwd boundary 검사 (fileWriteTool에서 로직 재사용)
	writeHelper := &fileWriteTool{cwd: t.cwd, additionalDirectories: t.additionalDirectories}
	if err := writeHelper.checkCwdBoundary(inp.Path); err != nil {
		return tools.ToolResult{IsError: true, Content: []byte(err.Error())}, nil
	}

	content, err := os.ReadFile(inp.Path)
	if err != nil {
		return tools.ToolResult{IsError: true, Content: []byte(fmt.Sprintf("read_error: %v", err))}, nil
	}

	original := string(content)
	if !strings.Contains(original, inp.OldString) {
		return tools.ToolResult{
			IsError: true,
			Content: []byte(fmt.Sprintf("edit_error: old_string not found in %q", inp.Path)),
		}, nil
	}

	var replaced string
	var count int
	if inp.ReplaceAll != nil && *inp.ReplaceAll {
		replaced = strings.ReplaceAll(original, inp.OldString, inp.NewString)
		count = strings.Count(original, inp.OldString)
	} else {
		replaced = strings.Replace(original, inp.OldString, inp.NewString, 1)
		count = 1
	}

	if err := os.WriteFile(inp.Path, []byte(replaced), 0644); err != nil {
		return tools.ToolResult{IsError: true, Content: []byte(fmt.Sprintf("write_error: %v", err))}, nil
	}

	return tools.ToolResult{
		Content:  []byte(fmt.Sprintf(`{"replacements":%d}`, count)),
		Metadata: map[string]any{"replacements": count},
	}, nil
}
