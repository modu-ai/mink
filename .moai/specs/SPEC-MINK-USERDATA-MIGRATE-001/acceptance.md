# SPEC-MINK-USERDATA-MIGRATE-001 — Acceptance Criteria

## HISTORY

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-05-13 | manager-spec | 초안 작성. 5개 main Given/When/Then scenario + 4 edge case scenario. 모든 AC 는 binary-verifiable (exit code, byte-level diff, command output, file existence). |
| 0.1.1 | 2026-05-13 | manager-spec | plan-auditor iter 1 fix — 13 defects addressed (D1-D13). REQ-004 (AC-006 tmp prefix), REQ-016 (AC-007 test marker), REQ-018 (AC-008a happy / AC-008b 4 negative paths) 의 traceability gap 보강. REQ-019 신설에 따라 AC-009 mode bits 보존 AC 추가. AC-001 #6 weasel "또는 동등 영문" 제거 → 절대 byte 검증 (`goose` 미포함 + `mink`/`밍크` 포함). AC-004 분리 → AC-004a (CLI fail-fast) + AC-004b (daemon graceful degrade). 신규 AC-010 brand marker 부재 시 best-effort warning 추가 (R4 trace). AC-005 의 `grep --exclude=<path>` 오류 fix (basename glob 한계 → post-filter pipeline). AC-003 stat 명령 Linux/macOS dual notation 명시. Total: 5 main → 12 main (AC-004 split 포함) + 4 edge. |
| 0.1.2 | 2026-05-13 | manager-spec | plan-auditor iter 2 fix — 8 new defects (ND-1 ~ ND-8) addressed. ND-1 (blocking): AC-001 #6 의 base 예시 메시지에서 옛 경로 literal `~/.goose`/`~/.mink` 인용을 "이전 디렉토리"/"새 MINK 디렉토리" prose 로 재작성 → gate `grep -c 'goose' = 0` binary 자체 위반 해소. ND-3: DoD "11 main scenarios" → "12 main scenarios" (AC-001/002/003/004a/004b/005/006/007/008a/008b/009/010 명시 enumeration). ND-5: DoD "Quality gate 13개" → "14개" (14 rows enumerate). ND-8: AC-005 #1 descriptive 문장 → binary grep 명령 (`... \| wc -l = 0`) 으로 전환. 본 행과 짝을 이루는 spec.md / plan.md / spec-compact.md 의 잔존 stale count (ND-2/4/6/7) 도 동일 commit 에서 동기화. |
| 0.1.3 | 2026-05-13 | MoAI orchestrator | plan-auditor iter 3 fast-track fix — ND3-1 (blocking): AC-001 #6 base 예시 메시지에서 brand 토큰이 `MINK` (uppercase) 만 등장하여 case-sensitive gate `grep -Ec 'mink\|밍크' ≥ 1` 와 정합 깨짐 → 예시 문구에 한국어 `밍크` 토큰 + 소문자 `mink` 토큰을 동시 포함하도록 정정. iter 3 mole-whack 패턴 차단. spec.md §7.1 self-ref AC scenario count 5 → 12 main + 4 edge 동기화 (ND3-2). |

---

본 문서는 SPEC-MINK-USERDATA-MIGRATE-001 의 모든 EARS 요구사항에 대한 **binary-verifiable Acceptance Criteria** 를 정의한다. 각 AC 는 Given / When / Then 형식이며, manual smoke test 와 automated test 양쪽에서 검증 가능하다.

플랫폼 주의: 본 SPEC 의 1차 릴리스는 Linux/macOS 만 검증 (plan.md R9). Windows 는 별도 SPEC. AC 의 모든 shell 명령은 `bash` 기준이며, `stat` 같은 platform-specific tool 은 dual notation 으로 기재한다.

---

## AC-MINK-UDM-001 — 자동 마이그레이션 + 일회성 알림

**Given**:
- `~/.goose/` 가 존재하며 다음 파일을 포함한다:
  - `~/.goose/memory/memory.db` (non-empty)
  - `~/.goose/permissions/grants.json` (valid JSON)
  - `~/.goose/config.yaml`
- `~/.mink/` 는 존재하지 않는다 (`! test -e ~/.mink/`)
- `MINK_HOME` env var 는 설정되어 있지 않다 (`unset MINK_HOME`)
- baseline 으로 `find ~/.goose -type f | sort` 의 출력과 각 파일의 SHA-256 hash 가 캡처된 상태

**When**:
- `mink` (또는 `minkd`) 가 처음 실행된다
- 첫 명령은 임의의 read 작업 (`mink --version`)

