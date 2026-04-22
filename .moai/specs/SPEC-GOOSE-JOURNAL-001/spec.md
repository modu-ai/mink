---
id: SPEC-GOOSE-JOURNAL-001
version: 0.1.0
status: Planned
created: 2026-04-22
updated: 2026-04-22
author: manager-spec
priority: P0
issue_number: null
phase: 7
size: 중(M)
lifecycle: spec-anchored
---

# SPEC-GOOSE-JOURNAL-001 — Evening Journal + Emotion Tagging + Long-term Memory Recall (E2E Local, Privacy-First)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성 (Phase 7 #37, MEMORY + INSIGHTS + SCHEDULER 의존) | manager-spec |

---

## 1. 개요 (Overview)

GOOSE v6.0 Daily Companion의 **저녁 리추얼**을 담당한다. 사용자 지시:

> "저녁에 자기 전 오늘 하루가 어땠는지 안부 묻고 일기 식으로 메모를 남기면 모두 저장했다가 사용자와 감성적으로 함께 성장해 가는 반려AI 컨셉"

SCHEDULER-001이 `EveningCheckInTime` HOOK 이벤트를 emit하면 본 SPEC이 활성화되어:

1. **안부 프롬프트**: "오늘 하루 어떠셨어요?"
2. **자유 입력**: 사용자가 텍스트(또는 간단 이모지 😊😐😔)로 응답
3. **감정 분석**: VAD 모델(Valence-Arousal-Dominance)로 감정 태깅
4. **일기 저장**: 전문 + 감정 태그 + 메타데이터를 MEMORY-001에 E2E 암호화 보관
5. **추억 회상**: "작년 오늘" / "한 달 전 비슷한 날" 메시지 자동 생성
6. **장기 트렌드**: 감정 궤적을 시각화(터미널 차트) 및 주간/월간 요약

**프라이버시 핵심**: 일기는 **가장 민감한 개인 정보**. 전제:
- 기본 저장소는 **로컬 only** (cloud 백업은 E2E 암호화된 경우에만, v0.2+)
- LLM 호출 시 원문 **전송 최소화** (요약·감정 태그만 외부로 나갈 수 있고, 원문 전체는 사용자 명시 승인 필요)
- LoRA 훈련 데이터 포함 여부 **사용자 개별 선택**

본 SPEC이 통과한 시점에서 `internal/ritual/journal/` 패키지는:

- `JournalWriter` + `EmotionAnalyzer` + `TrendAggregator` + `MemoryRecall`,
- SCHEDULER HOOK consumer,
- MEMORY-001 facts + 별도 `journal` 테이블 (더 긴 텍스트),
- VAD 감정 모델 (로컬 lightweight, 또는 LLM-assisted opt-in),
- 기념일·특별한 날 자동 인식 (생일, 결혼기념일 등),
- 터미널 감정 차트.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- 사용자 지시(2026-04-22): 저녁 일기 기능이 Phase 7의 **감정적 핵심**. "감성적 성장 파트너" 컨셉의 구체화.
- Identity Graph (IDENTITY-001) + Memory (MEMORY-001) + Insights (INSIGHTS-001) 가 모두 갖춰진 후 본 SPEC이 **사람과 AI의 감정적 연결**을 완성.
- adaptation.md §7 Mood Detection + §11 사용자 제어 (transparency/edit/export/delete/opt-out) 의 실현 장소.
- 다마고치 Bond Level 과 가장 강하게 연결 — 저녁 일기 1회 = +2 Bond (다른 리추얼보다 weight 높음).

### 2.2 상속 자산

- **SCHEDULER-001**: `EveningCheckInTime` 이벤트.
- **MEMORY-001**: facts + session_id="journal".
- **INSIGHTS-001**: 감정 트렌드 집계 소비.
- **IDENTITY-001**: Person entity의 important_dates (생일, 기념일).
- **ADAPTER-001**: LLM 감정 분석 (opt-in).
- **HOOK-001**: 이벤트 consumer.

### 2.3 범위 경계

