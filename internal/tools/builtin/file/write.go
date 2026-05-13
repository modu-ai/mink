package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modu-ai/mink/internal/tools"
	"github.com/modu-ai/mink/internal/tools/builtin"
	"github.com/modu-ai/mink/internal/userpath"
)

func init() {
	builtin.Register(NewFileWrite())
}

// fileWriteInput은 FileWrite tool 입력이다.
type fileWriteInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// fileWriteTool은 파일에 내용을 쓰는 tool이다.
type fileWriteTool struct {
	cwd                   string
	additionalDirectories []string
}

// NewFileWrite는 새 FileWrite tool을 반환한다.
func NewFileWrite() tools.Tool {
	return &fileWriteTool{}
}

// NewFileWriteWithCwd는 cwd boundary 제약을 가진 FileWrite tool을 반환한다.
func NewFileWriteWithCwd(cwd string, additionalDirs []string) tools.Tool {
	return &fileWriteTool{cwd: cwd, additionalDirectories: additionalDirs}
}

func (t *fileWriteTool) Name() string { return "FileWrite" }

func (t *fileWriteTool) Schema() json.RawMessage {
	return json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "쓸 파일 경로"
    },
    "content": {
      "type": "string",
      "description": "파일에 쓸 내용"
    }
  },
  "required": ["path", "content"],
  "additionalProperties": false
}`)
}

func (t *fileWriteTool) Scope() tools.Scope { return tools.ScopeShared }

func (t *fileWriteTool) Call(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	var inp fileWriteInput
	if err := json.Unmarshal(input, &inp); err != nil {
		return tools.ToolResult{IsError: true, Content: []byte("invalid input: " + err.Error())}, nil
	}

	// REQ-TOOLS-015: cwd boundary 검사
	if err := t.checkCwdBoundary(inp.Path); err != nil {
		return tools.ToolResult{IsError: true, Content: []byte(err.Error())}, nil
	}

	// 디렉토리 생성
	dir := filepath.Dir(inp.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return tools.ToolResult{IsError: true, Content: []byte(fmt.Sprintf("mkdir_error: %v", err))}, nil
	}

	// Atomic write: tmp 파일에 쓰고 rename
	// REQ-MINK-UDM-004 (AC-006): .mink- prefix 사용.
	tmpFile, err := os.CreateTemp(dir, userpath.TempPrefix()+"write-*")
	if err != nil {
		return tools.ToolResult{IsError: true, Content: []byte(fmt.Sprintf("create_temp_error: %v", err))}, nil
	}
	tmpPath := tmpFile.Name()

	_, writeErr := tmpFile.WriteString(inp.Content)
	closeErr := tmpFile.Close()

	if writeErr != nil || closeErr != nil {
		os.Remove(tmpPath)
		msg := writeErr
		if msg == nil {
			msg = closeErr
		}
		return tools.ToolResult{IsError: true, Content: []byte(fmt.Sprintf("write_error: %v", msg))}, nil
	}

	if err := os.Rename(tmpPath, inp.Path); err != nil {
		os.Remove(tmpPath)
		return tools.ToolResult{IsError: true, Content: []byte(fmt.Sprintf("rename_error: %v", err))}, nil
	}

	return tools.ToolResult{
		Content:  []byte(fmt.Sprintf(`{"bytes_written":%d}`, len(inp.Content))),
		Metadata: map[string]any{"bytes_written": len(inp.Content)},
	}, nil
}

// checkCwdBoundary는 path가 cwd 내부에 있는지 확인한다.
// REQ-TOOLS-015: R5 symlink 우회 방지를 위해 EvalSymlinks 사용.
func (t *fileWriteTool) checkCwdBoundary(path string) error {
	if t.cwd == "" {
		return nil // cwd 제약 없음
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("path resolution error: %v", err)
	}

	// symlink 해석
	resolvedPath := absPath
	if _, err := os.Lstat(absPath); err == nil {
		if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
			resolvedPath = resolved
		}
	}

	cwdAbs, err := filepath.Abs(t.cwd)
	if err != nil {
		return fmt.Errorf("cwd resolution error: %v", err)
	}
	resolvedCwd := cwdAbs
	if resolved, err := filepath.EvalSymlinks(cwdAbs); err == nil {
		resolvedCwd = resolved
	}

	cwdWithSep := resolvedCwd + string(filepath.Separator)
	if resolvedPath != resolvedCwd && !strings.HasPrefix(resolvedPath, cwdWithSep) {
		// additional_directories 확인
		for _, dir := range t.additionalDirectories {
			dirAbs, _ := filepath.Abs(dir)
			dirWithSep := dirAbs + string(filepath.Separator)
			if resolvedPath == dirAbs || strings.HasPrefix(resolvedPath, dirWithSep) {
				return nil
			}
		}
		return fmt.Errorf("write denied: path %q is outside cwd %q", path, t.cwd)
	}

	return nil
}
