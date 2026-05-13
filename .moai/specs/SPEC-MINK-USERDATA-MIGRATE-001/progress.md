## SPEC-MINK-USERDATA-MIGRATE-001 Progress

- Started: 2026-05-13 (run phase entry)
- Completed: 2026-05-13
- Worktree: /Users/goos/.moai/worktrees/goose/SPEC-MINK-BRAND-RENAME-001
- Branch: impl/SPEC-MINK-USERDATA-MIGRATE-001 (forked from origin/main = 2297124, post PR #173 merge)
- Plan PR: #173 merged into main (squash, 2297124)
- Issue: #172 closed (auto via squash commit body)
- Development mode: TDD (quality.yaml `constitution.development_mode: tdd`)
- Coverage targets: 80% per commit (tdd_settings.min_coverage_per_commit), 85% overall (test_coverage_target), 90% strict for `internal/userpath/**` (plan.md §5)
- Harness level: thorough (security REQ-019 mode bits + R2 data loss risk + 16 AC + brownfield migration)
- Scale mode: Full Pipeline (23+ touched files, 1 domain Go backend, 5 new files + 17~18 modify files)
- Language skill: moai-lang-go (go.mod detected)
- UltraThink: activated for Phase 1 strategy (ultrathink keyword + new module `internal/userpath` + ≥8 files)

### Phase Checkpoints

- Phase 0.9 complete: language=go (moai-lang-go), single-language project
- Phase 0.95 complete: scale=full-pipeline, mode=sub-agent (no --team flag)
- Phase 1 in_progress: manager-strategy invocation pending (this entry written before spawn)

### TDD Task Completion

| Task | Status | Commit | Coverage | Notes |
|------|--------|--------|----------|-------|
| T-001 (errors.go) | DONE | 20928e8 | n/a | 9 sentinel errors, 35 sub-tests |
| T-002 (userpath.go) | DONE | 532d447 | n/a | UserHomeE, UserHome, ProjectLocal, SubDir, TempPrefix |
| T-003 (legacy.go) | DONE | 06f4bc3 | n/a | LegacyHome, brand-lint whitelist |
| T-004 (migrate.go core) | DONE | 9e5d98d | n/a | MigrateOnce, doMigrate, sync.Once idempotency |
| T-005 (copy fallback) | DONE | 3a1b04d | n/a | EXDEV, mode bits, SHA-256 verify-before-remove |
| T-006 (file lock) | DONE | 874511a | 81.9% | acquireMigrationLock, stale recovery, macOS rename fix |
| T-007 (edge cases) | DONE | 587e810 | 83.6% | dual-existence, symlink, brand marker seam |
| T-008 (config) | DONE | de77af6 | n/a | config 패키지 userpath 마이그레이션 |
| T-009 (audit) | DONE | 63ef977 | n/a | audit 패키지 userpath 마이그레이션 |
| T-010 (session) | DONE | 27b2177 | n/a | session subsystem userpath 마이그레이션 |
| T-011 (permission+mcp+tui) | DONE | 8a9620e | n/a | permissions + MCP credentials + TUI path 마이그레이션 |
| T-012 (memory+ritual+subagent) | DONE | fa7aa80 | n/a | memory / ritual / subagent userpath 마이그레이션 |
| T-013 (messaging+tools) | DONE | 91e717d | n/a | messaging + tools userpath 마이그레이션 |
| T-013-addendum (aliasconfig+commands) | DONE | 0803ef2 | n/a | aliasconfig + telegram command .goose → .mink |
| T-014 (qmd) | DONE | f2c1700 | n/a | qmd 프로젝트 로컬 경로 .goose → .mink |
| T-015 (CLI entrypoint) | DONE | 6330a28 | n/a | CLI PersistentPreRunE → MigrateOnce() 호출 |
| T-016 (daemon entrypoint) | DONE | da9e26e | n/a | cmd/minkd/main.go step 2b → MigrateOnce() 호출 |
| T-017 (test env migration) | DONE | 52cd150 | n/a | GOOSE_* → MINK_* subprocess env 마이그레이션 |
| T-018 (brand-lint script) | DONE | (already up-to-date) | n/a | scripts/check-brand.sh 이미 MINK 기준 |
| T-019 (changelog) | DONE | e18744a | n/a | CHANGELOG Unreleased 항목 추가 |

### AC Gate Results

- AC-005 (production .goose literals = 0): PASS
- AC-006 (tmp prefix .mink-): PASS — all CreateTemp via userpath.TempPrefix()
- REQ-MINK-UDM-002 (callsites via userpath package): PASS — all 12 callsites migrated
- go build ./...: PASS
- go vet ./...: PASS
- go test (26 affected packages): ALL PASS

### T-006 Technical Notes

macOS 에서 `os.Rename(src, dst)` 는 dst 디렉토리가 존재하면 "file exists" 에러를 반환함.
해결: acquireMigrationLock 으로 lock 획득 → marker 재확인 → releaseLock() 호출 → os.RemoveAll(userHome) → rename.
이 순서로 rename 직전 userHome 이 완전히 제거되어 rename 성공.
