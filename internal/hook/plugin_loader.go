package hook

// PluginHookLoader 인터페이스는 types.go에 정의되어 있다.
// 이 파일은 PLUGIN-001을 위한 NoopPluginLoader stub을 제공한다.

// NoopPluginLoader는 항상 IsLoading() == false를 반환하는 stub 구현이다.
// 테스트 및 PLUGIN-001 구현 전 placeholder로 사용된다.
type NoopPluginLoader struct{}

// IsLoading은 항상 false를 반환한다.
func (n *NoopPluginLoader) IsLoading() bool { return false }

// Load는 아무것도 수행하지 않는다.
func (n *NoopPluginLoader) Load(manifest any, registry *HookRegistry) error { return nil }

// LoadingPluginLoader는 IsLoading() == true를 반환하는 테스트용 stub이다.
// AC-HK-014: Register 거부 테스트에 사용된다.
type LoadingPluginLoader struct{}

// IsLoading은 항상 true를 반환한다.
func (l *LoadingPluginLoader) IsLoading() bool { return true }

// Load는 아무것도 수행하지 않는다.
func (l *LoadingPluginLoader) Load(manifest any, registry *HookRegistry) error { return nil }
