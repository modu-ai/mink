// Package router_test는 router 패키지의 외부 테스트를 포함한다.
package router_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/modu-ai/goose/internal/llm/router"
)

// newTestConfig는 테스트용 기본 RoutingConfig를 생성한다.
func newTestConfig() router.RoutingConfig {
	return router.RoutingConfig{
		Primary: router.RouteDefinition{
			Model:    "claude-opus",
			Provider: "anthropic",
			Mode:     "chat",
			Command:  "messages.create",
		},
		CheapRoute: &router.RouteDefinition{
			Model:    "claude-haiku",
			Provider: "anthropic",
			Mode:     "chat",
			Command:  "messages.create",
		},
		ForceMode: router.ForceModeAuto,
	}
}

// newTestRouter는 DefaultRegistry와 nop logger로 라우터를 생성한다.
func newTestRouter(t *testing.T, cfg router.RoutingConfig) *router.Router {
	t.Helper()
	r, err := router.New(cfg, router.DefaultRegistry(), zap.NewNop())
	if err != nil {
		t.Fatalf("router.New() 실패: %v", err)
	}
	return r
}

// makeRequest는 단일 user 메시지를 담은 RoutingRequest를 생성한다.
func makeRequest(msg string) router.RoutingRequest {
	return router.RoutingRequest{
		Messages: []router.Message{
			{Role: "user", Content: msg},
		},
	}
}

// TestRouter_CheapRouteNil_FallsBackToPrimary는 CheapRoute가 nil일 때
// 단순 메시지도 primary route를 반환하고 reason이 "primary_only_configured"인지 검증한다.
// AC-ROUTER-006.
func TestRouter_CheapRouteNil_FallsBackToPrimary(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig()
	cfg.CheapRoute = nil

	r := newTestRouter(t, cfg)

	// 단순 메시지 (6기준 모두 통과해야 함)
	req := makeRequest("안녕하세요, 오늘 날씨 어때요?")
	route, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("Route() 에러: %v", err)
	}

	if route.Model != "claude-opus" {
		t.Errorf("Model=%q, want %q", route.Model, "claude-opus")
	}
	if route.RoutingReason != "primary_only_configured" {
		t.Errorf("RoutingReason=%q, want %q", route.RoutingReason, "primary_only_configured")
	}
	if route.Signature == "" {
		t.Error("Signature가 비어 있음")
	}
}

// TestRouter_Signature_Reproducible은 동일 RoutingRequest를 두 번 호출할 때
// 동일한 Signature가 반환되는지 검증한다. AC-ROUTER-007.
func TestRouter_Signature_Reproducible(t *testing.T) {
	t.Parallel()

	r := newTestRouter(t, newTestConfig())

	req := makeRequest("debug this function please")

	route1, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("첫 번째 Route() 에러: %v", err)
	}

	route2, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("두 번째 Route() 에러: %v", err)
	}

	if route1.Signature != route2.Signature {
		t.Errorf("Signature 불일치: %q vs %q", route1.Signature, route2.Signature)
	}
	if route1.Signature == "" {
		t.Error("Signature가 비어 있음")
	}
}

// TestRouter_UnregisteredProvider_ReturnsError는 미등록 provider를 사용할 때
// ProviderNotRegisteredError를 반환하는지 검증한다. AC-ROUTER-008.
func TestRouter_UnregisteredProvider_ReturnsError(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig()
	cfg.Primary = router.RouteDefinition{
		Model:    "some-model",
		Provider: "nonexistent_provider",
		Mode:     "chat",
		Command:  "messages.create",
	}

	_, err := router.New(cfg, router.DefaultRegistry(), zap.NewNop())
	if err == nil {
		t.Fatal("미등록 provider로 New() 성공함 — 에러 기대")
	}

	var pnrErr *router.ProviderNotRegisteredError
	if !errors.As(err, &pnrErr) {
		t.Errorf("에러 타입: %T, want *router.ProviderNotRegisteredError", err)
	}
	if pnrErr != nil && pnrErr.Name != "nonexistent_provider" {
		t.Errorf("ProviderNotRegisteredError.Name=%q, want %q", pnrErr.Name, "nonexistent_provider")
	}
}

