---
id: SPEC-GOOSE-JOURNAL-001
version: 0.2.0
status: planned
created_at: 2026-04-22
updated_at: 2026-04-25
author: manager-spec
priority: P0
issue_number: null
phase: 7
size: 중(M)
lifecycle: spec-anchored
labels:
  - phase-7
  - domain/journal
  - privacy-critical
  - memory
  - identity
  - emotion-analysis
  - ritual/evening
---

# SPEC-GOOSE-JOURNAL-001 — Evening Journal + Emotion Tagging + Long-term Memory Recall (E2E Local, Privacy-First)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성 (Phase 7 #37, MEMORY + INSIGHTS + SCHEDULER 의존) | manager-spec |
| 0.2.0 | 2026-04-25 | 감사 리포트(JOURNAL-001-audit.md, 0.55/FAIL) 결함 교정. MP-3 labels 채움. MP-2 EARS 라벨 정합성 확보(REQ-013/014/016 [Ubiquitous] 재라벨, REQ-015 `If ... then ... shall` 패턴으로 재작성 + 진단 금지 조항 REQ-020 분리). Traceability: AC-013~026 신설(기존 미커버 REQ 10건 + 신설 REQ 4건). D4: Search/TrendAggregator/RenderChart를 REQ-021/022/023으로 승격. D5: AC-006에서 존재하지 않는 `distance_years` 참조 제거(검증 방식 수정). D6: config 샘플에 `prompt_timeout_min`, `weekly_summary` 추가. REQ-001~019 번호·순서 불변. | manager-spec |

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
      enabled: false             # opt-in (REQ-001)
      emotion_llm_assisted: false  # 원문 외부 전송 금지 기본 (REQ-003, REQ-017)
      allow_lora_training: false   # 사용자 별도 동의 (REQ-010)
      cloud_backup: false          # v0.2+ (OUT OF SCOPE)
      retention_days: -1           # 무기한 (-1) / >0 이면 자동 정리 (REQ-011)
      prompt_timeout_min: 60       # 저녁 프롬프트 대기 시간(분) (REQ-005)
      weekly_summary: false        # 일요일 22:00 주간 요약 생성 (REQ-018)
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

**REQ-JOURNAL-009 [Event-Driven]** — **When** the most recently saved 3 entries (ordered by `created_at DESC`, ignoring calendar gaps) show consistent negative valence (all three with `Vad.Valence < 0.3`), the prompt **shall** soften its tone and include "언제든 이야기해주세요, 들을게요. 도움이 필요하시면 전문가 상담도 좋은 방법이에요." — but **shall not** recommend specific providers or diagnose. (D8 해소: "recent 3 entries" = "최근 저장 순 3건".)

**REQ-JOURNAL-021 [Event-Driven]** — **When** `JournalWriter.Search(ctx, userID, query)` is called, the writer **shall** execute a SQLite FTS5 query scoped to `user_id = userID` AND `session_id = 'journal'`, return matching `StoredEntry` records ordered by FTS5 rank, and **shall not** include entries belonging to other users. (D4 해소: Search 승격.)

**REQ-JOURNAL-022 [Event-Driven]** — **When** `TrendAggregator.WeeklyTrend(ctx, userID, week)` or `MonthlyTrend(ctx, userID, month)` is called, the aggregator **shall** return a `*Trend` containing `Period`, `From`, `To`, `AvgValence`, `AvgArousal`, `AvgDominance`, `MoodDistribution` (tag → count), `EntryCount`, and `SparklinePoints` (daily `Valence` for the requested window); missing days **shall** be represented by `math.NaN()` and **shall not** fabricate values. (D4 해소: TrendAggregator 승격.)

**REQ-JOURNAL-023 [Event-Driven]** — **When** `TrendAggregator.RenderChart(trend)` is called, the renderer **shall** emit a terminal sparkline using the Unicode block glyph set `{▁▂▃▄▅▆▇█}` with one glyph per day in the `Trend.From..Trend.To` window, label each line with the weekday short-name and the rounded `Valence` value, and **shall not** emit ANSI color codes when `os.Getenv("NO_COLOR")` is set. (D4 해소: RenderChart 승격.)

### 4.3 State-Driven

**REQ-JOURNAL-010 [State-Driven]** — **While** `config.journal.allow_lora_training == false` (default), entries **shall** have `AllowLoRATraining: false` and LORA-001 (Phase 6) **shall** exclude them from training datasets.

