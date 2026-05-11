---
id: SPEC-GOOSE-JOURNAL-001
artifact: plan
version: 0.1.0
created_at: 2026-05-12
updated_at: 2026-05-12
author: manager-spec
---

# SPEC-GOOSE-JOURNAL-001 — 구현 계획 (Plan)

본 문서는 SPEC-GOOSE-JOURNAL-001 의 milestone, task breakdown, 입력/출력 schema, storage 결정, test plan 을 담는다. 우선순위는 priority label(P0~P2) 로 표기하며 시간 추정은 사용하지 않는다.

본 SPEC 은 Sprint 1+2 이후 가용한 인프라를 다음과 같이 재사용한다:
- `internal/audit` AuditWriter — 모든 journal write/delete/export 호출 기록 (`EventType="ritual.journal.invoke"` 신규 추가).
- `internal/permission` Manager — local fs grant 만 사용 (network grant 불필요, JOURNAL 은 e2e local).
- MEMORY-001 SQLite — 일기 저장소 (별도 `journal` 테이블, FTS5 + foreign key user_id).
- IDENTITY-001 — `important_dates` 조회 (Anniversary 트리거).
- INSIGHTS-001 — `OnJournalEntry` consumer 호출 (mood 트렌드 공급).
- HOOK-001 — `EveningCheckInTime` 이벤트 consumer.

본 plan 의 task 는 journal 도메인 로직 (writer + storage 어댑터 + emotion analyzer + crisis detector + anniversary + trend + recall + chart + orchestrator + prompts) 에 집중한다.

---

## 1. Milestone 개요

| Milestone | Priority | 산출물 | 의존 |
|---|---|---|---|
| **M1 — Journal Core (Writer + Storage + Local emotion + Crisis)** | P0 (먼저) | `JournalWriter`, `EmotionAnalyzer` (local dict), `CrisisDetector`, `JournalEntry`/`StoredEntry` DTO, MEMORY-001 storage 어댑터, 저녁 프롬프트 flow (HOOK consumer), opt-in config, 사용자 제어 API (Export/Delete/OptOut) | MEMORY-001 (completed), HOOK-001 (M1 dispatch wired), AUDIT-001 (completed), PERMISSION-001 (completed), IDENTITY-001 (completed for user 격리) |
| **M2 — Long-term Memory Recall (Anniversary + Trend + Search)** | P0 | `MemoryRecall.FindAnniversaryEvents`, `MemoryRecall.FindSimilarMood`, `AnniversaryDetector`, `TrendAggregator.WeeklyTrend/MonthlyTrend/RenderChart`, `JournalWriter.Search` (FTS5), Weekly summary 잡 | M1, IDENTITY-001 important_dates 노출 API |
| **M3 — LLM-assisted Emotion + Summary (opt-in)** | P1 | LLM-assisted emotion analyzer (REQ-017), weekly summary LLM 향상 (REQ-018), payload 제약 enforce (AC-020) | M1, M2, ADAPTER-001 (LLM provider, Sprint 1 LLM-ROUTING-V2 의 LiteLLM router 이용 가능) |

각 milestone 완료 시점에 evaluator-active 회귀 + integration test + audit log 검증.

본 plan.md 의 §2 가 M1 의 22 atomic tasks 를 상세화한다. M2 / M3 는 §3 / §4 에서 high-level breakdown 만 정의 (M1 시점 audit-ready 진입 후 Sprint 3+ 에서 상세화).

---

## 2. M1 — Journal Core (P0)

### 2.1 산출 파일

```
internal/ritual/journal/
├── types.go               # JournalEntry, StoredEntry, Vad, Anniversary, Trend
├── config.go              # Config struct + LoadJournalConfig (yaml.v3)
├── writer.go              # JournalWriter interface + sqliteJournalWriter 구현
├── writer_test.go
├── storage.go             # MEMORY-001 SQLite 어댑터 (journal 테이블 + FTS5)
├── storage_test.go
├── analyzer.go            # EmotionAnalyzer interface + LocalDictAnalyzer
├── analyzer_test.go
├── emotion_dict.go        # 한국어 감정 사전 (12-15 categories, research.md §2.2 기준)
├── crisis.go              # CrisisDetector + crisisKeywords + crisisResponse
├── crisis_test.go
├── orchestrator.go        # JournalOrchestrator + HOOK consumer + Prompt flow
├── orchestrator_test.go
├── prompts.go             # 중립 프롬프트 템플릿 vault (research.md §7.3)
├── prompts_test.go
├── export.go              # ExportAll(userID) + DeleteAll/DeleteByDateRange/OptOut
├── export_test.go
└── audit.go               # internal/audit 어댑터 (EventTypeRitualJournalInvoke 신규)

testdata/journal/
├── emotion_dict.golden.yaml    # 감정 사전 goldenfile
├── crisis_keywords.golden.txt  # crisis 키워드 전수
└── prompts.golden.yaml         # 중립 프롬프트 vault
```

### 2.2 Task breakdown (M1)

총 22 atomic tasks (production 14 + test 6 + integration/seed 2). tasks.md §"M1 Task Decomposition" 와 동기화.

