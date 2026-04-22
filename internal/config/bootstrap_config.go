// Package config는 goosed 부트스트랩에 필요한 최소 설정 로더를 제공한다.
// SPEC-GOOSE-CORE-001 전용. 계층형 설정은 SPEC-GOOSE-CONFIG-001에서 확장된다.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultHealthPort는 헬스체크 서버의 기본 포트다.
	DefaultHealthPort = 17890
	// DefaultLogLevel은 기본 로그 레벨이다.
	DefaultLogLevel = "info"
	// DefaultShutdownTimeout은 최대 graceful shutdown 대기 시간(초)이다.
	DefaultShutdownTimeout = 30
)

// BootstrapConfig는 goosed 부트스트랩에 필요한 최소 설정이다.
type BootstrapConfig struct {
	// HealthPort는 헬스서버가 바인딩할 포트다. 기본값: 17890.
	HealthPort int `yaml:"health_port"`
	// LogLevel은 로그 레벨이다 (debug|info|warn|error). 기본값: info.
	LogLevel string `yaml:"log_level"`
	// ShutdownTimeout은 graceful shutdown 최대 대기 시간(초)이다. 기본값: 30.
	ShutdownTimeout int `yaml:"shutdown_timeout"`
}

// Defaults는 기본값이 채워진 BootstrapConfig를 반환한다.
func Defaults() *BootstrapConfig {
	return &BootstrapConfig{
		HealthPort:      DefaultHealthPort,
		LogLevel:        DefaultLogLevel,
		ShutdownTimeout: DefaultShutdownTimeout,
	}
}

// Load는 GOOSE_HOME 환경변수(기본값: ~/.goose)에서 config.yaml을 읽어 반환한다.
// 파일이 없으면 기본값으로 fallback한다.
// YAML 파싱에 실패하면 오류를 반환한다. (REQ-CORE-010)
func Load() (*BootstrapConfig, error) {
	gooseHome := os.Getenv("GOOSE_HOME")
	if gooseHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("홈 디렉토리 탐색 실패: %w", err)
		}
		gooseHome = filepath.Join(home, ".goose")
	}

	cfgPath := filepath.Join(gooseHome, "config.yaml")
	return LoadFromFile(cfgPath)
}

// LoadFromFile은 지정된 경로에서 YAML 설정 파일을 읽는다.
// 파일이 없으면 기본값 fallback. 파싱 실패 시 오류를 반환한다.
func LoadFromFile(path string) (*BootstrapConfig, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("설정 파일 읽기 실패 (%s): %w", path, err)
	}

	if err == nil {
		// 파일이 존재하면 파싱 시도
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config parse error (%s): %w", path, err)
		}
	}
	// 파일이 없으면 기본값 유지 (fallback)

	// 파싱 후 0값 필드는 기본값으로 보정
	if cfg.HealthPort == 0 {
		cfg.HealthPort = DefaultHealthPort
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = DefaultLogLevel
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = DefaultShutdownTimeout
	}

	// GOOSE_HEALTH_PORT 환경변수 오버라이드 (REQ-CORE-012)
	// 파일 존재 여부와 무관하게 항상 ENV가 우선
	if portStr := os.Getenv("GOOSE_HEALTH_PORT"); portStr != "" {
		var port int
		if _, err := fmt.Sscanf(portStr, "%d", &port); err == nil && port > 0 {
			cfg.HealthPort = port
		}
	}

	return cfg, nil
}