- **IN**: `JournalWriter`, 자유 텍스트 + 이모지 입력, VAD 감정 태깅, 장기 트렌드 집계, "작년 오늘" 회상, 기념일 인식, MEMORY 저장 (local), 터미널 감정 차트, 주간/월간 요약 생성, opt-out 즉시 삭제.
- **OUT**: 클라우드 백업 (v0.2+, E2E 암호화 필요), 음성 일기 (STT 별도 SPEC), 이미지 일기 (첨부만 path, 분석 없음), 타인과 일기 공유, 정신 건강 평가 (우울척도, 별도 SPEC), 심리 상담 기능, 감정 조작 — 사용자 감정을 특정 방향으로 유도 금지 (adaptation §15.2).

---

## 3. 스코프

### 3.1 IN SCOPE

1. `internal/ritual/journal/` 패키지.
2. `JournalWriter`:
   - `Write(ctx, userID, entry JournalEntry) (*StoredEntry, error)`
   - `Read(ctx, userID, entryID) (*StoredEntry, error)`
   - `ListByDate(ctx, userID, from, to time.Time)`
   - `Search(ctx, userID, query string) []StoredEntry` — 로컬 FTS5
3. `JournalEntry` 구조체:
   ```
   {
     UserID, Date, Text string,
     EmojiMood string (optional),
     AttachmentPaths []string,
     PrivateMode bool (LLM 분석 opt-out)
   }
   ```
4. `StoredEntry` (저장 후):
   ```
   {
     ID, Text, EmojiMood,
     Vad: {valence, arousal, dominance} (0-1),
     EmotionTags: []string ("happy", "tired", "anxious", ...),
     Anniversary *{type, name, year}, // 해당 일 기념일
     WordCount, CreatedAt,
     AllowLoRATraining bool,
     AttachmentPaths
   }
   ```
5. `EmotionAnalyzer`:
   - `Analyze(ctx, text, emojiMood) (*Vad, []string, error)`
   - 로컬 기본: 이모지 + 키워드 매칭 (감정 사전 goldenfile)
   - LLM-assisted (opt-in): 더 정확하지만 외부 전송
6. `TrendAggregator`:
   - `WeeklyTrend(ctx, userID, week time.Time) *Trend`
   - `MonthlyTrend(ctx, userID, month) *Trend`
   - `RenderChart(trend)` — 터미널 sparkline/heatmap
7. `MemoryRecall`:
   - `FindAnniversaryEvents(ctx, userID, date) []StoredEntry` — "작년 오늘", "2년 전 오늘"
   - `FindSimilarMood(ctx, userID, currentVad) []StoredEntry` — 유사 감정 과거 일기
8. `AnniversaryDetector`:
   - IDENTITY-001에서 important_dates (생일, 결혼기념일, 추모일 등) 로드
   - 오늘 날짜와 ±1일 매칭 시 flag
9. 저녁 프롬프트 flow:
   - SCHEDULER `EveningCheckInTime` → JournalOrchestrator.Prompt()
   - "오늘 하루 어떠셨어요?" 출력
   - 사용자 응답 대기 (또는 skip)
   - 응답 시 EmotionAnalyzer → JournalWriter.Write
   - "작년 오늘" 회상 기회 있으면 부드럽게 이어감
10. Config:
    ```yaml
    journal:
      enabled: false  # opt-in
      emotion_llm_assisted: false  # 원문 외부 전송 금지 기본
      allow_lora_training: false  # 사용자 별도 동의
      cloud_backup: false  # v0.2+
      retention_days: -1  # 무기한 (-1)
    ```
11. 사용자 제어 API (adaptation §11):
    - `ExportAll(userID) []byte` — JSON export
    - `DeleteAll(userID) error` — 전체 삭제
    - `DeleteByDateRange(userID, from, to)`
    - `OptOut(userID)` — 즉시 비활성화 + 옵션: 기존 데이터 삭제

### 3.2 OUT OF SCOPE

- **클라우드 백업** (E2E 암호화 필수) — v0.2+.
- **음성 일기 (STT)** — 별도 Voice SPEC.
- **이미지 인식 (첨부된 사진 분석)** — 별도 Vision SPEC.
- **타인과 공유** — 반려AI 컨셉과 상충, 금지.
- **정신 건강 임상 평가 (PHQ-9 등)** — 별도 Mental Health SPEC (규제 고려).
- **심리 상담 기능** — AI 상담 금지, 필요 시 전문가 연결 안내.
- **감정 조작** — 특정 감정 유도 프롬프트 금지 (HARD).
- **협업 일기 (커플, 친구)** — 범위 외.
- **Markdown 에디터 UI** — CLI-001 TUI.
- **Rich media (동영상)** — path만.
- **자동 생성 일기 (AI가 써주는 일기)** — 사용자 진정성 해침.