- **T-001**: `types.go` — `JournalEntry`, `StoredEntry`, `Vad`, `Anniversary`, `Trend` DTO 정의 (M2 의 `Trend` 도 미리 선언; 본 task 는 단순 struct + JSON tag).
- **T-002**: `config.go` — `Config` struct + `LoadJournalConfig(path)` 함수. yaml.v3 기반, 누락/빈 파일 시 default 반환 (`enabled=false`, `emotion_llm_assisted=false`, `allow_lora_training=false`, `cloud_backup=false`, `retention_days=-1`, `prompt_timeout_min=60`, `weekly_summary=false`).
- **T-003**: `storage.go` — MEMORY-001 SQLite 어댑터. `journal` 테이블 schema (id PK, user_id, date, text, emoji_mood, vad_valence/arousal/dominance, emotion_tags JSON, anniversary JSON nullable, word_count, created_at, allow_lora_training, crisis_flag, attachment_paths JSON, FTS5 mirror table). 0600 파일 권한 enforce (REQ-002, AC-013).
- **T-004**: `storage_test.go` RED: `TestStorage_InsertAndReadByID`, `TestStorage_FilePermissions_0600_0700` (AC-013), `TestStorage_ListByDateRange`, `TestStorage_DeleteAll_HardDelete` (AC-011), `TestStorage_DeleteByDateRange`, `TestStorage_UserScopedFTS5` (AC-024 부분 — Search 는 M2 에서 완성하지만 user 필터 layer 는 M1 에 포함).
- **T-005**: `crisis.go` — `crisisKeywords` exported var ([]string) + `crisisResponse` exported const + `CrisisDetector.Check(text string) bool` (case-insensitive substring match). research.md §4.1 의 직접 표현만 v0.1 도입.
- **T-006**: `crisis_test.go` RED: `TestCrisis_DirectKeyword_Match` (table-driven 8+ keywords), `TestCrisis_NoFalsePositive_HappyText`, `TestCrisis_CaseInsensitive`, `TestCrisis_CannedResponseHasHotline` (AC-005 의 1577-0199 / 1393 / 1388 모두 포함 검증).
- **T-007**: `prompts.go` — 중립 프롬프트 vault (research.md §7.3 의 `neutral` / `low_mood_sequence` / `anniversary_happy` / `anniversary_sensitive` 카테고리 4종, 각 3+ variant). `prompts.PickNeutral(seed)`, `prompts.PickLowMood()`, `prompts.PickAnniversary(date_name)` API. AC-019 enforce 를 위해 모든 템플릿이 금지 구문 미포함 + 물음표로 끝남.
- **T-008**: `prompts_test.go` RED: `TestPrompts_AllNeutral_NoForbiddenPhrase` (AC-019 — `가장 큰 비밀`/`서운한 점`/`숨기고 싶은`/`부끄러운`/`가장 후회` 부재), `TestPrompts_AllOpenQuestion` (모든 템플릿이 `?` 로 끝남), `TestPrompts_PickAnniversary_IncludesDateName`.
- **T-009**: `analyzer.go` — `EmotionAnalyzer` interface (`Analyze(ctx, text, emojiMood) (*Vad, []string, error)`) + `LocalDictAnalyzer` 구현. 알고리즘은 research.md §2 (토큰화 → tag 매칭 → Top-3 → VAD 가중평균 + 이모지 bonus + 부정어/강조어 처리).
- **T-010**: `emotion_dict.go` — 한국어 감정 사전 hardcoded (research.md §2.2 의 8 카테고리: happy/sad/anxious/angry/tired/calm/excited/grateful + 추가 4: lonely/regret/bored/proud). 각 카테고리는 `keywords []string`, `emoji []string`, `vad Vad` 포함. testdata/journal/emotion_dict.golden.yaml 와 동기화.
- **T-011**: `analyzer_test.go` RED: `TestVAD_LocalAnalysis_Happy` (AC-003), `TestEmoji_SadDetection_Tired` (AC-004), `TestVAD_NegationFlip` (research.md §2.3, "행복하지 않아"), `TestVAD_IntensityModifier` (research.md §2.4, "너무 행복해" arousal 증가), `TestVAD_NoMatch_NeutralFallback` (매칭 없으면 valence=0.5).
- **T-012**: `audit.go` — `internal/audit` 어댑터. `EventTypeRitualJournalInvoke` 신규 상수 등록 (`internal/audit/eventtypes.go` 에 추가). meta keys: `user_id_hash`, `operation` (`write`/`read`/`delete_all`/`delete_range`/`export`/`opt_out`/`evening_prompt_emit`/`evening_prompt_skip`/`evening_prompt_timeout`), `entry_length_bucket` (`<100`/`100-500`/`500+`), `emotion_tags_count`, `has_attachment`, `crisis_flag`, `outcome`. **entry text 미포함** (REQ-004, AC-008).
- **T-013**: `writer.go` — `JournalWriter` interface (`Write`, `Read`, `ListByDate`, `Search` (M2 stub 반환 + user 필터)) + `sqliteJournalWriter` 구현. `Write()` 11-step 시퀀스:
  1. config gate (`enabled == true` 아니면 `ErrJournalDisabled`)
  2. user 격리 검증 (entry.UserID 가 ctx user 와 일치)
  3. CrisisDetector.Check(text) → if hit, set `StoredEntry.CrisisFlag = true` + 응답 buffer 에 crisisResponse prepend
  4. local EmotionAnalyzer.Analyze(text, emojiMood) → vad + tags
  5. (M3 도입 시점) `config.emotion_llm_assisted == true && entry.PrivateMode == false` 분기 → LLM 호출. M1 은 unconditional skip.
  6. anniversary 후보 추출 (M2: AnniversaryDetector 호출, M1: nil)
  7. word count 계산
  8. allow_lora_training = config 값
  9. storage.Insert (재시도 max 3회, in-memory queue max 10) — REQ-012, AC-018
  10. 실패 시 `ErrPersistFailed` + 사용자 메시지 + queue 폐기 (프로세스 종료 시 영속화 없음)
  11. 성공 시 audit ok + INSIGHTS-001 `OnJournalEntry` consumer 호출 (M1 시점에 mock 으로 등록 가능, AC-022)
