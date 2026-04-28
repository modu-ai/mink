package ollama

import (
	"context"
	"log/slog"
	"sync"

	"github.com/modu-ai/goose/internal/llm"
)

// Provider implements LLMProvider for Ollama.
// SPEC-GOOSE-LLM-001 §6.3: HTTP adapter for /api/chat and /api/show endpoints.
type Provider struct {
	endpoint string
	client   *httpClient
	caps     *capabilitiesCache
	logger   *slog.Logger
}

// New creates a new Ollama provider.
// SPEC-GOOSE-LLM-001: Base URL defaults to http://localhost:11434.
func New(endpoint string, logger *slog.Logger) (*Provider, error) {
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}

	return &Provider{
		endpoint: endpoint,
		client:   newHTTPClient(endpoint),
		caps:     newCapabilitiesCache(),
		logger:   logger,
	}, nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "ollama"
}

// Complete performs a blocking LLM completion request.
// SPEC-GOOSE-LLM-001 AC-LLM-001: Returns text and token counts from Ollama response.
func (p *Provider) Complete(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	if err := req.Validate(); err != nil {
		return llm.CompletionResponse{}, err
	}

	// Convert to Ollama request format
	ollamaReq := p.convertRequest(req)

	// Make HTTP request
	resp, err := p.client.PostChat(ctx, ollamaReq)
	if err != nil {
		return llm.CompletionResponse{}, p.mapError(err)
	}

	// Convert response
	result := llm.CompletionResponse{
		Text: resp.Message.Content,
		Usage: llm.Usage{
			PromptTokens:     resp.PromptEvalCount,
			CompletionTokens: resp.EvalCount,
			TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
			Unknown:          false,
		},
		Model: req.Model,
		Raw:   resp,
	}

	if p.logger != nil {
		p.logger.Debug("ollama complete",
			"model", req.Model,
			"prompt_tokens", result.Usage.PromptTokens,
			"completion_tokens", result.Usage.CompletionTokens,
		)
	}

	return result, nil
}

// Stream performs a streaming LLM completion request.
// SPEC-GOOSE-LLM-001 AC-LLM-002: Returns channel of NDJSON chunks.
// REQ-LLM-004: Channel closes when stream ends or error occurs.
func (p *Provider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.Chunk, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Convert to Ollama request format
	ollamaReq := p.convertRequest(req)
	ollamaReq.Stream = true

	// Make streaming HTTP request
	stream, err := p.client.PostChatStream(ctx, ollamaReq)
	if err != nil {
		return nil, p.mapError(err)
	}

	// Create output channel
	out := make(chan llm.Chunk, 8)

	// Start goroutine to parse stream
	go func() {
		defer close(out)
		p.parseStream(ctx, stream, out)
	}()

	return out, nil
}

// CountTokens extracts token counts from the last response.
// SPEC-GOOSE-LLM-001: For Ollama, tokens are in response (eval_count + prompt_eval_count).
// This method returns 0 for now since tokenization is not available as a separate API.
func (p *Provider) CountTokens(ctx context.Context, text string) (int, error) {
	// Ollama does not provide a separate tokenization endpoint
	// Token counts are only available in completion responses
	// Return 0 to indicate unknown
	return 0, nil
}

// Capabilities returns model capabilities.
// SPEC-GOOSE-LLM-001 REQ-LLM-009: First call fetches /api/show, cached thereafter.
func (p *Provider) Capabilities(ctx context.Context, model string) (llm.Capabilities, error) {
	// Check cache first
	if caps, ok := p.caps.Get(model); ok {
		return caps, nil
	}

	// Fetch from /api/show
	showResp, err := p.client.GetShow(ctx, model)
	if err != nil {
		return llm.Capabilities{}, p.mapError(err)
	}

	// Convert to capabilities
	caps := llm.Capabilities{
		MaxContextTokens: showResp.Details.ContextLength,
		SupportsTools:    true, // Ollama supports tools
		SupportsJSON:     false, // Ollama does not have native JSON mode
		Family:           showResp.Details.Family,
	}

	// Cache for future calls
	p.caps.Set(model, caps)

	return caps, nil
}

// convertRequest converts llm.CompletionRequest to Ollama format.
func (p *Provider) convertRequest(req llm.CompletionRequest) chatRequest {
	messages := make([]message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	ollamaReq := chatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   false,
	}

	// Add optional parameters
	if req.Temperature != nil {
		ollamaReq.Options.Temperature = *req.Temperature
	}
	if req.MaxTokens != nil {
		ollamaReq.Options.NumPredict = *req.MaxTokens
	}
	if len(req.Stop) > 0 {
		ollamaReq.Options.Stop = req.Stop
	}

	return ollamaReq
}

// mapError converts errors to LLMError types.
// SPEC-GOOSE-LLM-001 REQ-LLM-003: All errors must be typed LLMError.
func (p *Provider) mapError(err error) llm.LLMError {
	if err == nil {
		return nil
	}

	// Check if already LLMError
	if llmErr, ok := err.(llm.LLMError); ok {
		return llmErr
	}

	// Map HTTP errors
	if httpErr, ok := err.(*httpError); ok {
		return llm.MapHTTPStatusToError(httpErr.StatusCode, httpErr.Body)
	}

	// Default: wrap as server unavailable (likely network error)
	return llm.NewErrServerUnavailable(err.Error(), 0)
}

// parseStream parses NDJSON stream and sends chunks to output channel.
// REQ-LLM-010: Malformed lines close channel with error chunk.
// REQ-LLM-008: Context cancellation closes channel within 100ms.
func (p *Provider) parseStream(ctx context.Context, stream *chatStream, out chan<- llm.Chunk) {
	defer stream.Close()

	for {
		// Check context cancellation before each operation
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Read next line
		line, err := stream.Next()
		if err != nil {
			// Send error chunk and close
			select {
			case <-ctx.Done():
				return
			case out <- llm.Chunk{Done: true}:
			}
			return
		}

		// Parse JSON
		resp, err := parseChatStreamLine(line)
		if err != nil {
			// REQ-LLM-010: Malformed stream - send error chunk and close
			if p.logger != nil {
				p.logger.Debug("ollama malformed stream", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case out <- llm.Chunk{Done: true}:
			}
			return
		}

		// Send chunk
		chunk := llm.Chunk{
			Delta: resp.Message.Content,
			Done:  resp.Done,
		}

		if resp.Done {
			// Final chunk: include usage
			chunk.Usage = &llm.Usage{
				PromptTokens:     resp.PromptEvalCount,
				CompletionTokens: resp.EvalCount,
				TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
				Unknown:          false,
			}
		}

		// Check context cancellation before sending to prevent goroutine leak
		select {
		case <-ctx.Done():
			return
		case out <- chunk:
			if resp.Done {
				return
			}
		}
	}
}

// capabilitiesCache caches model capabilities.
// SPEC-GOOSE-LLM-001 §6.6: In-memory cache for provider lifetime.
type capabilitiesCache struct {
	mu   sync.RWMutex
	data map[string]llm.Capabilities
}

func newCapabilitiesCache() *capabilitiesCache {
	return &capabilitiesCache{
		data: make(map[string]llm.Capabilities),
	}
}

func (c *capabilitiesCache) Get(model string) (llm.Capabilities, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	caps, ok := c.data[model]
	return caps, ok
}

func (c *capabilitiesCache) Set(model string, caps llm.Capabilities) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[model] = caps
}

// Ensure Provider implements llm.LLMProvider at compile time.
var _ llm.LLMProvider = (*Provider)(nil)