**Then** (모두 만족, binary 검증):
1. `test -d ~/.mink/` exit 0
2. `! test -e ~/.goose/` exit 0 (옛 디렉토리 제거됨, atomic rename 또는 verified copy+remove)
3. `find ~/.mink -type f | sort` 출력의 파일 목록이 baseline `find ~/.goose -type f | sort` 와 byte-identical (path prefix 만 다르고 나머지 동일)
4. 각 파일의 SHA-256 hash 가 baseline 과 일치 (verified migration)
5. `~/.mink/.migrated-from-goose` marker 파일이 존재 + timestamp + binary version 포함
6. stderr 에 정확히 1줄 알림이 출력되며, 다음 두 조건을 모두 만족한다 (D7 fix — weasel "또는 동등" 제거 / ND-1 fix — 예시 메시지를 prose 로 재작성하여 gate 와 정합화):
   - 메시지가 단어 `goose` 를 **포함하지 않는다** (`grep -c 'goose' stderr.log` = 0)
   - 메시지가 단어 `mink` 또는 한국어 `밍크` 중 하나 이상을 **포함한다** (`grep -Ec 'mink|밍크' stderr.log` ≥ 1)
   - 메시지 본문 (Korean primary line) 의 base 예시 (ND-1 fix — 옛 경로를 literal 인용하지 않고 "이전 디렉토리" 표현 사용 / ND3-1 fix — gate `grep -Ec 'mink|밍크'` 와 정합하도록 한국어 `밍크` 토큰을 포함): `INFO: 사용자 데이터가 이전 디렉토리에서 새 mink 디렉토리(밍크)로 마이그레이션되었습니다.`
   - REQ-008 에 따라 영문 subtext 한 줄이 선택적으로 동반될 수 있다 (예: `Migrated user data from the legacy directory to the new mink directory.`). 영문 subtext 가 있을 경우에도 위 두 grep 조건은 그대로 만족해야 한다 (`mink` 소문자 토큰이 포함됨).
   - 정확한 경로 안내가 필요할 경우 (사용자 actionable detail) README 또는 별도 안내 페이지에 link 만 제공하며, stderr 한 줄 알림에는 옛 brand 토큰 `goose` 를 포함하지 않는다.
7. 동일 명령 재실행 시 알림 출력 0건 (멱등성)

REQ 매핑: REQ-MINK-UDM-003, REQ-MINK-UDM-005, REQ-MINK-UDM-007, REQ-MINK-UDM-008, REQ-MINK-UDM-009, REQ-MINK-UDM-017

---

## AC-MINK-UDM-002 — 양쪽 디렉토리 동시 존재 시 `~/.mink/` 우선 + 경고

**Given**:
- `~/.goose/` 가 존재한다 (이전 사용자 데이터)
- `~/.mink/` 도 존재한다 (사용자가 수동 생성 또는 이전 마이그레이션 후 수동 복원)
- `~/.mink/.migrated-from-goose` marker 가 존재한다 (이전 마이그레이션 발생 증거)

**When**:
- `mink --version` 실행

**Then** (모두 만족):
1. exit 0
2. `userpath.UserHome()` 의 internal 결과 = `~/.mink/` (옛 디렉토리 무시)
3. `~/.goose/` 는 변경 0건 (rename / remove 발생 안 함)
4. stderr 에 정확히 1줄 warning. 메시지에는 `goose` 단어가 포함될 수 있으나 (옛 디렉토리 경로 안내 목적), 신규 brand identifier (`mink` 또는 `밍크`) 도 반드시 포함된다. base 예시: `WARN: ~/.goose 디렉토리가 여전히 존재하지만 무시됩니다. ~/.mink 디렉토리가 우선 사용됩니다.`
5. 동일 명령 재실행 시 warning 0건 (process-level 1회만, persistent marker 미사용)
6. `test -e ~/.goose/config.yaml` exit 0 (옛 데이터 보존됨, 사용자 책임으로 정리 가능)

REQ 매핑: REQ-MINK-UDM-012

---

## AC-MINK-UDM-003 — Fresh install (옛 디렉토리 부재, 신규 사용자)

**Given**:
- `~/.goose/` 부재 (`! test -e ~/.goose/`)
- `~/.mink/` 부재 (`! test -e ~/.mink/`)
- `MINK_HOME` env var 미설정

**When**:
- `mink --help` 실행

**Then** (모두 만족):
1. exit 0
2. `test -d ~/.mink/` exit 0 (신규 생성)
3. `~/.mink/` 권한 = 0700. 검증 명령은 platform-specific (D12 fix):
   - Linux: `[ "$(stat -c '%a' ~/.mink/)" = "700" ]` exit 0
   - macOS: `[ "$(stat -f '%Lp' ~/.mink/)" = "700" ]` exit 0
   - Portable fallback (Linux/macOS 양쪽): `ls -ld ~/.mink/ | awk '{print $1}'` 출력이 `drwx------` 와 일치
4. `~/.mink/.migrated-from-goose` marker **부재** (마이그레이션 미발생, fresh install)
5. stderr 에 마이그레이션 알림 출력 0건
6. 다음 실행 시에도 알림 0건 (정상 fresh install 흐름)

