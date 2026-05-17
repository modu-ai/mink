---
id: SPEC-GOOSE-HEALTH-001
version: 0.1.0
status: planned
created_at: 2026-04-22
updated_at: 2026-05-17
author: manager-spec
priority: P1
issue_number: null
phase: 7
size: 중(M)
lifecycle: spec-anchored
labels: [health, medication, meal-tracker, regulatory, phase-7]
target_milestone: v0.2.0
mvp_status: deferred
deferred_reason: "0.1.0 MVP 범위 외 — v0.2.0 이월 (2026-05-17 사용자 확정). 의료 규제·DUR 외부 의존 큼, MVP 후순위."
---

# SPEC-GOOSE-HEALTH-001 — Meal · Medication · Hydration Tracker (Post-Meal Ritual, Drug Interaction Warning, No Medical Advice)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성 (Phase 7 #36, SCHEDULER + MEMORY 의존) | manager-spec |

---

## 1. 개요 (Overview)

MINK v6.0 Daily Companion의 **매 끼니 이후 리추얼**을 담당한다. 사용자 지시: "매 끼니 이후 건강/약 먹도록 안내". SCHEDULER-001이 `PostBreakfastTime` / `PostLunchTime` / `PostDinnerTime` HOOK 이벤트를 emit하면 본 SPEC이 활성화되어:

1. **복약 리마인더** — 사용자가 등록한 약(이름·용량·복용시간) 알림
2. **식사 로그** — "뭐 드셨어요?" 질문 + 사용자 답변을 MEMORY-001에 기록
3. **수분 섭취 체크** — 오전/오후 특정 시점 물 마시기 권유
4. **약 상호작용 경고** — 신규 약 등록 시 기존 약과의 상호작용 DB 조회

**HARD**: 본 SPEC은 **의료 기기가 아니다**. 모든 출력에 다음 disclaimer:
> "본 앱은 의료 기기가 아니며, 복약·식사 로그는 참고용입니다. 의학적 결정은 반드시 주치의 또는 약사와 상담하세요."

약 상호작용 경고는 **교육적 참고**이며 최종 판단은 전문가. 응급 상황(호흡곤란, 의식불명 등) 관련 언급 시 119 호출 안내 우선.

본 SPEC이 통과한 시점에서 `internal/ritual/health/` 패키지는:

- `HealthTracker` + `MedicationManager` + `MealLogger` + `HydrationTracker`,
- `DrugInteractionDB` (한국 식약처 공개 DB 또는 WHO DrugBank subset),
- SCHEDULER-001 HOOK consumer (`PostMealTime`),
- MEMORY-001 영속 (`health:{userID}:{type}:{date}`),
- 사용자 맞춤 복약 시간표 (약마다 다를 수 있음 — 식후 30분, 취침 전 등).

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- 사용자 지시(2026-04-22): "매 끼니 이후 건강/약 먹도록 안내" — 핵심 요구.
- 한국 고령화: 만성질환자 1200만+, 복약 순응도 낮음(WHO 리포트). "빼먹지 않게 챙겨주는 AI"가 실제 가치.
- Phase 7의 Daily Rituals 중 **가장 민감한 영역** (건강·의료) — 엔지니어링 + 법적 guard 필수.
- MEMORY-001 facts 테이블이 완성되어 있으므로, "사용자가 어제 점심에 뭐 먹었는지" 쿼리 가능.

### 2.2 상속 자산

- **SCHEDULER-001**: `PostBreakfastTime` / `PostLunchTime` / `PostDinnerTime` 이벤트.
- **MEMORY-001**: `facts` 테이블에 medication/meal/hydration 레코드.
- **INSIGHTS-001**: 복약 순응률 리포트 (월간/주간).
- **IDENTITY-001**: Person entity의 `chronic_condition`, `allergies` attribute (visibility 엄격).
- **HOOK-001**: 이벤트 consumer.

### 2.3 범위 경계

