---
id: SPEC-GOOSE-JOURNAL-001
artifact: spec-compact
version: 0.1.0
created_at: 2026-05-12
---

# SPEC-GOOSE-JOURNAL-001 (Compact)

> 한 페이지 요약. LLM 시스템 프롬프트 / 작업 컨텍스트 inject 용.

## 목적

GOOSE v6.0 Daily Companion 의 **저녁 리추얼**. 사용자가 자유 텍스트(또는 이모지)로 일기를 작성 → 로컬 감정 분석(VAD) → MEMORY-001 SQLite 에 e2e local 저장 → "작년 오늘" / "한 달 전" 회상 + 장기 감정 트렌드 시각화. 다마고치 Bond Level +2 의 가장 강한 ritual.

**프라이버시 핵심**: 일기는 가장 민감한 개인 정보. 기본 로컬 only / LLM opt-in (원문 외부 전송 default 금지) / LoRA opt-in / hard delete / A2A 금지 / 로그 redaction / 진단/조언 금지.

## 패키지 위치

`internal/ritual/journal/*.go` (web tool 아님, 별도 ritual 도메인).

DB: `~/.goose/journal/journal.db` (0600), 디렉토리 `~/.goose/journal/` (0700). MEMORY-001 의 facts DB 와 격리.

## 핵심 계약

- **opt-in default off** (`config.journal.enabled=false`). 미설정 시 `ErrJournalDisabled`.
- **LLM analyzer opt-out default** (`config.emotion_llm_assisted=false`). M1/M2 시점 LLM 호출 0회 unconditional. M3 진입 시 분기 활성, payload 는 entry text 만 (AC-020).
- **PrivateMode entry**: LLM 호출 영구 금지 (M3 도 enforce, AC-012).
- **Crisis 키워드 hit**: canned response (1577-0199 / 1393 / 1388) literal 응답 + storage 저장 + `crisis_flag=true` + LLM 호출 0회 (AC-005, AC-023).
- **A2A 전송 금지**: A2A client import 부재 정적 검증 (AC-009).
- **Log redaction**: zap log + audit log 모두 entry text 부재 (AC-008). `user_id` 는 sha256(user_id)[:8] hash.
- **Multi-user 격리**: 모든 storage query 가 `WHERE user_id = ?` strict filter (AC-010, AC-024).
- **Hard delete**: `DeleteAll` 이 SQL `DELETE FROM` (soft delete 아님, AC-011).
- **File permission 0600/0700** enforce (AC-013). 잘못된 권한 디렉토리 발견 시 error 반환 (silent chmod 금지).
- **Prompt vault 강제**: 금지 구문 (`가장 큰 비밀`/`서운한 점`/`숨기고 싶은`/`부끄러운`/`가장 후회`) 부재 + 모든 템플릿 open question (AC-019).
- **No clinical language**: crisis canned response 외 응답에 `진단`/`우울`/`치료`/`처방`/`PHQ` 부재 (AC-023).

## 주요 타입

```
JournalEntry { UserID, Date, Text, EmojiMood, AttachmentPaths, PrivateMode }
StoredEntry { ID, UserID, Date, Text, EmojiMood, Vad, EmotionTags, Anniversary, WordCount, CreatedAt, AllowLoRATraining, CrisisFlag, AttachmentPaths }
Vad { Valence 0-1, Arousal 0-1, Dominance 0-1 }
Trend { Period "week"|"month", From, To, AvgValence, AvgArousal, AvgDominance, MoodDistribution map, EntryCount, SparklinePoints []float64 (NaN for missing days) }
```

## 주요 API

| API | 입력 | 출력 | M |
|---|---|---|---|
| `JournalWriter.Write` | `JournalEntry` | `*StoredEntry, error` | M1 |
| `JournalWriter.Read` | `(userID, entryID)` | `*StoredEntry, error` | M1 |
| `JournalWriter.ListByDate` | `(userID, from, to)` | `[]*StoredEntry, error` | M1 |
| `JournalWriter.Search` | `(userID, query)` | `[]*StoredEntry, error` (FTS5) | M2 (M1 stub) |
| `EmotionAnalyzer.Analyze` | `(text, emojiMood)` | `*Vad, []string, error` | M1 (Local) / M3 (LLM) |
| `MemoryRecall.FindAnniversaryEvents` | `(userID, date)` | `[]*StoredEntry, error` | M2 |
| `MemoryRecall.FindSimilarMood` | `(userID, currentVad, limit)` | `[]*StoredEntry, error` | M2 |
| `AnniversaryDetector.CheckToday` | `(userID)` | `*Anniversary, error` | M2 |
| `TrendAggregator.WeeklyTrend / MonthlyTrend` | `(userID, anchor)` | `*Trend, error` | M2 |
| `TrendAggregator.RenderChart` | `*Trend` | `string` (Unicode sparkline) | M2 |
| `JournalOrchestrator.Prompt` | `(userID)` | side-effect (HOOK consumer) | M1 |
| `Export.ExportAll` | `(userID)` | `[]byte` JSON | M1 |
| `Export.DeleteAll / DeleteByDateRange / OptOut` | `(userID, ...)` | `error` | M1 |

