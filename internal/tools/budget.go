package tools

import "fmt"

// TruncationMeta는 ApplyResultBudget의 메타데이터이다.
type TruncationMeta struct {
	// Truncated는 실제로 잘렸는지 여부이다.
	Truncated bool
	// OriginalSize는 원본 Content 크기(바이트)이다.
	OriginalSize int64
	// TruncatedSize는 잘린 Content 크기(바이트)이다.
	TruncatedSize int64
}

// ApplyResultBudget는 ToolResult Content를 cap 이하로 자른다.
// REQ-QUERY-007 보조: Executor 반환 전 QUERY-001이 호출한다.
// cap <= 0이면 무제한.
func ApplyResultBudget(result ToolResult, cap int64) (ToolResult, TruncationMeta) {
	original := int64(len(result.Content))
	if cap <= 0 || original <= cap {
		return result, TruncationMeta{Truncated: false, OriginalSize: original, TruncatedSize: original}
	}

	// 잘라서 반환
	truncated := result.Content[:cap]
	suffix := fmt.Sprintf("\n[truncated: %d bytes omitted]", original-cap)
	available := cap - int64(len(suffix))
	if available < 0 {
		available = 0
	}
	content := append(result.Content[:available], []byte(suffix)...)

	out := ToolResult{
		Content:  content,
		IsError:  result.IsError,
		Metadata: result.Metadata,
	}
	if out.Metadata == nil {
		out.Metadata = make(map[string]any)
	}
	out.Metadata["truncated"] = true
	out.Metadata["original_size"] = original

	_ = truncated // suppress lint
	return out, TruncationMeta{
		Truncated:     true,
		OriginalSize:  original,
		TruncatedSize: int64(len(content)),
	}
}
