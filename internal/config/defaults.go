// Package config의 기본값 정의.
// SPEC-GOOSE-CONFIG-001 §6.2 환경변수 매핑 표 기준 기본값
package config

// defaultConfig는 기본값이 채워진 Config를 반환한다.
// REQ-CFG-004: defaults → user → project → env 로딩 우선순위
func defaultConfig() *Config {
	return &Config{
		Log: LogConfig{
			Level: "info",
		},
		Transport: TransportConfig{
			HealthPort: 0, // Disabled: use gRPC health checking on GRPCPort
			GRPCPort:   9005,
		},
		LLM: LLMConfig{
			DefaultProvider: "",
			Providers: map[string]ProviderConfig{
				"ollama": {
					Host:   "http://localhost:11434",
					APIKey: "",
				},
				"openai": {
					Host:   "",
					APIKey: "",
				},
				"anthropic": {
					Host:   "",
					APIKey: "",
				},
			},
		},
		Learning: LearningConfig{
			Enabled: false,
		},
		UI: UIConfig{
			Locale: "en",
		},
		Audit: AuditConfig{
			Enabled:   true,
			MaxSizeMB: 100,
			GlobalDir: "~/.goose/logs",
			LocalDir:  "./.goose/logs",
		},
		FSAccess: FSAccessConfig{
			Enabled:        true,
			PolicyPath:     "./.goose/config/security.yaml",
			ReloadInterval: "5s",
		},
	}
}

const (
	// DefaultHealthPort는 헬스체크 서버 기본 포트다 (0 = disabled).
	DefaultHealthPort = 0
	// DefaultGRPCPort는 gRPC 서버 기본 포트다.
	DefaultGRPCPort = 9005
	// DefaultLogLevel은 기본 로그 레벨이다.
	DefaultLogLevel = "info"
	// DefaultLocale은 기본 UI 언어 코드다.
	DefaultLocale = "en"
)