- **T-014**: `writer_test.go` RED: 9 시나리오:
  - `TestWriter_OptInDefaultOff` (AC-001)
  - `TestWriter_LLMOptOutDefault` (AC-002 — M1 시점 LLM 호출 0회 검증, mock counter)
  - `TestWriter_PrivateMode_LocalOnly` (AC-012 — M1 동일하게 LLM 0회, M3 진입 시 보강)
  - `TestWriter_CrisisFlag_Set` (AC-005 부분 — flag set + canned response 응답에 포함)
  - `TestWriter_LogsRedacted` (AC-008 — zaptest/observer 로 entry text 부재 검증)
  - `TestWriter_A2A_NeverInvoked` (AC-009 — A2A mock counter==0)
  - `TestWriter_AllowLoRATraining_DefaultFalse` (AC-016)
  - `TestWriter_PersistRetry_AndErr` (AC-018 — 3회 fail → ErrPersistFailed + queue evict)
  - `TestWriter_INSIGHTSCallback_OnSuccess` (AC-022 — mock OnJournalEntry counter==1)
- **T-015**: `export.go` — `ExportAll(ctx, userID) ([]byte, error)` (storage layer 에서 `WHERE user_id = ?` strict filter, REQ-016 / AC-010), `DeleteAll(ctx, userID)` (hard delete, AC-011), `DeleteByDateRange`, `OptOut(ctx, userID, deleteData bool)`.
- **T-016**: `export_test.go` RED: `TestExport_UserFiltered` (AC-010 — u1 export 에 u2 entry 0건), `TestDeleteAll_Immediate` (AC-011), `TestDeleteByDateRange_PartialDelete`, `TestOptOut_PreservesData_WhenFlagFalse`, `TestOptOut_DeletesData_WhenFlagTrue`.
- **T-017**: `orchestrator.go` — `JournalOrchestrator` struct + `Prompt(ctx, userID)` (저녁 프롬프트 flow). 흐름:
  1. config.enabled 체크
  2. 오늘 entry 존재 여부 확인 (storage.ListByDate(today, today)) → 있으면 silent skip + INFO log (`operation=evening_prompt_skip`) — AC-014
  3. 없으면 최근 3 entry 의 valence 조회 → 모두 < 0.3 이면 `prompts.PickLowMood()` (REQ-009, AC-015), 그 외 `prompts.PickNeutral()`
  4. anniversary check (M2 에서 활성, M1 은 항상 false)
  5. 프롬프트 emit + `prompt_timeout_min` 만큼 대기 (timeout 시 INFO log `operation=evening_prompt_timeout`)
  6. 사용자 응답 수신 시 `JournalWriter.Write` 호출
- **T-018**: `orchestrator_test.go` RED: `TestOrchestrator_TodayEntryExists_SkipPrompt` (AC-014 — INFO log 1건 + 타이머 미가동), `TestOrchestrator_TimeoutWithoutResponse` (AC-014 보강 — operation=evening_prompt_timeout INFO), `TestOrchestrator_LowMoodSoftTone` (AC-015 — 응답 문자열에 "언제든 이야기해주세요" + "전문가 상담" 포함, "진단"/"우울증"/"PHQ" 미포함), `TestOrchestrator_NoLowMoodNeutralPrompt`.
- **T-019**: `internal/audit/eventtypes.go` 수정 — `EventTypeRitualJournalInvoke = "ritual.journal.invoke"` 상수 추가 + 기존 EventType 카탈로그 테스트 (`eventtypes_test.go`) expected list 갱신.
- **T-020**: HOOK-001 wiring 통합 테스트 — `cmd/goose-runtime` (또는 동등한 bootstrap) 에서 `HookManager.Subscribe("EveningCheckInTime", orchestrator.OnEveningCheckIn)` 호출. **별도 wiring 변경이 필요 없는 경우** (HOOK-001 이 이미 callback registry 만 노출) **본 task 는 integration test 작성만 담당**: `TestEveningHookDispatch_TriggersOrchestratorPrompt`.
- **T-021**: `.moai/docs/journal-quickstart.md` (신규) — opt-in 절차 (`config.journal.enabled=true`), 프라이버시 약관 (로컬 only / LLM opt-in / LoRA opt-in), 사용자 제어 API 사용법 (Export/Delete/OptOut), 위기 시 전문 상담 안내. 사용자 문서.
- **T-022**: 의존성 검증 — `mattn/go-sqlite3` (또는 `modernc.org/sqlite`) 가 MEMORY-001 의존성으로 이미 go.mod 에 존재하는지 `go list -m all | grep sqlite` 로 확인. 미존재 시 plan.md §6 갱신 + `go get` 실행. **본 SPEC 은 신규 외부 의존성 0 을 목표로 한다**(MEMORY-001 의 SQLite 그대로 재사용).