// TestRouter_ForceMode_Primary는 ForceMode가 primary일 때 항상 primary를 반환하는지 검증한다.
// REQ-ROUTER-009.
func TestRouter_ForceMode_Primary(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig()
	cfg.ForceMode = router.ForceModePrimary

	r := newTestRouter(t, cfg)

	// 단순 메시지라도 primary 반환
	req := makeRequest("hi")
	route, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("Route() 에러: %v", err)
	}

	if route.Model != "claude-opus" {
		t.Errorf("ForceMode=primary: Model=%q, want %q", route.Model, "claude-opus")
	}
	if route.RoutingReason != "forced:primary" {
		t.Errorf("ForceMode=primary: RoutingReason=%q, want %q", route.RoutingReason, "forced:primary")
	}
}

// TestRouter_ForceMode_Cheap는 ForceMode가 cheap일 때 cheap route를 반환하는지 검증한다.
func TestRouter_ForceMode_Cheap(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig()
	cfg.ForceMode = router.ForceModeCheap

	r := newTestRouter(t, cfg)

	// 복잡 메시지라도 cheap 반환
	req := makeRequest("debug the entire architecture of this system")
	route, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("Route() 에러: %v", err)
	}

	if route.Model != "claude-haiku" {
		t.Errorf("ForceMode=cheap: Model=%q, want %q", route.Model, "claude-haiku")
	}
	if route.RoutingReason != "forced:cheap" {
		t.Errorf("ForceMode=cheap: RoutingReason=%q, want %q", route.RoutingReason, "forced:cheap")
	}
}

// TestRouter_ForceMode_Cheap_NoCheapDefined_ReturnsErrCheapRouteUndefined는
// ForceMode=cheap인데 CheapRoute가 nil이면 ErrCheapRouteUndefined를 반환하는지 검증한다.
// REQ-ROUTER-009.
func TestRouter_ForceMode_Cheap_NoCheapDefined_ReturnsErrCheapRouteUndefined(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig()
	cfg.ForceMode = router.ForceModeCheap
	cfg.CheapRoute = nil

	r := newTestRouter(t, cfg)

	req := makeRequest("hello")
	_, err := r.Route(context.Background(), req)
	if err == nil {
		t.Fatal("ForceMode=cheap + CheapRoute=nil: 에러 없이 성공 — 에러 기대")
	}
	if !errors.Is(err, router.ErrCheapRouteUndefined) {
		t.Errorf("에러: %v, want ErrCheapRouteUndefined", err)
	}
}

// TestRouter_SimpleGreeting_ReturnsCheap는 단순 메시지가 cheap route로 라우팅되는지 검증한다.
// AC-ROUTER-001.
func TestRouter_SimpleGreeting_ReturnsCheap(t *testing.T) {
	t.Parallel()

	r := newTestRouter(t, newTestConfig())

	req := makeRequest("안녕하세요, 오늘 날씨 어때요?")
	route, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("Route() 에러: %v", err)
	}

	if route.Model != "claude-haiku" {
		t.Errorf("단순 메시지: Model=%q, want %q", route.Model, "claude-haiku")
	}
	if route.RoutingReason != "simple_turn" {
		t.Errorf("단순 메시지: RoutingReason=%q, want %q", route.RoutingReason, "simple_turn")
	}
	if route.Signature == "" {
		t.Error("Signature가 비어 있음")
	}
}

// TestRouter_ComplexKeyword_ReturnsPrimary는 복잡 키워드 포함 메시지가
// primary route로 라우팅되는지 검증한다. AC-ROUTER-002.
func TestRouter_ComplexKeyword_ReturnsPrimary(t *testing.T) {
	t.Parallel()

	r := newTestRouter(t, newTestConfig())

	req := makeRequest("debug this function please")
	route, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("Route() 에러: %v", err)
	}

	if route.Model != "claude-opus" {
		t.Errorf("복잡 키워드: Model=%q, want %q", route.Model, "claude-opus")
	}
	if route.RoutingReason != "complex_task" {
		t.Errorf("복잡 키워드: RoutingReason=%q, want %q", route.RoutingReason, "complex_task")
	}
}

