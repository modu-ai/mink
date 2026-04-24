package anthropic

import "github.com/modu-ai/goose/internal/llm/cache"

// ApplyCacheMarkers는 CachePlan의 Markers를 AnthropicMessage 목록에 적용한다.
// 각 마커의 MessageIndex에 해당하는 메시지의 마지막 ContentBlock에
// cache_control 필드를 추가한다.
//
// REQ-ADAPTER-015: plan.Markers가 비어 있으면 입력 무변경
func ApplyCacheMarkers(msgs []AnthropicMessage, plan cache.CachePlan) []AnthropicMessage {
	if len(plan.Markers) == 0 {
		return msgs
	}

	// 원본을 수정하지 않도록 복사
	result := make([]AnthropicMessage, len(msgs))
	copy(result, msgs)

	for _, marker := range plan.Markers {
		if marker.MessageIndex < 0 || marker.MessageIndex >= len(result) {
			// 범위 밖 인덱스는 무시
			continue
		}

		msg := &result[marker.MessageIndex]
		if len(msg.Content) == 0 {
			continue
		}

		// 마지막 ContentBlock에 cache_control 추가
		// 원본 슬라이스를 수정하지 않도록 복사
		newContent := make([]map[string]any, len(msg.Content))
		for i, block := range msg.Content {
			newBlock := make(map[string]any, len(block)+1)
			for k, v := range block {
				newBlock[k] = v
			}
			newContent[i] = newBlock
		}

		lastIdx := len(newContent) - 1
		newContent[lastIdx]["cache_control"] = map[string]any{
			"type": string(marker.TTL),
		}
		msg.Content = newContent
	}

	return result
}