---

## 4. EARS 요구사항

### 4.1 Ubiquitous

**REQ-JOURNAL-001 [Ubiquitous]** — The journaling subsystem **shall** require explicit opt-in via `config.journal.enabled == true`; default is `false`; a first-time opt-in flow **shall** present the privacy notice covering local storage, optional LLM analysis, and LoRA training.

**REQ-JOURNAL-002 [Ubiquitous]** — Every journal entry **shall** be stored in MEMORY-001's underlying SQLite with file permissions 0600 and directory 0700; no plaintext backup outside `~/.goose/journal/`.

**REQ-JOURNAL-003 [Ubiquitous]** — The journaling subsystem **shall never** transmit journal text to external services unless both (a) `config.journal.emotion_llm_assisted == true` AND (b) the entry's `PrivateMode == false`; private entries **shall** use local analyzer only.

**REQ-JOURNAL-004 [Ubiquitous]** — Structured zap logs **shall** record `{user_id_hash, operation, entry_length_bucket, emotion_tags_count, has_attachment}`; entry text content **shall not** appear in any log.

### 4.2 Event-Driven

**REQ-JOURNAL-005 [Event-Driven]** — **When** HOOK-001 dispatches `EveningCheckInTime`, the orchestrator **shall** (a) check if a journal entry for today already exists, (b) if yes, silently skip with INFO log, (c) if no, emit a prompt and wait up to `config.journal.prompt_timeout_min` (default 60) for user response.

**REQ-JOURNAL-006 [Event-Driven]** — **When** a user submits an entry via `Write`, the analyzer **shall** (a) compute VAD using local emotion dictionary, (b) if `emotion_llm_assisted == true AND privateMode == false`, optionally refine via LLM, (c) extract emotion tags (top-3 strongest), (d) store with timestamp.

**REQ-JOURNAL-007 [Event-Driven]** — **When** `MemoryRecall.FindAnniversaryEvents(userID, today)` is called, the recall **shall** return entries from (today - 1 year), (today - 2 years), ..., up to 10 years back where matching entries exist; same-date tolerance ±1 day.

**REQ-JOURNAL-008 [Event-Driven]** — **When** `AnniversaryDetector.CheckToday(userID)` matches a `important_date` in IDENTITY-001, the prompt **shall** include a reference to that date (e.g., "오늘은 결혼기념일이네요. 하루 어떻게 보내셨어요?").

**REQ-JOURNAL-009 [Event-Driven]** — **When** recent 3 entries show consistent negative valence (< 0.3), the prompt **shall** soften its tone and include "언제든 이야기해주세요, 들을게요. 도움이 필요하시면 전문가 상담도 좋은 방법이에요." — but **shall not** recommend specific providers or diagnose.

### 4.3 State-Driven

**REQ-JOURNAL-010 [State-Driven]** — **While** `config.journal.allow_lora_training == false` (default), entries **shall** have `AllowLoRATraining: false` and LORA-001 (Phase 6) **shall** exclude them from training datasets.

**REQ-JOURNAL-011 [State-Driven]** — **While** `config.journal.retention_days > 0`, entries older than the retention window **shall** be automatically deleted during nightly cleanup (03:00 local); the default (-1) means infinite retention.

**REQ-JOURNAL-012 [State-Driven]** — **While** MEMORY-001 is unavailable, `Write` **shall** buffer the entry in memory with a maximum queue of 10 and retry; after 3 retry failures, `Write` returns `ErrPersistFailed` and the user sees "일기 저장 실패, 다시 시도해주세요" — the text is not lost during the session but **shall** be discarded on process exit to prevent silent leakage.

### 4.4 Unwanted

**REQ-JOURNAL-013 [Unwanted]** — The prompt generator **shall not** use language designed to elicit specific emotions or extract sensitive information ("당신의 가장 큰 비밀은?", "부모님께 서운한 점은?") — only open, neutral prompts ("오늘 어떠셨어요?", "기억에 남는 순간 있으세요?").

