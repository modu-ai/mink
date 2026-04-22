# Research — SPEC-GOOSE-RITUAL-001

## 1. Bond Score 공식 설계

### 1.1 목표

- 사용자가 리추얼 완수 → 즉시 가시적 보상
- Evening journal 이 가장 높은 가중치 (가장 감정적·진정성 필요)
- Full day 달성 시 extra bonus
- 생일 등 특별일 2x multiplier

### 1.2 점수 표

| Ritual | Base | Full | Partial | None |
|--------|-----|------|---------|------|
| Morning briefing | 1.0 | 1.0 | 0.5 | 0 |
| Breakfast | 0.5 | 0.5 | 0.25 | 0 |
| Lunch | 0.5 | 0.5 | 0.25 | 0 |
| Dinner | 0.5 | 0.5 | 0.25 | 0 |
| Evening journal | 2.0 | 2.0 | 1.0 | 0 |
| **Full day bonus** | +1.0 | | | |
| **Birthday 2x** | × 2 | | | |

최대 하루 = (1+0.5×3+2) + 1 = 5.5 (생일 = 11.0)

### 1.3 Quality 판정

- Morning "full": briefing read + optional interaction 1회 (follow-up question 등)
- Morning "partial": briefing rendered but no interaction
- Meal "full": meal log + medication record (둘 다)
- Meal "partial": meal log 만 또는 medication record 만
- Evening "full": 일기 150자 이상 + emoji mood
- Evening "partial": 일기 150자 미만 또는 emoji mood 만

## 2. Streak 심리학

### 2.1 연구

- Duolingo: streak 유지가 retention 에 지대한 영향 (7일 streak 사용자 D30 = 60%+)
- Snapchat: streak 유지 desire 로 daily login 증가
- 부작용: streak 끊길 때 심리적 타격 (일부 연구)

### 2.2 Ethical Streak Design

Octalysis Group guideline 기반:
- Freeze day: 월 2회 "streak 보호 일" 제공 (v0.2+)
- Guilt-free break: 끊겨도 "괜찮아요, 다시 시작해요"
- 지속 가능성: streak 유지 자체가 목표가 아닌 well-being 수단
- 강요 금지: push notification 남발 금지 (HEALTH/JOURNAL 일일 1회만)

### 2.3 Streak 정의

**최소 조건**: Morning OR Evening 중 하나라도 "responded" + quality ≥ partial.

Meal 은 누락 허용 (외식·단식 등 일상 변수). Morning+Evening 중 1개만으로도 streak 유지.

## 3. Milestone 심리학

### 3.1 기념 이벤트

| Days | Name | Message |
|------|-----|---------|
| 3 | 첫 3일 | "3일 연속 함께해주셨네요. 작은 시작이 큰 변화로 이어져요." |
| 7 | 일주일 | "벌써 일주일이에요! 🎊 습관이 만들어지는 소리 들리세요?" |
| 14 | 2주 | "2주 완수! 이제 리듬이 느껴져요." |
| 30 | 한 달 | "한 달 🏆 축하해요! 대단한 성취예요." |
| 100 | 100일 | "100일. 말로 못할 자랑스러움이에요." |
| 365 | 1년 | "1년! 우리, 정말 오래 함께했네요. 💝" |

### 3.2 Birthday

생일 ritual:
- 모든 ritual 에 "생일 축하합니다 🎂" prefix
- Bond score 2x
- 특별 메시지: "goos님의 생일을 함께 보낼 수 있어서 기뻐요."

### 3.3 Anniversary

사용자의 GOOSE 사용 기념일 (첫 사용일 + 1년, 2년):
- 자동 감지
- "우리 함께한 지 1년이 되는 날이에요." 축하

## 4. Mood-Adaptive Strategy

### 4.1 입력

INSIGHTS-001의 최근 3-7일 VAD 평균.

### 4.2 매핑 테이블

| 조건 | RitualStyleOverride |
|------|-------------------|
| valence < 0.3 (지속 부정) | tone="gentle", length="short", skip_fortune=true |
| valence ≥ 0.3 AND arousal < 0.3 (평온) | tone="warm", length="medium" |
| valence ≥ 0.6 AND arousal ≥ 0.6 (활기) | tone="playful", length="medium", emoji_level=3 |
| valence 급변 (표준편차 > 0.3) | tone="stable", length="medium" (안정성 제공) |

### 4.3 사용자 override

`config.rituals.force_style: {tone: "professional"}` 같이 고정 가능. 이 경우 auto-adapt 비활성.

### 4.4 검증

Mock 시나리오:
- 사용자 A: 7일 평균 valence=0.2 → gentle + short → 브리핑이 "오늘 무리하지 마세요" 같은 톤
- 사용자 B: 사일 평균 valence=0.85 → playful + emoji → "굿모닝~ ✨"

## 5. Nudge 설계

### 5.1 발화 조건 matrix