- **IN**: `HealthTracker`, `MedicationManager` (CRUD), `MealLogger`, `HydrationTracker`, `DrugInteractionDB` 인터페이스, 한국 식약처 데이터 (optional), SCHEDULER HOOK consumer, 리마인더 알림 (text), MEMORY 영속, 월간 순응률 리포트.
- **OUT**: 의학적 진단·처방, 증상 체크, 운동 추천, 영양 분석 상세 (칼로리 계산은 단순 lookup만), 수면 트래킹 (wearable 필요), 정신 건강 평가 (별도 mental health SPEC), push 알림 (Gateway), 혈압/혈당 기기 연동, 처방전 OCR, 약국 재고 조회.

---

## 3. 스코프

### 3.1 IN SCOPE

1. `internal/ritual/health/` 패키지.
2. `MedicationManager`:
   - `AddMedication(userID, med Medication) error`
   - `ListMedications(userID, active_only bool) []Medication`
   - `UpdateMedication(userID, medID string, fields ...)`
   - `DeactivateMedication(userID, medID)` (soft delete, 히스토리 보존)
   - `RecordIntake(userID, medID string, takenAt time.Time, skipped bool, note string)`
3. `Medication` 구조체:
   ```
   {
     ID, UserID, Name, GenericName, DosageMg, DosageUnit,
     Schedule: [{time: "08:00", timing: "after_breakfast"|"before_sleep"|...}],
     Duration: {start, end_date_or_nil},
     SideEffects []string,
     Notes string,
     PrescriberName string,
     CreatedAt, DeactivatedAt
   }
   ```
4. `MealLogger`:
   - `LogMeal(userID, meal MealEntry) error`
   - `ListMeals(userID, from, to time.Time)`
   - MealEntry: `{when, type: breakfast/lunch/dinner/snack, items []string, notes, mood_after?, photo_path?}`
5. `HydrationTracker`:
   - `LogWater(userID, ml int, when time.Time)`
   - `TodayTotal(userID) int`
   - Daily goal: 기본 2000ml (사용자 조정 가능).
6. `DrugInteractionDB` 인터페이스:
   - `CheckInteraction(medA, medB string) (*Interaction, error)` — severity: minor/moderate/severe
   - `ListKnownInteractions(med string) []Interaction`
   - 기본 구현: 한국 식약처 DUR (Drug Utilization Review) 오픈 API 또는 static DB
7. SCHEDULER HOOK consumer:
   - `HandlePostBreakfast/Lunch/DinnerTime` — 등록된 약 중 해당 timing 매칭 확인 → 리마인더
8. 복약 순응률:
   - `AdherenceRate(userID, medID, days int) float64` = `taken_count / scheduled_count`
9. 상호작용 경고:
   - 신규 `AddMedication` 시 기존 약들과 순회 체크
   - severity=severe 이면 블록 + 사용자에게 경고 + "주치의 상담 필수"
   - severity=moderate 이면 경고 후 사용자 선택 (계속 등록 or 취소)
   - severity=minor 이면 정보 표시 후 자동 진행
10. 알레르기 체크:
    - IDENTITY-001의 `allergies` attribute와 medication.ingredients 비교
    - 일치 시 severe 경고
11. Disclaimer 자동 부착.
12. Config:
    ```yaml
    health:
      enabled: false  # default off — 민감 정보
      drug_interaction_db: "korean_dur" | "disabled"
      meal_log_prompts: true
      hydration_goal_ml: 2000
      reminder_delay_min: 15  # 식후 15분 뒤 약 알림
    ```

### 3.2 OUT OF SCOPE