### 2.3 입력/출력 schema (M1)

`JournalWriter.Write` 는 LLM Tool 이 아닌 internal Go API 이므로 JSON Schema 가 아닌 Go struct 를 계약으로 한다. `JournalEntry` 와 `StoredEntry` 의 정확한 필드는 spec.md §6.2 참조.

defensive validation (parser 단에서 enforce):
- `entry.UserID` non-empty
- `entry.Date` non-zero (default = `time.Now().Local()`)
- `entry.Text` 길이 ≥ 1, ≤ 10000 (`MaxEntryTextBytes` const)
- `entry.EmojiMood` 길이 ≤ 8 (단일 grapheme cluster 가정)
- `entry.AttachmentPaths` 각 path 가 `~/.goose/attachments/` prefix (path traversal 방지)
- `entry.PrivateMode` 는 default false

### 2.4 SQLite Schema (storage.go)

```sql
CREATE TABLE IF NOT EXISTS journal_entries (
    id              TEXT PRIMARY KEY,           -- UUID
    user_id         TEXT NOT NULL,
    date            TEXT NOT NULL,              -- YYYY-MM-DD
    text            TEXT NOT NULL,              -- 평문 (E2E 로컬, 0600 파일 권한)
    emoji_mood      TEXT,
    vad_valence     REAL NOT NULL,              -- 0-1
    vad_arousal     REAL NOT NULL,
    vad_dominance   REAL NOT NULL,
    emotion_tags    TEXT NOT NULL,              -- JSON array
    anniversary     TEXT,                        -- JSON object nullable
    word_count      INTEGER NOT NULL,
    created_at      TEXT NOT NULL,              -- RFC3339
    allow_lora_training INTEGER NOT NULL DEFAULT 0,
    crisis_flag     INTEGER NOT NULL DEFAULT 0,
    attachment_paths TEXT                        -- JSON array nullable
);

CREATE INDEX IF NOT EXISTS idx_journal_user_date ON journal_entries(user_id, date);
CREATE INDEX IF NOT EXISTS idx_journal_user_created ON journal_entries(user_id, created_at DESC);

-- FTS5 mirror (M2 Search 에서 사용; M1 은 schema 만 생성)
CREATE VIRTUAL TABLE IF NOT EXISTS journal_fts USING fts5(
    text,
    content='journal_entries',
    content_rowid='rowid',
    tokenize='unicode61 remove_diacritics 1'
);

CREATE TRIGGER IF NOT EXISTS journal_fts_ai AFTER INSERT ON journal_entries BEGIN
  INSERT INTO journal_fts(rowid, text) VALUES (new.rowid, new.text);
END;
CREATE TRIGGER IF NOT EXISTS journal_fts_ad AFTER DELETE ON journal_entries BEGIN
  DELETE FROM journal_fts WHERE rowid = old.rowid;
END;
CREATE TRIGGER IF NOT EXISTS journal_fts_au AFTER UPDATE ON journal_entries BEGIN
  DELETE FROM journal_fts WHERE rowid = old.rowid;
  INSERT INTO journal_fts(rowid, text) VALUES (new.rowid, new.text);
END;
```

DB 파일: `~/.goose/journal/journal.db` (0600), 디렉토리 `~/.goose/journal/` (0700). MEMORY-001 의 base directory 와 별도 격리 (privacy-critical 분리).

### 2.5 Storage backend 선택 근거

| 후보 | 채택 | 이유 |
|---|---|---|
| **SQLite (MEMORY-001 reuse)** | ✅ M1 채택 | (1) MEMORY-001 의존성 이미 존재. (2) FTS5 (M2 Search) 가 native 지원. (3) 날짜 범위 / user 필터 query 가 indexed access. (4) Backup/export 가 단일 파일 copy. |
| bbolt (TOOLS-WEB-001 reuse) | ❌ | KV store 라 range query 가 약함. FTS 미지원. journal 의 query 패턴 (날짜 / FTS / valence) 부적합. |
| flat JSON files | ❌ | concurrent write 위험. query 가 O(n). |
| LevelDB | ❌ | 신규 의존성. SQLite 대비 이점 없음. |

**M1 채택**: SQLite via MEMORY-001 의 SQLite driver.

### 2.6 LLM Integration Boundary (M3)

