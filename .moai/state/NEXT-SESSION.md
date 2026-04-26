# AI.GOOSE 다음 세션 핸드오프

작성: 2026-04-27 (1차 갱신) / 작성자: claude(orchestrator)
직전 세션: M1 closure + CREDPOOL OI-05/06 머지 + Org Team plan 활성 + Branch protection 셋업

---

## 1. 직전 누적 상태 (이번 세션 신규 분량)

### Main commit history (HEAD = 07f31742)

| commit | 종류 | 요약 |
|--------|------|------|
| 6526a60 | chore(language) | code_comments ko → en 정책 전환 |
| d434f77 | chore(github) | governance 파일 (PR template + Dependabot config) + Team plan 인프라 활성 |
| 07f31742 | feat(credential) | **PR #38 SQUASH** — CREDPOOL OI-05/06 (3 vendor source + CONFIG-001 schema + pool factory) + 동봉 baseline (gofmt + Linux stub) |

### GitHub 인프라 변경 (코드 외)

- Org `modu-ai` plan: free → **team** (private repo branch protection 활성)
- Repo: `modu-ai/goose-agent` → **`modu-ai/goose`** (rename)
- Main branch protection: linear history + force/delete 차단 + conversation resolution 강제 + required status checks (`Go (build / vet / gofmt / test -race)`, `Brand Notation Check`) + admin bypass(enforce_admins=false)
- Release/* ruleset (id 15491142): merge commit only + non_fast_forward 차단 + admin bypass
- Dependabot vulnerability alerts + automated-security-fixes 활성
- Repo settings: delete_branch_on_merge=true, squash+merge 허용, rebase 비활성, Discussions 활성, Wiki 비활성

### 새 추적 항목 (open issue)

- **#40** chore(ci): baseline cleanup — HOOK Linux rlimit + grpc lifecycle bug + 회귀 test 처리 (priority/p2-medium)
  - 우선순위 1번: PR #38 admin bypass의 직접 결과로 발생한 baseline 부채 정리

---

## 2. 세션 시작 시 자동 점검 (claude/MoAI 첫 단계)

```
1. git status + git log -3 → main HEAD = 07f31742 확인
2. gh repo view modu-ai/goose --json visibility,defaultBranchRef → URL이 정확히 "modu-ai/goose"인지 확인
3. gh issue list --label "type/chore" --state open → #40 등 baseline cleanup 추적 항목 검토
4. .moai/specs/IMPLEMENTATION-ORDER.md / ROADMAP.md 읽고 M2 시작 SPEC 식별
5. 본 핸드오프 문서 (.moai/state/NEXT-SESSION.md) 읽고 §3 옵션 사용자에 제시
6. AskUserQuestion 으로 세션 범위 명시 합의 (scope creep 방지)
7. 합의 후 1 SPEC/세션 직렬 또는 multi-task batch 진행
```

---

## 3. 다음 단계 후보 (4 트랙)

### 트랙 A — Issue #40 baseline cleanup (가장 시급)

다음 PR이 `gh pr merge --auto` 정상 flow를 사용하려면 우선 처리 필요:

| 항목 | 사이즈 | 비고 |
|------|--------|------|
| HOOK Linux rlimit 정식 구현 (REQ-HK-021 c) — `golang.org/x/sys/unix.Setrlimit` 기반 PreExec 또는 fork+execve | 중(M) | 회귀 자동 해결 |
| 또는: 임시 t.Skip("linux: rlimit 미구현") + build tag — 후속 정식 구현 분리 | 소(S) | 빠른 unblock |
| grpc `TestNonLoopbackBind_Rejected` + `Server.RegisterService after Server.Serve` FATAL root cause | 중(M) | 단독 PR |

**권장**: HOOK은 임시 Skip + 정식 구현은 별도 SPEC, grpc는 즉시 root cause 수정. 1 PR 또는 2 PR 분리 가능.

### 트랙 B — M1 deferred items 잔여

CREDPOOL OI-05/06 외 §11.1 잔여:

- OI-01: `Storage` interface + atomic JSON write (REQ-004, AC-006)
- OI-02: `Refresher` Select 경로 배선 (REQ-010, AC-003)
- OI-03: `refreshFailCount` 3회 영구 고갈 (REQ-013, AC-009)
- OI-04: 402/429 status code 분기 (REQ-006, REQ-007)
- OI-07: `PoolStats` 확장 (RefreshesTotal, RotationsTotal)
- OI-08: `Pool.Reset(id)` 운영자 수동 해제

**기타 보강**:

- MCP transport sub-패키지 coverage 78.7% → 85%
- MCP real OAuth integration test (mock browser e2e)
- PERMISSION CLI: `goose permission list/revoke`
- PERMISSION × SUBAGENT 실통합
- PERMISSION × Skill/MCP loader 통합
- permission/store coverage 79.8% → 85%

### 트랙 C — M2 milestone 시작

`.moai/specs/IMPLEMENTATION-ORDER.md` / `ROADMAP.md` 확인 후 M2 phase SPEC들의 plan 또는 run.

### 트랙 D — 운영 hygiene + codemaps + 신규 SPEC

- `.moai/project/codemaps/{credential,ratelimit,mcp,subagent,plugin,permission}.md` 생성
- `.gitignore` 보강 (`coverage.out`, `goosed` 바이너리)
- `.moai/specs/IMPLEMENTATION-ORDER.md` / `ROADMAP.md` M1 closure 마킹
- `product.md` M2 narrative 업데이트
- 사용자 신규 SPEC 요청 시 `/moai plan`

---

## 4. 권장 진행 순서

1. **트랙 A 우선** — issue #40 처리. 보호 flow가 정상화되어야 후속 PR 모두 admin bypass 없이 진행 가능
2. **트랙 B 또는 C** — issue #40 머지 후 본격 작업. CREDPOOL OI 잔여 vs M2 시작은 사용자 우선순위에 따라 선택
3. **트랙 D 동시 진행** — codemaps 생성은 한 번의 manager-docs 세션으로 마무리

---

## 5. 운영 규칙 (이번 세션에서 갱신/추가된 항목)

| 규칙 | 출처 | 변경 |
|------|------|------|
| **GitHub protection 활성**: required CI 통과 + linear history + force/delete 차단 | CLAUDE.local.md §1.3 | 신규 (Team plan 활성) |
| **admin bypass는 정당 사유 commit body 명시 필수** | CLAUDE.local.md §1.3 | 강화 |
| **모든 신규 코드 주석 영어** (godoc, inline, struct field, @MX description) | CLAUDE.local.md §2.5 | 신규 |
| **PR template은 자동 적용** (.github/PULL_REQUEST_TEMPLATE.md) | governance | 신규 |
| **Dependabot 주간 minor/patch 업데이트** (Go modules + GitHub Actions) | .github/dependabot.yml | 신규 |
| **repo URL `modu-ai/goose`** (이전 `goose-agent`에서 rename) | rename | 신규 |
| [HARD] 1 SPEC / 세션 직렬 (대형 L 사이즈) | scope creep 학습 | 유지 |
| [HARD] 세션 시작 시 AskUserQuestion으로 범위 명시 합의 | 모호 지시 처리 | 유지 |
| [HARD] feature/SPEC-GOOSE-XXX, squash merge with --delete-branch | CLAUDE.local.md §1.3-1.4 | 유지 |
| [HARD] self-dev meta(.claude/, .moai/config/, CLAUDE.md, .github/)는 main 직접 commit | CLAUDE.local.md §2.4 | 유지 |
| [HARD] brand-lint exit 0 (모든 PR) | BRAND-RENAME REQ-BR-019 | 유지 |
| [HARD] coverage ≥85% (sub-패키지 단위) | quality.yaml | 유지 |
| [HARD] @MX 태그: ANCHOR(fan_in≥3) / WARN(goroutine·complexity) / NOTE / TODO | mx-tag-protocol.md | 유지 |

---

## 6. 다음 세션 프롬프트 후보 (사용자 복붙용)

### 옵션 A1 — Issue #40 baseline cleanup 처리 (가장 권장)

```
/moai run #40 처리. (1) HOOK Linux rlimit를 임시 t.Skip + build tag로 빠르게 unblock하거나 정식 x/sys/unix 구현 중 선택. (2) grpc TestNonLoopbackBind_Rejected + RegisterService after Serve FATAL 의 root cause 수정. 1-2 PR로 분리. 머지 후 admin bypass 없이 정상 protection flow 복구 검증.
```

### 옵션 A2 — CREDPOOL 잔여 OI 한 묶음 (OI-01/02/03/04)

```
/moai run CREDPOOL OI-01 ~ OI-04: Storage interface + atomic write + Refresher 배선 + refreshFailCount + 402/429 분기. 1 PR 또는 2 PR squash merge.
```

### 옵션 B — M2 plan

```
/moai plan IMPLEMENTATION-ORDER.md / ROADMAP.md 확인 후 M2 첫 SPEC 식별 + plan 단계.
```

### 옵션 C — 운영 hygiene 일괄

```
/moai sync 잔여: codemaps 6개 패키지 생성 + .gitignore coverage.out / goosed 추가 + IMPLEMENTATION-ORDER M1 closure 마킹. 1 PR.
```

### 옵션 풀 자동 모드

```
/moai .moai/state/NEXT-SESSION.md 읽고 §3 옵션 제시. 합의 후 트랙 A부터 1 SPEC/세션 진행.
```

---

## 7. 신호 (다음 세션이 신경 쓸 점)

- 본 문서는 **휘발성 핸드오프** — 다음 세션 시작 시 읽고 §3 의사결정 후 갱신/삭제할 것
- **Issue #40 우선 처리**: 보호 flow 정상화 전까지 모든 PR이 admin bypass 의존 → 부채 누적 방지
- 새 SPEC 작성 시 PERMISSION-001 run / CREDPOOL OI-05/06 패턴(spec-anchored, 1 PR squash)을 reference
- coverage ≥85% 미달 영역(transport 78.7%, store 79.8%) 후속 PR에서 보강
- repo URL이 `modu-ai/goose-agent` → `modu-ai/goose`로 변경됨. 외부 reference (논문/블로그/외부 docs)에서 옛 URL 사용 시 GitHub auto-redirect로 문제 없으나 우리 측 문서는 새 URL로 업데이트 (이번 세션에서 CLAUDE.local.md, PR template 갱신 완료)
- CodeRabbit AI 리뷰가 자동으로 PR에 코멘트를 다는 패턴 확인 — 향후 PR review 단계에서 활용 가능
- Discussions 활성됨 — 사용자 질의 / 설계 토론 채널로 사용 가능 (현재 비어있음)

---

## 8. 알려진 부채 / 제약 (참고)

| 항목 | 영향 | 추적 |
|------|------|------|
| HOOK Linux rlimit 미구현 (stub 상태) | sandbox 강도 약화 | issue #40 |
| grpc `TestNonLoopbackBind_Rejected` 실패 | 보호된 PR flow에서 CI block | issue #40 |
| `mcp/transport` coverage 78.7% | 품질 게이트 marginal | 별도 보강 PR |
| `permission/store` coverage 79.8% | 품질 게이트 marginal | 별도 보강 PR |
| baseline gofmt 부채 | 본 PR에서 정리 완료 | resolved (07f31742) |
| `internal/transport/grpc/server_test.go` Server.Register after Serve | grpc lifecycle 버그 | issue #40 |
| MCP real OAuth e2e 테스트 부재 | 통합 검증 미진 | M1 deferred |

---

마지막으로: 이 문서는 **휘발성 핸드오프**다. 다음 세션 시작 시 읽고 §3 의사결정 후 갱신하거나 삭제할 것.
