// Package router는 LLM 요청의 라우팅 결정 레이어를 제공한다.
// Hermes Agent의 model_router.py를 Go로 포팅한 것으로,
// 사용자 메시지의 단순성을 판정하여 primary/cheap 모델 간 전환을 결정한다.
package router

import (
	"errors"
	"fmt"
)

// ErrCheapRouteUndefined는 ForceMode=cheap이지만 CheapRoute가 설정되지 않았을 때 반환된다.
var ErrCheapRouteUndefined = errors.New("router: cheap route undefined")

// ProviderNotRegisteredError는 ProviderRegistry에 등록되지 않은 provider를
// 사용하려 할 때 반환되는 에러이다.
type ProviderNotRegisteredError struct {
	Name string
}

// Error는 에러 메시지를 반환한다.
func (e *ProviderNotRegisteredError) Error() string {
	return fmt.Sprintf("router: provider %q is not registered", e.Name)
}
