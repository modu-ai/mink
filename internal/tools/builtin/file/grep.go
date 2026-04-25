package file

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/modu-ai/goose/internal/tools"
	"github.com/modu-ai/goose/internal/tools/builtin"
)

func init() {
	builtin.Register(NewGrep())
}

// grepFlags는 Grep tool 옵션이다.
type grepFlags struct {
	I bool `json:"i,omitempty"` // case-insensitive
	N bool `json:"n,omitempty"` // line number
	C int  `json:"C,omitempty"` // context lines
}

// grepInput은 Grep tool 입력이다.
type grepInput struct {
	Pattern string     `json:"pattern"`
	Path    string     `json:"path"`
	Flags   *grepFlags `json:"flags,omitempty"`
}

// grepMatch는 Grep 결과의 단일 일치 항목이다.
type grepMatch struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// grepTool은 파일 내용 검색 tool이다.
type grepTool struct{}

// NewGrep은 새 Grep tool을 반환한다.
func NewGrep() tools.Tool {
	return &grepTool{}
}

func (t *grepTool) Name() string { return "Grep" }

func (t *grepTool) Schema() json.RawMessage {
	return json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "pattern": {
      "type": "string",
      "description": "검색할 정규식 패턴"
    },
    "path": {
      "type": "string",
      "description": "검색할 파일 또는 디렉토리 경로"
    },
    "flags": {
      "type": "object",
      "properties": {
        "i": {"type": "boolean", "description": "대소문자 무시"},
        "n": {"type": "boolean", "description": "줄 번호 포함"},
        "C": {"type": "integer", "description": "전후 컨텍스트 줄 수", "minimum": 0}
      },
      "additionalProperties": false
    }
  },
  "required": ["pattern", "path"],
  "additionalProperties": false
}`)
}

func (t *grepTool) Scope() tools.Scope { return tools.ScopeShared }

func (t *grepTool) Call(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	var inp grepInput
	if err := json.Unmarshal(input, &inp); err != nil {
		return tools.ToolResult{IsError: true, Content: []byte("invalid input: " + err.Error())}, nil
	}

	// 정규식 컴파일
	patternStr := inp.Pattern
	if inp.Flags != nil && inp.Flags.I {
		patternStr = "(?i)" + patternStr
	}
	re, err := regexp.Compile(patternStr)
	if err != nil {
		return tools.ToolResult{IsError: true, Content: []byte(fmt.Sprintf("regex_error: %v", err))}, nil
	}

	// 파일 목록 수집
	var files []string
	info, err := os.Stat(inp.Path)
	if err != nil {
		return tools.ToolResult{IsError: true, Content: []byte(fmt.Sprintf("path_error: %v", err))}, nil
	}
	if info.IsDir() {
		err = filepath.WalkDir(inp.Path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return tools.ToolResult{IsError: true, Content: []byte(fmt.Sprintf("walk_error: %v", err))}, nil
		}
	} else {
		files = []string{inp.Path}
	}

	var matches []grepMatch
	for _, f := range files {
		fMatches, err := grepFile(f, re)
		if err != nil {
			continue // 읽기 실패 파일 skip
		}
		matches = append(matches, fMatches...)
	}

	result, err := json.Marshal(matches)
	if err != nil {
		return tools.ToolResult{IsError: true, Content: []byte("marshal_error: " + err.Error())}, nil
	}

	return tools.ToolResult{
		Content:  result,
		Metadata: map[string]any{"match_count": len(matches)},
	}, nil
}

// grepFile은 단일 파일에서 패턴을 검색한다.
func grepFile(path string, re *regexp.Regexp) ([]grepMatch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var matches []grepMatch
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, grepMatch{
				File: path,
				Line: lineNum,
				Text: strings.TrimRight(line, "\r"),
			})
		}
	}
	return matches, scanner.Err()
}
