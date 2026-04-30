package qmd

import (
	"context"
	"fmt"
	"sort"
)

// Pipeline defines the 3-stage hybrid search pipeline.
// @MX:ANCHOR: Core pipeline interface (expected fan_in >= 3)
type Pipeline interface {
	// Execute runs the full 3-stage pipeline: BM25 → Vector → Rerank.
	Execute(ctx context.Context, query Query, k int) ([]Result, error)
}

// BM25Stage defines the BM25 full-text retrieval stage.
type BM25Stage interface {
	// Query performs BM25 retrieval and returns candidate results.
	Query(ctx context.Context, query string, k int) ([]Result, error)
}

// VectorStage defines the vector similarity search stage.
type VectorStage interface {
	// Query performs vector similarity search and rescores candidates.
	Query(ctx context.Context, query string, candidates []Result) ([]Result, error)
}

// RerankStage defines the LLM reranking stage.
type RerankStage interface {
	// Rerank reorders results using LLM-based scoring.
	Rerank(ctx context.Context, query string, results []Result) ([]Result, error)
}

// HybridPipeline implements the 3-stage hybrid search pipeline.
// @MX:NOTE: HybridPipeline combines BM25, vector, and LLM reranking
type HybridPipeline struct {
	bm25    BM25Stage
	vector  VectorStage
	rerank  RerankStage
	enabled map[string]bool // Feature flags for each stage
}

// NewHybridPipeline creates a new hybrid search pipeline.
// @MX:ANCHOR: Pipeline factory (expected fan_in >= 2)
func NewHybridPipeline(bm25 BM25Stage, vector VectorStage, rerank RerankStage) *HybridPipeline {
	return &HybridPipeline{
		bm25:   bm25,
		vector: vector,
		rerank: rerank,
		enabled: map[string]bool{
			"bm25":   true,
			"vector": true,
			"rerank": true,
		},
	}
}

// Execute runs the full 3-stage pipeline and returns top-k results.
func (p *HybridPipeline) Execute(ctx context.Context, query Query, k int) ([]Result, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// Stage 1: BM25 retrieval (get more candidates than needed)
	candidates, err := p.stage1BM25(ctx, query.Text, k*3)
	if err != nil {
		return nil, fmt.Errorf("BM25 stage failed: %w", err)
	}

	// Stage 2: Vector re-scoring
	vectorResults, err := p.stage2Vector(ctx, query.Text, candidates)
	if err != nil {
		return nil, fmt.Errorf("vector stage failed: %w", err)
	}

	// Stage 3: LLM reranking
	finalResults, err := p.stage3Rerank(ctx, query.Text, vectorResults)
	if err != nil {
		return nil, fmt.Errorf("rerank stage failed: %w", err)
	}

	// Return top-k results
	if len(finalResults) > k {
		finalResults = finalResults[:k]
	}

	return finalResults, nil
}

// stage1BM25 performs BM25 full-text retrieval.
func (p *HybridPipeline) stage1BM25(ctx context.Context, query string, k int) ([]Result, error) {
	if !p.enabled["bm25"] {
		return []Result{}, nil
	}

	results, err := p.bm25.Query(ctx, query, k)
	if err != nil {
		return nil, err
	}

	// Mark results with source
	for i := range results {
		results[i].Source = "bm25"
	}

	return results, nil
}

// stage2Vector performs vector similarity search.
func (p *HybridPipeline) stage2Vector(ctx context.Context, query string, candidates []Result) ([]Result, error) {
	if !p.enabled["vector"] {
		return candidates, nil
	}

	results, err := p.vector.Query(ctx, query, candidates)
	if err != nil {
		return nil, err
	}

	// Mark results with source
	for i := range results {
		results[i].Source = "vector"
	}

	return results, nil
}

// stage3Rerank performs LLM-based reranking.
func (p *HybridPipeline) stage3Rerank(ctx context.Context, query string, results []Result) ([]Result, error) {
	if !p.enabled["rerank"] {
		return results, nil
	}

	reranked, err := p.rerank.Rerank(ctx, query, results)
	if err != nil {
		return nil, err
	}

	// Mark results with source
	for i := range reranked {
		reranked[i].Source = "rerank"
	}

	// Sort by final score (descending)
	sort.Slice(reranked, func(i, j int) bool {
		return reranked[i].Score > reranked[j].Score
	})

	return reranked, nil
}

// EnableStage enables or disables a specific pipeline stage.
func (p *HybridPipeline) EnableStage(stage string, enabled bool) {
	p.enabled[stage] = enabled
}