## EARS 26 REQ (요약)

- **Ubiquitous**: REQ-001 (opt-in), REQ-002 (storage 0600), REQ-003 (LLM gate), REQ-004 (log redaction), REQ-013 (neutral prompt only), REQ-014 (A2A 금지), REQ-016 (export user filter), REQ-020 (no clinical advice).
- **Event-Driven**: REQ-005 (HOOK consumer + skip + timeout), REQ-006 (analyze flow), REQ-007 (FindAnniversaryEvents), REQ-008 (anniversary prompt), REQ-009 (low valence soft tone), REQ-021 (Search FTS5 user scope), REQ-022 (Trend aggregation), REQ-023 (RenderChart Unicode).
- **State-Driven**: REQ-010 (allow_lora_training=false default), REQ-011 (retention nightly cleanup), REQ-012 (MEMORY 불가 시 buffer + ErrPersistFailed).
- **Unwanted**: REQ-015 (crisis canned response).
- **Optional**: REQ-017 (LLM-assisted), REQ-018 (weekly summary), REQ-019 (INSIGHTS callback).

## AC 26 (요약)

- **M1 (19)**: AC-001~005, 008~019, 022, 023.
- **M2 (6)**: AC-006, 007, 021, 024, 025, 026.
- **M3 (1 + 보강 3)**: AC-020 + AC-002/012/023 의 LLM 분기 보강.

## Milestones (priority)

- **M1 (P0)**: Journal Core — Writer + Storage + Local emotion + Crisis + Orchestrator + Export. 22 atomic tasks. 19 AC GREEN. **Sprint 2 진입 첫 milestone**.
- **M2 (P0)**: Long-term Memory Recall — Anniversary + Trend + Search + Weekly summary cadence. 8 추가 task. 6 AC.
- **M3 (P1)**: LLM-assisted emotion + Summary 향상. 5 추가 task. 1 AC + 3 보강.

## OUT (명시적 제외)

- 클라우드 백업 (v0.2+ E2E 암호화 필수).
- 음성 일기 (STT 별도 SPEC).
- 이미지 분석 (첨부 path 만).
- 타인과 공유 (반려AI 컨셉 상충).
- 임상 정신 건강 평가 (PHQ-9 별도 SPEC).
- AI 심리 상담 (전문가 연결만).
- 감정 조작 프롬프트.
- 협업 일기 (커플/친구).
- A2A 전송.
- LoRA 자동 사용 (명시 opt-in 필요).
- AI 가 사용자 대신 일기 작성 (진정성 원칙).
- Rich media (동영상) 분석.
- Push notification (Gateway).

## 의존

- 선행 SPEC: SCHEDULER-001 (EveningCheckInTime, Sprint 1 v0.2.2 completed), MEMORY-001 (SQLite + FTS5), HOOK-001 (callback registry), AUDIT-001 (completed), PERMISSION-001 (completed), IDENTITY-001 (M2 important_dates), INSIGHTS-001 (consumer).
- 후속 SPEC: RITUAL-001 (Bond Level +2), LORA-001 (Phase 6 training data, opt-in).
- 외부: `mattn/go-sqlite3` 또는 `modernc.org/sqlite` (MEMORY-001 재사용), `google/uuid` (재사용), `gopkg.in/yaml.v3` (재사용), `go.uber.org/zap` (재사용).
- **신규 외부 의존성 (M1)**: 0 (T-022 검증).

## 핵심 invariants (LLM 컨텍스트용)

1. `enabled=false` 가 default — 첫 호출 시 privacy notice 후 user opt-in 받음.
2. entry text 는 (a) LLM 으로 (M1/M2 unconditional skip, M3 config 분기) (b) A2A 로 (영구 금지) (c) log/audit 으로 (영구 금지) (d) export 로 (해당 user 만) 의 4 경로 외 외부로 나가지 않는다.
3. crisis 키워드 hit entry 는 canned response literal 외 어떤 추가 텍스트도 emit 하지 않는다 (LLM 호출 0회).
4. AI 가 사용자 대신 일기를 쓰거나 감정을 유도하는 프롬프트는 vault 에 없다.
5. 모든 storage access 가 `WHERE user_id = ?` 로 격리된다 (export, delete, list, search 모두).
6. retention 자동 삭제, hard delete (soft delete 아님), file permission 0600/0700 이 storage layer 책임.

---

Version: 0.1.0
Last Updated: 2026-05-12