**REQ-JOURNAL-014 [Unwanted]** — The subsystem **shall not** transmit journal content via A2A protocol under any circumstances; journal data is **never** inter-agent shareable.

**REQ-JOURNAL-015 [Unwanted]** — The subsystem **shall not** diagnose mental health conditions; entries containing self-harm indicators ("죽고 싶다", "사라지고 싶다" 등) **shall** trigger a canned response with Korean crisis hotline (1577-0199 생명의전화) **before** any other processing, and the entry is still stored locally but flagged.

**REQ-JOURNAL-016 [Unwanted]** — The export function **shall not** include other users' data in multi-user scenarios; `ExportAll(userID)` **shall** filter strictly by userID.

### 4.5 Optional

**REQ-JOURNAL-017 [Optional]** — **Where** `config.journal.emotion_llm_assisted == true`, the LLM **shall** be given only the text (not user metadata) with a prompt: "다음 일기의 감정을 VAD 모델로 분석하고 Top-3 감정 태그를 JSON으로 반환하세요. 분석 결과 외에 어떤 조언이나 해석도 포함하지 마세요."

**REQ-JOURNAL-018 [Optional]** — **Where** `config.journal.weekly_summary == true`, every Sunday 22:00 local, a weekly summary **shall** be generated (valence trend, top 3 moods, wordcloud of frequent words) and offered to the user the next evening.

**REQ-JOURNAL-019 [Optional]** — **Where** INSIGHTS-001 is available, the journal entry saving **shall** call `insights.OnJournalEntry(entry)` for downstream mood pattern analysis.

---

## 5. 수용 기준

**AC-JOURNAL-001 — Opt-in default off**
- **Given** config.journal 설정 없음
- **When** `JournalWriter.Write(u1, entry)` 호출
- **Then** `ErrJournalDisabled` 반환.

**AC-JOURNAL-002 — LLM 분석 opt-out 기본**
- **Given** `config.journal.enabled=true, emotion_llm_assisted=false`
- **When** 엔트리 저장 + LLM mock 캡처
- **Then** LLM mock 호출 0회, 로컬 emotion analyzer로만 처리.

**AC-JOURNAL-003 — VAD 로컬 분석**
- **Given** text="오늘 정말 행복했어, 오랜만에 웃었어"
- **When** `EmotionAnalyzer.Analyze(text, "")` (local only)
- **Then** `vad.Valence >= 0.7`, `emotion_tags` 에 "happy" 또는 "joy" 포함.

**AC-JOURNAL-004 — 이모지 기반 분석**
- **Given** text="😔 힘들다"
- **When** Analyze
- **Then** `vad.Valence < 0.4`, tags에 "sad" 또는 "tired" 포함.

**AC-JOURNAL-005 — 자해 키워드 crisis response**
- **Given** text="죽고 싶다"
- **When** `Write`
- **Then** 응답에 "1577-0199 생명의전화" 포함, 엔트리 저장되며 `crisis_flag=true` 설정, INFO 로그.

**AC-JOURNAL-006 — 작년 오늘 회상**
- **Given** u1이 2025-04-22 에 일기 작성, 오늘=2026-04-22
- **When** `MemoryRecall.FindAnniversaryEvents(u1, today)`
- **Then** 반환 리스트에 2025-04-22 엔트리 포함, distance_years=1.

**AC-JOURNAL-007 — 기념일 프롬프트**
- **Given** IDENTITY-001 u1 important_dates=[{type:"wedding", date:"2020-04-22"}], 오늘=2026-04-22
- **When** `JournalOrchestrator.Prompt(u1)`
- **Then** 프롬프트 text에 "결혼기념일" 또는 "특별한 날" 포함.

**AC-JOURNAL-008 — 일기 텍스트 로그 미노출**
- **Given** 엔트리 "비밀 이야기입니다"
- **When** Write + zap 로그 캡처
- **Then** 로그에 "비밀 이야기" 미포함, `entry_length_bucket` 같은 메타만.

**AC-JOURNAL-009 — A2A 전송 금지**
- **Given** A2A mock connection 설정됨
- **When** Write
- **Then** A2A mock 호출 0회 (어떤 경로로도).

**AC-JOURNAL-010 — Export 필터링**
- **Given** MEMORY에 u1, u2 엔트리 혼재
- **When** `ExportAll(u1)`
- **Then** 반환 JSON에 u2 엔트리 0건.

