# CMDCTX-* 의존성 분석 리포트

**작성일**: 2026-04-30
**대상**: 8 SPEC (CMDCTX-* 7건 + CMDLOOP-WIRE-001 1건)
**작성자**: Explore agent (자동 분석) + 메타 정리 SPEC-META-CLEANUP-2026-04-30
**현황**: 진행 상태 표기와 실제 구현 간 불일치 발견. 5건이 commit 기반 사실상 DONE 이지만 progress.md 미갱신으로 IN PROGRESS 로 표기됨.

---

## 1. 각 SPEC 현재 상태

| SPEC-ID | 핵심 책임 | 실제 상태 | 외부 의존 |
|---------|----------|----------|----------|
| **CMDCTX-001** | ContextAdapter + LoopController 인터페이스 정의 | ✅ 완료 (PR#52, c018ec5) | COMMAND-001, ROUTER-001, CONTEXT-001, SUBAGENT-001 (모두 FROZEN) |
| **CMDLOOP-WIRE-001** | LoopController 구현체 (RequestClear, RequestModelChange 등) | ✅ 완료 (PR#54, 7d40f8e) | CMDCTX-001, QUERY-001, CONTEXT-001 (모두 FROZEN) |
| **CMDCTX-CREDPOOL-WIRE-001** | OnModelChange → credential pool swap wiring | ✅ 완료 (PR#55, f0506a3) | CMDCTX-001, CMDLOOP-WIRE-001, CREDPOOL-001 |
| **CMDCTX-CLI-INTEG-001** | CLI 진입점에서 App struct 로 adapter/dispatcher wiring | 🟡 진행 중 (2f31bac) | CMDCTX-001, CMDLOOP-WIRE-001, **CLI-001 (planned)** |
| **CMDCTX-DAEMON-INTEG-001** | 데몬 부트스트랩에서 adapter/dispatcher wiring | 🟡 진행 중 (e241702) | CMDCTX-001, CMDLOOP-WIRE-001, **DAEMON-WIRE-001 (planned)** |
| **CMDCTX-HOTRELOAD-001** | ContextAdapter registry/aliasMap hot-reload (fsnotify) | 📋 계획 중 | CMDCTX-001 (amendment), **ALIAS-CONFIG-001 (planned)** |
| **CMDCTX-PERMISSIVE-ALIAS-001** | ResolveModelAlias step 7 strict/permissive 모드 전환 | 📋 계획 중 | CMDCTX-001 (amendment) |
| **CMDCTX-TELEMETRY-001** | 메트릭 emission (calls/errors/duration) | 📋 계획 중 | CMDCTX-001 (amendment), **OBS-METRICS-001 (TBD — BLOCKER)** |

---

## 2. 의존성 그래프

```
실제 구현 완료 층:
════════════════════════════════════════════════════════════
  COMMAND-001  ROUTER-001  CONTEXT-001  SUBAGENT-001
       ↓             ↓           ↓             ↓
  ╔═══════════════════════════════════════════════════╗
  ║      CMDCTX-001 (PR#52)  ✅ COMPLETED           ║
  ║   (ContextAdapter + LoopController 인터페이스)    ║
  ╚═══════════════════════════════════════════════════╝
       ↓                          ↓
    ┌──┴────────────┬─────────────┴──┐
    ↓               ↓                 ↓
  CMDLOOP-WIRE-001  CREDPOOL-001  (기타 SPEC)
  ✅ COMPLETED      (FROZEN)
  (PR#54)
    ↓
    ├─→ CMDCTX-CREDPOOL-WIRE-001 ✅ COMPLETED (PR#55)
    │
    └─→ ┌──────────────────────────────────┐
        │  🟡 IN PROGRESS                  │
        │  • CLI-INTEG-001 (2f31bac)       │
        │  • DAEMON-INTEG-001 (e241702)    │
        └──────────────────────────────────┘
            ↓
        ┌──────────────────────────────────┐
        │  📋 PLANNED                      │
        │  • HOTRELOAD-001 (fsnotify)      │
        │  • PERMISSIVE-ALIAS-001          │
        │  • TELEMETRY-001 (metrics)       │
        └──────────────────────────────────┘
            └─→ OBS-METRICS-001 (TBD — 🔴 BLOCKER)
```

---

## 3. Critical Path

**완료됨 (✅)**:
- CMDCTX-001 → CMDLOOP-WIRE-001 → CMDCTX-CREDPOOL-WIRE-001

**진행 중 (🟡)** — 다음 단계 진행 대기:
- CMDLOOP-WIRE-001 실제 구현체 완료 → CLI/DAEMON-INTEG 진행 가능

**계획 단계 (📋)** — 진행 불가 사유:
- TELEMETRY-001: OBS-METRICS-001 미존재 (인프라 없음)
- HOTRELOAD-001: ALIAS-CONFIG-001 아직 미구현
- PERMISSIVE-ALIAS-001: 독립적으로 진행 가능하나, CMDCTX-001 amendment 동기화 필요

---

## 4. 권장 진행 순서

| 순위 | SPEC | 현황 | 사유 | 즉시 진행 가능? |
|------|------|------|------|-----------------|
| 1 | CMDCTX-001 | ✅ 완료 | 기초 인터페이스 (FROZEN) | N/A |
| 2 | CMDLOOP-WIRE-001 | ✅ 완료 | LoopController 구현 | N/A |
| 3 | CMDCTX-CREDPOOL-WIRE-001 | ✅ 완료 | credential pool swap | N/A |
| 4 | CMDCTX-CLI-INTEG-001 | 🟡 진행 중 | CLI 진입점 wiring | YES (이미 진행 중) |
| 5 | CMDCTX-DAEMON-INTEG-001 | 🟡 진행 중 | 데몬 진입점 wiring | YES (이미 진행 중) |
| 6 | CMDCTX-PERMISSIVE-ALIAS-001 | 📋 계획 | CMDCTX-001 amendment (독립) | YES (병렬 진행 권장) |
| 7 | CMDCTX-HOTRELOAD-001 | 📋 계획 | ALIAS-CONFIG-001 구현 필요 | NO (외부 대기) |
| 8 | CMDCTX-TELEMETRY-001 | 📋 계획 | OBS-METRICS-001 신설 필요 | NO (완전 blocker) |

---

## 5. 외부 Blocker 상태

| 항목 | 상태 | 영향 SPEC | 해소 방법 |
|------|------|----------|----------|
| **OBS-METRICS-001** (metrics 인프라) | 🔴 미존재 | TELEMETRY-001 | 신규 SPEC 생성 필요 (OTel/Prometheus/expvar 선택 후) |
| **ALIAS-CONFIG-001** (alias 파일 로더) | 📋 계획 중 | HOTRELOAD-001 | SPEC plan 단계, run phase 진행 필요 |
| **DAEMON-WIRE-001** (daemon bootstrap) | ✅ 완료 (2c6acae) | DAEMON-INTEG-001 | 의존 해소됨 |
| **CLI-001 v0.2.0** (cobra + bubbletea) | 🟡 PARTIAL (27d9cda) | CLI-INTEG-001 | TUI 보강 후 완전 통합 가능 |

---

## 6. 병렬 가능 그룹

### 즉시 병렬 진행 (의존성 0)

**그룹 A** (현재 진행 중, 서로 독립):
- CMDCTX-CLI-INTEG-001 + CMDCTX-DAEMON-INTEG-001
- 둘 다 CMDLOOP-WIRE-001, CMDCTX-001 완료 후 진행 가능
- 병렬 진행 권장 (다른 진입점)

### 다음 라운드 병렬

**그룹 B** (CLI/DAEMON 완료 후):
- CMDCTX-PERMISSIVE-ALIAS-001 (xs, CMDCTX-001 amendment)
- CMDCTX-001 v0.2.0 amendment 동기화 필요

### 순서 직렬화 강제

**그룹 C** (외부 대기):
- CMDCTX-HOTRELOAD-001 → ALIAS-CONFIG-001 구현 후 진행 + fsnotify 추가 외부 의존

**그룹 D** (완전 blocker):
- CMDCTX-TELEMETRY-001 → OBS-METRICS-001 신설 필수 (metrics sink interface + backend 선택)

---

## 7. 정체 원인 진단

### 상태 표기 불일치 (Meta-issue)

**발견**: progress.md에서 모든 8 SPEC이 "planned" 또는 "IN PROGRESS"로 표시되어 있으나, git log와 spec.md frontmatter에서 **5건이 이미 completed/implemented 상태**.

| 원인 | 설명 |
|------|------|
| 🔴 progress.md 미갱신 | 각 SPEC 폴더의 progress.md가 run phase 완료 후 status 업데이트 미수행 |
| ✅ spec.md frontmatter 갱신됨 | version, completed 날짜, updated_at은 최신 |
| 🔴 순차 작업 지연 | 5개 완료된 SPEC의 merged PR 이후, 다음 파이프라인(HOTRELOAD, TELEMETRY 등) 계획 단계로 멈춤 |

### 의존성 체인 중단점

| 단계 | SPEC | 블록 사유 |
|------|------|----------|
| 완료 | 1-3번 | 없음 ✅ |
| 진행 중 | 4-5번 | 없음 (이미 진행 중) ✅ |
| 정체 | 6번 (PERMISSIVE) | CMDCTX-001 amendment governance 동기화 필요 (낮은 우선도) |
| 정체 | 7번 (HOTRELOAD) | 외부 대기: ALIAS-CONFIG-001 구현 완료 필요 |
| 정체 | 8번 (TELEMETRY) | 완전 blocker: OBS-METRICS-001 인프라 부재 (metrics sink 정의 필요) |

---

## 8. 다음 액션 권장 TOP 3

### 1. 즉시 (1-2일)

**CMDCTX-CLI-INTEG-001 + CMDCTX-DAEMON-INTEG-001 마무리**

- 현재 진행 중 (2f31bac, e241702)
- 두 SPEC 모두 의존성 해결됨
- 액션: PR 리뷰 및 merge 완료 → progress.md 갱신

### 2. 단기 (3-5일)

**CMDCTX-PERMISSIVE-ALIAS-001 run phase 시작**

- 현재 plan 단계
- CMDCTX-001 과 동일 base (v0.1.1)에 amendment
- 액션:
  1. CMDCTX-001 spec.md frontmatter 갱신 (v0.1.1 → v0.2.0)
  2. PERMISSIVE-ALIAS run phase 진행 (xs 사이즈)
  3. 단일 PR로 CMDCTX-001 + PERMISSIVE-ALIAS 동시 merge

### 3. 중기 (1주 이내)

**ALIAS-CONFIG-001 구현 + HOTRELOAD-001 계획 + OBS-METRICS-001 신설**

- HOTRELOAD의 외부 대기 사항 해소
- 액션:
  1. ALIAS-CONFIG-001 run phase 시작
  2. ALIAS-CONFIG 완료 후 HOTRELOAD-001 run phase 가능
  3. OBS-METRICS-001 plan phase 시작 (metrics sink interface 정의)
  4. TELEMETRY-001은 OBS-METRICS-001 완료 후 진행

---

## 결론

**8 SPEC 정체의 근본 원인**:
- ✅ 3건 이미 완료 (progress.md 미갱신 = 메타 이슈, 본 정리에서 해소)
- 🟡 2건 진행 중 (의존성 解 후 PR merge 대기 중)
- 📋 3건 계획 단계 — 그 중 1건 완전 blocker (OBS-METRICS-001 신설 필요)

**추천 진행 전략**:
1. **지금**: CLI-INTEG + DAEMON-INTEG PR 마무리 (의존성 해소됨)
2. **다음**: PERMISSIVE-ALIAS 병렬 진행 (독립적)
3. **그 다음**: ALIAS-CONFIG 구현 → HOTRELOAD 진행
4. **별도 추적**: TELEMETRY는 OBS-METRICS-001 신설 후 plan phase 재시작

병렬 진행 가능한 그룹: (CLI-INTEG + DAEMON-INTEG) 완료 후, PERMISSIVE-ALIAS 와 ALIAS-CONFIG 동시 진행.

---

**Status Source**: SPEC-META-CLEANUP-2026-04-30
**연계 작업**:
- CMDCTX-001, CMDLOOP-WIRE-001, CMDCTX-CREDPOOL-WIRE-001 progress.md DONE 상태 갱신 (별도 진행)
- ROADMAP.md 진행 상태 섹션 추가 (별도 진행)
