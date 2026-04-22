# Research — SPEC-GOOSE-JOURNAL-001

## 1. VAD Model 배경

### 1.1 정의

- Valence: 긍정 ↔ 부정 (happy vs sad)
- Arousal: 각성 ↔ 이완 (excited vs calm)
- Dominance: 통제 ↔ 피통제 (in-control vs overwhelmed)

Russell & Mehrabian (1977) 이후 사회심리학·HCI 표준.

### 1.2 왜 VAD인가

- Categorical model (6 basic emotions) 보다 연속값 → 미묘한 감정 표현
- Dimension 독립 → "흥분했지만 부정적인" (불안) 같은 조합 구분 가능
- 시각화에 유리 (2D/3D plot)

### 1.3 범위 값

본 SPEC은 0-1 normalize. [-1, +1] 대신.
- Valence: 0 = 가장 부정, 0.5 = 중립, 1 = 가장 긍정
- Arousal: 0 = 최저 각성, 1 = 최고 각성
- Dominance: 0 = 무력감, 1 = 통제감

## 2. 한국어 감정 사전 구축

### 2.1 출처

- KoSAC (Korean Sentiment Analysis Corpus) - SKT
- KOSAC (Korea University Sentiment Corpus)
- 자체 확장 goldenfile (goos 사용자 커뮤니티 피드백)

### 2.2 구조

```yaml
emotions:
  happy:
    ko:
      keywords: [행복, 기쁘, 좋, 웃, 즐거, 신나, 설레, 만족, 뿌듯, 감사]
      emoji: [😊, 😄, 🥰, 😁, 🎉]
    vad: {valence: 0.9, arousal: 0.7, dominance: 0.7}

  sad:
    ko:
      keywords: [슬프, 우울, 힘들, 외로, 울었, 눈물, 쓸쓸, 허전, 공허]
      emoji: [😢, 😭, 😔, 😞, 💔]
    vad: {valence: 0.15, arousal: 0.3, dominance: 0.3}

  anxious:
    ko:
      keywords: [불안, 걱정, 초조, 긴장, 떨리, 걱정돼, 안절, 초조해]
      emoji: [😰, 😟, 😨]
    vad: {valence: 0.3, arousal: 0.75, dominance: 0.25}

  angry:
    ko:
      keywords: [화, 짜증, 열받, 빡, 분노, 억울, 답답, 짜증나]
      emoji: [😠, 😡, 🤬, 💢]
    vad: {valence: 0.2, arousal: 0.85, dominance: 0.6}

  tired:
    ko:
      keywords: [피곤, 지쳐, 힘들, 졸리, 나른, 녹초]
      emoji: [😴, 🥱, 😪]
    vad: {valence: 0.4, arousal: 0.15, dominance: 0.35}

  calm:
    ko:
      keywords: [평온, 차분, 느긋, 조용, 고요, 편안, 안정]
      emoji: [😌, 🧘, 🌸]
    vad: {valence: 0.65, arousal: 0.25, dominance: 0.6}

  excited:
    ko:
      keywords: [설렘, 기대, 두근, 흥분, 신나, 짜릿]
      emoji: [🤩, 🥳, 🎊]
    vad: {valence: 0.85, arousal: 0.9, dominance: 0.7}

  grateful:
    ko:
      keywords: [감사, 고마, 감동, 뭉클, 따뜻]
      emoji: [🙏, 🥹, ❤️]
    vad: {valence: 0.88, arousal: 0.5, dominance: 0.5}
```

총 12-15 emotion 카테고리 커버.

### 2.3 부정어 처리

"행복하지 않아", "안 기뻐" 같은 표현:

```
detect_negation(text, keyword_pos):
  window = text[max(0, keyword_pos-5):keyword_pos]
  if contains(["안", "못", "없", "않"]) in window:
    return flipped_vad  # valence: 1-v
```

단순 휴리스틱이지만 baseline 충분.

### 2.4 강조어 처리

"너무 행복해", "정말 슬퍼":

```
intensity_modifiers = ["너무", "정말", "엄청", "매우", "진짜"]
if modifier in window:
    arousal *= 1.2 (cap 1.0)
```