// IsStageEnabled checks if a stage is enabled.
func (p *HybridPipeline) IsStageEnabled(stage string) bool {
	return p.enabled[stage]
}

// MockBM25Stage provides a mock BM25 implementation for testing.
type MockBM25Stage struct{}

// NewMockBM25Stage creates a new mock BM25 stage.
func NewMockBM25Stage() *MockBM25Stage {
	return &MockBM25Stage{}
}

// Query returns mock BM25 results.
func (m *MockBM25Stage) Query(ctx context.Context, query string, k int) ([]Result, error) {
	return []Result{}, nil
}

// MockVectorStage provides a mock vector stage for testing.
type MockVectorStage struct{}

// NewMockVectorStage creates a new mock vector stage.
func NewMockVectorStage() *MockVectorStage {
	return &MockVectorStage{}
}

// Query returns mock vector results.
func (m *MockVectorStage) Query(ctx context.Context, query string, candidates []Result) ([]Result, error) {
	return candidates, nil
}

// MockRerankStage provides a mock reranker for testing.
type MockRerankStage struct{}

// NewMockRerankStage creates a new mock reranker stage.
func NewMockRerankStage() *MockRerankStage {
	return &MockRerankStage{}
}

// Rerank returns mock reranked results.
func (m *MockRerankStage) Rerank(ctx context.Context, query string, results []Result) ([]Result, error) {
	return results, nil
}

// TraceResult represents a result with debug information.
type TraceResult struct {
	Result   Result
	DebugMap map[string]float64
}

// GetDebugMap extracts debug scores from a result if available.
func GetDebugMap(r Result) map[string]float64 {
	// In full implementation, this would extract from r.Debug
	// For Sprint 0, we return an empty map
	return map[string]float64{}
}

// CombineResults merges results from multiple stages with weighted scores.
// @MX:NOTE: Result combination uses configurable stage weights
func CombineResults(bm25Results, vectorResults []Result, weights map[string]float64) []Result {
	combined := make(map[string]Result)

	// Add BM25 results
	bm25Weight := weights["bm25"]
	if bm25Weight == 0 {
		bm25Weight = 0.3
	}

	for _, r := range bm25Results {
		combined[r.DocumentID] = Result{
			DocumentID: r.DocumentID,
			Path:       r.Path,
			Content:    r.Content,
			Score:      r.Score * bm25Weight,
			Source:     "bm25",
		}
	}

	// Add/vector results and combine scores
	vectorWeight := weights["vector"]
	if vectorWeight == 0 {
		vectorWeight = 0.7
	}

	for _, r := range vectorResults {
		if existing, ok := combined[r.DocumentID]; ok {
			// Combine scores
			combined[r.DocumentID] = Result{
				DocumentID: r.DocumentID,
				Path:       r.Path,
				Content:    r.Content,
				Score:      existing.Score + (r.Score * vectorWeight),
				Source:     "combined",
			}
		} else {
			combined[r.DocumentID] = Result{
				DocumentID: r.DocumentID,
				Path:       r.Path,
				Content:    r.Content,
				Score:      r.Score * vectorWeight,
				Source:     "vector",
			}
		}
	}

	// Convert map to slice and sort by score
	resultSlice := make([]Result, 0, len(combined))
	for _, r := range combined {
		resultSlice = append(resultSlice, r)
	}

	sort.Slice(resultSlice, func(i, j int) bool {
		return resultSlice[i].Score > resultSlice[j].Score
	})

	return resultSlice
}

// NormalizeScores normalizes result scores to 0-1 range.
func NormalizeScores(results []Result) []Result {
	if len(results) == 0 {
		return results
	}

	// Find min and max scores
	minScore := results[0].Score
	maxScore := results[0].Score

	for _, r := range results {
		if r.Score < minScore {
			minScore = r.Score
		}
		if r.Score > maxScore {
			maxScore = r.Score
		}
	}

	// Avoid division by zero
	if maxScore == minScore {
		for i := range results {
			results[i].Score = 0.5
		}
		return results
	}

	// Normalize to 0-1 range
	normalized := make([]Result, len(results))
	for i, r := range results {
		normalized[i] = r
		normalized[i].Score = (r.Score - minScore) / (maxScore - minScore)
	}

	return normalized
}

// FilterByScore removes results below the minimum score threshold.
func FilterByScore(results []Result, minScore float64) []Result {
	filtered := make([]Result, 0, len(results))

	for _, r := range results {
		if r.Score >= minScore {
			filtered = append(filtered, r)
		}
	}

	return filtered
}
