package router

// ForceMode는 라우팅 강제 모드를 나타낸다.
type ForceMode string

const (
	// ForceModeAuto는 classifier 결과에 따라 자동 라우팅한다 (기본값).
	ForceModeAuto ForceMode = "auto"
	// ForceModePrimary는 classifier 결과에 관계없이 항상 primary route를 사용한다.
	ForceModePrimary ForceMode = "primary"
	// ForceModeCheap는 classifier 결과에 관계없이 항상 cheap route를 사용한다.
	// CheapRoute가 nil이면 ErrCheapRouteUndefined를 반환한다.
	ForceModeCheap ForceMode = "cheap"
)

// RouteDefinition은 특정 provider/model로 향하는 라우트의 정의이다.
type RouteDefinition struct {
	// Model은 사용할 LLM 모델 이름이다.
	Model string
	// Provider는 provider 이름이다 (ProviderRegistry의 키).
	Provider string
	// BaseURL은 provider의 API base URL이다. 비어 있으면 Registry의 기본값을 사용한다.
	BaseURL string
	// Mode는 호출 모드이다 ("chat" | "completion" | "embed"). 기본값: "chat".
	Mode string
	// Command는 provider별 command이다 (예: "messages.create"). 기본값: "messages.create".
	Command string
	// Args는 모델별 추가 파라미터이다.
	Args map[string]any
}

// RoutingConfig는 Router의 라우팅 설정이다.
type RoutingConfig struct {
	// Primary는 기본 라우트 정의이다. 필수.
	Primary RouteDefinition
	// CheapRoute는 단순 메시지용 저비용 라우트이다. nil이면 항상 primary를 사용한다.
	CheapRoute *RouteDefinition
	// ForceMode는 라우팅 강제 모드이다. 기본값: ForceModeAuto.
	ForceMode ForceMode
	// MaxChars는 char_count 판정 기준값이다. 0이면 기본값(160) 사용.
	MaxChars int
	// MaxWords는 word_count 판정 기준값이다. 0이면 기본값(28) 사용.
	MaxWords int
	// MaxNewlines는 newline_count 판정 기준값이다. 0이면 기본값(2) 사용.
	MaxNewlines int
	// ComplexKeywords는 복잡 키워드 목록이다. nil이면 DefaultComplexKeywords 사용.
	ComplexKeywords []string
	// CustomClassifier는 커스텀 classifier이다. 설정되면 SimpleClassifier 대신 사용된다.
	CustomClassifier Classifier
	// RoutingDecisionHooks는 라우팅 결정 후 호출되는 hook 목록이다 (observational only).
	RoutingDecisionHooks []RoutingDecisionHook
}

// RoutingDecisionHook은 라우팅 결정 후 호출되는 관찰용 hook 함수 타입이다.
// Route를 수정해선 안 된다 (REQ-ROUTER-015).
type RoutingDecisionHook func(req RoutingRequest, route *Route)