- **의학적 진단 / 처방 제안** — 엄격히 금지.
- **증상 체크 챗봇** — "머리 아픈데 무슨 병이에요?" 같은 질문은 "주치의 상담" 안내만.
- **운동 코칭 / 칼로리 계산** — 별도 Fitness SPEC.
- **수면 트래킹** — wearable 연동 필요, 별도 SPEC.
- **정신 건강 평가 (우울척도 등)** — 별도 Mental Health SPEC, 더 엄격한 규제 고려.
- **Push 알림 발송** — Gateway SPEC.
- **혈압·혈당 기기 연동** — 별도 IoT SPEC.
- **처방전 OCR / 이미지 인식** — 별도 SPEC.
- **약국 재고 조회 / 주문** — 범위 외.
- **의사/약사 상담 API** — 범위 외.
- **원격 진료 연계** — 법적 고려사항 복잡, 범위 외.

---

## 4. EARS 요구사항

### 4.1 Ubiquitous

**REQ-HEALTH-001 [Ubiquitous]** — Every user-facing health output (reminder, interaction warning, meal log summary) **shall** include the disclaimer: "본 앱은 의료 기기가 아니며, 의학적 결정은 주치의 또는 약사와 상담하세요."

**REQ-HEALTH-002 [Ubiquitous]** — The health subsystem **shall** require explicit opt-in via `config.health.enabled == true`; default is `false` because medical data is sensitive.

**REQ-HEALTH-003 [Ubiquitous]** — All medication and meal records **shall** be stored in MEMORY-001 with session_id="health" and encrypted via the underlying SQLite file permissions (0600) as per MEMORY-001 security contract.

**REQ-HEALTH-004 [Ubiquitous]** — Structured zap logs **shall** include `{user_id_hash, operation, med_count?, meal_logged?, severity?}`; medication names and dosages **shall not** appear in logs.

### 4.2 Event-Driven

**REQ-HEALTH-005 [Event-Driven]** — **When** HOOK-001 dispatches `PostBreakfastTime` / `PostLunchTime` / `PostDinnerTime`, the orchestrator **shall** (a) load active medications for the user, (b) filter by `schedule.timing == after_{meal}`, (c) wait `config.health.reminder_delay_min`, (d) emit reminder text with med name + dosage + "이미 드셨으면 기록해주세요" prompt.

**REQ-HEALTH-006 [Event-Driven]** — **When** `AddMedication(userID, med)` is called, the manager **shall** (a) check for severity=severe interactions with existing meds, (b) if severe, reject with `ErrSevereDrugInteraction` containing interaction details and message "주치의와 반드시 상담하세요", (c) if moderate, return `WarnModerateInteraction` requiring user confirmation, (d) if minor or none, proceed.

**REQ-HEALTH-007 [Event-Driven]** — **When** `AddMedication` is called and IDENTITY-001's `allergies` attribute contains an ingredient of the new medication, the manager **shall** return `ErrAllergyConflict` with the conflicting allergen name.

**REQ-HEALTH-008 [Event-Driven]** — **When** a user responds "안 먹었어요" or "skip" to a reminder, `RecordIntake` **shall** persist with `skipped=true`; adherence calculations **shall** count this as skipped, not taken.

**REQ-HEALTH-009 [Event-Driven]** — **When** a medication's `Duration.EndDate` is reached, the scheduler hook consumer **shall** automatically deactivate it and send a one-time notification "[약 이름] 복용 기간이 종료되었어요. 처방 갱신이 필요하시면 병원을 방문해주세요."

### 4.3 State-Driven

**REQ-HEALTH-010 [State-Driven]** — **While** a user has zero active medications, `PostMealTime` events **shall not** trigger any reminder; meal log prompt behavior is unaffected (controlled by `config.health.meal_log_prompts`).

**REQ-HEALTH-011 [State-Driven]** — **While** `config.health.drug_interaction_db == "disabled"`, `AddMedication` **shall** skip interaction checks entirely; a warning log "drug interaction DB disabled, skipping checks" **shall** be emitted once per session.

**REQ-HEALTH-012 [State-Driven]** — **While** MEMORY-001 is unavailable, health operations **shall** return `ErrMemoryUnavailable` — no in-memory fallback, medication data integrity is critical.

### 4.4 Unwanted

