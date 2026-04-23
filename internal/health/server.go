// Package health는 goosed의 최소 HTTP 헬스체크 서버를 제공한다.
// SPEC-GOOSE-CORE-001 REQ-CORE-005, REQ-CORE-006, REQ-CORE-007, REQ-CORE-008
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/modu-ai/goose/internal/core"
	"go.uber.org/zap"
)

const (
	// ResponseTimeout은 헬스체크 응답 제한 시간이다 (REQ-CORE-005: 50ms 이내).
	ResponseTimeout = 45 * time.Millisecond
)

// Server는 /healthz 엔드포인트를 제공하는 HTTP 서버다.
// @MX:ANCHOR: [AUTO] 헬스서버는 상태 조회 팬인 지점
// @MX:REASON: bootstrap, shutdown, 외부 probe가 모두 이 서버를 통해 상태를 확인
type Server struct {
	state   *core.StateHolder
	version string
	logger  *zap.Logger
	srv     *http.Server
	ln      net.Listener
}

// healthResponse는 /healthz 응답 본문이다.
type healthResponse struct {
	Status  string `json:"status"`
	State   string `json:"state,omitempty"`
	Version string `json:"version,omitempty"`
}

// New는 새 헬스서버를 생성한다. Listen은 아직 하지 않는다.
func New(state *core.StateHolder, version string, logger *zap.Logger) *Server {
	s := &Server{
		state:   state,
		version: version,
		logger:  logger,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	s.srv = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	return s
}

// ListenAndServe는 지정 포트에서 listen을 시작한다.
// 포트가 이미 사용 중이면 오류를 반환한다. (REQ-CORE-006)
func (s *Server) ListenAndServe(port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("health-port in use: %d: %w", port, err)
	}
	return s.ServeListener(ln)
}

// ServeListener는 미리 생성된 listener로 서버를 시작한다.
// 테스트에서 포트 race condition을 방지하기 위해 사용한다.
func (s *Server) ServeListener(ln net.Listener) error {
	s.ln = ln
	s.srv.Addr = ln.Addr().String()
	go func() {
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Error("health server error", zap.Error(err))
		}
	}()
	s.logger.Info("health server started", zap.String("addr", s.srv.Addr))
	return nil
}

// Shutdown은 헬스서버를 정지한다.
// listener를 먼저 닫아 새 연결을 차단한다. (REQ-CORE-008)
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// handleHealthz는 GET /healthz 요청을 처리한다.
// draining 상태이면 503, 그 외에는 200을 반환한다. (REQ-CORE-005, REQ-CORE-007)
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	state := s.state.Load()
	w.Header().Set("Content-Type", "application/json")

	var resp healthResponse
	if state == core.StateDraining {
		w.WriteHeader(http.StatusServiceUnavailable)
		resp = healthResponse{Status: "draining"}
	} else {
		w.WriteHeader(http.StatusOK)
		resp = healthResponse{
			Status:  "ok",
			State:   state.String(),
			Version: s.version,
		}
	}

	_ = json.NewEncoder(w).Encode(resp)
}
