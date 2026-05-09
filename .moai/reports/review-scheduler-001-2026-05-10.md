# SCHEDULER-001 v0.2.0 4-Perspective Code Review

- **Reviewed**: 2026-05-10
- **Reviewer**: manager-quality (single-agent mode, 4-perspective sequential)
- **Invocation**: `/moai:review` → SCHEDULER-001 풀스캔 + 4-perspective 균형
- **Scope**: commits `ddee87f..cd35297` (PRs #133/#135/#136/#137/#138/#139)
- **Stats**: 26 files, +4450 / -20 lines
- **Code root**: `internal/ritual/scheduler/` (22 new files) + `internal/hook/types.go` (+21)

---

## 요약 (Executive Summary)

| 항목 | 값 |
|---|---|
| 종합 평가 | **WARNING** |
| TRUST 5 점수 | **4/5** |
| Critical | **0건** |
| Warning | **5건** |
| Suggestion | **6건** |
| `go test -race` | **PASS** (data race 0건, 9.719s) |
| `go vet` | **CLEAN** |
| Coverage | **84.1%** (목표 85% 대비 -0.9%p) |

**한 줄 결론**: Critical 없고 race-free. 즉시 배포 차단 사유는 없으나 Stop() 재진입 동시성, AC-007 MEMORY-001 미완, HolidayName i18n debt 3건은 다음 SPEC 진입 전 처리 권장.

---

## go test -race 출력 (요지)

```
ok  github.com/modu-ai/goose/internal/ritual/scheduler  9.719s
```

Race detector: **PASS**. Data race 감지 0건.

---

## Critical Issues (즉시 조치 필요)

없음.

---

## Warnings (권장 수정)

### W1 [CONCURRENCY] `scheduler.go:344-348` — Stop() 재진입 시 workerDone double-close 위험

`Stop()`은 `workerDone != nil` 확인 후 `select { case <-s.workerDone: ... default: close(s.workerDone) }` 패턴으로 double-close를 방어하고 있으나, `Stop()`이 여러 고루틴에서 동시에 호출될 경우 두 호출이 동시에 `default` 분기에 진입하면 `close`가 두 번 실행된다. 현재 테스트는 단일 고루틴 호출만 검증한다. 또한 `s.cron`이 nil 이 아닌 상태에서 `Stop()` 중 `Start()`가 동시에 불리면 `s.cron` 포인터도 무보호 경쟁 상태다.

**근거**: `scheduler.go:331-364` `Stop()` 구현. `s.cron`은 `atomic.Value` 가 아닌 일반 포인터. Scheduler 자체에 state mutation용 mutex가 없다.

**제안 조치**: `Stop()` 진입부에 `sync.Once` 또는 CAS 패턴 추가. `s.cron` 을 atomic pointer 또는 단일 뮤텍스로 보호. 동시 Stop()/Start() reproduction test 추가.

---

### W2 [QUALITY/TESTED] `scheduler_test.go` — AC-SCHED-007 (MEMORY-001 round-trip) 미검증

20개 AC 중 AC-SCHED-007만 `scheduler_test.go`에 직접 테스트가 없다. `TestPersistAndReload`(AC-003)가 파일 round-trip을 커버하지만, AC-007의 Given/Then 조건("MEMORY-001 facts 테이블의 `ritual_schedule` 네임스페이스 동일 3 엔트리 존재")은 구현 및 테스트 모두 미완성이다.

**근거**: `acceptance.md`의 AC-SCHED-007에 MEMORY-001 persistence 요구사항이 명시됨. `suppression.go:18` 주석에도 "future implementations may persist via MEMORY-001" 표기.

**제안 조치**: 해당 부분이 P5+ 연동 사항임을 `@MX:TODO`로 명시. AC-007을 "PARTIAL" 상태로 acceptance.md에 기록. MEMORY-001 SPEC 완성 후 round-trip integration test 추가.

---

### W3 [QUALITY/READABLE] `scheduler.go:368-502` — makeCallback 135줄, 순환복잡도 높음

단일 클로저가 TZ pause 체크 → 주말 skip → 공휴일 skip → 야간 override warn → backoff → suppression key 체크 → 이벤트 빌드 → firedKeys 기록 → workerCh 큐잉의 8단계 책임을 모두 수행한다. 조건 분기(if-return)가 8개 이상으로 `@MX:WARN` 기준(cyclomatic ≥ 15)에 근접한다.

**근거**: `scheduler.go:368-502` (135줄). 현재 `@MX:WARN`은 goroutine 생명주기 기준만 존재.

**제안 조치**: `checkSkipConditions(rt, localTime, now) bool` 과 `buildEvent(rt, localTime, now) ScheduledEvent` 로 추출하여 가독성 향상. 복잡도 경고용 `@MX:WARN complexity_15+` 태그 추가.

---

### W4 [SECURITY/i18n] `holiday_data.go:37-191` — 한국어 문자열 리터럴이 공개 API로 직접 노출

`HolidayName`은 `ScheduledEvent` 필드로 serialized되어 hook dispatcher를 통해 외부로 전달된다. "설날 전날", "추석 대체공휴일" 등 한국어 문자열이 고정 리터럴로 하드코딩되어 있어 국제화(i18n) 불가, 로그 파싱 시 예상치 못한 인코딩 문제 야기 가능성이 있다.

**근거**: `holiday_data.go:41-43`, `events.go:43` `HolidayName string` 필드.

**제안 조치**: `HolidayName`을 영문 canonical key("seollal_eve", "chuseok_substitute" 등)로 변환하거나, `LocalizedName map[string]string` 구조 추가. 단기적으로는 English alias 추가.

---

### W5 [QUALITY/READABLE] `internal/hook/types.go:1-115` — 한국어 패키지 godoc, 인라인 주석 다수

`CLAUDE.local.md §2.5`에 의해 코드 주석은 전체 영어 의무화됨. `hook/types.go` 상단 godoc이 한국어, 인라인 주석(기념일, 대체공휴일, 디스패처, 핸들러 등) 다수 한국어.

**근거**: `hook/types.go:1-10`, 88~115 라인 구조체 주석. `language.yaml: code_comments: en`.

**제안 조치**: CLAUDE.local.md §2.5에 따라 소급 영문화. `hook/types.go`는 scheduler와의 계약 파일이므로 PR 기회에 함께 정리 권장. (참고: `holiday.go:9, holiday.go:38`도 한국어 godoc 1-2줄 — 동일 작업에 묶을 것)

---

## Suggestions (nice to have)

### S1 [SECURITY] `suppression.go:97-122` — Mark() 중 .tmp 파일 잔류 가능성

`os.WriteFile(tmp, ...)` 후 `os.Rename(tmp, s.path)` 사이에서 프로세스가 비정상 종료되면 `.tmp` 파일이 잔류한다. 재시작 시 `.tmp` 파일 자동 정리 로직이 없다. 파일 내용은 안전(data는 이미 직렬화 완료)하지만 디렉터리 오염은 발생한다. `persist.go:58-64`도 동일 패턴이나 공유. `NewFilePersister` 또는 `NewJSONFiredKeyStore` 생성 시 `.tmp` 잔류 파일 정리 1-liner 추가 제안.

### S2 [PERFORMANCE] `backoff.go:43-70` — ShouldDefer 내 atomic 없는 count 읽기

쓰기(RecordDefer)와 읽기(ShouldDefer)가 별개 락으로 분리되어 있어 count 갱신과 read 사이의 TOCTOU 가능성 존재 — 이는 race detector에서도 잡히지 않는다(각각 락 보유 후 접근). 다만 "cron callbacks only enqueue"(단일 워커 직렬화) 설계로 실질 위험은 낮다. 주석으로 TOCTOU 허용 근거("single-cron-worker serialization")를 명시하면 다음 개발자의 혼란 방지.

### S3 [UX] `config.go:189-203` — checkQuietHours 파싱 오류 무음 처리

`checkQuietHours("abc:xx")` 는 `strconv.Atoi` 실패 시 `return nil`을 반환한다. 즉 잘못된 형식 클럭 문자열은 조용히 무시되고 `Validate()`가 PASS된다. 이후 `parseClock()`이 Start()에서 해당 오류를 잡지만, Validate()와 Start() 사이 정책 불일치가 혼란을 줄 수 있다. `checkQuietHours`를 `parseClock` 기반으로 리팩터링하거나, 파싱 오류도 에러로 전파.

### S4 [QUALITY] `learner.go:120-141` — recordPeak SupportingDays 계산 단위 불일치

연속 trailing peak 계산은 `peakHour * 60` 과 `hist[i] * 60` 차이를 비교한다. hist에는 시(hour, 0-23)가 저장되고, 비교 단위는 분(minutes)이므로 `absInt((hist[i]-peakHour)*60) <= 15` 는 실제로 `|deltaHour| * 60 <= 15`이다 — 같은 정시(hour)끼리만 일치함을 의미한다. 즉 "08:30 관찰, 08:00 현재 설정"의 경우 delta=0.5h=30min이지만 hist는 hour(8)만 저장하므로 proximity 계산이 integer granularity로만 이루어진다. hist를 분(minutes) 단위로 저장하거나, 테스트 케이스에서 sub-hour proximity를 명시적으로 검증.

### S5 [UX] `scheduler.go:264-268` — Start()의 state=Running 설정이 replayMissedEvents 전 실행

`engine.Start()` → `s.state.Store(Running)` → `s.replayMissedEvents(ctx, rituals)` 순. `replayMissedEvents`가 오래 걸리거나 실패해도 state는 이미 Running. 외부 관찰자는 replay가 완료되지 않은 상태에서 Running을 보게 된다. `replayMissedEvents` 완료 후 state 설정 또는, replay를 비동기로 처리하고 별도 상태 플래그 추가.

### S6 [SUGGESTION] `holiday_data.go` — 2029년 이후 커버리지 없음, 만료 명시 부재

2026~2028 3년치 데이터만 존재. 2029년 1월 1일 이후에는 `Lookup`이 항상 `HolidayInfo{}`를 반환한다. 연도 경계를 감지하는 경고 로직이나, `// Valid through: 2028-12-31` 형태의 명시적 expiry 주석이 없다. `@MX:NOTE`로 유효 기간 명시 권장.

---

## Perspective 평가

### Security

| 검증 항목 | 결과 | 발견 사항 |
|---|---|---|
| cron.go DoS / 무제한 재귀 | PASS | `HH:MM` 파싱은 split+atoi만 사용. robfig/cron은 신뢰된 프레임워크 |
| persist.go path traversal | PASS | `filepath.Join` 사용, 사용자 입력 경로 없음. 디렉터리 0700, 파일 0600 |
| persist.go atomic write | PASS | `.tmp` → rename 패턴 올바르게 구현 |
| suppression.go file mode | PASS | `os.WriteFile(tmp, data, 0600)` — 600 적절 |
| holiday_data.go 정적 데이터 | PASS | 네트워크 fetch 없음, 정적 빌드. 스텔니스/포이즈닝 위험 없음 |
| logfields.go PII 노출 | PASS | 이벤트 이름, 시간, TZ, bool 값만 로깅. 사용자 개인 정보 없음 |
| 한국어 문자열 API 노출 | WARN | `HolidayName`이 한국어 리터럴 직접 전달 (W4) |
| clock manipulation | PASS | `clockwork` 추상화 사용, FakeClock 격리됨 |

**평가: WARN** (Critical 없음, 한국어 API 노출이 i18n 설계 debt)

### Performance + Concurrency

| 검증 항목 | 결과 | 발견 사항 |
|---|---|---|
| go test -race | **PASS** | 0 race conditions detected |
| goroutine 종료 경로 | PASS | `workerDone` channel + `workerWG.Wait()` + 3s timeout |
| mutex coverage | PASS | `BackoffManager.deferCounts`, `JSONFiredKeyStore.entries`, `TimezoneDetector` 모두 보호 |
| Stop() 재진입 (concurrent) | **WARN** | workerDone double-close + `s.cron` 무보호 포인터 (W1) |
| suppression O(N) | PASS | map lookup O(1), 비선형 성장 없음 |
| holiday lookup | PASS | `map[holidayKey]HolidayInfo` O(1) |
| backoff 산술 오버플로 | PASS | `count * activeWindow`는 int * Duration, 실용 범위 내 안전 |
| workerCh 버퍼 포화 | PASS | 버퍼 32, 포화 시 WARN 로그 + drop (blocking 없음) |
| PatternLearner.history 메모리 | PASS | `RollingWindowDays` cap으로 상한됨 |

**평가: WARN** (race-free이나 Stop() 재진입 시 잠재적 panic)

### Quality (TRUST 5)

| 차원 | 평가 | 비고 |
|---|---|---|
| Tested | **WARNING** | 커버리지 84.1% (목표 85% 미달 0.9%p). AC-007 MEMORY-001 연동 미검증 |
| Readable | **PASS** | 전반적으로 명확. `makeCallback` 135줄 개선 권장 (W3) |
| Unified | **PASS** | `errors.Join`, `fmt.Errorf("%w")` 일관, sentinel error 명시 |
| Secured | **PASS** | panic 없음, recover는 robfig/cron middleware로 위임 |
| Trackable | **PASS** | @MX 태그 13개, 모두 `[AUTO]` + `@MX:REASON` 구비 |

**평가: WARNING** (84.1% 미달, AC-007 미완)

### UX (Operator + Developer)

| 검증 항목 | 결과 | 비고 |
|---|---|---|
| 에러 메시지 언어 | PASS | `config.go`, `cron.go`, `persist.go`, `suppression.go` 모두 영어 |
| config.go 필드명 | PASS | Go 관례 준수, Zero-value 안전, effective() 패턴 일관 |
| Suppression 동작 예측성 | PASS | BuildFiredKey 문서화, TZ-aware 명시, Log schema 7-field AC 검증 |
| hook/types.go breaking change | PASS | 5개 상수 추가 전용, 기존 상수 수정 없음, HookEventNames() 갱신 완료 |
| hook_test.go 29개 검증 | PASS | `TestHookEventNames_Exactly24`가 실제로 29개를 assert |
| Replay/FastForward 관찰 가능성 | PASS | IsReplay, DelayMinutes 필드 공개. FastForward build-tag 격리 |
| Start() 전 state Running 설정 | WARN | replayMissedEvents 완료 전 Running 반환 (S5) |

**평가: PASS** (구조 건전, 운영 관찰 가능성 양호)

---

## @MX tag 준수 표

| File:Line | 함수/타입 | 태그 상태 | 비고 |
|---|---|---|---|
| `scheduler.go:40` | `Scheduler` struct | @MX:ANCHOR OK | ✓ |
| `scheduler.go:70` | `workerCh` field | @MX:WARN OK | ✓ |
| `scheduler.go:154` | `withCronSpecOverride` | @MX:WARN OK | ✓ |
| `scheduler.go:286` | `runWorker` | @MX:WARN OK | ✓ |
| `scheduler.go:612` | `runDailyLearner` | @MX:NOTE OK | ✓ |
| `scheduler.go:368` | `makeCallback` | **누락: @MX:WARN** | cyclomatic 분기 8+, W3와 함께 처리 |
| `backoff.go:14` | `BackoffManager` | @MX:ANCHOR OK | ✓ |
| `holiday.go:37` | `KoreanHolidayProvider` | @MX:ANCHOR OK | ✓ |
| `learner.go:24` | `PatternLearner` | @MX:ANCHOR OK | ✓ |
| `logfields.go:15` | `EmitFireLog` | @MX:NOTE OK | ✓ |
| `pattern.go:25` | `PatternReader` interface | @MX:NOTE OK | ✓ |
| `suppression.go:16` | `FiredKeyStore` interface | @MX:NOTE OK | ✓ |
| `timezone.go:24` | `TimezoneDetector` | @MX:ANCHOR OK | ✓ |
| `activity.go:10` | `ActivityClock` | @MX:NOTE OK | ✓ |

누락 1건: `scheduler.go:368 makeCallback` — W3 리팩터링 시 함께 추가.

---

## 한국어 주석 검출 (CLAUDE.local.md §2.5 위반)

| File | Line | 성격 | 조치 |
|---|---|---|---|
| `internal/hook/types.go` | 1-10, 88-115 | 패키지 godoc + 구조체 인라인 주석 | W5 영문화 |
| `internal/ritual/scheduler/holiday.go` | 9, 38 | `(대체공휴일)` 괄호 병기 godoc | W5와 묶음 |
| `internal/ritual/scheduler/holiday_data.go` | 전체 | 공휴일 이름 리터럴 + 법령 근거 주석 | W4(API)와 W5(주석) 두 축 모두 영향 |

---

## 결론

**종합 판정: WARNING**

- Critical 없음 — 즉시 배포 차단 사항 부재
- 5건 Warning 중 W1 Stop() 재진입 동시성이 가장 실질적 위험 (단일 호출 시 무해, 향후 다중 Stop() 호출 시 panic 가능)
- 커버리지 84.1%는 목표 85% 대비 0.9%p 미달 — 다음 SPEC에서 AC-007 MEMORY-001 연동 구현 시 자연 해소 예상
- W4 holiday_data.go 한국어 문자열 리터럴의 HolidayName API 노출은 i18n 설계 debt으로 P5+ 이전에 추적 필요
- `go test -race` PASS, `go vet` CLEAN — 코드 품질 기반은 견고

**즉시 수정 권장**: W1 Stop() 동시성 결함 (sync.Once 적용, 낮은 비용)

**다음 SPEC에서 처리**: W2 AC-007 MEMORY-001 연동 완성, W4 HolidayName 영문 key 전환, W3 makeCallback 분리 리팩터링

**차순위**: W5 hook/types.go + holiday.go 주석 영문화, S3 checkQuietHours 파싱 오류 전파, S5 Start() state 순서 조정

---

Generated by `/moai:review` (single-agent mode, manager-quality)
Source agent: `aa27f7260e2466736`