**AC-JOURNAL-011 — DeleteAll 즉시 반영**
- **Given** u1 엔트리 100개 저장
- **When** `DeleteAll(u1)` 호출
- **Then** `ListByDate` 반환 0건, SQLite 레벨에서도 삭제 완료 (soft delete 아님).

**AC-JOURNAL-012 — Private mode LLM 미호출**
- **Given** `emotion_llm_assisted=true`, 엔트리 `PrivateMode=true`
- **When** Write
- **Then** LLM mock 호출 0회, 로컬 분석만.

---

## 6. 기술적 접근

### 6.1 패키지 레이아웃

```
internal/
└── ritual/
    └── journal/
        ├── writer.go          # JournalWriter
        ├── analyzer.go        # EmotionAnalyzer (VAD)
        ├── emotion_dict.go    # 감정 키워드 사전 (goldenfile)
        ├── analyzer_llm.go    # LLM-assisted (opt-in)
        ├── trend.go           # TrendAggregator
        ├── recall.go          # MemoryRecall (anniversary, similar)
        ├── anniversary.go     # AnniversaryDetector
        ├── orchestrator.go    # HOOK consumer + prompt flow
        ├── prompts.go         # 저녁 프롬프트 템플릿
        ├── crisis.go          # 자해 키워드 + canned response
        ├── chart.go           # 터미널 sparkline
        ├── storage.go         # MEMORY-001 어댑터
        ├── export.go
        ├── types.go
        ├── config.go
        └── *_test.go
```

### 6.2 핵심 타입

```
JournalEntry {
  UserID, Date time.Time, Text string
  EmojiMood string  // "😊" | "😐" | "😔" | ...
  AttachmentPaths []string
  PrivateMode bool
}

StoredEntry {
  ID string (UUID)
  UserID, Date, Text
  EmojiMood
  Vad Vad  // valence, arousal, dominance
  EmotionTags []string
  Anniversary *Anniversary
  WordCount int
  CreatedAt time.Time
  AllowLoRATraining bool
  CrisisFlag bool
  AttachmentPaths []string
}

Vad {
  Valence   float64  // 0-1 (negative-positive)
  Arousal   float64  // 0-1 (calm-excited)
  Dominance float64  // 0-1 (submissive-dominant)
}

EmotionAnalyzer interface {
  Analyze(ctx, text, emojiMood) (*Vad, []string, error)
}

MemoryRecall interface {
  FindAnniversaryEvents(ctx, userID, date) ([]StoredEntry, error)
  FindSimilarMood(ctx, userID, currentVad Vad, limit int) ([]StoredEntry, error)
}

Trend {
  Period string ("week"|"month")
  From, To time.Time
  AvgValence, AvgArousal, AvgDominance float64
  MoodDistribution map[string]int  // tag -> count
  EntryCount int
  SparklinePoints []float64  // 일별 valence
}
```

### 6.3 Local Emotion Analyzer (사전 기반)

```
감정 사전 구조:
{
  "ko": {
    "happy": ["행복", "기쁘", "좋", "웃", "즐거", "신나"],
    "sad": ["슬프", "우울", "힘들", "외로", "울었", "눈물"],
    "anxious": ["불안", "걱정", "초조", "긴장", "떨리"],
    "angry": ["화", "짜증", "열받", "빡", "분노"],
    "tired": ["피곤", "지쳐", "힘들", "졸리"],
    ...
  }
}

VAD 계산:
1. text 를 토큰화
2. 각 tag 당 매칭 단어 카운트
3. Top-3 tag 선정
4. Valence = tag별 base valence 가중 평균
   happy: 0.9, sad: 0.2, anxious: 0.3, angry: 0.4, tired: 0.4, calm: 0.6
5. 이모지 bonus: 😊 +0.2, 😔 -0.2, 😠 arousal+0.3
```

구현 간단, 정확도 중간. 충분한 baseline.

### 6.4 Crisis Keywords

```go
var crisisKeywords = []string{
    "죽고 싶", "자살", "사라지고 싶", "살기 싫", "끝내고 싶",
    "자해", "칼로", "극단적",
}

func (c *CrisisDetector) Check(text string) bool {
    for _, kw := range crisisKeywords {
        if strings.Contains(text, kw) { return true }
    }
    return false
}

var crisisResponse = `
오늘 하루가 많이 힘드셨나 봐요. 이야기해주셔서 고마워요.