M1/M2 시점에는 LLM 호출 0 (config 무관). M3 에서 도입할 때 **반드시** 다음 invariants 만족:

1. `config.journal.emotion_llm_assisted == true` AND `entry.PrivateMode == false` 만 LLM 경로 진입.
2. LLM 에 전달되는 payload 는 entry text 만 (user_id, date, attachment, anniversary, emoji_mood 모두 미전송 — AC-020).
3. system prompt 는 REQ-017 의 정확한 문구 ("VAD 모델로 분석하고 Top-3 감정 태그를 JSON으로 반환" + "분석 결과 외에 어떤 조언이나 해석도 포함하지 마세요").
4. 응답이 JSON 파싱 실패 시 silent fallback to local analyzer (사용자 가시 에러 없음).
5. crisis 키워드 감지 entry 는 LLM 으로 전송 금지 (REQ-020 / AC-023, M3 추가 enforce).

LLM provider 는 Sprint 1 의 LLM-ROUTING-V2 (LiteLLM router) 를 호출하되, journal 전용 model 선택 (cost-efficient: claude-haiku 또는 gpt-4o-mini). routing key = `journal.emotion`.

### 2.7 Privacy Invariants (M1 부터 enforce)

| Invariant | 강제 layer | 검증 AC |
|---|---|---|
| 일기 텍스트는 외부로 나가지 않는다 (LLM/A2A/log/error) | writer.go (LLM gate) + audit.go (log redact) + A2A 미연결 | AC-002, AC-008, AC-009, AC-012, AC-020, AC-023 |
| 파일 권한 0600/0700 | storage.go (open mode 강제) | AC-013 |
| 다중 user 격리 | storage.go (모든 query 가 `WHERE user_id = ?`) | AC-010, AC-024 (M2) |
| Hard delete (soft delete 아님) | export.go DeleteAll (`DELETE FROM`) | AC-011 |
| Crisis 응답이 진단/조언 아님 | crisis.go canned response literal-only | AC-005, AC-023 |
| 중립 프롬프트만 emit | prompts.go vault enforce | AC-019 |

---

## 3. M2 — Long-term Memory Recall (P0)

### 3.1 산출 파일 (high-level)

```
internal/ritual/journal/
├── recall.go              # MemoryRecall.FindAnniversaryEvents/FindSimilarMood
├── recall_test.go
├── anniversary.go         # AnniversaryDetector (IDENTITY-001 important_dates)
├── anniversary_test.go
├── trend.go               # TrendAggregator.WeeklyTrend/MonthlyTrend
├── trend_test.go
├── chart.go               # RenderChart (Unicode sparkline ▁▂▃▄▅▆▇█)
├── chart_test.go
├── search.go              # JournalWriter.Search 본 구현 (FTS5 + user filter)
├── search_test.go
├── summary.go             # WeeklySummaryJob (일요일 22:00, REQ-018)
└── summary_test.go
```

### 3.2 Task breakdown (요약, Sprint 3 에서 상세화)

- T-023: `recall.go` — Anniversary SQL (spec.md §6.5) + FindSimilarMood (cosine similarity on Vad).
- T-024: `anniversary.go` — IDENTITY-001 client + ±1day 매칭. 부정 감정 과거 회상 필터 (research.md §5.3).
- T-025: `trend.go` — WeeklyTrend / MonthlyTrend + SparklinePoints (`math.NaN()` for missing days, AC-025).
- T-026: `chart.go` — RenderChart with Unicode block + `NO_COLOR` env 지원 (AC-026).
- T-027: `search.go` — FTS5 query + user_id 강제 + rank 정렬 + tiebreak (AC-024).
- T-028: `summary.go` — Weekly summary job + LLM 호출은 M3 에서 (M2 는 local aggregation 만 — top tags, valence avg, wordcloud).
- T-029: orchestrator.go 수정 — anniversary 분기 활성, AC-007 (결혼기념일 프롬프트) 통과.
- T-030: tasks.md M2 task breakdown append + progress.md M2 Run Phase 섹션 append.

M2 신규 AC: AC-006, AC-007, AC-021 (cadence), AC-022 (INSIGHTS callback 의 anniversary 필드 보강), AC-024, AC-025, AC-026.

### 3.3 의존성 (M2 신규)

- 외부 의존성 신규 0 (stdlib `math` + 기존 SQLite + IDENTITY-001 client).

---

## 4. M3 — LLM-assisted Emotion + Summary (P1)

### 4.1 산출 파일 (high-level)

```
internal/ritual/journal/
├── analyzer_llm.go        # LLMEmotionAnalyzer (REQ-017)
├── analyzer_llm_test.go
├── summary_llm.go         # Weekly summary LLM 향상 (REQ-018)
└── summary_llm_test.go
```

### 4.2 Task breakdown (요약)