REQ 매핑: REQ-MINK-UDM-003

---

## AC-MINK-UDM-004a — CLI (`mink`) 마이그레이션 실패 시 fail-fast (D8 split)

**Given**:
- `~/.goose/` 존재 + 다음 파일 포함:
  - `~/.goose/large_file.bin` (≥ 100 MB, 의도적으로 큰 파일)
- `~/.mink/` 부재
- `~/.mink/` 의 부모 디렉토리 (`~/`) 의 디스크 free space 가 인위적으로 `large_file.bin` 의 크기 미만으로 제한된 상태 (또는 `~/.mink/` target 이 read-only filesystem 에 위치, 예: bind-mount via `chattr +i` 또는 ro mount)

**When**:
- CLI 형태로 `mink --version` 실행 (TTY 또는 explicit `--client` 모드)

**Then** (모두 만족):
1. exit 코드 **non-zero** (fail-fast). 사용자가 정확히 어디서 막혔는지 즉시 인지 가능.
2. `~/.goose/` 는 byte-identical 보존 (`find ~/.goose -type f -exec sha256sum {} +` baseline 과 일치)
3. `~/.mink/` 는 부분적으로 생성되었더라도 **삭제됨** (`! test -e ~/.mink/` 또는 `find ~/.mink -mindepth 1` 출력 비어 있음) — partial state cleanup (REQ-MINK-UDM-015)
4. stderr 에 user-actionable error 출력 — disk space 또는 read-only 안내 + `MINK_HOME` env var 로 대안 path 지정 방법 안내. 메시지에 `mink` 단어 포함, `goose` 단어 포함 가능 (옛 디렉토리 안내 목적).
5. `~/.goose/large_file.bin` 의 SHA-256 hash 가 baseline 과 동일 (verification gate 가 작동, source 미삭제 보장)

REQ 매핑: REQ-MINK-UDM-009, REQ-MINK-UDM-013, REQ-MINK-UDM-015

---

## AC-MINK-UDM-004b — Daemon (`minkd`) 마이그레이션 실패 시 graceful degrade (D8 split)

**Given**:
- `~/.goose/` 존재 + 다음 파일 포함:
  - `~/.goose/large_file.bin` (≥ 100 MB)
- `~/.mink/` 부재
- 동일 disk space 제한 또는 read-only filesystem
- 실행 binary 가 daemon (`minkd`) — long-running service, 갑작스러운 fail-fast 가 부적절

**When**:
- `minkd` 가 system service 로 시작됨 (`systemctl start minkd` 또는 직접 실행)

