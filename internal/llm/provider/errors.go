package provider

import "fmt"

// ErrProviderNotFound는 레지스트리에 등록되지 않은 provider를 요청했을 때 반환된다.
type ErrProviderNotFound struct {
	// Name은 찾지 못한 provider 이름이다.
	Name string
}

func (e ErrProviderNotFound) Error() string {
	return fmt.Sprintf("provider: %q not found in registry", e.Name)
}

// ErrCapabilityUnsupported는 provider가 지원하지 않는 기능을 요청했을 때 반환된다.
// REQ-ADAPTER-017: vision unsupported 등
type ErrCapabilityUnsupported struct {
	// Feature는 지원되지 않는 기능 이름이다 (예: "vision").
	Feature string
	// ProviderName은 해당 기능을 지원하지 않는 provider 이름이다.
	ProviderName string
}

func (e ErrCapabilityUnsupported) Error() string {
	return fmt.Sprintf("provider: %q does not support capability %q", e.ProviderName, e.Feature)
}