// TestRouter_DecisionHook_Called는 RoutingDecisionHook이 라우팅 결정 후 호출되는지 검증한다.
// REQ-ROUTER-015.
func TestRouter_DecisionHook_Called(t *testing.T) {
	t.Parallel()

	var (
		hookCallCount int
		hookReq       router.RoutingRequest
		hookRoute     *router.Route
		mu            sync.Mutex
	)

	cfg := newTestConfig()
	cfg.RoutingDecisionHooks = []router.RoutingDecisionHook{
		func(req router.RoutingRequest, route *router.Route) {
			mu.Lock()
			defer mu.Unlock()
			hookCallCount++
			hookReq = req
			hookRoute = route
		},
	}

	r := newTestRouter(t, cfg)

	req := makeRequest("hello world")
	route, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("Route() 에러: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if hookCallCount != 1 {
		t.Errorf("hook 호출 횟수=%d, want 1", hookCallCount)
	}
	if hookRoute == nil {
		t.Fatal("hook에 전달된 route가 nil")
	}
	if hookRoute.Signature != route.Signature {
		t.Errorf("hook route.Signature=%q != returned route.Signature=%q",
			hookRoute.Signature, route.Signature)
	}
	if len(hookReq.Messages) != len(req.Messages) {
		t.Errorf("hook req.Messages 길이 불일치")
	}
}

// TestRouter_Stateless_Concurrent_IdenticalOutput은 100개 고루틴이 동시에 Route를 호출해도
// 동일 입력에 대해 동일 출력을 반환하는지 검증한다. REQ-ROUTER-001.
func TestRouter_Stateless_Concurrent_IdenticalOutput(t *testing.T) {
	t.Parallel()

	r := newTestRouter(t, newTestConfig())

	req := makeRequest("hello world")

	// 첫 번째 결과를 기준으로 사용
	expected, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("기준 Route() 에러: %v", err)
	}

	const numGoroutines = 100
	const rounds = 5

	var wg sync.WaitGroup
	errCh := make(chan string, numGoroutines*rounds)

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				route, err := r.Route(context.Background(), req)
				if err != nil {
					errCh <- "Route() 에러: " + err.Error()
					return
				}
				if route.Model != expected.Model {
					errCh <- "Model 불일치: " + route.Model
					return
				}
				if route.Signature != expected.Signature {
					errCh <- "Signature 불일치: " + route.Signature
					return
				}
				if route.RoutingReason != expected.RoutingReason {
					errCh <- "RoutingReason 불일치: " + route.RoutingReason
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for msg := range errCh {
		t.Error("동시성 불일치: " + msg)
	}
}

// TestRouter_InputImmutable은 Route() 호출 후 입력 RoutingRequest가
// 변경되지 않았음을 검증한다. REQ-ROUTER-004.
func TestRouter_InputImmutable(t *testing.T) {
	t.Parallel()

	r := newTestRouter(t, newTestConfig())

	originalMsg := "hello world, please check this"
	req := router.RoutingRequest{
		Messages: []router.Message{
			{Role: "user", Content: originalMsg},
		},
		ConversationLength: 5,
		HasPriorToolUse:    true,
	}

	// Route 호출 전 스냅샷
	originalLen := len(req.Messages)
	originalRole := req.Messages[0].Role
	originalContent := req.Messages[0].Content
	originalConvLen := req.ConversationLength
	originalToolUse := req.HasPriorToolUse

	_, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("Route() 에러: %v", err)
	}

	// Route 호출 후 원본이 변경되지 않아야 함
	if len(req.Messages) != originalLen {
		t.Errorf("Messages 길이 변경됨: %d → %d", originalLen, len(req.Messages))
	}
	if req.Messages[0].Role != originalRole {
		t.Errorf("Messages[0].Role 변경됨: %q → %q", originalRole, req.Messages[0].Role)
	}
	if req.Messages[0].Content != originalContent {
		t.Errorf("Messages[0].Content 변경됨")
	}
	if req.ConversationLength != originalConvLen {
		t.Errorf("ConversationLength 변경됨: %d → %d", originalConvLen, req.ConversationLength)
	}
	if req.HasPriorToolUse != originalToolUse {
		t.Errorf("HasPriorToolUse 변경됨: %v → %v", originalToolUse, req.HasPriorToolUse)
	}
}