**Then** (모두 만족):
1. exit 코드 0 (startup 계속). daemon 은 단일 마이그레이션 실패로 startup 자체를 막지 않는다.
2. stderr (또는 daemon log) 에 `ErrReadOnlyFilesystem` 또는 `ErrPermissionDenied` warning 출력 — daemon 은 read-only fallback mode 로 동작 (REQ-MINK-UDM-013).
3. `userpath.UserHome()` 호출 시 `~/.goose/` 의 read-only 접근으로 graceful fallback (read 가능, write 차단). 추후 사용자 개입 (디스크 확보, `MINK_HOME` 지정 등) 후 daemon restart 시 정상 마이그레이션 가능.
4. `~/.goose/` 는 byte-identical 보존 (위 AC-004a #2 와 동일 검증).
5. `~/.mink/` partial state 가 있었다면 cleanup 된다 (`! test -e ~/.mink/` 또는 verified empty) — REQ-MINK-UDM-015.

REQ 매핑: REQ-MINK-UDM-009, REQ-MINK-UDM-013, REQ-MINK-UDM-015

---

## AC-MINK-UDM-005 — Production source 의 `.goose` literal 0건 (legacy.go 예외)

**Given**:
- 본 SPEC PR 의 모든 phase commit 이 적용된 상태
- baseline: Phase 1 시작 전 `grep -rEn '"\.goose' --include='*.go' --exclude='*_test.go' .` 가 30+ 라인 매치 캡처

**When**:
- 동일 grep 명령을 본 SPEC PR squash merge 후 재실행

**Then** (모두 만족 — D4 fix: `grep --exclude=<path>` 는 basename glob 만 매칭하므로 post-filter pipeline 으로 전환):
1. (ND-8 fix — descriptive → binary command) Non-legacy production file 의 `.goose` literal 0건: `grep -rEn '"\.goose' --include='*.go' --exclude='*_test.go' . | grep -v '^./internal/userpath/legacy\.go:' | wc -l` 출력 = 0
2. `internal/userpath/legacy.go` 에서의 매치 라인 수 = 1 (단일 `LegacyHome()` 함수 정의 안의 string literal 1건). 검증: `grep -c '"\.goose' internal/userpath/legacy.go` 출력 = 1
3. `filepath.Join` 패턴 검증 (post-filter form — D4 fix): `grep -rEn 'filepath\.Join\([^)]*"\.goose"' --include='*.go' --exclude='*_test.go' . | grep -v '^./internal/userpath/legacy\.go:' | wc -l` 출력 = 0
4. 직접 `os.UserHomeDir` 호출 검증 (post-filter form — D4 fix): `grep -rEn 'os\.UserHomeDir\(\)' --include='*.go' --exclude='*_test.go' . | grep -v '^./internal/userpath/userpath\.go:' | wc -l` 출력 = 0 (모든 호출이 `userpath` 패키지를 통과)
5. `bash scripts/check-brand.sh` exit 0 (brand-lint 통과)
6. `go build ./...` exit 0
7. `go vet ./...` exit 0
8. `go test ./... -race` exit 0

REQ 매핑: REQ-MINK-UDM-001, REQ-MINK-UDM-002, REQ-MINK-UDM-006, REQ-MINK-UDM-014

---

## AC-MINK-UDM-006 — Tmp file prefix `.mink-` (REQ-004 trace, D1 fix)

**Given**:
- 본 SPEC Phase 2 완료 상태
- `~/.mink/` 정상 마이그레이션 완료 또는 fresh install (AC-001 또는 AC-003 의 final state)
- baseline: `~/.mink/` 내에 활성 tmp file 0건 (`find ~/.mink -name '.mink-*' -o -name '.goose-*' | wc -l` = 0)

**When**:
- `mink chat "hello"` 또는 `mink` 가 tmp file 을 작성하는 임의의 명령 실행 (예: builtin file write 도구 호출, session create)
- 명령 완료 직후 `find ~/.mink -type f -name '.*-*'` 으로 신규 생성 tmp file 검색

**Then** (모두 만족):
1. 신규 생성된 tmp file 의 basename 이 모두 정규식 `^\.mink-[a-zA-Z0-9]+$` 와 일치 (`find ~/.mink -type f | xargs -I{} basename {} | grep -E '^\.mink-' | wc -l` ≥ 1)
2. `~/.mink/` 내 어떤 신규 tmp file 도 basename `.goose-` 로 시작하지 않는다 (`find ~/.mink -type f -name '.goose-*' | wc -l` = 0)
3. Production source 에서도 `.goose-` literal 미사용 (`grep -rEn '"\.goose-' --include='*.go' --exclude='*_test.go' . | grep -v '^./internal/userpath/legacy\.go:' | wc -l` = 0)
4. Production source 에서 신규 prefix 의 단일 정의 지점은 `internal/userpath/userpath.go` 의 `TempPrefix()` 만 (`grep -rEn '\.mink-' --include='*.go' --exclude='*_test.go' .` 결과 첫 매치가 `userpath.go` 내 `TempPrefix` 함수 정의 라인)

REQ 매핑: REQ-MINK-UDM-004

---

## AC-MINK-UDM-007 — 테스트 파일의 `.goose` literal marker 강제 (REQ-016 trace, D2 fix)

**Given**:
- 본 SPEC Phase 5 (test file 마이그레이션) 완료 상태
- baseline: Phase 5 이전 `grep -rEln '"\.goose' --include='*_test.go' .` 가 다수 파일 매치

**When**:
- 다음 enforcement grep 명령을 본 SPEC PR squash merge 후 실행

**Then** (모두 만족 — D4 적용된 post-filter form):
1. 다음 한 줄 명령의 출력이 0 이어야 한다 (D4 fix — `--exclude=<path>` 가 아닌 post-filter pipeline 사용):
   ```bash
   grep -rEn '"\.goose' --include='*_test.go' . \
     | grep -v '^./internal/userpath/legacy_test\.go:' \
     | grep -v 'MINK migration fallback test' \
     | wc -l
   ```
   = 0
2. `internal/userpath/legacy_test.go` 의 `.goose` literal 매치 라인 수는 ≥ 1 (legacy fallback 검증 자체가 본 파일의 책임)
3. `// MINK migration fallback test` marker 가 존재하는 다른 `*_test.go` 파일이 있다면, 그 파일들의 `.goose` literal 사용처는 모두 해당 marker comment 와 같은 함수 또는 같은 블록 내에 위치해야 한다 (manual review checklist).
4. CI / quality gate 에 이 grep 명령이 등록되어, 위 조건 위반 시 `bash scripts/check-brand.sh` 또는 별도 lint script 가 fail 한다 (`scripts/check-brand.sh` 에 본 검증 라인 추가, exit 1 on violation).

REQ 매핑: REQ-MINK-UDM-016

---

## AC-MINK-UDM-008a — `MINK_HOME` 정상 override (REQ-018 happy path, D3 fix part 1)

**Given**:
- `~/.goose/` 가 존재한다 (data 포함)
- `~/.mink/` 부재
- 사용자가 명시적으로 별도 디렉토리를 지정하기 위해 `MINK_HOME=/tmp/custom-mink` 를 export. `/tmp/custom-mink` 는 writable 디렉토리.

**When**:
- `MINK_HOME=/tmp/custom-mink mink --version` 실행

**Then** (모두 만족, **자동 마이그레이션이 발생하지 않는다**):
1. exit 0
2. `userpath.UserHome()` 의 internal 결과 = `/tmp/custom-mink` (env var override 그대로)
3. `~/.goose/` 변경 0건 (`find ~/.goose -type f -exec sha256sum {} +` baseline 일치). `test -e ~/.goose/` exit 0 잔존.
4. `! test -e ~/.mink/` exit 0 (default home 은 생성되지 않음)
5. `/tmp/custom-mink` 에는 마이그레이션이 발생하지 않는다 (`/tmp/custom-mink/.migrated-from-goose` 부재). 단, `/tmp/custom-mink/` 자체는 `userpath.SubDir` 호출 시 ensure-exists 로 생성될 수 있다 (0700 권한).
6. stderr 에 마이그레이션 알림 출력 0건 (`grep -c '마이그레이션\|Migrated' stderr.log` = 0)

REQ 매핑: REQ-MINK-UDM-018

---

## AC-MINK-UDM-008b — `MINK_HOME` 비정상 값 경계 검증 (REQ-018 negative, D3 fix part 2 — security boundary)

본 AC 는 `MINK_HOME` 이 보안 경계 (security boundary) 라는 점을 명시 검증한다. env var 가 신뢰 불가능한 값일 때 silent fallback 이 발생하면 path traversal / privilege escalation 위험이 있다.

**Given**:
- `~/.goose/` 가 존재한다 (data 포함)
- `~/.mink/` 부재
- 다음 비정상 값 케이스 4개를 순차 검증한다

**When + Then** — 각 케이스 모두 검증:

**Case 1 — Empty string**:
- When: `MINK_HOME="" mink --version` 실행
- Then:
  1. exit non-zero (또는 daemon graceful degrade 의 경우 startup warning + fallback)
  2. stderr 에 typed error 메시지 — `MINK_HOME` 이 빈 값임을 명시 + 사용자 안내 (env unset 또는 valid path 지정)
  3. `~/.goose/` 변경 0건, `~/.mink/` 생성 0건 (silent fallback 차단)
  4. 절대로 `~/.goose/` 가 새 home 으로 채택되지 않는다 (옛 brand 의 디렉토리를 신뢰하면 안 됨)

**Case 2 — 기존 `.goose/` 를 가리키는 값** (정책 위반):
- When: `MINK_HOME="$HOME/.goose" mink --version` 실행
- Then:
  1. exit non-zero. typed error 이름은 `ErrMinkHomeIsLegacyPath` (Run phase 에서 정확 식별자 확정 — 본 SPEC 은 typed error contract 의 existence + semantics 만 명시). 에러 메시지에 `MINK_HOME` 와 `.goose` 두 단어가 모두 포함된다.
  2. 자동 마이그레이션 발생 0건 (REQ-018 가 마이그레이션을 차단)
  3. stderr 안내: 사용자에게 별도 디렉토리를 지정하도록 요청 + 옛 `.goose/` 의 데이터를 새 디렉토리로 migrate 하는 방법 안내

**Case 3 — Path traversal 포함** (`..`):
- When: `MINK_HOME="/tmp/../etc/foo" mink --version` 실행 (또는 `MINK_HOME="$HOME/../../../etc/foo"`)
- Then:
  1. exit non-zero. typed error 이름은 `ErrMinkHomePathTraversal` (Run phase 에서 정확 식별자 확정). 에러 메시지에 `MINK_HOME` 와 `..` 두 token 이 모두 포함된다.
  2. `userpath.UserHome()` 은 cleaned path (`filepath.Clean`) 를 절대 무조건 수용하지 않는다 — 입력 raw 값에 `..` 가 있으면 reject. (방어적 검증, OWASP Path Traversal mitigation)
  3. 어떤 파일도 작성되지 않는다 (resolved target 이 `/etc/foo` 같은 system dir 으로 이동 시도 0건)

**Case 4 — Non-writable path**:
- When: 사전에 `mkdir /tmp/ro-mink && chmod 0400 /tmp/ro-mink` 로 read-only directory 생성. `MINK_HOME=/tmp/ro-mink mink --version` 실행.
- Then:
  1. exit non-zero (CLI) 또는 graceful degrade (daemon, AC-004b 와 일관) — typed error `ErrPermissionDenied` 또는 `ErrReadOnlyFilesystem`
  2. stderr 에 user-actionable error 출력 — 디렉토리 권한 변경 또는 다른 경로 지정 방법 안내
  3. `~/.goose/` 변경 0건, silent fallback 0건 (env var override 의 의도를 존중하되 실패는 명시적으로 보고)
  4. 후속 사용자 개입 (chmod) 후 재실행 시 정상 동작

REQ 매핑: REQ-MINK-UDM-018 (negative paths), REQ-MINK-UDM-013 (typed error contract)

---

## AC-MINK-UDM-009 — 파일 mode bits 보존 (REQ-019 trace, D6 fix — security gate)

본 AC 는 민감 파일의 mode bits 가 copy fallback 경로에서도 보존됨을 명시 검증한다. atomic rename 은 inode 를 옮기므로 mode 가 자동 보존되지만, cross-filesystem copy fallback (REQ-009) 은 명시적 `chmod` 가 없으면 default umask 결과로 채워지는 위험이 있다.

**Given**:
- `~/.goose/` 가 존재하며 다음 mode 가 적용된 민감 파일을 포함한다:
  - `~/.goose/permissions/grants.json` — mode 0600 (사전에 `chmod 600`)
  - `~/.goose/messaging/telegram.db` — mode 0600
  - `~/.goose/mcp-credentials/anthropic.json` — mode 0600
  - `~/.goose/ritual/schedule.json` — mode 0600
- `~/.goose/permissions/` 디렉토리 자체 — mode 0700
- `~/.mink/` 부재
- 마이그레이션이 atomic rename 으로 처리되지 않도록 인위적으로 cross-filesystem 환경 설정 (`~/.mink/` 의 부모를 별도 filesystem 으로 bind-mount, 또는 unit test 의 mock 으로 `os.Rename` 이 `EXDEV` 반환 → copy fallback 강제)

**When**:
- `mink --version` 실행 → `userpath.MigrateOnce()` 가 copy fallback 경로 (REQ-009) 사용

**Then** (모두 만족, mode 보존 검증):
1. 마이그레이션 성공: `test -e ~/.mink/permissions/grants.json` exit 0
2. 각 destination 파일의 mode 가 0600 이하 (never weakened) — D6 보안 게이트:
   - Linux: `[ "$(stat -c '%a' ~/.mink/permissions/grants.json)" = "600" ]` exit 0
   - macOS: `[ "$(stat -f '%Lp' ~/.mink/permissions/grants.json)" = "600" ]` exit 0
   - 동일 검증을 `~/.mink/messaging/telegram.db`, `~/.mink/mcp-credentials/anthropic.json`, `~/.mink/ritual/schedule.json` 에 반복
3. Directory mode 보존 — `~/.mink/permissions/` 자체:
   - Linux: `[ "$(stat -c '%a' ~/.mink/permissions/)" = "700" ]` exit 0
   - macOS: `[ "$(stat -f '%Lp' ~/.mink/permissions/)" = "700" ]` exit 0
4. mode 가 더 제한적 (예: source 가 0644 였는데 dest 가 0600) 인 경우는 허용 — never-weakened 가 invariant.
5. mode 가 더 완화된 (예: source 0600 → dest 0644) 경우 0건 — Phase 1 unit test 에서 `t.Fatal` 보장.
6. ownership (uid/gid) 는 가능한 경우 (same uid context) 보존, cross-uid 시나리오는 silent skip (`chown` 권한이 없는 경우 — 일반 사용자 unit test 환경 default).

REQ 매핑: REQ-MINK-UDM-019, REQ-MINK-UDM-009 (copy fallback 의 부속 검증)

---

## AC-MINK-UDM-010 — Brand marker 부재 시 best-effort 진행 + warning (REQ-017 negative path, D9 fix / R4 trace)

본 AC 는 brand collision risk (third-party `goose` Block AI 가 같은 경로를 쓸 가능성, plan.md R4) 의 안전망 검증.

**Given**:
- `~/.goose/` 가 존재하며 `memory/`, `config.yaml` 등의 데이터를 포함한다
- `~/.goose/` 가 brand marker 를 **포함하지 않는다**:
  - `! test -e ~/.goose/.mink-managed`
  - `~/.goose/config.yaml` 의 모든 키가 generic 이며, MINK 전용 식별자 키 (예: `mink_version`, `mink_managed`) 가 부재
- `~/.mink/` 부재
- `MINK_HOME` 미설정

**When**:
- `mink --version` 실행

**Then** (모두 만족, best-effort fallback):
1. 마이그레이션은 진행된다 (사용자가 강제로 차단하지 않음 — silent abort 는 더 큰 사용자 혼란).
2. stderr 에 다음 best-effort warning 이 정확히 1줄 추가 출력된다 (AC-001 #6 의 마이그레이션 알림과는 별도 줄):
   - Base 예시: `WARN: ~/.goose 디렉토리에서 MINK brand marker 를 찾지 못했습니다. 마이그레이션은 best-effort 로 진행됩니다 (third-party Goose 프로젝트가 이 경로를 사용 중이라면 작업을 중단하고 MINK_HOME 환경 변수로 다른 경로를 지정하세요).`
   - 검증: stderr 출력에 `MINK brand marker` 또는 `best-effort` 문구 포함 + `MINK_HOME` 안내 포함
3. 마이그레이션 후 `~/.mink/.migrated-from-goose` marker 가 생성되되, 내부 field 에 `brand_verified: false` flag (정확 키 이름 Run phase 확정) 가 기록되어 후속 audit 가능. marker 파일 grep 결과 `brand_verified: false` 또는 `brand_verified=false` 매치 ≥ 1.
4. AC-001 의 #1~#5 + #7 (멱등성) 검증은 동일하게 통과 — 마이그레이션 자체는 정상.
5. 동일 명령 재실행 시 best-effort warning 출력 0건 (멱등성, marker file 기반).

REQ 매핑: REQ-MINK-UDM-017 (negative path), plan.md R4 (third-party brand collision)

---

## Edge Case Scenarios

### EC-MINK-UDM-001 — Symlink 감지 시 graceful error

**Given**:
- `~/.goose` 는 symlink 이며 `/Volumes/External/goose-data` 를 가리킨다
- target 도 존재한다 (`/Volumes/External/goose-data/memory/memory.db` 등)
- `~/.mink/` 부재

**When**:
- `mink --version` 실행

**Then**:
1. `~/.goose` symlink 는 그대로 보존 (`test -L ~/.goose` exit 0)
2. symlink target (`/Volumes/External/goose-data`) 의 내용도 그대로 (자동 resolve 후 이동 발생 안 함)
3. exit 코드는 user-actionable error 출력과 함께 non-zero (CLI) 또는 graceful warning + fallback to legacy path (daemon)
4. stderr 메시지: `ERROR: ~/.goose 가 symlink 입니다. 자동 마이그레이션이 안전하지 않습니다. 수동 마이그레이션 가이드: <URL>`. 메시지에 `mink` 또는 `밍크` 포함, 사용자 안내 URL 또는 명령 동반.
5. `~/.mink/` 는 생성되지 않음 (또는 부분 생성된 경우 cleanup)

REQ 매핑: REQ-MINK-UDM-013 (graceful error)

### EC-MINK-UDM-002 — 동시 실행 시 lock 획득 (R5)

**Given**:
- `~/.goose/` 존재, `~/.mink/` 부재
- 두 프로세스 (`mink CLI A` + `minkd daemon B`) 가 거의 동시에 시작

**When**:
- A 와 B 가 모두 `userpath.MigrateOnce(ctx)` 를 호출

**Then**:
1. 둘 중 하나의 프로세스 (편의상 A 로 가정) 가 `~/.mink/.migration.lock` 을 획득 + 마이그레이션 수행
2. 다른 프로세스 (B) 는 lock 획득 실패 → blocking wait (max 30s)
3. A 가 마이그레이션 완료 + lock 해제 후, B 는 lock 재시도 → post-migration state 감지 → no-op
4. 마이그레이션은 **정확히 1회** 발생 (`find ~/.mink/.migrated-from-goose -newer ~/.profile` 또는 timestamp 검증)
5. 데이터 손실 0건, double-migration 0건
6. A 와 B 모두 정상 종료 (각자 의도한 명령 실행 성공)

REQ 매핑: REQ-MINK-UDM-011

### EC-MINK-UDM-003 — Mid-copy crash 후 next-run recovery

**Given**:
- 이전 실행에서 마이그레이션 중 process kill (SIGKILL) → `~/.mink/` 부분 생성 + `~/.mink/.migration.lock` 잔존 + `~/.goose/` 여전 존재
- lock 파일 내 PID 가 더 이상 실행 중이지 않음 (stale lock)

**When**:
- 새 `mink` process 가 시작

**Then**:
1. stale lock 감지 (`os.Kill(pid, 0)` returns error → process dead)
2. `~/.mink/` 부분 디렉토리 전체 삭제 (cleanup)
3. lock 파일 삭제
4. 마이그레이션 재시작 (clean state 에서)
5. 정상 완료 시 AC-MINK-UDM-001 과 동일한 final state 도달
6. stderr 에 `INFO: 이전 마이그레이션이 중단되었습니다. 다시 시작합니다.` 출력. `mink` 단어 포함.

REQ 매핑: REQ-MINK-UDM-011, REQ-MINK-UDM-015

### EC-MINK-UDM-004 — Project-local lazy migration

**Given**:
- `~/.mink/` 는 이미 마이그레이션 완료 상태 (user-home 마이그레이션 끝)
- `<cwd>/.goose/` 가 존재 (project-local 디렉토리)
- `<cwd>/.mink/` 부재

**When**:
- 사용자가 해당 cwd 에서 `mink chat` (또는 project-scoped command) 실행
- 첫 호출 시 `userpath.ProjectLocal(cwd)` 가 invoke 됨

**Then**:
1. `<cwd>/.goose/` → `<cwd>/.mink/` 마이그레이션 발생 (same algorithm as user-home)
2. `<cwd>/.mink/.migrated-from-goose` marker 생성
3. stderr 에 1줄 알림: `INFO: 프로젝트 디렉토리 ./.goose 에서 ./.mink 로 마이그레이션되었습니다.`. AC-001 #6 와 동일하게 `goose` 단어 미포함 검증은 적용되지 않음 (옛 경로 안내가 메시지의 핵심 정보), 대신 `mink` 또는 `밍크` 포함 검증만 적용.
4. 같은 cwd 에서 두 번째 명령 실행 시 알림 0건 (멱등성)
5. 다른 cwd 에서 `mink chat` 실행 시 그 cwd 의 `./.goose/` 가 있다면 동일 마이그레이션 발생 (per-cwd 독립)

REQ 매핑: REQ-MINK-UDM-010

---

## Quality Gate Criteria

본 SPEC 머지 시 다음 모두 만족해야 한다:

| Gate | Criteria | 검증 명령 (Linux/macOS portable) |
|------|----------|----------|
| Coverage (전체) | ≥ 85% (회귀 없음) | `go test ./... -coverprofile=cover.out && go tool cover -func=cover.out` |
| Coverage (`internal/userpath/**`) | ≥ 90% | `go test ./internal/userpath/... -coverprofile=u.out && go tool cover -func=u.out \| grep total` |
| Race detector | clean | `go test ./... -race` |
| Lint | 0 warnings | `golangci-lint run` |
| Vet | 0 errors | `go vet ./...` |
| Build | exit 0 | `go build ./...` |
| Brand-lint | exit 0 | `bash scripts/check-brand.sh` |
| LSP | 0 errors, 0 warnings | LSP report |
| Path-resolver enforcement (D4 fix — post-filter form) | 0 violations | `grep -rEn '"\.goose' --include='*.go' --exclude='*_test.go' . \| grep -v '^./internal/userpath/legacy\.go:' \| wc -l` = 0 |
| Test file marker enforcement (REQ-016, D2 fix) | 0 violations | `grep -rEn '"\.goose' --include='*_test.go' . \| grep -v '^./internal/userpath/legacy_test\.go:' \| grep -v 'MINK migration fallback test' \| wc -l` = 0 |
| Mode bits preservation (REQ-019, D6 fix) | 모든 민감 파일 dest mode ≤ source mode | AC-009 의 stat 검증 (Linux: `stat -c '%a'` / macOS: `stat -f '%Lp'`) |
| `MINK_HOME` boundary (REQ-018, D3 fix) | 4 negative cases reject | AC-008b 의 unit test 4 케이스 통과 |
| TRUST 5 Tested | All AC verified | acceptance test report |
| TRUST 5 Secured | OWASP path traversal (AC-008b Case 3) | `internal/userpath/userpath.go` 의 path join 시 `filepath.Clean` + raw `..` reject |

---

## Definition of Done

본 SPEC 은 다음 모두 만족 시 `status: completed` 로 전환한다:

- [ ] Phase 1-6 모두 commit + squash merge 완료
- [ ] AC-MINK-UDM-001 ~ AC-MINK-UDM-010 (12 main scenarios — AC-001, 002, 003, 004a, 004b, 005, 006, 007, 008a, 008b, 009, 010; AC-004/008 split 포함) 모두 binary verified
- [ ] EC-MINK-UDM-001 ~ EC-MINK-UDM-004 모두 manual smoke test 또는 integration test 통과
- [ ] Quality gate 14개 항목 모두 통과 (Coverage전체 / Coverage userpath / Race / Lint / Vet / Build / Brand-lint / LSP / Path-resolver / Test marker / Mode bits / MINK_HOME boundary / TRUST Tested / TRUST Secured; v0.1.1 신규 gate 3개 — test marker, mode bits, MINK_HOME boundary 포함)
- [ ] CHANGELOG `[Unreleased]` entry 작성 (BREAKING + auto-migration 완화 명시)
- [ ] README `## Migration from .goose` section 작성
- [ ] `.moai/project/structure.md` 갱신 (신규 `internal/userpath/` 등록)
- [ ] `.moai/project/codemaps/internal-userpath.md` 신설
- [ ] 본 SPEC HISTORY 에 v0.2.0 row 추가 (implemented marker)
- [ ] sync phase 의 `/moai sync SPEC-MINK-USERDATA-MIGRATE-001` 실행

---

Version: 0.1.2
Status: draft
Last Updated: 2026-05-13