**REQ-JOURNAL-011 [State-Driven]** — **While** `config.journal.retention_days > 0`, entries older than the retention window **shall** be automatically deleted during nightly cleanup (03:00 local); the default (-1) means infinite retention.

**REQ-JOURNAL-012 [State-Driven]** — **While** MEMORY-001 is unavailable, `Write` **shall** buffer the entry in memory with a maximum queue of 10 and retry; after 3 retry failures, `Write` returns `ErrPersistFailed` and the user sees "일기 저장 실패, 다시 시도해주세요" — the text is not lost during the session but **shall** be discarded on process exit to prevent silent leakage.

### 4.4 Unwanted / Ubiquitous Negations

이 섹션은 두 가지 EARS 카테고리를 함께 다룬다:
- **[Ubiquitous]** 재라벨: 조건절이 없는 순수 금지 사항 — "The system shall not …" 형태. 기존 [Unwanted] 라벨이 EARS canonical 패턴("If [undesired], then [system] shall [response]")에 부합하지 않아 재라벨함(MP-2 해소).
- **[Unwanted]**: 특정 입력/상태가 감지되었을 때("If ... then ... shall ...") 강제 응답이 있는 금지 사항.

**REQ-JOURNAL-013 [Ubiquitous]** — The prompt generator **shall not** use language designed to elicit specific emotions or to extract sensitive information (e.g., "당신의 가장 큰 비밀은?", "부모님께 서운한 점은?"); only open, neutral prompts from the approved template set defined in `§6 prompts.go` (e.g., "오늘 어떠셨어요?", "기억에 남는 순간 있으세요?") **shall** be emitted. (D2 해소: 조건절 없는 순수 금지 → Ubiquitous negation.)

**REQ-JOURNAL-014 [Ubiquitous]** — The subsystem **shall not** transmit journal content via the A2A protocol under any circumstances; journal data is never inter-agent shareable. (D2 해소: Ubiquitous negation.)

**REQ-JOURNAL-015 [Unwanted]** — **If** an entry's text (case-insensitive substring match) contains any indicator from the documented crisis keyword list (see `§6.4 crisisKeywords`), **then** the system **shall** (a) emit the canned crisis response from `§6.4 crisisResponse` before any other downstream processing, (b) still persist the entry locally with `StoredEntry.CrisisFlag = true`, (c) emit an INFO-level structured log without the entry text. (D2/D10 해소: canonical Unwanted 패턴 `If … then … shall …`로 재작성; 진단 금지 조항은 REQ-JOURNAL-020으로 분리; crisis keyword list는 `§6.4`를 정규 참조로 승격.)

**REQ-JOURNAL-016 [Ubiquitous]** — In multi-user scenarios the export function **shall not** include other users' data; `ExportAll(userID)` **shall** filter strictly by `userID` at the storage layer, not in post-processing. (D2 해소: Ubiquitous negation.)

**REQ-JOURNAL-020 [Ubiquitous]** — The subsystem **shall not** produce clinical diagnoses, therapeutic advice, or mental-health risk assessments of the user; any response triggered by crisis indicators **shall** be limited to the canned response text defined in `§6.4 crisisResponse` and **shall not** be paraphrased, extended with additional advice, or augmented with LLM-generated commentary. (REQ-JOURNAL-015에서 분리된 진단 금지 조항. D2/one-requirement-per-entry 원칙 준수.)

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

**AC-JOURNAL-006 — 작년 오늘 회상 (REQ-007)**
- **Given** u1이 2025-04-22에 `StoredEntry` 저장(`CreatedAt=2025-04-22T21:00:00Z`), 오늘=2026-04-22
- **When** `MemoryRecall.FindAnniversaryEvents(ctx, u1, 2026-04-22)` 호출
- **Then** 반환 슬라이스 길이 ≥ 1, 해당 슬라이스 중 적어도 한 엔트리의 `CreatedAt.Year() == 2025` AND `CreatedAt.Month() == 4` AND `|CreatedAt.Day() - 22| ≤ 1`; 반환 엔트리의 스키마 필드는 `StoredEntry` 정의(§6.2)에 한정하며 `distance_years` 같은 확장 필드는 기대하지 않는다. (D5 해소: 존재하지 않는 필드 참조 제거; 검증은 `CreatedAt` 기반 연 차이로 수행.)

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

