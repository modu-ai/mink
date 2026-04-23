package router

import (
	"context"

	"go.uber.org/zap"
)

// Message는 대화에서 하나의 메시지를 나타낸다.
type Message struct {
	// Role은 메시지 역할이다 ("user" | "assistant" | "system").
	Role string
	// Content는 메시지 내용이다.
	Content string
}

// RoutingRequest는 라우팅 결정을 위한 입력이다.
// Router는 이 구조체를 읽기만 하고 변경하지 않는다 (REQ-ROUTER-004).
type RoutingRequest struct {
	// Messages는 대화 메시지 배열이다. Router는 마지막 user 메시지를 분류 대상으로 사용한다.
	Messages []Message
	// ConversationLength는 대화 턴 수이다 (observability용).
	ConversationLength int
	// HasPriorToolUse는 이전 턴에서 tool call이 있었는지 여부이다.
	HasPriorToolUse bool
	// Meta는 추가 컨텍스트 정보이다.
	Meta map[string]any
}

// Route는 라우팅 결정 결과이다.
type Route struct {
	// Model은 사용할 LLM 모델 이름이다.
	Model string
	// Provider는 provider 이름이다.
	Provider string
	// BaseURL은 provider API base URL이다.
	BaseURL string
	// Mode는 호출 모드이다.
	Mode string
	// Command는 provider별 command이다.
	Command string
	// Args는 모델별 추가 파라미터이다.
	Args map[string]any
	// RoutingReason은 라우팅 결정 근거이다.
	// "simple_turn" | "complex_task" | "primary_only_configured" |
	// "forced:primary" | "forced:cheap" | "no_user_message"
	RoutingReason string
	// Signature는 라우팅 결정의 canonical fingerprint이다 (REQ-ROUTER-002).
	// PII, 시간, credential을 포함하지 않는다 (REQ-ROUTER-014).
	Signature string
	// ClassifierReasons는 classifier가 제공한 근거 목록이다 (observability).
	ClassifierReasons []string
}

// Router는 stateless 라우팅 결정 엔진이다.
//
// 내부 상태를 변경하지 않으므로 여러 고루틴에서 동시에 Route()를 호출해도 안전하다.
// mutex를 사용하지 않으며, 모든 결정은 입력과 생성 시점의 설정에만 의존한다.
// @MX:ANCHOR: [AUTO] Router 구조체 — 라우팅 결정 레이어의 핵심 진입점
// @MX:REASON: Route() 메서드는 모든 LLM 요청 경로에서 호출됨 (fan_in >= 3 예상)
type Router struct {
	cfg      RoutingConfig
	registry *ProviderRegistry
	cls      Classifier
	logger   *zap.Logger
}

// New는 RoutingConfig, ProviderRegistry, zap.Logger로 Router를 생성한다.
//
// primary provider가 registry에 등록되어 있지 않으면 *ProviderNotRegisteredError를 반환한다.
// @MX:ANCHOR: [AUTO] Router 생성 진입점
// @MX:REASON: 모든 Router 사용처에서 이 함수를 통해 생성 (fan_in >= 3)
func New(cfg RoutingConfig, registry *ProviderRegistry, logger *zap.Logger) (*Router, error) {
	// primary provider 등록 여부 검증 (REQ-ROUTER-011)
	if _, ok := registry.Get(cfg.Primary.Provider); !ok {
		return nil, &ProviderNotRegisteredError{Name: cfg.Primary.Provider}
	}

	// classifier 선택: CustomClassifier가 설정되면 우선 사용 (REQ-ROUTER-016)
	var cls Classifier
	if cfg.CustomClassifier != nil {
		cls = cfg.CustomClassifier
	} else {
		clsCfg := ClassifierConfig{
			MaxChars:        cfg.MaxChars,
			MaxWords:        cfg.MaxWords,
			MaxNewlines:     cfg.MaxNewlines,
			ComplexKeywords: cfg.ComplexKeywords,
		}
		cls = NewSimpleClassifier(clsCfg)
	}

	return &Router{
		cfg:      cfg,
		registry: registry,
		cls:      cls,
		logger:   logger,
	}, nil
}

