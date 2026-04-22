# Research — SPEC-GOOSE-SCHEDULER-001

## 1. Cron 라이브러리 결정

### 후보 비교

| 라이브러리 | 장점 | 단점 | 결정 |
|----------|------|------|------|
| `robfig/cron/v3` | 커뮤니티 1위, 10k+ stars, location-aware `cron.WithLocation`, panic-safe wrapper 제공 | `@every` 외 custom trigger 한정 | **채택** |
| `go-co-op/gocron` | Fluent API | cron 스펙 표준 미지원(자체 syntax), 테스트용 mock 부족 | 기각 |
| stdlib `time.Ticker` | 의존성 0 | 복잡한 스케줄 직접 구현 부담 | 기각 |

### robfig/cron/v3 사용 패턴

```go
import "github.com/robfig/cron/v3"

loc, _ := time.LoadLocation("Asia/Seoul")
c := cron.New(cron.WithLocation(loc), cron.WithChain(
    cron.Recover(logger),           // panic guard
    cron.SkipIfStillRunning(logger),// dup guard
))
c.AddFunc("30 7 * * *", func() { s.fireEvent("morning") })
c.Start()
```

주의사항:
- `cron.WithLocation`은 **모든 엔트리에 동일 적용**. 사용자별 TZ 분리 시 Scheduler per-user 분리 필요(본 SPEC은 단일 사용자 전제).
- `cron.WithSeconds()` 사용 시 6-field spec. 본 SPEC은 5-field (minute 단위) 유지.

## 2. 한국 공휴일 정확도

`rickar/cal/v2/kr` 검증 (2026년 기준):

| 공휴일 | 날짜 | cal/v2 인식 | 비고 |
|-------|------|-----------|------|
| 신정 | 1/1 | O | 고정 |
| 설날 연휴 | 2/16-18 | O | 음력 1/1 ± 1 |
| 삼일절 | 3/1 | O | 고정 |
| 어린이날 | 5/5 | O | 고정 |
| 부처님 오신 날 | 음력 4/8 → 5/24 | O | |
| 현충일 | 6/6 | O | 고정 |
| 광복절 | 8/15 | O | 고정 |
| 추석 | 음력 8/15 ± 1 → 9/25-27 | O | |
| 개천절 | 10/3 | O | 고정 |
| 한글날 | 10/9 | O | 고정 |
| 크리스마스 | 12/25 | O | 고정 |
| 대체공휴일 | 동적 | O | 주말 겹침 시 다음 평일 |

**리스크**: 향후 법 개정으로 신규 공휴일 추가 시 cal/v2 업데이트 지연 가능 → custom override YAML 경로 제공.

## 3. PatternLearner 알고리즘

### 3.1 입력

INSIGHTS-001 `ActivityPattern.ByHour` (24-bucket 히스토그램). 매일 증분 제공.

### 3.2 Ritual Kind 매핑

| RitualKind | 예상 시간대 | 감지 로직 |
|-----------|----------|----------|
| Morning (기상) | 06~10시 | 연속 4시간 이상 활동 없다가 첫 turn |
| Breakfast | 06~10시 | Morning + 30~90분 경과 |
| Lunch | 11~14시 | 해당 구간 activity peak |
| Dinner | 17~21시 | 해당 구간 activity peak |
| Evening (취침 전) | 21~24시 | 마지막 turn 시간 - 30분 |

### 3.3 신뢰도 공식

```
confidence(N days, σ_hours) = N / (N + σ_hours² * 0.5)

예:
  N=7일, σ=0.2시간 (일정)  → 7 / (7 + 0.02) ≈ 0.997
  N=7일, σ=1.5시간 (불규칙) → 7 / (7 + 1.125) ≈ 0.862
  N=3일, σ=0.1시간         → 3 / (3 + 0.005) ≈ 0.998
```

INSIGHTS-001 §6.5의 Bayesian confidence 재사용.

### 3.4 Rolling Window 갱신