<!-- 이하 AC-013 ~ AC-026: SPEC 0.2.0 감사 교정으로 신설 (D3 + D4 + REQ-020) -->

**AC-JOURNAL-013 — 파일 권한 0600/0700 (REQ-002)**
- **Given** 초기화된 journal 저장소 경로(`~/.goose/journal/`)
- **When** 신규 엔트리 저장 후 `os.Stat` 으로 파일/디렉토리 mode 조회
- **Then** SQLite 데이터 파일 mode `0600`, 포함 디렉토리 mode `0700`; 그 외 bit 설정 없음.

**AC-JOURNAL-014 — 저녁 프롬프트 이미 기록된 날 skip (REQ-005)**
- **Given** 오늘 날짜에 대한 `StoredEntry`가 이미 존재, `config.journal.prompt_timeout_min=60`
- **When** HOOK-001 이 `EveningCheckInTime` 을 dispatch
- **Then** 사용자에게 프롬프트 미노출; INFO 로그 1건(`operation=evening_prompt_skipped`); 프롬프트 대기 타이머 미가동.
- **And Given** 오늘 엔트리 없음 상태에서 동일 이벤트 발생 시 프롬프트 출력 및 최대 60분 대기 후 미응답이면 `operation=evening_prompt_timeout` INFO 로그.

**AC-JOURNAL-015 — 연속 저가 Valence 시 소프트 톤 (REQ-009)**
- **Given** u1의 최근 저장 3건 `Vad.Valence` 가 각각 0.20, 0.25, 0.28
- **When** `JournalOrchestrator.Prompt(u1)` 호출
- **Then** 렌더된 프롬프트 문자열에 "언제든 이야기해주세요" AND "전문가 상담" 포함; "진단", "우울증", "PHQ" 같은 진단성 용어 **미포함**.

**AC-JOURNAL-016 — AllowLoRATraining 기본 false (REQ-010)**
- **Given** `config.journal.allow_lora_training` 미지정(기본)
- **When** `JournalWriter.Write` 후 `StoredEntry.AllowLoRATraining` 조회
- **Then** `AllowLoRATraining == false`; LORA-001 의 training dataset exporter mock 호출 시 본 엔트리 포함되지 않음.

**AC-JOURNAL-017 — Retention 기반 자동 삭제 (REQ-011)**
- **Given** `config.journal.retention_days=30`, 현재 저장소에 `CreatedAt=now-40d` 엔트리 1건, `CreatedAt=now-10d` 엔트리 1건
- **When** 03:00 nightly cleanup 잡 실행(트리거 수동 호출)
- **Then** 40일 전 엔트리는 SQLite에서 삭제(`ListByDate` 에 미포함), 10일 전 엔트리는 유지; `retention_days=-1` 일 때는 동일 시나리오에서 두 엔트리 모두 유지.

**AC-JOURNAL-018 — MEMORY 불가 시 버퍼링 + ErrPersistFailed (REQ-012)**
- **Given** MEMORY-001 mock 이 `Write` 호출 시 에러 반환하도록 설정
- **When** `JournalWriter.Write` 를 연속 3회 호출
- **Then** (a) 1~2회차는 내부 큐(max 10)에 버퍼 + 재시도, (b) 3회차 최종 실패 후 `ErrPersistFailed` 반환, (c) 사용자에게 "일기 저장 실패, 다시 시도해주세요" 메시지 전달, (d) 프로세스 종료(SIGTERM) 이후 재시작 시 큐가 비어 있음(디스크 영속화 없음).

**AC-JOURNAL-019 — 중립 프롬프트 강제 (REQ-013)**
- **Given** 프롬프트 템플릿 레지스트리 전수
- **When** `prompts.All()` 반환 문자열 각각을 검사
- **Then** 어떤 템플릿도 금지 구문(`가장 큰 비밀`, `서운한 점`, `숨기고 싶은`, `부끄러운`, `가장 후회`) 을 포함하지 않음; 모든 템플릿이 open question(물음표) 로 끝남.