**REQ-HEALTH-013 [Unwanted]** — The LLM narrative layer (if used for reminder text generation) **shall not** suggest adjusting dosage, skipping, or substituting medications; forbidden phrases: "복용량을 줄여보세요", "대신 ~를 드셔보세요", "이 약은 필요 없을 수 있어요".

**REQ-HEALTH-014 [Unwanted]** — The subsystem **shall not** diagnose symptoms; inputs like "머리 아프다" or "열 난다" **shall** receive the canned response: "증상이 있으시다면 반드시 의사나 약사에게 상담하세요. 응급 상황 시 119에 전화해주세요."

**REQ-HEALTH-015 [Unwanted]** — Drug interaction responses **shall not** include phrases implying safety ("이 조합은 괜찮아요") for any severity; even "no known interaction" **shall** be expressed as "현재 DB에 등록된 상호작용은 없지만, 의료진에게 확인하세요".

**REQ-HEALTH-016 [Unwanted]** — Medication data **shall not** be shared via A2A protocol or exported to external services without explicit per-operation user confirmation (not a one-time opt-in).

### 4.5 Optional

**REQ-HEALTH-017 [Optional]** — **Where** `config.health.drug_interaction_db == "korean_dur"`, the manager **shall** query 식약처 DUR OpenAPI with medication ingredients; on network failure, fall back to cached static DB.

**REQ-HEALTH-018 [Optional]** — **Where** `config.health.hydration_reminders == true`, `HydrationTracker` **shall** emit a reminder every 2 hours during waking hours (06:00-22:00 local) if daily intake is below `(goal_ml × current_fraction_of_day)`.

**REQ-HEALTH-019 [Optional]** — **Where** LLM-generated reminder text is enabled (`config.health.llm_tone == true`), the LLM prompt **shall** explicitly forbid the phrases in REQ-HEALTH-013 and include the disclaimer REQ-HEALTH-001.

---

## 5. 수용 기준

**AC-HEALTH-001 — Opt-in default off**
- **Given** config.health 설정 없음
- **When** `AddMedication(u1, {...})` 호출
- **Then** `ErrHealthDisabled` 반환.

**AC-HEALTH-002 — 복약 리마인더 발화**
- **Given** user u1 에 active med {name:"혈압약A", schedule:[{timing:"after_breakfast"}]}, `reminder_delay_min=15`
- **When** HOOK이 `PostBreakfastTime` dispatch
- **Then** 15분 뒤 reminder text 출력, 내용에 "혈압약A" + disclaimer 포함.

**AC-HEALTH-003 — Severe interaction 차단**
- **Given** u1 기존 med "warfarin", mock DUR DB가 "warfarin + aspirin = severe" 반환
- **When** `AddMedication(u1, {name:"aspirin"})`
- **Then** `ErrSevereDrugInteraction` 반환, 메시지에 "warfarin" + "반드시 상담" 포함.

**AC-HEALTH-004 — Moderate interaction 사용자 확인**
- **Given** mock DUR 가 severity=moderate 반환
- **When** `AddMedication(u1, {...}, AcceptModerate=false)`
- **Then** `WarnModerateInteraction` 반환, 약은 저장되지 않음.
- **When** 다시 `AddMedication(u1, {...}, AcceptModerate=true)`
- **Then** 정상 등록, 로그에 "moderate interaction accepted by user" WARN.

**AC-HEALTH-005 — 알레르기 충돌**
- **Given** IDENTITY-001 u1 allergies=["penicillin"], 신규 약 ingredients=["amoxicillin"] (penicillin계)
- **When** `AddMedication`
- **Then** `ErrAllergyConflict` 반환, 메시지에 "penicillin" 포함.

**AC-HEALTH-006 — 증상 언급 시 canned response**
- **Given** 사용자가 brief 인터랙션에서 "머리가 너무 아파" 입력 (health-related)
- **When** HealthTracker가 인식
- **Then** 응답에 "의사나 약사" + "응급 시 119" 포함, 진단·치료 제안 미포함.