- T-031: `analyzer_llm.go` — LLM-ROUTING-V2 client + system prompt enforce + JSON parse fallback to local.
- T-032: `analyzer_llm_test.go` — LLM mock 호출 시 payload assertion (entry text 만 전송, AC-020), system prompt 정확 일치, crisis entry 시 LLM 호출 0회 (AC-023).
- T-033: `summary_llm.go` — weekly summary 의 한 줄 요약을 LLM 으로 생성 (input: 평균 vad + top tags + word frequencies, output: 1줄 자연어).
- T-034: writer.go 수정 — LLM 분기 활성 (M1 `unconditional skip` 제거).
- T-035: tasks.md M3 task breakdown append + progress.md M3 Run Phase 섹션 append.

M3 신규 AC: AC-020, AC-023 (LLM 분기 검증), AC-002 / AC-012 의 보강 (LLM mock counter 검증).

---

## 5. 공통 인프라 (Sprint 1+2 재사용)

### 5.1 Audit (AUDIT-001)

- `internal/audit` AuditWriter 그대로 재사용.
- 신규 EventType: `EventTypeRitualJournalInvoke = "ritual.journal.invoke"` (T-019).
- meta keys (AC-008 enforce): `user_id_hash`, `operation`, `entry_length_bucket`, `emotion_tags_count`, `has_attachment`, `crisis_flag`, `outcome`. **entry text 절대 포함 금지**.
- audit log 자체는 plaintext (AUDIT-001 정책) 이므로 user_id 도 sha256(user_id)[:8] 로 hash.

### 5.2 Permission (PERMISSION-001)

- network grant: 불필요 (M1/M2). M3 LLM 분기 진입 시 ADAPTER-001 자체의 grant 메커니즘 사용.
- fs grant: `~/.goose/journal/**` 첫 write 시 grant 요청. CI / non-interactive 환경은 default seed (FS-ACCESS-001 의 default_allow_paths) 에 등록하거나 설치 시 사전 grant 등록.

### 5.3 Storage (MEMORY-001)

- SQLite driver 재사용. journal 전용 DB 파일 분리 (`~/.goose/journal/journal.db`) 로 MEMORY 의 facts DB 와 격리.
- migration: `journal_entries` 테이블 + FTS5 mirror + 트리거 3개 (idempotent CREATE IF NOT EXISTS).

### 5.4 HOOK (HOOK-001)

- `EveningCheckInTime` 이벤트 consumer 등록 (T-020).
- HOOK-001 의 callback registry 패턴에 부합.

### 5.5 INSIGHTS (INSIGHTS-001)

- `OnJournalEntry(entry *StoredEntry)` consumer 호출 (writer.go Step 11).
- INSIGHTS-001 미등록 상태에서는 호출 skip (AC-022 후반).

### 5.6 IDENTITY (IDENTITY-001)

- `important_dates` 조회 API (M2 anniversary.go).

### 5.7 Logging (zap)

- 구조화 로그 + entry text redaction. zaptest/observer 로 negative test (AC-008).

---

## 6. 외부 의존성 (M1 시점)

| 패키지 | 버전 (목표) | 신규/재사용 | 용도 |
|---|---|---|---|
| `github.com/mattn/go-sqlite3` 또는 `modernc.org/sqlite` | MEMORY-001 정렬 | 재사용 (T-022 검증) | SQLite driver |
| `github.com/google/uuid` | latest | 재사용 (다른 SPEC 사용 중) | StoredEntry.ID |
| `gopkg.in/yaml.v3` | latest | 재사용 (config) | journal.yaml 파싱 |
| `go.uber.org/zap` | latest | 재사용 (CORE) | structured log |

**신규 외부 의존성 (M1)**: 0 (모두 기존 의존성 재사용 확인 후). 신규 발견 시 plan.md §6 갱신 + AskUserQuestion.

M2/M3 는 모두 stdlib (math/strings/sort) + 기존 의존성 (LLM-ROUTING-V2 client, IDENTITY-001 client) 으로 구현.

---

## 7. 테스트 전략

### 7.1 단위 테스트

- 각 journal 파일마다 `*_test.go` (Go convention).
- SQLite: `t.TempDir()` + in-memory 또는 임시 파일 (`?cache=shared&mode=rwc`).
- LLM mock: M3 시점에 stub interface (M1/M2 는 mock counter 만 사용해 호출 0회 검증).
- HOOK mock: dispatch 시뮬레이터 (`hook.NewSyncDispatcher()`).
- INSIGHTS mock: `OnJournalEntry` 호출 카운터 + 인자 캡처.

### 7.2 통합 테스트 (`integration_test.go` build tag)

- 실제 SQLite + 임시 디렉토리 + 실제 audit log writer.
- 시나리오: HOOK dispatch → orchestrator prompt → user 응답 → writer 저장 → audit 검증.

### 7.3 Goldenfile

- `testdata/journal/emotion_dict.golden.yaml` — 한국어 감정 사전 12-15 카테고리.
- `testdata/journal/crisis_keywords.golden.txt` — 8+ 키워드 전수.
- `testdata/journal/prompts.golden.yaml` — 중립 프롬프트 vault.
- M2: `testdata/journal/sparkline_seven_days.golden.txt` — RenderChart 출력 baseline.

### 7.4 보안 테스트

