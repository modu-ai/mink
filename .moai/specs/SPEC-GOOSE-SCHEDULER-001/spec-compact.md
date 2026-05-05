---
id: SPEC-GOOSE-SCHEDULER-001
artifact: spec-compact
version: 0.2.0
spec_version: 0.2.0
status: audit-ready
created_at: 2026-04-25
updated_at: 2026-05-05
author: manager-spec
priority: critical
phase: 7
size: 중(M)
lifecycle: spec-anchored
labels: [scheduler, ritual, hook, phase-7, daily-companion, compact]
---

# Spec Compact — SPEC-GOOSE-SCHEDULER-001

> 1-page 요약. 상세는 `spec.md` (v0.2.0), `plan.md`, `acceptance.md`, `research.md` 참조.

## 한 줄 정의

GOOSE v6.0 **Daily Companion Edition** Phase 7 Layer 3 **Daily Rituals** 구동을 위한 **proactive scheduler**. cron-like, timezone-/holiday-aware, user-pattern learning 기반으로 5개 핵심 시간 이벤트를 HOOK-001 dispatcher 로 emit.

## 5 핵심 이벤트

`MorningBriefingTime` · `PostBreakfastTime` · `PostLunchTime` · `PostDinnerTime` · `EveningCheckInTime`

## IN / OUT 한 줄

- **IN**: `internal/ritual/scheduler/`, RitualScheduler struct, 5 이벤트 emit, 사용자 시간표 학습 (INSIGHTS 소비), Timezone detector, Holiday calendar (한국 + custom), Backoff manager, HOOK-001 연동, MEMORY-001 영속.
- **OUT**: 리추얼 본체 실행 (BRIEFING/HEALTH/JOURNAL/RITUAL), TTS 음성, Calendar read 구현, Sleep 트래킹, 외부 푸시 알림, iOS/Android 백그라운드 실행, 가족 모드, 1분 미만 정밀 스케줄, custom 사용자 정의 이벤트, Tool registration, CLI 관리 명령.

## 필수 의존

- **HARD 선행**: HOOK-001 (5 신규 HookEvent 등록), CORE-001 (zap, ctx), CONFIG-001 (config 로드), MEMORY-001 (영속).
- **MEDIUM 선행**: INSIGHTS-001 (PatternLearner), QUERY-001 (TurnCounter).
- **외부**: `robfig/cron/v3`, `rickar/cal/v2`, `jonboulle/clockwork` (test-only).

## 22 REQ → 20 AC 매핑 (요약)

| 카테고리 | REQ | AC |
|--------|----|----|
| Ubiquitous (4) | 001~004 | 001, 007, 010 |
| Event-Driven (5) | 005~009 | 002, 003, 004, 009, 011 |
| State-Driven (3) | 010~012 | 006, 012 |
| Unwanted (4) | 013~016 | 005, 008, 014, 015 |
| Optional (4) | 017~020 | 004, 016, 017, 018 |
| Additional v0.2 (2) | 021, 022 | 019, 020 |
| (backoff 분기) | — | 003, 013 (REQ-005/011/014 다중 매핑) |

전체 매핑은 `acceptance.md` §4 Coverage Map 참조.

## 4 Milestone (P1~P4) 요약

| Milestone | 주요 산출물 | AC 누적 | Exit Criteria |
|----------|----------|--------|--------------|
| **P1** Cron + Persist | scheduler/cron/events/persist/config | 5/20 | AC-001/002/007/011/012, coverage ≥80%, HOOK-001 5 이벤트 등록 |
| **P2** TZ + Holiday | timezone/holiday + Korean provider | 8/20 | +AC-004/009/016, 향후 3년 goldenfile, custom YAML override |
| **P3** Backoff + Decoupling + Quiet | backoff/dispatcher worker/quiet hours | 13/20 | +AC-003/005/013/014/019, race-clean |
| **P4** Pattern + Replay + Logs + FastForward | pattern/proposal/replay/log + test-only FastForward | 20/20 | +AC-006/008/010/015/017/018/020, coverage ≥85%, build-tag gating 검증 |

## 핵심 안전 장치

- **Quiet Hours HARD floor** `[23:00, 06:00]` (REQ-014), `allow_nighttime=true` 만 override.
- **Backoff Cap**: max_defer_count=3, 4회차 강제 emit + DelayHint (REQ-021).
- **Missed Event Replay**: ≤1h 지체 → 1회 replay + IsReplay=true; 초과 → skip (REQ-022).
- **3-tuple SuppressionKey**: `{event}:{userLocalDate}:{TZ}` (REQ-013, TZ 변경 시 새 key).
- **PatternLearner ±2h cap + 3일 연속 commit** (REQ-016).
- **Cron-Dispatcher Decoupling**: cron goroutine → buffered eventCh(32) → worker (REQ-015).
- **FastForward Build-Tag Gating**: `//go:build test_only`, production binary 에서 심볼 부재 (REQ-020).

## 진행 상태

- spec.md v0.2.0 (plan-auditor iter-1 수정 완료, MP-2/MP-3/D6/D7+D21/D11/D9+D10/D12/D16/D19 반영)
- research.md (cron 라이브러리, 한국 공휴일 정확도, PatternLearner 알고리즘, Backoff heuristic, Quiet hours 정책, SuppressionKey 정규형, 테스트 전략)
- plan.md (4 milestone, 외부 의존 점검 항목, TDD 진입 순서 20개)
- acceptance.md (20 AC 결정론·이진 PASS/FAIL, Coverage Map, Edge Cases, DoD, TRUST 5)
- spec-compact.md (본 문서)

**다음 단계**: plan-auditor 라운드 1 → PASS 시 status `draft → audit-ready` 전환.

---

**End of Spec Compact — SPEC-GOOSE-SCHEDULER-001**