| 조건 | Nudge 메시지 | 우선순위 |
|------|-----------|--------|
| Evening 3일 연속 skip | "요즘 저녁에 못 보인 것 같아 조금 걱정됐어요. 잘 지내시죠?" | 10 |
| Streak 어제 끊김 | "어제 잠시 쉬었네요. 오늘 또 시작해봐요, 부담 없이." | 5 |
| 1주일 이상 Morning skip | "최근 아침 브리핑을 안 받으셨어요. 혹시 시간 조정이 필요하실까요?" | 7 |
| 특별한 날 (사별 추정 등) | "오늘은 쉬어가도 괜찮아요." | 3 |

### 5.2 Priority 규칙

여러 조건 동시 성립 시 priority 높은 1개만 발화. 과잉 nudge 방지.

### 5.3 Rate limit

- 같은 nudge 는 7일 내 재발화 금지
- 하루 총 nudge 1개 limit

## 6. Weekly/Monthly Report

### 6.1 Weekly 예 (Markdown + 터미널)

```
📖 지난 주 돌아보기 (2026-04-15 ~ 2026-04-21)

✅ 전체 완수: 5/7일 (71%)
🏆 최장 연속: 3일
💝 얻은 Bond: 28.5

🌅 아침: 6/7
🍽️ 식사: 15/21
🌙 일기: 5/7

감정 트렌드: ▃▅▆▇▇▁▂ (평균 0.58)

"수요일이 조금 힘들었지만 주말엔 활기가 많았어요."
```

### 6.2 Monthly 예

```
📅 2026년 4월 리뷰

완수율: 21/30일 (70%)
최장 streak: 12일
얻은 Bond: 118.5 pts

Highlights:
- 🎉 Easter 일요일: Full day
- 💝 2주 연속 아침 완수
- 😴 셋째 주는 조금 힘드셨네요

다음 달을 위한 Tip:
- 아침 시간 좀 더 여유 있게? 7:30 → 7:45 추천
```

## 7. Customization UX

### 7.1 기본 config

```yaml
rituals:
  enabled: true
  morning:
    enabled: true
    time: "07:30"
    sections: [fortune, weather, calendar]
  meals:
    enabled: true
    breakfast: {time: "08:30", after_minutes: 15}
    lunch: {time: "12:30", after_minutes: 15}
    dinner: {time: "19:00", after_minutes: 15}
  evening:
    enabled: true
    time: "22:30"
    prompt_timeout_min: 60
  mood_adaptive: true
  milestones_enabled: true
```

### 7.2 사용자 CLI

```
goose ritual config --morning-time 08:00
goose ritual disable breakfast
goose ritual restore-defaults
goose ritual status  # today's progress
```

## 8. Testing Strategy

### 8.1 3년치 fake history

Deterministic fake user data 생성기:
- 90% completion rate 사용자
- 50% rate 사용자 (realistic)
- Streak/break 패턴 의도적 주입
- Edge cases: birthday, streak break at 99일

### 8.2 Time freeze

`clockwork.Clock` 으로 연속 7일 simulate → milestone 발화 확인.

### 8.3 Mood shift

INSIGHTS mock 이 매일 다른 VAD 반환 → AdjustStyle 변화 확인.

## 9. Privacy & Ethics Review

### 9.1 수집 최소화

Ritual completion = 메타데이터. 실제 content (briefing text, meal items, journal entry) 는 각 SPEC이 자체 관리. RITUAL은 status/quality/timestamp만.

### 9.2 Export

사용자가 `goose ritual export` → JSON:
```json
{
  "user_id": "u1",
  "days": [
    {"date": "2026-04-22", "completions": {...}, "bond_earned": 4.5}
  ]
}
```

### 9.3 A2A 격리

Completion 자체도 기본 로컬. opt-in 시 aggregated (숫자만, 날짜·내용 없음) 공유 허용.

### 9.4 Gamification 윤리

- Streak anxiety 방지: "freeze day" 계획 (v0.2+)
- 비교·순위 없음: 혼자만의 여정
- 강제 알림 없음: 사용자 호출 시만 나타남

## 10. 오픈 이슈

1. **"Full day" 정의**: 5개 전부 full 이 현실적으로 너무 엄격? 4/5 로 완화? → 사용자 피드백 후 v0.2 조정.
2. **Meal 선택지**: 아침·점심·저녁 중 하나만 먹는 사용자 (간헐적 단식) → "N/A" 옵션 추가.
3. **Global timezone**: 여행 시 local 기준 변경. SCHEDULER-001 TZ shift 정책 상속.
4. **Multi-device**: PC + 모바일 둘 다 goosed 실행 시 중복 집계. 공유 MEMORY 전제.
5. **사용자 사망**: 민감 주제. 오랜 inactivity 시 "괜찮으세요?" → 이후 "3개월 inactive → 자동 알림 비활성" 정책 필요.
6. **개인 vs 가족 모드**: adaptation §9.2 후속. 본 SPEC은 개인 only.

## 11. 참고

- BJ Fogg: "Tiny Habits" (2019)
- Duolingo Streak psychology: https://blog.duolingo.com/
- Octalysis (Gamification ethics): https://octalysisgroup.com/
- Atomic Habits (James Clear): habit stacking 개념
- Ritual in HCI: https://dl.acm.org/doi/10.1145/3411764.3445710