**AC-HEALTH-007 — 복약 순응률 계산**
- **Given** 지난 7일, med="X" 가 매일 2회 스케줄 (14건 중 12건 taken, 2건 skipped)
- **When** `AdherenceRate(u1, "X", 7)`
- **Then** 반환값 = 12/14 ≈ 0.857.

**AC-HEALTH-008 — 약 이름 로그 미노출**
- **Given** 사용자가 `AddMedication({name:"prozac"})` 호출
- **When** zap 로그 캡처
- **Then** 로그에 "prozac" 미포함, `med_count=1` 같은 메타데이터만.

**AC-HEALTH-009 — Duration 만료 자동 deactivate**
- **Given** med `{end_date: 2026-04-22}`
- **When** SCHEDULER가 2026-04-23 새벽 tick 발화
- **Then** med.DeactivatedAt 채워짐, 사용자에게 만료 알림 1회, 이후 PostMealTime event에서 해당 med 무시.

**AC-HEALTH-010 — 상호작용 DB 비활성 시 skip**
- **Given** `config.health.drug_interaction_db="disabled"`
- **When** `AddMedication`
- **Then** DUR mock 호출 0회, 약 정상 저장, 로그에 "disabled, skipping" WARN 1회 (session당).

**AC-HEALTH-011 — 식사 로그 프라이버시**
- **Given** 사용자가 meal log에 "삼겹살 3인분" 기록
- **When** MEMORY-001 저장
- **Then** SQLite 파일 권한 0600, 외부 A2A 전송 0건.

---

## 6. 기술적 접근

### 6.1 패키지 레이아웃

```
internal/
└── ritual/
    └── health/
        ├── tracker.go           # HealthTracker facade
        ├── medication.go        # MedicationManager
        ├── meal.go              # MealLogger
        ├── hydration.go         # HydrationTracker
        ├── interactions.go      # DrugInteractionDB interface
        ├── dur_korean.go        # 식약처 DUR API 클라이언트
        ├── dur_static.go        # fallback static DB
        ├── allergy.go           # 알레르기 체크
        ├── adherence.go         # 순응률 계산
        ├── reminder.go          # 리마인더 텍스트 생성
        ├── safety.go            # disclaimer + canned responses
        ├── types.go
        ├── config.go
        └── *_test.go
```

### 6.2 핵심 타입 (의사코드)

```
MedicationManager interface
  - AddMedication(ctx, userID, med) error
  - ListMedications(ctx, userID, activeOnly bool) ([]Medication, error)
  - UpdateMedication(ctx, userID, medID, fields) error
  - DeactivateMedication(ctx, userID, medID, reason) error
  - RecordIntake(ctx, userID, medID, IntakeRecord) error

Medication {
  ID, UserID, Name, GenericName
  Ingredients []string
  DosageMg, DosageUnit
  Schedule []MedSchedule
  Duration {StartDate, EndDate *time.Time}
  SideEffects []string
  Notes
  PrescriberName
  CreatedAt, DeactivatedAt *time.Time
}

MedSchedule {
  LocalClock string  // "08:00"
  Timing     string  // "after_breakfast" | "before_sleep" | "fixed"
  DaysOfWeek []int   // [] = every day
}

MealEntry {
  When time.Time
  Type string  ("breakfast"|"lunch"|"dinner"|"snack")
  Items []string
  Notes string
  MoodAfter *int  // 1-5
  PhotoPath string
}

DrugInteractionDB interface
  - CheckInteraction(ctx, medA, medB) (*Interaction, error)
  - ListKnownInteractions(ctx, med) ([]Interaction, error)

Interaction {
  MedA, MedB string
  Severity string  // "minor" | "moderate" | "severe"
  Mechanism string
  Recommendation string
  Source string  // "korean_dur" | "who_drugbank" | ...
}
```

### 6.3 식약처 DUR API