## 3. LLM-Assisted VAD (opt-in)

### 3.1 LLM Prompt

```
다음 일기 텍스트의 감정을 VAD 모델로 분석하세요.

텍스트:
{entry_text}

출력 JSON (다른 설명 없이 JSON만):
{
  "valence": 0.0-1.0,
  "arousal": 0.0-1.0,
  "dominance": 0.0-1.0,
  "top_emotion_tags": ["tag1", "tag2", "tag3"],
  "reasoning_summary": "20자 이내 요약 (감정 조언이나 해석 금지)"
}
```

### 3.2 비용 고려

매일 1 엔트리 × 1000 tokens = 연 $0.50 내외 (claude-haiku 기준). 부담 적음.

### 3.3 정확도 비교 (mock experiment)

| 방법 | Valence MAE | Top Tag F1 |
|------|-----------|----------|
| Local dict | 0.18 | 0.65 |
| LLM (Claude Haiku) | 0.09 | 0.82 |
| LLM (GPT-4o) | 0.07 | 0.85 |

Local 도 충분한 수준. LLM은 nuance 포착에 유리.

## 4. Crisis Keywords 확장

### 4.1 직접 표현

```
죽고 싶, 자살, 사라지고 싶, 살기 싫, 끝내고 싶, 없어지고 싶
```

### 4.2 간접 표현 (v0.2+에서 확대)

```
지쳐서 못 버티, 포기하고 싶, 의미 없, 도망가고 싶, 잠들어서 안 깼으면
```

v0.1은 직접 표현만. 간접은 false positive 많아 신중.

### 4.3 응답 내용

```
지금 많이 힘드시죠. 말씀해주셔서 고마워요.

혼자 감당하기 어려운 순간에는 전문가의 도움이 큰 힘이 되어요:
- 생명의전화: 1577-0199 (24시간, 무료)
- 자살예방상담전화: 1393 (24시간)
- 청소년전화: 1388

당신의 이야기를 진심으로 들어줄 사람들이 있어요.
```

### 4.4 추적

crisis_flag=true 엔트리는 INSIGHTS 트렌드에도 반영. 지속적인 경우 (주 2회 이상 3주 연속) 사용자에게 "혹시 전문가와 이야기 나누는 것 생각해보셨나요?" 부드러운 nudge. 단, 강요 절대 금지.

## 5. Anniversary Logic

### 5.1 매치 범위

- 1년 전: today ± 1일 (작년 같은 날 기준)
- 2-10년 전: 동일

### 5.2 "2주년", "10주년" 강조

```
if distance_years % 5 == 0:
    emphasize = true  // "벌써 10년이네요..."
```

### 5.3 부정 감정 과거 회상 주의

과거 엔트리 valence < 0.3 이면 자동 recall 생략. 예: 사별·이별 관련 일기를 매년 자동 소환하면 재트라우마.

```
filter_out_if_valence_low(entry):
    return entry.vad.valence >= 0.3 OR user_opt_in_triggered
```

## 6. Terminal Chart Design

### 6.1 Sparkline (unicode blocks)

```
▁ ▂ ▃ ▄ ▅ ▆ ▇ █
```

Valence 0-1 값을 8 bucket 으로 매핑:

```go
func sparkline(values []float64) string {
    blocks := "▁▂▃▄▅▆▇█"
    result := ""
    for _, v := range values {
        idx := int(v * 8)
        if idx >= 8 { idx = 7 }
        result += string(blocks[idx])
    }
    return result
}
```

### 6.2 주간 표시

```
📊 이번 주 감정 트렌드:
Mon ▆  Tue ▇  Wed ▃  Thu ▂  Fri ▅  Sat █  Sun ▇
평균 valence: 0.65 (중간 +)
```

### 6.3 Heatmap (월간)

```
4월 감정 히트맵:
  W1: ▃▅▆▇▇▁▂
  W2: ▂▃▄▅▇█▆
  W3: ▁▁▂▄█▇▇
  W4: ▂▃▄▃▁▂▁
```

## 7. Prompt Design 원칙

### 7.1 금지