혹시 지금 많이 힘드시다면, 전문가의 도움을 받을 수 있어요:
- 생명의전화: 1577-0199 (24시간)
- 자살예방상담전화: 1393 (24시간)
- 청소년전화: 1388

혼자 감당하지 마세요. 당신의 이야기를 들어줄 사람들이 있어요.
`
```

### 6.5 Anniversary 계산

```
오늘 = 2026-04-22
과거 엔트리 쿼리:
- (2025-04-21, 22, 23) 중 엔트리 있음 → 1년 전
- (2024-04-21, 22, 23) → 2년 전
...
10년까지
```

SQL (MEMORY-001 facts):
```sql
SELECT * FROM facts
WHERE session_id = 'journal'
AND user_id = ?
AND DATE(created_at) BETWEEN DATE('now', '-1 year', '-1 day') 
                         AND DATE('now', '-1 year', '+1 day')
```

### 6.6 Trend Sparkline

터미널 출력:

```
📈 최근 7일 감정 트렌드:
Mon ████▁▁▁     0.6
Tue ██████▁     0.8
Wed ▁▁██████    0.3
Thu ▁▁▁▁█▁▁     0.2
Fri ██▁▁▁▁▁     0.5
Sat ████████    0.9
Sun ██████▁▁    0.7

요일별 평균: 0.57 (중간)
가장 행복했던 요일: 토요일
```

Unicode block characters (▁▂▃▄▅▆▇█) 사용.

### 6.7 라이브러리 결정

| 용도 | 라이브러리 | 근거 |
|------|----------|-----|
| FTS5 검색 | MEMORY-001 | 재사용 |
| 감정 사전 | goldenfile YAML | |
| Unicode 차트 | stdlib | |
| 정규식 | stdlib | |
| UUID | `google/uuid` | |

### 6.8 TDD 진입

1. RED: `TestOptInDefaultOff` — AC-JOURNAL-001
2. RED: `TestLLM_OptOutDefault` — AC-JOURNAL-002
3. RED: `TestVAD_LocalAnalysis` — AC-JOURNAL-003
4. RED: `TestEmoji_SadDetection` — AC-JOURNAL-004
5. RED: `TestCrisis_CannedResponse` — AC-JOURNAL-005
6. RED: `TestAnniversary_LastYear` — AC-JOURNAL-006
7. RED: `TestAnniversaryPrompt_Wedding` — AC-JOURNAL-007
8. RED: `TestEntry_NotInLogs` — AC-JOURNAL-008
9. RED: `TestA2A_Blocked` — AC-JOURNAL-009
10. RED: `TestExport_UserFiltered` — AC-JOURNAL-010
11. RED: `TestDeleteAll_Immediate` — AC-JOURNAL-011
12. RED: `TestPrivateMode_LocalOnly` — AC-JOURNAL-012
13. GREEN → REFACTOR

### 6.9 TRUST 5 매핑

| 차원 | 달성 |
|-----|-----|
| **T**ested | 85%+, 감정 사전 100+ 코퍼스, crisis 키워드 전수 검증 |
| **R**eadable | writer/analyzer/recall/crisis 명확 분리 |
| **U**nified | StoredEntry 스키마 엄격, VAD 0-1 범위 |
| **S**ecured | opt-in, 로컬 only, A2A 금지, 로그 redaction, crisis response, 즉시 삭제 |
| **T**rackable | 모든 변경 구조화 로그 (text 제외), export/delete audit |

---

## 7. 의존성

| 타입 | 대상 | 설명 |
|-----|------|-----|
| 선행 SPEC | SCHEDULER-001 | EveningCheckInTime 이벤트 |
| 선행 SPEC | MEMORY-001 | E2E 로컬 저장소 |
| 선행 SPEC | INSIGHTS-001 | mood 트렌드 공급 |
| 선행 SPEC | IDENTITY-001 | important_dates |
| 선행 SPEC | HOOK-001 | 이벤트 consumer |
| 후속 SPEC | RITUAL-001 | Bond Level +2 |
| 후속 SPEC | LORA-001 (Phase 6) | 훈련 데이터 (opt-in) |
| 외부 | `google/uuid` | entry ID |