한국 식품의약품안전처 공공데이터: https://www.data.go.kr/ → "DUR 성분-성분 병용금기" API
- 엔드포인트: `/DURPrdlstInfoService/getUsjntTabooInfoList`
- Free, 인증키 발급 필요
- Response XML → parse → `Interaction` 구조체

Static fallback: 주요 50-100종 상호작용 goldenfile (`testdata/dur_static.json`)

### 6.4 Safety Responses (canned)

```
var safetyResponses = map[string]string{
  "symptom_pain": "증상이 심하시다면 반드시 의사나 약사에게 상담하세요. 응급 상황 시 119에 전화해주세요. 본 앱은 진단이나 치료를 제공할 수 없어요.",
  "emergency": "지금 증상이 긴급해 보입니다. 즉시 119에 전화하거나 가까운 응급실로 가세요.",
  "dosage_question": "복용량 변경은 반드시 처방 의사와 상의해야 해요. 임의로 조절하지 마세요.",
}
```

### 6.5 리마인더 생성 (template + optional LLM)

Template (LLM 없이 안전 기본):
```
안녕하세요 {name}님. 식사 잘 하셨어요? 💊
{medName} ({dosage}) 드실 시간이에요.

[드셨어요] [아직이에요] [나중에 다시 알려주세요]

⚠️ 본 앱은 의료 기기가 아니며, 복약 관련 모든 결정은 주치의와 상담하세요.
```

### 6.6 알레르기 교차반응 체크

Penicillin계, Sulfa계, NSAID 등 주요 교차반응 그룹 goldenfile:
```json
{
  "penicillin": ["amoxicillin", "ampicillin", "piperacillin", "오구멘틴", "키로신"],
  "sulfa": ["sulfamethoxazole", "trimethoprim", ...]
}
```

### 6.7 라이브러리 결정

| 용도 | 라이브러리 | 근거 |
|------|----------|-----|
| HTTP client | stdlib | |
| XML parsing (식약처) | `encoding/xml` | |
| JSON | stdlib | |
| Memory ops | MEMORY-001 | |

### 6.8 TDD 진입

1. RED: `TestOptIn_DefaultOff` — AC-HEALTH-001
2. RED: `TestPostBreakfastReminder_15min` — AC-HEALTH-002
3. RED: `TestSevereInteraction_Blocks` — AC-HEALTH-003
4. RED: `TestModerateInteraction_UserConfirm` — AC-HEALTH-004
5. RED: `TestAllergyConflict` — AC-HEALTH-005
6. RED: `TestSymptom_CannedResponse` — AC-HEALTH-006
7. RED: `TestAdherence_857Percent` — AC-HEALTH-007
8. RED: `TestMedName_NotInLogs` — AC-HEALTH-008
9. RED: `TestDurationExpiry_AutoDeactivate` — AC-HEALTH-009
10. RED: `TestDurDisabled_SkipsCheck` — AC-HEALTH-010
11. RED: `TestMealLog_Privacy` — AC-HEALTH-011
12. GREEN → REFACTOR

### 6.9 TRUST 5 매핑

| 차원 | 달성 |
|-----|-----|
| **T**ested | 85%+, 100+ interaction goldenfile, canned response 보장 |
| **R**eadable | medication/meal/hydration/interactions 명확 분리 |
| **U**nified | DTO 통일, Interaction severity enum |
| **S**ecured | opt-in, MEMORY-001 암호화, A2A 전송 금지, 로그 redaction, 증상 canned response 강제 |
| **T**rackable | 모든 변경 구조화 로그 (med name 제외), 순응률 월간 리포트 |

---

## 7. 의존성

| 타입 | 대상 | 설명 |
|-----|------|-----|
| 선행 SPEC | SCHEDULER-001 | PostMealTime 이벤트 |
| 선행 SPEC | MEMORY-001 | 영속 저장소 |
| 선행 SPEC | IDENTITY-001 | 사용자 allergies, chronic_condition |
| 선행 SPEC | HOOK-001 | 이벤트 consumer |
| 후속 SPEC | INSIGHTS-001 | 월간 순응률 리포트 |
| 후속 SPEC | RITUAL-001 | 리추얼 완수 Bond Level |
| 외부 | 식약처 DUR API | 한국 상호작용 |
| 외부 | Go stdlib | |

