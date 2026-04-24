package google

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modu-ai/goose/internal/message"
	"google.golang.org/genai"
)

// realGeminiClient는 실제 google.golang.org/genai SDK를 사용하는 구현이다.
// 라이브 API에서만 사용된다. 단위 테스트는 fake client를 주입한다.
type realGeminiClient struct {
	apiKey string
}

func newRealGeminiClient(apiKey string) GeminiClientIface {
	return &realGeminiClient{apiKey: apiKey}
}

func (c *realGeminiClient) GenerateStream(ctx context.Context, req GeminiRequest) (GeminiStream, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  c.apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("google: genai client 생성 실패: %w", err)
	}

	// 메시지를 genai 형식으로 변환
	contents := convertMessagesToGenai(req.Messages)

	// 스트리밍 생성 시작 — iter.Seq2[*genai.GenerateContentResponse, error]
	seqIter := client.Models.GenerateContentStream(ctx, req.Model, contents, nil)
	return &realGeminiStream{seqIter: seqIter}, nil
}

// realGeminiStream은 genai SDK iter.Seq2 스트림을 GeminiStream 인터페이스로 감싼다.
type realGeminiStream struct {
	seqIter   func(func(*genai.GenerateContentResponse, error) bool)
	nextCh    chan *genai.GenerateContentResponse
	errCh     chan error
	initiated bool
}

func (s *realGeminiStream) init() {
	if s.initiated {
		return
	}
	s.initiated = true
	s.nextCh = make(chan *genai.GenerateContentResponse, 8)
	s.errCh = make(chan error, 1)
	go func() {
		defer close(s.nextCh)
		s.seqIter(func(resp *genai.GenerateContentResponse, err error) bool {
			if err != nil {
				s.errCh <- err
				return false
			}
			s.nextCh <- resp
			return true
		})
	}()
}

func (s *realGeminiStream) Next() (*GeminiChunk, error) {
	s.init()
	select {
	case err := <-s.errCh:
		return nil, err
	case resp, ok := <-s.nextCh:
		if !ok {
			return nil, ErrStreamDone
		}
		return convertResponseToChunk(resp), nil
	}
}

func (s *realGeminiStream) Close() {}

// convertResponseToChunk는 genai.GenerateContentResponse를 GeminiChunk로 변환한다.
func convertResponseToChunk(resp *genai.GenerateContentResponse) *GeminiChunk {
	if resp == nil {
		return &GeminiChunk{IsDone: true}
	}
	chunk := &GeminiChunk{}
	for _, cand := range resp.Candidates {
		if cand.Content == nil {
			continue
		}
		for _, part := range cand.Content.Parts {
			if part.Text != "" {
				chunk.Text += part.Text
			}
			if part.FunctionCall != nil {
				chunk.HasTool = true
				chunk.ToolName = part.FunctionCall.Name
				if part.FunctionCall.Args != nil {
					if b, err := json.Marshal(part.FunctionCall.Args); err == nil {
						chunk.ToolArgs = string(b)
					}
				}
			}
		}
	}
	return chunk
}

// convertMessagesToGenai는 message.Message 목록을 genai.Content 목록으로 변환한다.
func convertMessagesToGenai(msgs []message.Message) []*genai.Content {
	result := make([]*genai.Content, 0, len(msgs))
	for _, m := range msgs {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}

		var parts []*genai.Part
		for _, block := range m.Content {
			switch block.Type {
			case "text":
				parts = append(parts, genai.NewPartFromText(block.Text))
			case "image":
				parts = append(parts, genai.NewPartFromBytes(block.Image, block.ImageMediaType))
			}
		}

		result = append(result, &genai.Content{
			Role:  role,
			Parts: parts,
		})
	}
	return result
}
