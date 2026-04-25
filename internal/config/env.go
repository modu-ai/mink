// Package config의 환경변수 오버레이 컴포넌트.
// SPEC-GOOSE-CONFIG-001 §6.2 환경변수 매핑 표
package config

import (
	"strconv"

	"go.uber.org/zap"
)

// envOverlay는 환경변수를 읽어 cfg 필드를 오버라이드한다.
// REQ-CFG-006: env vars > all file-based sources
//
// envLookup은 환경변수 조회 함수다. 테스트에서는 맵 기반 함수를 주입하여
// process-wide 환경변수 오염 없이 병렬 테스트를 수행할 수 있다.
//
// @MX:NOTE: [AUTO] 환경변수 오버레이 — 10개 ENV 매핑 처리
// @MX:SPEC: SPEC-GOOSE-CONFIG-001 §6.2
func envOverlay(cfg *Config, sources sourceMap, logger *zap.Logger, envLookup func(string) string) error {
	// GOOSE_LOG_LEVEL → log.level
	if v := envLookup("GOOSE_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
		sources.set("log.level", SourceEnv)
	}

	// GOOSE_HEALTH_PORT → transport.health_port
	if v := envLookup("GOOSE_HEALTH_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			// AC-CFG-010b: 파싱 실패 시 WARN 로그 + 기존 값 유지
			logger.Warn("GOOSE_HEALTH_PORT 정수 파싱 실패, 기존 값 유지",
				zap.String("value", v),
				zap.Error(err),
			)
		} else {
			cfg.Transport.HealthPort = port
			sources.set("transport.health_port", SourceEnv)
		}
	}

	// GOOSE_GRPC_PORT → transport.grpc_port
	if v := envLookup("GOOSE_GRPC_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			// AC-CFG-010b: 파싱 실패 시 WARN 로그 + 기존 값 유지
			logger.Warn("GOOSE_GRPC_PORT 정수 파싱 실패, 기존 값 유지",
				zap.String("value", v),
				zap.Error(err),
			)
		} else {
			cfg.Transport.GRPCPort = port
			sources.set("transport.grpc_port", SourceEnv)
		}
	}

	// GOOSE_LOCALE → ui.locale
	if v := envLookup("GOOSE_LOCALE"); v != "" {
		cfg.UI.Locale = v
		sources.set("ui.locale", SourceEnv)
	}

	// OLLAMA_HOST → llm.providers.ollama.host
	if v := envLookup("OLLAMA_HOST"); v != "" {
		ensureProvider(cfg, "ollama")
		p := cfg.LLM.Providers["ollama"]
		p.Host = v
		cfg.LLM.Providers["ollama"] = p
		sources.set("llm.providers.ollama.host", SourceEnv)
	}

	// OPENAI_API_KEY → llm.providers.openai.api_key
	if v := envLookup("OPENAI_API_KEY"); v != "" {
		ensureProvider(cfg, "openai")
		p := cfg.LLM.Providers["openai"]
		p.APIKey = v
		cfg.LLM.Providers["openai"] = p
		sources.set("llm.providers.openai.api_key", SourceEnv)
	}

	// ANTHROPIC_API_KEY → llm.providers.anthropic.api_key
	if v := envLookup("ANTHROPIC_API_KEY"); v != "" {
		ensureProvider(cfg, "anthropic")
		p := cfg.LLM.Providers["anthropic"]
		p.APIKey = v
		cfg.LLM.Providers["anthropic"] = p
		sources.set("llm.providers.anthropic.api_key", SourceEnv)
	}

	// GOOSE_LEARNING_ENABLED → learning.enabled
	if v := envLookup("GOOSE_LEARNING_ENABLED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			logger.Warn("GOOSE_LEARNING_ENABLED bool 파싱 실패, 기존 값 유지",
				zap.String("value", v),
				zap.Error(err),
			)
		} else {
			cfg.Learning.Enabled = b
			sources.set("learning.enabled", SourceEnv)
		}
	}

	return nil
}

// ensureProvider는 Providers 맵에 지정된 프로바이더 항목이 없으면 빈 값으로 초기화한다.
func ensureProvider(cfg *Config, name string) {
	if cfg.LLM.Providers == nil {
		cfg.LLM.Providers = make(map[string]ProviderConfig)
	}
	if _, ok := cfg.LLM.Providers[name]; !ok {
		cfg.LLM.Providers[name] = ProviderConfig{}
	}
}
