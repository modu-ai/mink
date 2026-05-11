# Journal (저녁 일지) 빠른 시작 가이드

**SPEC**: SPEC-GOOSE-JOURNAL-001 M1  
**버전**: v0.1.0  
**최종 수정**: 2026-05-12

---

## 개요

Goose 저녁 일지 기능은 매일 저녁 짧은 프롬프트를 통해 하루를 기록하는 개인 일지입니다. 모든 데이터는 기기 내부(SQLite)에만 저장되며, 사용자 동의 없이 외부 서버나 LLM으로 전송되지 않습니다.

---

## 1. 기능 활성화 (옵트인)

일지 기능은 **기본적으로 비활성** 상태입니다. 사용하려면 아래 설정 파일을 생성하거나 편집하세요.

```yaml
# ~/.goose/journal.yaml
enabled: true
data_dir: ~/.goose/journal/   # 데이터 저장 경로 (기본값)
retention_days: -1            # 보관 기간 (-1: 무기한 보관)
prompt_timeout_min: 60        # 응답 대기 시간 (분)
allow_lora_training: false    # 기본 false: 내 글로 AI 학습 금지
```

**중요**: `allow_lora_training: true`로 설정하지 않는 한, 작성한 일지는 AI 모델 학습에 사용되지 않습니다.

---

## 2. 저녁 체크인 흐름

저녁 체크인 시간(`EveningCheckInTime`)이 되면 Goose가 짧은 질문을 합니다.

```
Goose: 오늘 하루 어떠셨어요?
나:    친구들과 즐겁게 저녁 먹었어요. 오랜만에 웃었어요.
```

- 응답하지 않으면(타임아웃) 기록 없이 넘어갑니다.
- 당일 이미 기록한 경우 중복 프롬프트를 표시하지 않습니다.

---

## 3. 프라이버시 정책

| 항목 | 정책 |
|------|------|
| 데이터 저장 위치 | 기기 내 SQLite (`~/.goose/journal/journal.db`) |
| 외부 전송 | 없음 (M1 기준; M3에서 사용자 동의 시 LLM 감정 분석 옵션 추가 예정) |
| A2A 메시지 | 미지원 — 일지 패키지는 A2A 임포트 없음 |
| 로그 기록 | 일지 텍스트는 로그에 기록되지 않음 |
| 사용자 ID | SHA-256 해시(앞 8자리)로만 로그에 남음 |
| 파일 권한 | 디렉토리 0700, DB 파일 0600 |
| LoRA 학습 | 기본 false — `allow_lora_training: true` 명시 동의 필요 |

---

## 4. 사용자 제어 API

### 4.1 데이터 내보내기

모든 일지를 JSON 형식으로 내보낼 수 있습니다.

```go
mgr := journal.NewExportManager(storage, auditor)
data, err := mgr.ExportAll(ctx, userID)
// data는 JSON bytes, entry_count / entries 포함
```

내보내기 파일 형식:

```json
{
  "user_id_hash": "ab12cd34",
  "exported_at": "2026-05-12T10:00:00Z",
  "entry_count": 42,
  "entries": [...]
}
```

### 4.2 기간별 삭제

```go
// from ~ to 범위의 일지 삭제 (양 끝 포함)
err := mgr.DeleteByDateRange(ctx, userID, from, to)
```

### 4.3 전체 삭제

```go
// 모든 일지 영구 삭제 (복구 불가)
err := mgr.DeleteAll(ctx, userID)
```

### 4.4 옵트아웃

```go
// deleteData=true: 데이터 삭제 후 옵트아웃
// deleteData=false: 데이터 유지, 옵트아웃만 기록
err := mgr.OptOut(ctx, userID, deleteData)
```

---

## 5. 위기 감지 및 안전망

일지에 위기 표현이 감지되면 아래 자원 정보가 응답에 포함됩니다. 이 기능은 전문적인 진단을 대체하지 않습니다.

```
지금 많이 힘드시겠어요. 혼자 감당하지 않아도 괜찮아요.

도움받을 수 있는 곳:
• 자살예방상담전화: 1393 (24시간)
• 정신건강 위기상담: 1577-0199 (24시간)
• 청소년 상담: 1388 (24시간)
```

위기 감지 여부와 무관하게 **일지 텍스트는 외부로 전송되지 않습니다**.

---

## 6. 감정 분석 (M1 로컬 방식)

M1에서는 LLM 없이 로컬 사전 기반 VAD(Valence-Arousal-Dominance) 모델을 사용합니다.

- VAD 범위: 0.0 ~ 1.0 (정규화)
- 12개 감정 카테고리: 행복, 슬픔, 불안, 분노, 피로, 평온, 흥분, 감사, 외로움, 후회, 지루함, 뿌듯함
- 부정어 처리: "행복하지 않아" → 낮은 valence
- 강도 수식어: "매우 행복해" → valence 상향 보정
- M3에서는 `emotion_llm_assisted: true` 설정 시 LLM 기반 분석으로 전환 예정

---

## 7. 데이터 보관 정책

| 설정 | 동작 |
|------|------|
| `retention_days: -1` | 무기한 보관 (기본값) |
| `retention_days: 30` | 30일 이상 된 일지 매일 자동 삭제 |
| `retention_days: 0` | 매일 전체 삭제 (권장하지 않음) |

---

## 8. 주요 AC 목록 (M1)

| AC | 내용 |
|----|------|
| AC-001 | 기능 비활성 시 쓰기 요청 반환 오류 |
| AC-005 | 위기 표현 감지 시 CrisisFlag=true 저장 |
| AC-008 | 로그에 일지 텍스트 미포함 |
| AC-009 | A2A 패키지 임포트 없음 (정적 검증) |
| AC-010 | 내보내기는 요청 사용자 데이터만 포함 |
| AC-011 | 삭제는 즉시 하드 삭제 (복구 불가) |
| AC-012 | Private Mode 활성 시 LLM 미호출 |
| AC-013 | 디렉토리 0700 / DB 파일 0600 |
| AC-014 | 당일 이미 기록한 경우 중복 프롬프트 스킵 |
| AC-015 | 최근 3일 모두 저기분 시 부드러운 톤 프롬프트 |
| AC-016 | `allow_lora_training` 기본 false |
| AC-018 | 저장 실패 시 최대 3회 재시도 후 오류 반환 |
| AC-022 | 쓰기 성공 시 INSIGHTS 콜백 1회 호출 |

---

## 9. 관련 파일

| 파일 | 설명 |
|------|------|
| `internal/ritual/journal/` | 핵심 구현 패키지 |
| `internal/ritual/journal/config.go` | 설정 로드 |
| `internal/ritual/journal/writer.go` | 일지 쓰기 인터페이스 |
| `internal/ritual/journal/orchestrator.go` | 저녁 체크인 오케스트레이터 |
| `internal/ritual/journal/export.go` | 내보내기/삭제 API |
| `internal/ritual/journal/storage.go` | SQLite 저장소 |
| `.moai/specs/SPEC-GOOSE-JOURNAL-001/` | 전체 SPEC 문서 |