// TestRouter_NoUserMessage_FallsBackToPrimary는 메시지 배열에 user 메시지가 없을 때
// primary route를 반환하는지 검증한다.
func TestRouter_NoUserMessage_FallsBackToPrimary(t *testing.T) {
	t.Parallel()

	r := newTestRouter(t, newTestConfig())

	// assistant 메시지만 있는 경우
	req := router.RoutingRequest{
		Messages: []router.Message{
			{Role: "assistant", Content: "I can help you"},
		},
	}

	route, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("Route() 에러: %v", err)
	}

	if route.Provider != "anthropic" {
		t.Errorf("no user message: Provider=%q, want primary provider", route.Provider)
	}
}

// TestRouter_SignatureNoPII는 Signature에 타임스탬프나 사용자 식별자가
// 포함되지 않는지 검증한다. REQ-ROUTER-014.
func TestRouter_SignatureNoPII(t *testing.T) {
	t.Parallel()

	r := newTestRouter(t, newTestConfig())

	req := makeRequest("hello")
	route, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("Route() 에러: %v", err)
	}

	sig := route.Signature
	if sig == "" {
		t.Fatal("Signature가 비어 있음")
	}

	// Signature는 "model|provider|base_url|mode|command|hash" 형태여야 함
	// 타임스탬프 형태(숫자만으로 된 긴 부분)가 없어야 함
	// 최소한 | 구분자 5개 이상 있어야 함
	parts := strings.Split(sig, "|")
	if len(parts) < 6 {
		t.Errorf("Signature 형식 오류: %q (파이프 구분자 < 5개)", sig)
	}
}

// TestRouter_MultipleHooks_AllCalled는 여러 hook이 등록되어 있을 때 모두 호출되는지 검증한다.
func TestRouter_MultipleHooks_AllCalled(t *testing.T) {
	t.Parallel()

	callCounts := make([]int, 3)
	var mu sync.Mutex

	cfg := newTestConfig()
	for i := range 3 {
		i := i
		cfg.RoutingDecisionHooks = append(cfg.RoutingDecisionHooks, func(_ router.RoutingRequest, _ *router.Route) {
			mu.Lock()
			callCounts[i]++
			mu.Unlock()
		})
	}

	r := newTestRouter(t, cfg)
	_, err := r.Route(context.Background(), makeRequest("hello"))
	if err != nil {
		t.Fatalf("Route() 에러: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for i, count := range callCounts {
		if count != 1 {
			t.Errorf("hook[%d] 호출 횟수=%d, want 1", i, count)
		}
	}
}

// TestRouter_CustomClassifier_Used는 CustomClassifier가 설정되면
// 기본 SimpleClassifier 대신 사용되는지 검증한다. REQ-ROUTER-016.
func TestRouter_CustomClassifier_Used(t *testing.T) {
	t.Parallel()

	// 항상 complex로 분류하는 커스텀 classifier
	alwaysComplex := &alwaysComplexClassifier{}

	cfg := newTestConfig()
	cfg.CustomClassifier = alwaysComplex

	r := newTestRouter(t, cfg)

	// 단순 메시지여도 custom classifier가 complex라 하면 primary 반환
	req := makeRequest("hi")
	route, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("Route() 에러: %v", err)
	}

	if route.Model != "claude-opus" {
		t.Errorf("custom classifier (always complex): Model=%q, want %q", route.Model, "claude-opus")
	}
}

// TestRouter_ProviderNotRegisteredError_Message는 ProviderNotRegisteredError의
// Error() 메서드가 올바른 메시지를 반환하는지 검증한다.
func TestRouter_ProviderNotRegisteredError_Message(t *testing.T) {
	t.Parallel()

	err := &router.ProviderNotRegisteredError{Name: "test-provider"}
	msg := err.Error()

	if msg == "" {
		t.Error("Error() 메시지가 비어 있음")
	}
	if !strings.Contains(msg, "test-provider") {
		t.Errorf("Error() 메시지에 provider 이름이 없음: %q", msg)
	}
}

// alwaysComplexClassifier는 항상 complex로 분류하는 테스트용 Classifier이다.
type alwaysComplexClassifier struct{}

func (a *alwaysComplexClassifier) Classify(_ string) router.ClassifierResult {
	return router.ClassifierResult{
		IsSimple: false,
		Reasons:  []string{"always_complex_test"},
	}
}