- AC-008 (log redaction): zap observer 로 entry text 부재 검증.
- AC-009 (A2A): A2A mock counter==0 검증 (writer 가 A2A 미참조).
- AC-013 (file permissions): `os.Stat` mode 비교.
- AC-019 (prompt vault): prompts 전수 검사 (forbidden phrase 부재).
- AC-023 (no clinical advice): 응답 string 에 진단성 어휘 부재 + LLM mock counter==0.

### 7.5 동시성 테스트

- writer.go 의 retry queue (max 10) 가 race-free 한지 `-race` 플래그로 검증.
- HOOK dispatch 와 user 응답 race condition 시 timeout 의 deterministic 동작 검증.

---

## 8. Test plan 매핑 (AC ↔ Test file/function)

| AC | Milestone | Test file | Test function |
|---|---|---|---|
| AC-JOURNAL-001 (opt-in default off) | M1 | `writer_test.go` | `TestWriter_OptInDefaultOff` |
| AC-JOURNAL-002 (LLM opt-out default) | M1 | `writer_test.go` | `TestWriter_LLMOptOutDefault` |
| AC-JOURNAL-003 (VAD local) | M1 | `analyzer_test.go` | `TestVAD_LocalAnalysis_Happy` |
| AC-JOURNAL-004 (emoji 분석) | M1 | `analyzer_test.go` | `TestEmoji_SadDetection_Tired` |
| AC-JOURNAL-005 (crisis canned response) | M1 | `crisis_test.go` + `writer_test.go` | `TestCrisis_CannedResponseHasHotline` + `TestWriter_CrisisFlag_Set` |
| AC-JOURNAL-006 (작년 오늘 회상) | M2 | `recall_test.go` | `TestRecall_AnniversaryEvents_LastYear` |
| AC-JOURNAL-007 (기념일 프롬프트) | M2 | `orchestrator_test.go` | `TestOrchestrator_AnniversaryPrompt_Wedding` |
| AC-JOURNAL-008 (log redaction) | M1 | `writer_test.go` | `TestWriter_LogsRedacted` |
| AC-JOURNAL-009 (A2A 금지) | M1 | `writer_test.go` | `TestWriter_A2A_NeverInvoked` |
| AC-JOURNAL-010 (export user filter) | M1 | `export_test.go` | `TestExport_UserFiltered` |
| AC-JOURNAL-011 (DeleteAll) | M1 | `export_test.go` | `TestDeleteAll_Immediate` |
| AC-JOURNAL-012 (PrivateMode LLM 미호출) | M1 (보강 M3) | `writer_test.go` | `TestWriter_PrivateMode_LocalOnly` |
| AC-JOURNAL-013 (file permissions) | M1 | `storage_test.go` | `TestStorage_FilePermissions_0600_0700` |
| AC-JOURNAL-014 (today entry skip) | M1 | `orchestrator_test.go` | `TestOrchestrator_TodayEntryExists_SkipPrompt` + `TestOrchestrator_TimeoutWithoutResponse` |
| AC-JOURNAL-015 (low mood soft tone) | M1 | `orchestrator_test.go` | `TestOrchestrator_LowMoodSoftTone` |
| AC-JOURNAL-016 (allow_lora_training default false) | M1 | `writer_test.go` | `TestWriter_AllowLoRATraining_DefaultFalse` |
| AC-JOURNAL-017 (retention auto delete) | M1 | `storage_test.go` | `TestStorage_RetentionDays_NightlyCleanup` |
| AC-JOURNAL-018 (persist retry + ErrPersistFailed) | M1 | `writer_test.go` | `TestWriter_PersistRetry_AndErr` |
| AC-JOURNAL-019 (prompt vault forbidden phrase) | M1 | `prompts_test.go` | `TestPrompts_AllNeutral_NoForbiddenPhrase` + `TestPrompts_AllOpenQuestion` |
| AC-JOURNAL-020 (LLM payload 제약) | M3 | `analyzer_llm_test.go` | `TestLLMAnalyzer_PayloadIsTextOnly` |
| AC-JOURNAL-021 (weekly summary cadence) | M2 | `summary_test.go` | `TestWeeklySummary_SundayCadence_Generates` |
| AC-JOURNAL-022 (INSIGHTS callback) | M1 | `writer_test.go` | `TestWriter_INSIGHTSCallback_OnSuccess` |
| AC-JOURNAL-023 (clinical advice 금지) | M1 (보강 M3) | `crisis_test.go` + `analyzer_llm_test.go` (M3) | `TestCrisis_NoClinicalLanguage` + `TestLLMAnalyzer_NeverCalledOnCrisis` |
| AC-JOURNAL-024 (Search FTS5 user scope) | M2 | `search_test.go` | `TestSearch_FTS5_UserScoped` |
| AC-JOURNAL-025 (WeeklyTrend/MonthlyTrend 집계) | M2 | `trend_test.go` | `TestWeeklyTrend_AggregationWithGaps` |
| AC-JOURNAL-026 (RenderChart) | M2 | `chart_test.go` | `TestRenderChart_SevenDaysWithNaN` |