- 특정 답 유도: "가장 행복했던 순간은?" (긍정 bias)
- 감정 추궁: "왜 그런 감정을 느꼈어요?" (분석 압박)
- 민감 정보 요구: "누구랑 싸웠어요?", "비밀 이야기"

### 7.2 권장

- 열린 질문: "오늘 어떠셨어요?"
- 선택지 제공: "짧게 한 단어? 긴 이야기? 편한 방식으로"
- 침묵 허용: "안 쓰셔도 괜찮아요, 그저 하루 수고하셨어요"

### 7.3 프롬프트 vault

```yaml
prompts:
  neutral:
    - "오늘 하루 어떠셨어요?"
    - "잠들기 전, 생각나는 순간이 있어요?"
    - "기분 한 줄로 표현한다면?"
  anniversary_happy:
    - "오늘은 [date_name]이네요. 어떻게 보내셨어요?"
  anniversary_sensitive:
    - "오늘은 특별한 날이죠. 무리하지 마시고, 편하게 이야기해주세요."
  low_mood_sequence:
    - "요즘 많이 힘드시죠. 이야기해주셔도, 안 하셔도 괜찮아요."
```

랜덤 선택으로 지루함 방지.

## 8. Weekly Summary

일요일 밤 22:00:

```
📖 이번 주 돌아보기 (2026-04-16 ~ 2026-04-22)

엔트리: 5개
평균 감정: 0.72 (긍정적)
가장 행복한 날: 토요일 (0.92)
가장 힘든 날: 수요일 (0.45)

많이 등장한 감정:
  🥰 행복 (5회)
  💫 기대 (3회)
  😴 피곤 (3회)

자주 언급된 단어:
  "회의" 7, "친구" 4, "운동" 3, "커피" 3

한 줄 요약:
  업무 스트레스 있었지만, 주말에 회복된 한 주였어요.
```

Summary 생성은 LLM 사용 (opt-in).

## 9. Export 스키마

### 9.1 JSON

```json
{
  "schema_version": "1.0",
  "user_id": "u1",
  "exported_at": "2026-04-22T23:00:00Z",
  "entry_count": 150,
  "entries": [
    {
      "id": "...",
      "date": "2026-04-22",
      "text": "...",
      "emoji_mood": "😊",
      "vad": {"valence": 0.8, "arousal": 0.6, "dominance": 0.7},
      "emotion_tags": ["happy", "grateful"],
      "anniversary": null,
      "word_count": 150,
      "created_at": "..."
    },
    ...
  ]
}
```

### 9.2 Markdown (human-readable)

```markdown
# Journal Export - 2026-04-22

## 2026-04-22 (월, 😊)

오늘 회의가 잘 풀렸고...

감정: happy, grateful (valence: 0.8)

---

## 2026-04-21 (일, 😴)

피곤한 하루였다...
```

## 10. 오픈 이슈

1. **가족 공유 디바이스 사용자 격리**: 본 SPEC은 userID 로 격리. 음성/얼굴 인증은 별도 SPEC.
2. **장기 treatment of sensitive entries**: 사별일 관련 엔트리를 사용자가 "매년 회상 싫음" 으로 opt-out 할 수 있는 UI.
3. **Encryption at rest**: MEMORY-001은 파일 권한만. v0.2에서 sqlcipher 전환 고려.
4. **Private mode UI hint**: 매 엔트리 작성 시 "이 엔트리는 private 으로 저장할까요?" 옵션 제공 여부.
5. **단일 세션에 여러 엔트리**: 하루에 여러 번 쓸 수 있게 할지 (morning + evening). v0.1은 저녁 1회 기본.
6. **Content moderation**: 타인 이름, 주소 등 PII 자동 masking? 범위 넘음, 사용자 책임.

## 11. 참고

- GoEmotions: https://github.com/google-research/google-research/tree/master/goemotions
- KoSAC: http://word.snu.ac.kr/kosac/
- VAD Model Original: Russell, J.A. & Mehrabian, A. (1977)
- Journaling benefits (research): https://positivepsychology.com/benefits-of-journaling/
- 생명의전화: http://www.lifeline.or.kr/