---

## 8. 리스크 & 완화

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 일기 데이터 유출 | 낮 | 치명적 | 로컬 only, 파일 권한 0600, A2A 금지, export per-op 확인 |
| R2 | LLM 분석 시 원문 외부 전송 | 중 | 고 | opt-in + PrivateMode, 프롬프트 명시 제약, 감사 로그 |
| R3 | 자해 키워드 miss (오타, 은어) | 중 | 치명적 | 다양한 표현 코퍼스 유지, 의심스러운 경우 보수적 응답 |
| R4 | 감정 사전 false positive ("행복해야 하는데 안 행복") | 중 | 중 | 부정어 ("안", "못") 감지 시 valence 반전 heuristic |
| R5 | 장기 사용자 DB 크기 (10년 × 365 일기) | 낮 | 낮 | SQLite 충분 (~100MB), 필요 시 archiving |
| R6 | "작년 오늘" 회상이 트라우마 재발 유발 (예: 사별일) | 중 | 고 | 부정 감정 강한 과거 엔트리는 recall 할 때 경고 + 사용자 opt-out |
| R7 | 가족 공유 디바이스에서 본인 외 접근 | 중 | 고 | IDENTITY-001의 user 격리 + 추후 voice auth 확장 |
| R8 | 사용자가 의존하여 전문가 상담 회피 | 중 | 고 | 부정 감정 지속 시 전문가 안내 repeat, AI 한계 명시 |
| R9 | 감정 분석 bias (문화/연령) | 중 | 중 | 감정 사전 한국 구어체 + 고령층 표현 포함 |

---

## 9. 참고

### 9.1 프로젝트 문서

- `.moai/project/adaptation.md` §7 Mood Detection, §11 사용자 제어, §15.2 Emotion Manipulation 금지
- `.moai/specs/SPEC-GOOSE-MEMORY-001/spec.md` — 저장소
- `.moai/specs/SPEC-GOOSE-INSIGHTS-001/spec.md` — 트렌드 소비자
- `.moai/specs/SPEC-GOOSE-IDENTITY-001/spec.md` — important_dates
- `.moai/specs/SPEC-GOOSE-SCHEDULER-001/spec.md` — 이벤트 소스
- `.moai/specs/SPEC-GOOSE-RITUAL-001/spec.md` — Bond Level 연동

### 9.2 외부 참조

- VAD Emotion Model (Russell & Mehrabian 1977)
- GoEmotions Dataset (Google): https://github.com/google-research/google-research/tree/master/goemotions
- 생명의전화: http://www.lifeline.or.kr/
- 자살예방상담전화: https://www.mohw.go.kr/ (1393)
- SQLite FTS5: https://www.sqlite.org/fts5.html

### 9.3 부속 문서

- `./research.md` — 감정 사전 구축 상세, crisis 응답 코퍼스, VAD 정확도 벤치

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **클라우드 백업을 기본 제공하지 않는다** (v0.2+에서 E2E 암호화 조건으로).
- 본 SPEC은 **음성 일기를 포함하지 않는다** (STT 별도 SPEC).
- 본 SPEC은 **이미지 분석을 포함하지 않는다** (첨부 path만).
- 본 SPEC은 **타인과의 공유 기능을 포함하지 않는다** (반려AI 컨셉과 상충).
- 본 SPEC은 **임상 정신 건강 평가를 포함하지 않는다**.
- 본 SPEC은 **AI 심리 상담을 포함하지 않는다** — 전문가 연결만.
- 본 SPEC은 **감정 조작적 프롬프트를 금지한다**.
- 본 SPEC은 **협업 일기 (커플/친구) 를 포함하지 않는다**.
- 본 SPEC은 **A2A 프로토콜을 통한 일기 데이터 전송을 금지한다**.
- 본 SPEC은 **LoRA 훈련에 일기 자동 사용을 금지한다** (사용자 명시 opt-in 필요).
- 본 SPEC은 **AI가 사용자 대신 일기를 쓰지 않는다** (진정성 원칙).
- 본 SPEC은 **Rich media (동영상) 분석을 포함하지 않는다**.
- 본 SPEC은 **Push notification을 포함하지 않는다** (Gateway).

---

**End of SPEC-GOOSE-JOURNAL-001**