M1 단계는 16 AC (001/002/003/004/005/008/009/010/011/012/013/014/015/016/017/018/019/022/023) 를 GREEN 으로 한다 (총 19 AC, 일부 보강은 M3).
M2 단계는 6 AC (006/007/021/024/025/026) 추가.
M3 단계는 1 AC (020) + AC-002/012/023 보강.

---

## 9. Risk mitigation 작업

| Risk | Plan-level 작업 |
|---|---|
| R1 일기 데이터 유출 | M1 시점 file permission 0600/0700 enforce + log redaction + A2A 미연결 + multi-user storage filter. AC-008/009/010/013 GREEN gate 가 회귀 차단 |
| R2 LLM 분석 시 원문 외부 전송 | M1/M2 LLM 호출 0 (`config.emotion_llm_assisted` 무관 unconditional skip). M3 진입 시 AC-020 / AC-023 GREEN 필수 |
| R3 자해 키워드 miss (오타/은어) | research.md §4.1 직접 표현 8 개 + crisis_keywords.golden.txt 검토. v0.2+ 에서 간접 표현 확장 (§4.2). false positive 우려는 응답이 진단/조언 아닌 정보 제공이므로 보수적 매칭 허용 |
| R4 감정 사전 false positive ("행복해야 하는데 안") | research.md §2.3 의 negation flip heuristic 구현 (T-009). T-011 의 `TestVAD_NegationFlip` 가 회귀 차단 |
| R5 SQLite DB 크기 (10년 × 365 entry ≈ 3650 row) | 100MB 미만 추정. archiving 미필요. T-022 의존성 검증 시 SQLite WAL 설정 적용 |
| R6 "작년 오늘" 회상이 트라우마 재발 | M2 anniversary.go 의 valence < 0.3 필터 (research.md §5.3) + user opt-out flag |
| R7 가족 공유 디바이스 격리 | M1 storage 의 user_id 격리 + 모든 query strict filter. voice/face auth 는 별도 SPEC |
| R8 사용자가 AI 의존하여 전문가 회피 | M1 crisis 응답이 항상 전문 상담 안내 + AC-023 의 진단/조언 금지 enforce. 사용자 nudge 강요 금지 |
| R9 감정 분석 bias (문화/연령) | research.md §2.2 의 사전 + 한국 구어체 + 고령층 표현 추가. golden test 로 회귀 차단 |
| R10 신규 외부 의존성 발견 | T-022 에서 sqlite driver 확인. 미존재 시 user 승인 후 추가 + plan.md §6 갱신 |
| R11 audit log 가 entry text 누출 | T-012 audit.go 가 meta keys 만 emit. AC-008 의 zap observer test 가 회귀 차단 |
| R12 prompt vault drift | T-008 의 `TestPrompts_AllNeutral_NoForbiddenPhrase` + `TestPrompts_AllOpenQuestion` 가 모든 변경 회귀 차단 |

---

## 10. 종료 조건 (Plan-level DoD)

### M1 DoD

- [ ] T-001 ~ T-022 모든 task 완료.
- [ ] `internal/ritual/journal/` 패키지가 `cmd/goose-runtime` (또는 동등) bootstrap 에 wired.
- [ ] AC-JOURNAL-001 / 002 / 003 / 004 / 005 / 008 / 009 / 010 / 011 / 012 / 013 / 014 / 015 / 016 / 017 / 018 / 019 / 022 / 023 (M1 16 AC) 모두 GREEN.
- [ ] Coverage ≥ 85% (`internal/ritual/journal/*.go` 파일 기준).
- [ ] golangci-lint 0 warning, go vet 0 issue, gofmt clean.
- [ ] log redaction 검증 (zap observer 로 negative test, AC-008).
- [ ] AUDIT-001 + PERMISSION-001 + HOOK-001 e2e GREEN (`TestEveningHookDispatch_TriggersOrchestratorPrompt`).
- [ ] `.moai/docs/journal-quickstart.md` 작성 (사용자 가이드).
- [ ] MEMORY-001 / IDENTITY-001 (M2 의존만 — M1 에서는 user 격리만) / INSIGHTS-001 회귀 0.
- [ ] crisis 키워드 전수 테스트 + canned response 핫라인 번호 (1577-0199, 1393, 1388) 모두 포함 검증.

### M2 DoD (Sprint 3+)

- [ ] T-023 ~ T-030 모든 task 완료.
- [ ] AC-JOURNAL-006 / 007 / 021 / 024 / 025 / 026 GREEN.
- [ ] Anniversary 자동 회상이 valence < 0.3 entry 자동 제외 (R6).
- [ ] Search FTS5 가 100 entry 미만에서 100ms 이내 응답.

### M3 DoD (Sprint 3+)

- [ ] T-031 ~ T-035 모든 task 완료.
- [ ] AC-JOURNAL-020 GREEN.
- [ ] AC-JOURNAL-002 / 012 / 023 보강 (LLM mock counter 검증) GREEN.
- [ ] LLM-ROUTING-V2 client 통합 + journal 전용 model 선택 정상 동작.
- [ ] LLM payload 가 entry text 만 포함 + system prompt 가 REQ-017 정확 일치.

---

Version: 0.1.0
Last Updated: 2026-05-12
