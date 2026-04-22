# Research — SPEC-GOOSE-BRIEFING-001

## 1. LLM Prompt 실험 결과 (Mock)

### 1.1 Baseline (Flat 3-section)

Input: weather="맑음 23도", calendar="09:00 스탠드업, 14:00 김과장 미팅", fortune="감성 바이오리듬 +80%"

Output:
```
날씨: 맑음 23도
일정: 09:00 스탠드업, 14:00 김과장 미팅
운세: 감성 +80%
```

→ 건조. 사용자 피드백 실험: "재미없다", "AI가 만들어준 느낌"

### 1.2 Narrative 스타일 (warm, medium)

시스템 프롬프트에 "자연스러운 인사처럼 연결" 추가:

```json
{
  "greeting": "좋은 아침입니다, goos님.",
  "main": "오늘 서울은 맑고 23도로 딱 좋은 날씨예요. 9시에 스탠드업이 있으시니 그 전에 커피 한 잔 어떠세요? 오후 2시 김과장님과 미팅은 감성 바이오리듬이 좋은 시간대라 대화가 술술 풀릴 것 같아요.",
  "closing": "기운 내서 출발해보세요!",
  "full_text": "좋은 아침입니다, goos님. 오늘 서울은 맑고 23도로 딱 좋은 날씨예요. 9시에 스탠드업이 있으시니 그 전에 커피 한 잔 어떠세요? 오후 2시 김과장님과 미팅은 감성 바이오리듬이 좋은 시간대라 대화가 술술 풀릴 것 같아요. 기운 내서 출발해보세요!"
}
```

→ 사용자 피드백: "친구가 말하는 것 같다", "개인 비서 같다".

### 1.3 Tone 변형 비교

| Tone | Greeting 예 |
|------|-----------|
| warm | "좋은 아침입니다, goos님. 🌞" |
| professional | "안녕하세요 goos님. 오늘의 브리핑입니다." |
| playful | "굿모닝~ goos님! 🎶 오늘도 파이팅~" |

Tone 선택에 따라 전체 문체 크게 달라짐. 사용자 mood가 낮을 때 playful 피하기 (REQ-BRIEF-018).

### 1.4 Length별 character count

- short: 80-120자 (단일 문단)
- medium: 180-250자 (3-4 문장)
- long: 350-450자 (5-7 문장, 각 섹션 상세)

## 2. TTS 엔진 비교

| 엔진 | 한국어 품질 | 로컬 | 크기 | 라이선스 | 결정 |
|------|---------|-----|-----|--------|-----|
| Piper (ko_KR-lee) | 상 (자연스러움) | O | 100MB 모델 | MIT | **default** |
| espeak-ng | 하 (로봇음) | O | 5MB | GPLv3 | fallback |
| Google Cloud TTS | 최상 | X (네트워크) | - | 유료 | opt-in |
| Amazon Polly Neural | 최상 | X | - | 유료 | opt-in |
| Coqui TTS | 상 | O | 500MB | MPL-2.0 | 후보2 |

### 2.1 Piper 설치

```
# macOS
brew install piper

# Linux
wget https://github.com/rhasspy/piper/releases/download/v1.2.0/piper_linux_x86_64.tar.gz

# 한국어 모델
wget https://huggingface.co/rhasspy/piper-voices/resolve/main/ko/ko_KR/lee/low/ko_KR-lee-low.onnx
```

### 2.2 Piper 호출 latency

- Warm-up: 300-800ms (모델 로드)
- 200자 생성: ~500ms
- 총 latency: 300자 → 1.3초

허용 범위 (10s 이내).

## 3. 스타일 적응 예시

### 3.1 사용자 mood 낮음 + warm

```
안녕, goos야. 오늘 많이 지쳤지? 비가 오려나 봐. 무리하지 말고, 오전 스탠드업 마치고 한 숨 돌리자. 무슨 일 있으면 언제든 이야기해 줘.
```

### 3.2 사용자 mood 높음 + playful

```
goos님 굿모닝! 🌞 오늘 날씨 완전 최고예요! 9시 스탠드업에서 멋지게 한마디 해주시구요 ✨ 오후 미팅도 파이팅!
```

### 3.3 professional 톤 (직장인)

```
안녕하세요, goos 님. 오늘 서울 기온 23도, 맑음. 오전 9시 스탠드업, 오후 2시 김과장 미팅이 예정되어 있습니다. 집중이 잘되는 오후 시간대이니 미팅 자료 최종 점검 권장드립니다.
```

## 4. 공기질 경보 처리

### 4.1 Level 매핑

