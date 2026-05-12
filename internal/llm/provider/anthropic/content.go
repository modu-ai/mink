package anthropic

import (
	"encoding/base64"
	"strings"

	"github.com/modu-ai/mink/internal/message"
)

// AnthropicMessage는 Anthropic API의 메시지 형식이다.
type AnthropicMessage struct {
	// Role은 메시지 역할이다 ("user" | "assistant").
	Role string `json:"role"`
	// Content는 콘텐츠 블록 목록이다.
	Content []map[string]any `json:"content"`
}

// ConvertMessages는 message.Message 목록을 Anthropic API 형식으로 변환한다.
//
// 반환값:
//   - system: 추출된 system 메시지 텍스트 (여러 개면 "\n"으로 합침)
//   - converted: user/assistant 메시지 목록
//   - error: 변환 오류
func ConvertMessages(msgs []message.Message) (system string, converted []AnthropicMessage, err error) {
	if len(msgs) == 0 {
		return "", nil, nil
	}

	var systemParts []string
	var result []AnthropicMessage

	for _, msg := range msgs {
		if msg.Role == "system" {
			// system 메시지는 분리하여 합친다
			for _, block := range msg.Content {
				if block.Type == "text" {
					systemParts = append(systemParts, block.Text)
				}
			}
			continue
		}

		// user / assistant 메시지 변환
		converted, convErr := convertContentBlocks(msg.Content)
		if convErr != nil {
			return "", nil, convErr
		}

		am := AnthropicMessage{
			Role:    normalizeRole(msg.Role),
			Content: converted,
		}
		result = append(result, am)
	}

	systemText := strings.Join(systemParts, "\n")
	return systemText, result, nil
}

// normalizeRole은 메시지 역할을 Anthropic API 형식으로 정규화한다.
func normalizeRole(role string) string {
	switch role {
	case "assistant":
		return "assistant"
	default:
		return "user"
	}
}

// convertContentBlocks는 ContentBlock 슬라이스를 Anthropic API 블록 형식으로 변환한다.
func convertContentBlocks(blocks []message.ContentBlock) ([]map[string]any, error) {
	result := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		converted, err := convertContentBlock(block)
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return result, nil
}

// convertContentBlock은 단일 ContentBlock을 Anthropic API 블록으로 변환한다.
func convertContentBlock(block message.ContentBlock) (map[string]any, error) {
	switch block.Type {
	case "text":
		return map[string]any{
			"type": "text",
			"text": block.Text,
		}, nil

	case "image":
		encoded := base64.StdEncoding.EncodeToString(block.Image)
		mediaType := block.ImageMediaType
		if mediaType == "" {
			mediaType = "image/jpeg"
		}
		return map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": mediaType,
				"data":       encoded,
			},
		}, nil

	case "tool_use":
		return map[string]any{
			"type": "tool_use",
			"id":   block.ToolUseID,
			"name": block.Text,
		}, nil

	case "tool_result":
		return map[string]any{
			"type":        "tool_result",
			"tool_use_id": block.ToolUseID,
			"content":     block.ToolResultJSON,
		}, nil

	case "thinking":
		return map[string]any{
			"type":     "thinking",
			"thinking": block.Thinking,
		}, nil

	default:
		// 알 수 없는 블록 타입은 text로 처리
		return map[string]any{
			"type": "text",
			"text": block.Text,
		}, nil
	}
}