---

## 8. 리스크 & 완화

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 상호작용 DB 오류로 위험 조합 허용 | 중 | 치명적 | 이중 DB (식약처 + static fallback), severity=severe 시 HARD block + 주치의 안내 |
| R2 | LLM이 의학적 조언 생성 | 중 | 고 | LLM 미사용 기본, 사용 시 프롬프트 제약 + 후처리 keyword filter |
| R3 | 복약 데이터 유출 | 낮 | 치명적 | MEMORY-001 0600, A2A 금지, export 시 per-op 확인 |
| R4 | 식약처 API 다운타임 | 중 | 중 | static fallback, network failure 시 기존 약 기반으로만 체크 |
| R5 | 사용자가 앱 리마인더를 잊어서 약 복용 누락 | 고 | 중 | 사용자 책임 강조 disclaimer, multiple reminders (15분 → 60분 → 120분) |
| R6 | 증상 질문 ("~병인가요?") 에 AI가 과잉 응답 | 중 | 고 | canned response 강제, LLM으로 처리 금지 |
| R7 | 어르신 사용자 UI 불편 | 고 | 중 | CLI-001/TUI에서 대형 글꼴 + 단순 버튼 요구 (범위 외지만 연계) |
| R8 | 잘못된 schedule 등록 (시간 오타) | 중 | 중 | 입력 검증 (HH:MM 범위), 저장 전 사용자 확인 |
| R9 | 약 이름 오타로 상호작용 체크 miss | 중 | 고 | 입력 시 fuzzy match (Levenshtein ≤ 2) 제안 |

---

## 9. 참고

### 9.1 프로젝트 문서

- `.moai/specs/SPEC-GOOSE-SCHEDULER-001/spec.md` — 이벤트 소스
- `.moai/specs/SPEC-GOOSE-MEMORY-001/spec.md` — 저장소
- `.moai/specs/SPEC-GOOSE-IDENTITY-001/spec.md` — allergies attribute

### 9.2 외부 참조

- 식약처 DUR 공공 API: https://www.data.go.kr/data/15039046/openapi.do
- WHO Medication Adherence: https://www.who.int/publications/i/item/9241545992
- Drug interaction databases: https://go.drugbank.com/
- Medical device regulation (FDA): https://www.fda.gov/medical-devices

### 9.3 부속 문서

- `./research.md` — DUR API 상세, canned response 코퍼스, 순응률 알고리즘

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **의학적 진단·처방을 제공하지 않는다**. 반드시 "주치의 상담" 안내.
- 본 SPEC은 **증상 체크 챗봇을 포함하지 않는다** (canned response 만).
- 본 SPEC은 **운동·칼로리·영양 상세 분석을 포함하지 않는다**.
- 본 SPEC은 **수면 트래킹을 포함하지 않는다** (wearable 필요).
- 본 SPEC은 **정신 건강 평가를 포함하지 않는다** (별도 SPEC).
- 본 SPEC은 **Push 알림 발송을 포함하지 않는다** (Gateway).
- 본 SPEC은 **혈압·혈당 기기 연동을 포함하지 않는다**.
- 본 SPEC은 **처방전 OCR을 포함하지 않는다**.
- 본 SPEC은 **약국 재고·주문 기능을 포함하지 않는다**.
- 본 SPEC은 **원격 진료 연계를 포함하지 않는다**.
- 본 SPEC은 **A2A 프로토콜로 의료 데이터를 전송하지 않는다**.
- 본 SPEC은 **가족 구성원 간 의료 정보 공유를 포함하지 않는다**.

---

**End of SPEC-GOOSE-HEALTH-001**
