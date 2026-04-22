package core

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger는 stderr에 JSON 포맷으로 출력하는 구조화 로거를 생성한다.
// (SPEC-GOOSE-CORE-001 REQ-CORE-001, REQ-CORE-002)
//
// levelStr은 "debug", "info", "warn", "error" 중 하나다.
// service와 version은 모든 로그 라인에 공통 필드로 주입된다.
func NewLogger(levelStr, service, version string) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(levelStr)
	if err != nil {
		level = zapcore.InfoLevel
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(level)
	// REQ-CORE-001: stderr 출력
	cfg.OutputPaths = []string{"stderr"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	// REQ-CORE-001: line-delimited JSON, mandatory fields (ts, level, msg, caller)
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.LevelKey = "level"
	cfg.EncoderConfig.MessageKey = "msg"
	cfg.EncoderConfig.CallerKey = "caller"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

	base, err := cfg.Build(zap.AddCallerSkip(0))
	if err != nil {
		return nil, err
	}

	// REQ-CORE-002: service, version 필드를 모든 라인에 주입
	logger := base.With(
		zap.String("service", service),
		zap.String("version", version),
	)
	return logger, nil
}