매일 03:00 local (사용자 수면 중) 에 전일 ActivityPattern을 ingest. 7일 rolling window. 변화가 ±30분 초과 시 RitualTimeProposal 발생.

## 4. Backoff Heuristic

### 4.1 검증 시나리오

- **활발한 작업 세션 중 (0분 이내 turn)**: 지연.
- **3분 이내 turn**: 지연.
- **10분 이내 turn**: 지연.
- **10분 이상 turn 없음**: 즉시 emit.

### 4.2 Defer 최대 횟수

3회까지 defer (최대 30분 지연). 이후 강제 emit. 사용자가 계속 대화 중이라도 "아침 브리핑 시간이 많이 지났어요" 같은 tone으로 자연스럽게 이어지도록 BRIEFING-001 에 힌트 전달.

## 5. Quiet Hours 정책

### 5.1 기본값

`[23:00, 06:00]` local — HARD floor. `allow_nighttime: true` 로만 해제.

### 5.2 야간 근무자 대응

PatternLearner가 연속 7일 이상 "활동 피크 22:00-04:00" 감지 시:
- AskUserQuestion notification: "야간 활동이 주를 이루는 것 같아요. 야간 모드로 전환할까요?"
- 사용자 승인 시 `allow_nighttime=true` + 리추얼 시간 전체 shift.

## 6. Process Restart 중복 방지

### 6.1 SuppressionKey

```
key = fmt.Sprintf("%s:%s:%s", event, userLocalDate, tz)
예: "MorningBriefingTime:2026-04-22:Asia/Seoul"
```

MEMORY-001 `facts` 테이블의 `ritual_fired` 네임스페이스에 key 저장. cron 발화 전 key 존재 확인.

### 6.2 Missed Event Replay

- 1시간 이하 지체 (cron 스케줄 후 1시간 내 프로세스 부활): 즉시 1회 emit + "조금 늦었지만..." flag.
- 1시간 초과: 스킵, zap INFO 로그.

## 7. 테스트 전략

### 7.1 Mock Clock

`clockwork.Clock` (jonboulle/clockwork) 채택. `time.Now()`를 mock 가능하게 DI.

### 7.2 FastForward API

테스트 전용:

```go
func (s *Scheduler) FastForward(d time.Duration) {
    s.clock.Advance(d)
    time.Sleep(100 * time.Millisecond) // tick 전파
}
```

### 7.3 통합 테스트

실제 `robfig/cron/v3` 인스턴스 + mock hook dispatcher로 10초 타임라인 압축 검증.

## 8. 한국 시장 특화 고려

1. **설날·추석 연휴 강화**: IsHoliday=true + HolidayName="설날 연휴" → BRIEFING이 세뱃돈·가족 관련 톤.
2. **학생 모드**: `persona.occupation=="student"` + 수능 시험일 (매년 11월 셋째 목요일) 자동 감지 → 시험 전날 격려, 시험 당일 quiet mode.
3. **군대 시간표**: `persona.military_service=true` → 06:00 기상, 22:00 취침 고정.

## 9. 오픈 이슈

1. Cron 엔트리 교체 시 in-flight trigger 처리 방식 (현재 정책: 신규 엔트리는 다음 틱부터).
2. Multi-device 동기화 (사용자가 PC와 모바일에서 동시에 goosed 실행) → 양쪽이 같은 MEMORY-001을 공유한다면 SuppressionKey가 중복 차단하지만, 분리된 MEMORY는 문제 발생. A2A-001 범위.
3. 여행 시 TZ shift 감지 정확도 — GPS/IP 기반 detection 불포함(이 SPEC), 시스템 TZ 변경만 감지.

## 10. 참고 문헌

- Cron timing: https://datatracker.ietf.org/doc/html/rfc5545 (iCalendar RRULE)
- Korean Statutory Holidays: https://www.law.go.kr/법령/관공서의공휴일에관한규정
- IANA TZ: https://data.iana.org/time-zones/tzdb/
