package permission

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// IntegrityChecker는 plugin integrity를 검증하는 인터페이스다.
// REQ-PE-020: 기본 no-op stub, PLUGIN-001이 실 구현을 wire한다.
type IntegrityChecker interface {
	Check(subjectID string, subjectType SubjectType) error
}

// NoopIntegrityChecker는 항상 통과하는 기본 IntegrityChecker다.
type NoopIntegrityChecker struct{}

func (NoopIntegrityChecker) Check(string, SubjectType) error { return nil }

// tripleKey는 (subjectID, capability, scope) 조합의 해시 키다.
// REQ-PE-016: per-triple mutex에 사용한다.
func tripleKey(subjectID string, cap Capability, scope string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%d\x00%s", subjectID, cap, scope)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// tripleInflight는 진행 중인 첫 호출 확인 요청을 추적한다.
// REQ-PE-016: 동일 triple에 대해 Confirmer를 단 한 번만 호출한다.
type tripleInflight struct {
	mu     sync.Mutex
	result *Decision
	err    error
	done   chan struct{}
}

// Manager는 permission 시스템의 최상위 facade다.
// Check / Register / Revoke 등의 공개 API를 제공한다.
//
// @MX:ANCHOR: [AUTO] fan_in >= 3 — SKILLS-001 / MCP-001 / SUBAGENT-001 / PLUGIN-001이 모두 호출
// @MX:REASON: 본 패키지의 유일한 진입점; API surface 변경 시 모든 호출자 영향
// @MX:SPEC: SPEC-GOOSE-PERMISSION-001 REQ-PE-006
type Manager struct {
	store     Store
	confirmer Confirmer
	auditor   Auditor
	blocked   BlockedAlwaysMatcher
	checker   IntegrityChecker
	logger    *zap.Logger

	// registry는 subject ID → Manifest 맵이다.
	// REQ-PE-012: Register 없는 subject에 대해 ErrSubjectNotReady 반환.
	registry map[string]Manifest
	// contractedScopes는 manifest contraction으로 무효화된 triple 키 집합이다.
	// regMu로 보호한다.
	contractedScopes map[string]bool
	regMu            sync.RWMutex

	// inflight는 진행 중인 first-call confirm 요청 맵이다.
	// REQ-PE-016: per-triple mutex.
	inflight   map[string]*tripleInflight
	inflightMu sync.Mutex
}

// New는 Manager를 생성한다.
// confirmer가 nil이면 ErrConfirmerRequired를 반환한다 (R7 미티게이션).
func New(s Store, confirmer Confirmer, auditor Auditor, blocked BlockedAlwaysMatcher, logger *zap.Logger) (*Manager, error) {
	if confirmer == nil {
		return nil, ErrConfirmerRequired
	}
	if auditor == nil {
		auditor = NoopAuditor{}
	}
	if blocked == nil {
		blocked = NoopBlockedAlwaysMatcher{}
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		store:            s,
		confirmer:        confirmer,
		auditor:          auditor,
		blocked:          blocked,
		checker:          NoopIntegrityChecker{},
		logger:           logger,
		registry:         make(map[string]Manifest),
		contractedScopes: make(map[string]bool),
		inflight:         make(map[string]*tripleInflight),
	}, nil
}

// SetIntegrityChecker는 plugin integrity checker를 설정한다.
// REQ-PE-020: PLUGIN-001이 호출한다.
func (m *Manager) SetIntegrityChecker(c IntegrityChecker) {
	m.checker = c
}

// Register는 loader(SKILLS-001 / MCP-001 / SUBAGENT-001)가 호출하여 subject의 manifest를 등록한다.
// manifest contraction(REQ-PE-015) 시 기존 grant 무효화를 수행한다.
func (m *Manager) Register(subjectID string, manifest Manifest) error {
	m.regMu.Lock()
	old, hadOld := m.registry[subjectID]
	m.registry[subjectID] = manifest
	m.regMu.Unlock()

	if hadOld {
		// REQ-PE-015: manifest contraction — 사라진 scope의 grant를 revoke한다.
		m.invalidateContractedScopes(subjectID, old, manifest)
	}
	return nil
}

// invalidateContractedScopes는 이전 manifest에 있던 scope가 새 manifest에서 사라진 경우 해당 grant를 revoke한다.
func (m *Manager) invalidateContractedScopes(subjectID string, old, newMani Manifest) {
	// net 카테고리
	for _, scope := range old.NetHosts {
		if !newMani.Declares(CapNet, scope) {
			m.revokeSpecificGrant(subjectID, CapNet, scope)
		}
	}
	// fs_read
	for _, scope := range old.FSReadPaths {
		if !newMani.Declares(CapFSRead, scope) {
			m.revokeSpecificGrant(subjectID, CapFSRead, scope)
		}
	}
	// fs_write
	for _, scope := range old.FSWritePaths {
		if !newMani.Declares(CapFSWrite, scope) {
			m.revokeSpecificGrant(subjectID, CapFSWrite, scope)
		}
	}
	// exec
	for _, scope := range old.ExecBinaries {
		if !newMani.Declares(CapExec, scope) {
			m.revokeSpecificGrant(subjectID, CapExec, scope)
		}
	}
}

// revokeSpecificGrant는 특정 (subject, capability, scope) triple을 무효화한다.
// manifest contraction(REQ-PE-015) 전용.
// Manager 레벨에서 contractedScopes 집합으로 추적하여 다음 Check에서 ErrUndeclaredCapability를 반환한다.
//
// @MX:WARN: [AUTO] regMu Lock 필수 — contractedScopes 동시 변이 방지
// @MX:REASON: manifest contraction 무효화 추적; regMu 없이 쓰면 data race
func (m *Manager) revokeSpecificGrant(subjectID string, cap Capability, scope string) {
	key := tripleKey(subjectID, cap, scope)
	m.regMu.Lock()
	m.contractedScopes[key] = true
	m.regMu.Unlock()
}

// Check는 모든 capability 진입점이다.
// §6.3 결정 흐름 의사코드를 구현한다.
//
// @MX:ANCHOR: [AUTO] fan_in >= 4 — SKILLS-001 / MCP-001 / SUBAGENT-001 / PLUGIN-001
// @MX:REASON: permission 시스템의 단일 진입점; 이 함수의 시그니처 변경은 전체 호출자 영향
// @MX:SPEC: SPEC-GOOSE-PERMISSION-001 REQ-PE-006 REQ-PE-007 REQ-PE-009 REQ-PE-012 REQ-PE-015 REQ-PE-016
func (m *Manager) Check(ctx context.Context, req PermissionRequest) (Decision, error) {
	now := time.Now()
	if req.RequestedAt.IsZero() {
		req.RequestedAt = now
	}

	// 1. Subject 등록 여부 (REQ-PE-012)
	m.regMu.RLock()
	manifest, ok := m.registry[req.SubjectID]
	m.regMu.RUnlock()
	if !ok {
		m.audit(PermissionEvent{
			Type:       "grant_denied",
			SubjectID:  req.SubjectID,
			Capability: req.Capability,
			Scope:      req.Scope,
			Reason:     "not_ready",
			Timestamp:  now,
		})
		return Decision{}, ErrSubjectNotReady{SubjectID: req.SubjectID}
	}

	// 2. Plugin integrity check (REQ-PE-020)
	if req.SubjectType == SubjectPlugin {
		if err := m.checker.Check(req.SubjectID, req.SubjectType); err != nil {
			m.audit(PermissionEvent{
				Type:       "grant_denied",
				SubjectID:  req.SubjectID,
				Capability: req.Capability,
				Scope:      req.Scope,
				Reason:     "integrity_check_failed",
				Timestamp:  now,
			})
			return Decision{}, err
		}
	}

	// 3. blocked_always 교차 차단 (REQ-PE-009)
	if m.blocked.Matches(req.Capability, req.Scope) {
		m.audit(PermissionEvent{
			Type:       "grant_denied",
			SubjectID:  req.SubjectID,
			Capability: req.Capability,
			Scope:      req.Scope,
			Reason:     "blocked_always",
			Timestamp:  now,
		})
		return Decision{}, ErrBlockedByPolicy{Capability: req.Capability, Scope: req.Scope}
	}

	// 4. declared 검사 (REQ-PE-001, REQ-PE-015)
	m.regMu.RLock()
	contractedKey := tripleKey(req.SubjectID, req.Capability, req.Scope)
	isContracted := m.contractedScopes[contractedKey]
	m.regMu.RUnlock()

	if isContracted || !manifest.Declares(req.Capability, req.Scope) {
		m.audit(PermissionEvent{
			Type:       "grant_denied",
			SubjectID:  req.SubjectID,
			Capability: req.Capability,
			Scope:      req.Scope,
			Reason:     "undeclared",
			Timestamp:  now,
		})
		return Decision{}, ErrUndeclaredCapability{Capability: req.Capability, Scope: req.Scope}
	}

	// 5. per-triple lock (REQ-PE-016)
	return m.checkWithTripleLock(ctx, req, now)
}

// checkWithTripleLock은 per-triple mutex를 사용해 동시 first-call을 직렬화한다.
//
// @MX:WARN: [AUTO] goroutine 동기화 — inflight map 접근에 inflightMu 필수
// @MX:REASON: 동일 triple에 대해 Confirmer.Ask를 단 한 번만 호출 (REQ-PE-016)
func (m *Manager) checkWithTripleLock(ctx context.Context, req PermissionRequest, now time.Time) (Decision, error) {
	key := tripleKey(req.SubjectID, req.Capability, req.Scope)

	// 이미 진행 중인 confirm이 있으면 결과를 기다린다.
	m.inflightMu.Lock()
	if fl, exists := m.inflight[key]; exists {
		m.inflightMu.Unlock()
		// 진행 중인 confirm 완료 대기
		select {
		case <-fl.done:
		case <-ctx.Done():
			return Decision{}, ctx.Err()
		}
		if fl.err != nil {
			return Decision{}, fl.err
		}
		// inflight 리더의 결과를 재사용한 waiter 도 grant_reused 로 기록 (REQ-PE-016).
		// 동시 first-call N건 중 1건만 confirmer를 호출하고 나머지는 grant_reused 로 audit.
		if fl.result != nil && fl.result.Allow {
			m.audit(PermissionEvent{
				Type:       "grant_reused",
				SubjectID:  req.SubjectID,
				Capability: req.Capability,
				Scope:      req.Scope,
				Timestamp:  now,
			})
		}
		return *fl.result, nil
	}

	// 첫 번째 waiter — inflight 등록
	fl := &tripleInflight{done: make(chan struct{})}
	m.inflight[key] = fl
	m.inflightMu.Unlock()

	// 완료 시 inflight 제거 및 결과 broadcast
	defer func() {
		m.inflightMu.Lock()
		delete(m.inflight, key)
		m.inflightMu.Unlock()
		close(fl.done)
	}()

	// Store lookup (REQ-PE-007)
	if grant, hit := m.store.Lookup(req.SubjectID, req.Capability, req.Scope); hit {
		dec := Decision{Allow: true, Choice: DecisionAlwaysAllow}
		if grant.ExpiresAt != nil {
			dec.ExpiresAt = grant.ExpiresAt
		}
		m.audit(PermissionEvent{
			Type:       "grant_reused",
			SubjectID:  req.SubjectID,
			Capability: req.Capability,
			Scope:      req.Scope,
			Timestamp:  now,
		})
		fl.result = &dec
		return dec, nil
	}

	// Sub-agent inheritance fallback (REQ-PE-011)
	if req.InheritGrants && req.ParentSubjectID != "" {
		if parentGrant, hit := m.store.Lookup(req.ParentSubjectID, req.Capability, req.Scope); hit {
			dec := Decision{Allow: true, Choice: DecisionAlwaysAllow}
			if parentGrant.ExpiresAt != nil {
				dec.ExpiresAt = parentGrant.ExpiresAt
			}
			m.audit(PermissionEvent{
				Type:          "grant_reused",
				SubjectID:     req.SubjectID,
				Capability:    req.Capability,
				Scope:         req.Scope,
				InheritedFrom: req.ParentSubjectID,
				Timestamp:     now,
			})
			fl.result = &dec
			return dec, nil
		}
	}

	// First-call confirm (REQ-PE-006)
	confirmerDecision, err := m.confirmer.Ask(ctx, req)
	if err != nil {
		fl.err = err
		return Decision{Allow: false, Reason: err.Error()}, err
	}

	var result Decision
	switch confirmerDecision.Choice {
	case DecisionAlwaysAllow:
		g := Grant{
			ID:          uuid.New().String(),
			SubjectID:   req.SubjectID,
			SubjectType: req.SubjectType,
			Capability:  req.Capability,
			Scope:       req.Scope,
			GrantedAt:   now,
			GrantedBy:   "user",
			ExpiresAt:   confirmerDecision.ExpiresAt,
		}
		if saveErr := m.store.Save(g); saveErr != nil {
			m.logger.Warn("failed to persist grant",
				zap.Error(saveErr),
				zap.String("subject_id", req.SubjectID),
			)
		}
		m.audit(PermissionEvent{
			Type:       "grant_created",
			SubjectID:  req.SubjectID,
			Capability: req.Capability,
			Scope:      req.Scope,
			Timestamp:  now,
		})
		result = Decision{Allow: true, Choice: DecisionAlwaysAllow, ExpiresAt: confirmerDecision.ExpiresAt}

	case DecisionOnceOnly:
		m.audit(PermissionEvent{
			Type:       "grant_created",
			SubjectID:  req.SubjectID,
			Capability: req.Capability,
			Scope:      req.Scope,
			Reason:     "once_only",
			Timestamp:  now,
		})
		result = Decision{Allow: true, Choice: DecisionOnceOnly}

	case DecisionDeny:
		m.audit(PermissionEvent{
			Type:       "grant_denied",
			SubjectID:  req.SubjectID,
			Capability: req.Capability,
			Scope:      req.Scope,
			Reason:     "user_denied",
			Timestamp:  now,
		})
		result = Decision{Allow: false, Choice: DecisionDeny, Reason: "user denied"}
	}

	fl.result = &result
	return result, nil
}

// Revoke는 subjectID의 모든 grant를 revoke한다.
func (m *Manager) Revoke(subjectID string) (int, error) {
	return m.store.Revoke(subjectID)
}

// List는 필터 조건에 맞는 grant 목록을 반환한다.
func (m *Manager) List(filter Filter) ([]Grant, error) {
	return m.store.List(filter)
}

// audit은 Auditor.Record를 호출한다. 실패해도 결정에 영향 없다 (REQ-PE-005).
func (m *Manager) audit(event PermissionEvent) {
	if err := m.auditor.Record(event); err != nil {
		m.logger.Warn("auditor failed", zap.Error(err), zap.String("event_type", event.Type))
	}
}