| Level | 문구 |
|------|------|
| good / moderate | 언급 안 함 |
| unhealthy | 본문에 포함 ("공기질 주의") |
| very_unhealthy | ⚠️ 프리픽스 + 마스크 권장 |
| hazardous | ⚠️ 프리픽스 + 외출 자제 권장 |

### 4.2 실제 예

```
⚠️ 오늘 주의사항: 미세먼지가 "매우 나쁨" 수준이에요. 외출 시 KF94 마스크 꼭 챙기시고, 가능하면 실외 활동을 줄여주세요.

---

[나머지 narrative 이어짐...]
```

## 5. 임박 일정 감지

### 5.1 기준

브리핑 시간 기준 2시간 이내 시작하는 일정 = 임박.

### 5.2 표현

- 30분 이내: "30분 뒤 [제목]"
- 1시간 이내: "곧 [시간]에 [제목]"
- 2시간 이내: "[시간]에 [제목]"

## 6. 중복 억제 세부

### 6.1 Key 구조

```
briefing:{userID}:{userLocalDate}:{TZ}
```

### 6.2 Is_read flag

- 생성 시 `is_read=false`
- 사용자가 응답하거나 10분 경과 시 `is_read=true`
- 재트리거는 `is_read=false`인 경우에만 차단

## 7. Privacy 고려

### 7.1 Visibility 속성

IDENTITY-001 Person entity의 각 attribute에 `visibility` 필드:

```
birth_date: {value, visibility: "fortune_only"}
occupation: {value, visibility: "briefing_ok"}
phone: {value, visibility: "never"}
```

본 SPEC은 LLM 프롬프트에 `visibility ∈ {briefing_ok, *}` 만 포함.

### 7.2 LLM 전송 최소화

- 사용자 본명: "OOO님" 형태로 허용 (공개 정보 가정)
- 가족 구성: 기본 미포함
- 주소: 도시 단위만
- 전화: 절대 포함 금지

## 8. 에러 메시지 UX

### 8.1 전체 실패 (all sources fail)

```
죄송해요, 지금 아침 정보를 불러오지 못했어요. 잠시 후 다시 시도하거나,
설정에서 개별 데이터 소스를 확인해주세요.

[다시 시도] [설정 열기]
```

### 8.2 부분 실패 (1-2 sources)

LLM에게 "~ 정보는 제공받지 못했습니다" 컨텍스트 전달 → 자연스럽게 생략.

## 9. TDD Fixture

### 9.1 Mock MorningBriefing

```
briefing := &MorningBriefing{
  UserID: "u1",
  Date: time.Date(2026,4,22,0,0,0,0, KST),
  TriggeredBy: "scheduler",
  Sections: Sections{
    Weather: &weather.WeatherReport{TemperatureC: 23, ConditionKo: "맑음"},
    Calendar: &calendar.DailySchedule{Events: [...]},
    Fortune: nil,  // birth_date 없음
  },
  Narrative: "...",
  FallbackUsed: false,
}
```

### 9.2 LLM Mock 패턴

- Valid JSON response
- Invalid JSON (fallback 트리거)
- 3000자 초과 (truncation 트리거)
- Timeout (10s 초과)
- PII 포함 ("010-1234-5678" 등) → visibility 필터가 차단

## 10. 통합 테스트

실제 Piper + mock 소스로 e2e 검증:
1. Mock event 발화
2. Orchestrator HandleMorningBriefingTime 호출
3. 3개 소스 mock response
4. LLM mock JSON 반환
5. Piper 실행 → audio 파일 생성
6. MEMORY-001 save
7. Terminal 출력 검증

## 11. 오픈 이슈

1. **LLM 호출 비용**: 매일 1회 × 365일 = 연 365 LLM call. 2000 tokens 평균 → 연 $2-5. 부담 없으나 무료 local LLM fallback 고려.
2. **TTS 캐시 재활용**: 유사한 날씨·일정이면 이전 audio 재생? (사용자 부정적 반응 가능, 매일 새로 생성이 심리적으로 좋음)
3. **다국어 혼용**: 한국어 briefing에 영어 회의명 섞이면 TTS 어색. Piper ko 모델이 영어 알파벳을 음절 단위로 읽음 → 후처리 필요.
4. **사용자 interactive 응답**: 브리핑 후 "자세히 알려줘" 같은 질문에 어떻게 이어갈지 — 별도 CLI conversation flow.
5. **adaptive timing**: SCHEDULER-001과의 양방향 feedback loop 신중히 설계.

## 12. 참고

- Piper voices: https://huggingface.co/rhasspy/piper-voices
- Korean TTS evaluation: https://github.com/deepinsight/insightface  (관련 benchmark)
- SSML (Speech Synthesis Markup): https://www.w3.org/TR/speech-synthesis/
- Chatbot briefing 사례: Slack's Donut bot, Lifesum morning brief