**AC-JOURNAL-020 — LLM 프롬프트 payload 제약 (REQ-017)**
- **Given** `config.journal.emotion_llm_assisted=true`, 엔트리 `PrivateMode=false`, LLM adapter mock
- **When** Write 처리 중 LLM 호출 캡처
- **Then** LLM 호출 payload는 (a) 시스템 프롬프트가 "VAD 모델로 분석하고 Top-3 감정 태그를 JSON으로 반환" 문구와 "분석 결과 외에 어떤 조언이나 해석도 포함하지 마세요" 문구를 모두 포함, (b) user payload는 entry text 외의 필드(user_id, date, attachment path, 기념일) 를 **전송하지 않음**(mock 어설션).

**AC-JOURNAL-021 — 주간 요약 생성 cadence (REQ-018)**
- **Given** `config.journal.weekly_summary=true`, 가상 시계를 일요일 22:00 로 설정, 지난 7일 간 엔트리 5건 저장
- **When** weekly summary 잡 실행
- **Then** 생성된 요약 객체에 (a) 지난 7일 평균 `Valence`, (b) 빈도 상위 3 emotion tag, (c) wordcloud 상위 토큰 리스트 포함; 다음 저녁 프롬프트 렌더 시 요약 제시 flag 활성. `weekly_summary=false` 일 때 동일 시나리오에서 요약 미생성.

**AC-JOURNAL-022 — INSIGHTS 연동 (REQ-019)**
- **Given** INSIGHTS-001 mock 이 등록됨
- **When** `JournalWriter.Write` 성공
- **Then** `insights.OnJournalEntry(entry)` mock 이 정확히 1회 호출되며, 전달된 `entry` 는 저장된 `StoredEntry` 와 동일한 `ID`, `Vad`, `EmotionTags` 필드를 지님. INSIGHTS 미등록 상태에서는 호출이 발생하지 않고 Write 는 정상 성공.

**AC-JOURNAL-023 — 진단/조언 금지 (REQ-020)**
- **Given** crisis 키워드 포함 입력("죽고 싶다") 및 저가 Valence 입력 두 가지 시나리오
- **When** 시스템 응답 수집
- **Then** 두 경우 모두 응답 텍스트가 `crisisResponse`(§6.4) 리터럴과 **완전 일치**하거나 빈 문자열(기본 프롬프트); "진단", "우울", "장애", "처방", "상담받으세요" 등 임상/지시적 어휘 **미포함**; LLM adapter mock 호출 0회.

**AC-JOURNAL-024 — Search FTS5 사용자 스코프 (REQ-021)**
- **Given** u1 엔트리 20건(그 중 5건 텍스트에 "산책" 포함), u2 엔트리 10건(그 중 3건 "산책" 포함)
- **When** `JournalWriter.Search(ctx, u1, "산책")`
- **Then** 반환 건수 == 5; 반환 전 항목의 `UserID == u1`; u2 엔트리 0건; 결과 순서는 FTS5 rank 기준 내림차순(동률 시 `CreatedAt DESC` tiebreak).

**AC-JOURNAL-025 — WeeklyTrend/MonthlyTrend 집계 (REQ-022)**
- **Given** u1, 지난 7일 각 날짜에 엔트리 1건씩 저장(`Valence` = 0.2, 0.4, NaN 없음, ...), 중간 1일은 엔트리 없음
- **When** `TrendAggregator.WeeklyTrend(ctx, u1, today)`
- **Then** 반환 `*Trend` 의 `Period == "week"`, `EntryCount == 6`, `AvgValence` 는 실제 저장 6건의 산술 평균, `SparklinePoints` 길이 == 7, 엔트리 없는 날의 해당 index 는 `math.IsNaN(SparklinePoints[i]) == true`; `MoodDistribution` 합계 == `EntryCount`.

**AC-JOURNAL-026 — RenderChart 출력 (REQ-023)**
- **Given** `Trend.SparklinePoints = [0.2, 0.5, 0.9, NaN, 0.3, 0.7, 0.6]`, `From..To` = 7일, 환경변수 `NO_COLOR=""`
- **When** `TrendAggregator.RenderChart(trend)` 문자열 캡처
- **Then** 출력은 7줄(요일 label 포함), 각 줄의 막대 부분은 `{▁▂▃▄▅▆▇█}` 집합에 속하는 정확히 1개의 글리프, NaN 인 날은 공백 또는 `·` 로 표기; ANSI color escape sequence(`\x1b[`) 포함 여부는 `NO_COLOR=""` 일 때 허용, `NO_COLOR=1` 일 때 0 occurrences.

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
