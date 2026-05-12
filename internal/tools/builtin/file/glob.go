package file

import (
	"context"
	"encoding/json"
	"os"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/modu-ai/mink/internal/tools"
	"github.com/modu-ai/mink/internal/tools/builtin"
)

func init() {
	builtin.Register(NewGlob())
}

// globInput은 Glob tool 입력이다.
type globInput struct {
	Pattern string `json:"pattern"`
	Cwd     string `json:"cwd,omitempty"`
}

// globTool은 파일 경로 패턴 매칭 tool이다.
type globTool struct {
	defaultCwd string
}

// NewGlob는 새 Glob tool을 반환한다.
func NewGlob() tools.Tool {
	return &globTool{}
}

// NewGlobWithCwd는 기본 cwd를 가진 Glob tool을 반환한다.
func NewGlobWithCwd(cwd string) tools.Tool {
	return &globTool{defaultCwd: cwd}
}

func (t *globTool) Name() string { return "Glob" }

func (t *globTool) Schema() json.RawMessage {
	return json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "pattern": {
      "type": "string",
      "description": "glob 패턴 (** 지원)"
    },
    "cwd": {
      "type": "string",
      "description": "검색 기준 디렉토리 (기본: registry cwd)"
    }
  },
  "required": ["pattern"],
  "additionalProperties": false
}`)
}

func (t *globTool) Scope() tools.Scope { return tools.ScopeShared }

func (t *globTool) Call(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	var inp globInput
	if err := json.Unmarshal(input, &inp); err != nil {
		return tools.ToolResult{IsError: true, Content: []byte("invalid input: " + err.Error())}, nil
	}

	searchDir := inp.Cwd
	if searchDir == "" {
		searchDir = t.defaultCwd
	}
	if searchDir == "" {
		var err error
		searchDir, err = os.Getwd()
		if err != nil {
			return tools.ToolResult{IsError: true, Content: []byte("cwd_error: " + err.Error())}, nil
		}
	}

	fsys := os.DirFS(searchDir)
	matches, err := doublestar.Glob(fsys, inp.Pattern)
	if err != nil {
		return tools.ToolResult{IsError: true, Content: []byte("glob_error: " + err.Error())}, nil
	}

	result, err := json.Marshal(matches)
	if err != nil {
		return tools.ToolResult{IsError: true, Content: []byte("marshal_error: " + err.Error())}, nil
	}

	return tools.ToolResult{
		Content:  result,
		Metadata: map[string]any{"count": len(matches)},
	}, nil
}