// Route는 RoutingRequest를 분석하여 라우팅 결정을 반환한다.
//
// 네트워크 I/O와 credential 접근을 수행하지 않는다 (REQ-ROUTER-012).
// 입력 req를 변경하지 않는다 (REQ-ROUTER-004).
// 여러 고루틴에서 동시 호출해도 안전하다 (REQ-ROUTER-001).
func (r *Router) Route(ctx context.Context, req RoutingRequest) (*Route, error) {
	// ForceMode 처리 (REQ-ROUTER-009)
	switch r.cfg.ForceMode {
	case ForceModePrimary:
		route := r.buildRoute(r.cfg.Primary, "forced:primary", nil)
		r.callHooks(req, route)
		r.logDecision(route)
		return route, nil

	case ForceModeCheap:
		if r.cfg.CheapRoute == nil {
			return nil, ErrCheapRouteUndefined
		}
		route := r.buildRoute(*r.cfg.CheapRoute, "forced:cheap", nil)
		r.callHooks(req, route)
		r.logDecision(route)
		return route, nil
	}

	// 마지막 user 메시지 추출
	lastUserMsg := findLastUserMessage(req.Messages)
	if lastUserMsg == "" {
		route := r.buildRoute(r.cfg.Primary, "no_user_message", nil)
		r.callHooks(req, route)
		r.logDecision(route)
		return route, nil
	}

	// classifier 실행
	result := r.cls.Classify(lastUserMsg)

	// 라우팅 결정
	var route *Route
	if result.IsSimple && r.cfg.CheapRoute != nil {
		// 단순 메시지 + cheap route 존재 → cheap
		route = r.buildRoute(*r.cfg.CheapRoute, "simple_turn", result.Reasons)
	} else if r.cfg.CheapRoute == nil {
		// cheap route 미정의 → primary (REQ-ROUTER-010)
		route = r.buildRoute(r.cfg.Primary, "primary_only_configured", result.Reasons)
	} else {
		// 복잡 메시지 → primary
		route = r.buildRoute(r.cfg.Primary, "complex_task", result.Reasons)
	}

	r.callHooks(req, route)
	r.logDecision(route)
	return route, nil
}

// buildRoute는 RouteDefinition으로 Route를 생성하고 Signature를 계산한다.
// registry의 기본값을 사용하여 비어 있는 필드를 채운다.
func (r *Router) buildRoute(def RouteDefinition, reason string, classifierReasons []string) *Route {
	// registry에서 base URL 등 기본값 조회
	baseURL := def.BaseURL
	if baseURL == "" {
		if meta, ok := r.registry.Get(def.Provider); ok {
			baseURL = meta.DefaultBaseURL
		}
	}

	mode := def.Mode
	if mode == "" {
		mode = "chat"
	}
	command := def.Command
	if command == "" {
		command = "messages.create"
	}

	route := &Route{
		Model:             def.Model,
		Provider:          def.Provider,
		BaseURL:           baseURL,
		Mode:              mode,
		Command:           command,
		Args:              def.Args,
		RoutingReason:     reason,
		ClassifierReasons: classifierReasons,
	}
	route.Signature = makeSignature(route)
	return route
}

// callHooks는 등록된 RoutingDecisionHooks를 순서대로 호출한다 (observational only).
func (r *Router) callHooks(req RoutingRequest, route *Route) {
	for _, hook := range r.cfg.RoutingDecisionHooks {
		hook(req, route)
	}
}

// logDecision은 라우팅 결정을 zap logger로 기록한다.
func (r *Router) logDecision(route *Route) {
	r.logger.Debug("routing decision",
		zap.String("provider", route.Provider),
		zap.String("model", route.Model),
		zap.String("reason", route.RoutingReason),
		zap.String("signature_prefix", route.Signature[:min(len(route.Signature), 12)]),
	)
}

// findLastUserMessage는 메시지 배열에서 마지막 user role 메시지의 Content를 반환한다.
// user 메시지가 없으면 빈 문자열을 반환한다.
func findLastUserMessage(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

// min은 두 정수 중 작은 값을 반환한다.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
