# GOOSE-AGENT - User Adaptation Document v4.0

> **문서 목적:** GOOSE가 사용자를 어떻게 이해하고 적응하는지 (learning-engine.md가 내부 엔진이라면, 이 문서는 사용자 관점).

---

## 1. 개요: 100% 개인화란 무엇인가

GOOSE는 사용할수록 사용자를 이해하는 AI.
하지만 "이해"란 구체적으로 무엇인가?

**8가지 차원의 사용자 적응**:

1. **Persona Detection** - 사용자가 어떤 사람인지
2. **Style Adaptation** - 어떻게 말하는지
3. **Cultural Context** - 어떤 문화권인지
4. **Domain Expertise** - 무엇에 전문성이 있는지
5. **Time-based Adaptation** - 시간별 모드
6. **Mood Detection** - 감정 상태
7. **Energy Awareness** - 바쁨/여유
8. **Group Dynamics** - 개인 vs 그룹 vs 가족

각 차원은 GOOSE의 응답과 행동에 직접 영향을 준다.

---

## 2. Persona Detection (사용자가 어떤 사람인지)

### 2.1 명시적 Persona (사용자 설정)

온보딩 중 선택적 입력:
- 성별 (optional, 완전 생략 가능)
- 나이대 (optional, 범위로)
- 직업
- 관심사 (태그)
- 목표 (단기/장기)
- 가족 구성 (optional)

### 2.2 암묵적 Persona (GOOSE 추론)

사용자가 명시하지 않아도 감지:

**언어 패턴 분석**:
- 어휘 수준 (기술 용어 사용 빈도)
- 문장 복잡도
- 특정 분야 용어 (의료, 법률, 프로그래밍 등)

**시간대 패턴**:
- 주로 새벽 활동 = 개발자 or 야행성
- 평일 오전 = 직장인
- 낮 시간 자유 = 프리랜서/주부/학생

**질문 스타일**:
- 깊이 있는 분석 질문 = 학자/연구자
- 실용적 문제 해결 = 직장인
- 학습 질문 = 학생

### 2.3 Big-5 Personality Inference (연구 기반)

**Big-5 Traits** (OCEAN):
- **Openness** (개방성): 새로운 것 탐색 경향
- **Conscientiousness** (성실성): 계획성, 꼼꼼함
- **Extraversion** (외향성): 사교성
- **Agreeableness** (친화성): 협력성
- **Neuroticism** (신경성): 감정 안정성

**추론 방법**:
- 대화 임베딩 분석
- LinearSVC classifier (문헌 연구 기반)
- 주기적 업데이트 (매주)
- 신뢰도 0.6+ 이상만 적용

### 2.4 Persona 적용

**응답 톤 조정**:
- High Openness → 창의적 제안 많이
- High Conscientiousness → 체계적, 단계적
- High Extraversion → 따뜻, 대화 연장
- High Agreeableness → 공감 표현
- High Neuroticism → 안심 메시지

**추천 알고리즘**:
- 스킬 추천 시 성격 일치도 반영
- 사용 가능성 높은 기능 상위

---

## 3. Style Adaptation (말하는 방식)

### 3.1 Formality Detection

**감지 시그널**:
- 존댓말 vs 반말 (한국어)
- 敬語 레벨 (일본어)
- Formal vs Casual (영어)

**Automatic Mirroring**:
- 사용자가 존댓말 → GOOSE 존댓말
- 사용자가 반말 → GOOSE 해요체 (한 단계 공손)
- 사용자 명시적 변경 → 즉시 반영

### 3.2 Verbosity Preference

**분석 메트릭**:
- 사용자 응답 길이 평균
- "짧게", "더 자세히" 같은 명시적 피드백
- 대화 이탈 시점 (너무 길면 이탈)

**자동 조정**:
- Short responses 선호 → 3-5문장 기본
- Detailed 선호 → 전체 설명 + 예시

### 3.3 Humor & Emoji Tolerance

**감지**:
- 사용자 이모지 사용 빈도
- 유머 반응 (웃음, 긍정)
- 톤 (캐주얼 vs 프로페셔널)

**반영**:
- Emoji frequency matching
- 유머 레벨 (0-10)
- 비공식 표현 허용도

### 3.4 Technical Level

**Auto-detect**:
- 기술 용어 이해도
- 코드 예시 요청 빈도
- Jargon tolerance

**조정**:
- 개발자: 코드 + 기술 용어 OK
- 비개발자: 쉬운 언어, 비유 많이
- 하이브리드: 상황별 전환

---

## 4. Cultural Context

### 4.1 다국어 자동 감지

GOOSE는 4개 주요 언어 지원:
- English
- 한국어
- 日本語
- 中文

**자동 전환**:
- 초기 입력 언어 감지
- 시스템 로케일
- 사용자 명시 설정 override

**Mixed Language 지원**:
예: "이 `async function`을 Promise로 바꿔줘"
→ 한국어 기본 + 기술 용어 영어 유지

### 4.2 문화별 뉘앙스

**한국**:
- 나이 서열 존중 (사용자 나이 > GOOSE 나이 가정)
- 존댓말 기본
- 가족 중시 (부모님 언급 시 정중)
- 체면 고려

**일본**:
- 敬語 레벨 (丁寧語 → 尊敬語 → 謙譲語)
- 間 (ma, 일시 정지) 존중
- お疲れ様 같은 관용구
- 직접적 거절 회피

**중국**:
- 관시 (guanxi) 이해
- 서열 (上下級) 고려
- 체면 중시
- 간결, 직접적

**서구**:
- First-name basis
- 평등주의
- 직접성 (beat around the bush 지양)
- 개인주의

### 4.3 명절 & 기념일

**사용자 지역 자동 감지**:
- 한국: 설날, 추석, 어린이날, 어버이날
- 일본: 正月, お盆, 卒業式
- 중국: 春节, 中秋节, 清明节
- 미국: Thanksgiving, Christmas, 4th of July
- 이슬람: Ramadan, Eid

**적용**:
- 자동 인사말
- 기념일 알림
- 선물 추천
- 문화별 예절

---

## 5. Domain Expertise

### 5.1 자동 도메인 감지

대화 내용으로 추론:

**Developer** (개발자):
- 코드 블록 빈번
- 기술 용어 (API, framework, bug)
- GitHub/StackOverflow 언급

**Designer** (디자이너):
- 시각적 표현 (색상, 레이아웃)
- Figma, Sketch 언급
- UX/UI 용어

**Business** (비즈니스):
- 전략, 마케팅, ROI
- 재무 용어
- 미팅, 제안서

**Student** (학생):
- 학습, 시험, 숙제
- 교과서 용어
- 연구 논문

**Creative** (크리에이티브):
- 글쓰기, 음악, 영상
- 아이디어, 영감
- 창작 도구

### 5.2 도메인별 기본 에이전트

사용자 도메인 감지 시 관련 에이전트 자동 활성화:
- Developer → CodeReviewer, TestWriter, Debugger
- Designer → ColorAdvisor, LayoutChecker
- Business → PitchPrepper, DataAnalyst
- Student → Tutor, NoteMaker
- Creative → IdeaGenerator, Editor

### 5.3 서브 도메인 세분화

**Developer 서브**:
- Rust vs Python vs TS
- Frontend vs Backend
- Web vs Mobile vs Embedded

**Designer 서브**:
- UX vs Graphic vs Brand
- Print vs Digital

### 5.4 Cross-Domain 사용자

다중 도메인 감지:
- 풀스택 개발자 (Frontend + Backend)
- 디자이너-엔지니어 (Design + Code)
- 연구자 (Academic + Business)

각 모드 간 매끄러운 전환.

---

## 6. Time-based Adaptation

### 6.1 시간대별 모드

**아침 (6-10시)**:
- 일정 브리핑 모드
- 간단, 명확한 응답
- 오늘 할 일 요약

**오전 (10-12시)**:
- 집중 업무 모드
- 긴 설명 OK
- 복잡한 분석

**점심 (12-14시)**:
- 가벼운 모드
- 짧고 친근
- 음식, 휴식 관련

**오후 (14-18시)**:
- 생산 모드
- 협업 관련 증가
- 미팅 대응

**저녁 (18-22시)**:
- 휴식 + 계획 모드
- 오늘 회고
- 내일 준비

**심야 (22-02시)**:
- 조용한 모드
- 조명 어둡게 (UI)
- 부드러운 음성

**새벽 (02-06시)**:
- 긴급 모드만
- 알림 최소화
- 필수 응답만

### 6.2 요일 모드

- **월요일**: 주 시작 계획 강조
- **화-목**: 일반 업무
- **금요일**: 주 마무리, 정리
- **토요일**: 여가, 가족
- **일요일**: 휴식, 내주 준비

### 6.3 시즌 모드

- **연말연초**: 회고, 결심
- **봄**: 새 시작
- **여름**: 휴가 계획
- **가을**: 수확, 준비
- **겨울**: 정리, 쉼

### 6.4 특수 기간

- 시험 기간 (학생 감지)
- 프로젝트 마감 (업무 강도 감지)
- 휴가 시즌 (여유 증가)
- 명절 (가족 관련)

---

## 7. Mood Detection

### 7.1 감정 차원 (VAD Model)

**Valence** (긍정/부정):
- 긍정적 어휘 비율
- 감사 표현
- 불만 표현

**Arousal** (에너지):
- 느낌표 사용
- 타이핑 속도
- 대문자 사용

**Dominance** (통제감):
- 직접적 명령 vs 도움 요청
- 확신 vs 불확실

### 7.2 감지 방법

**Text Analysis**:
- Sentiment 모델 (BERT 기반)
- 이모지 분석
- 문장 구조

**Behavioral Signals**:
- 타이핑 속도/패턴
- 대화 길이 변화
- 질문 빈도
- 재요청 비율

**Explicit**:
- 사용자가 "오늘 피곤해" 같이 말함
- GOOSE가 주기적으로 "오늘 어떠세요?" 물음 (선택적)

### 7.3 감정별 대응

**스트레스 감지**:
- 단순화, 짧은 응답
- 위로 메시지
- 작업 덜어주기 제안

**행복 감지**:
- 함께 축하
- 공유 제안
- 긍정 에너지

**피곤 감지**:
- 짧게, 효율
- 휴식 제안
- 내일로 미룰지 묻기

**흥분 감지**:
- 에너지 따라가기
- 상세 논의 OK
- 열정 지지

**슬픔 감지**:
- 공감
- 강요 X
- 부드러운 대응

---

## 8. Energy Awareness

### 8.1 바쁨 감지

**시그널**:
- 빠른 질문 연속 (시간 간격 짧음)
- 짧은 답변 선호
- 요청 우선순위 명시
- "빨리", "간단히" 같은 키워드

**대응**:
- 즉답, 핵심만
- 단계별 질문 생략
- 확인 최소화

### 8.2 여유 감지

**시그널**:
- 긴 대화
- 깊이 있는 질문
- 학습 모드 ("설명해줘")
- "천천히" 같은 표현

**대응**:
- 상세 설명
- 배경 정보
- 대안 제시
- 학습 리소스

### 8.3 전환 감지

사용자 에너지가 바뀌면 GOOSE도 전환:
- 바쁨 → 여유: 이전 대화 요약, 심화 제안
- 여유 → 바쁨: 즉시 핵심으로

---

## 9. Group Dynamics

### 9.1 Individual 모드 (혼자)

- 개인화 최대
- 민감 정보 접근 허용
- 사적 대화 OK
- 페르소나 자유 선택

### 9.2 Family 모드 (공유 디바이스)

**가족 구성원 감지**:
- 음성 ID (voice biometric)
- 접근 권한 분리
- 각자 독립 GOOSE 인스턴스

**예시 시나리오**:
- 거실 기가지니 스피커 (한 가정)
- 아빠, 엄마, 아이 각각 다른 GOOSE
- 공유 일정은 가족 공유
- 사적 대화는 개인별 격리

**자녀 보호**:
- 연령 필터링
- 부모 승인 필요 (중요 결정)
- 학습 시간 제한 (교육 관련)

### 9.3 Team 모드 (업무)

**팀 컨텍스트**:
- 프로젝트 컨텍스트 공유
- 팀원 간 정보 격리
- 역할별 권한

**A2A 협업**:
- 팀원 GOOSE 간 통신
- 태스크 분배
- 진행 상황 공유

---

## 10. 적응 데이터 저장

### 10.1 Identity Graph에 저장

```yaml
user:
  persona:
    age_bracket: 30-40
    occupation: developer
    interests: [rust, AI, design]
    big5:
      openness: 0.82
      conscientiousness: 0.75
      extraversion: 0.45
      agreeableness: 0.68
      neuroticism: 0.32
  style:
    formality: semi-formal
    verbosity: medium
    emoji_tolerance: high
    humor_level: 7/10
  cultural:
    primary: ko-KR
    secondary: en
    holidays: [korean, western]
  time_patterns:
    work_hours: 9-18 Mon-Fri
    peak_focus: 14-17
    timezone: Asia/Seoul
  mood_baseline:
    valence_avg: 0.65
    arousal_avg: 0.50
  domain:
    primary: developer
    secondary: designer
    stack: [rust, typescript, go]
```

### 10.2 LoRA Adapter 반영

매주 다음 데이터로 LoRA 재훈련:
- 스타일 선호
- 응답 길이
- 톤 매칭
- 도메인 용어

결과: 사용자 스타일 100% 복제한 미니 모델

---

## 11. 사용자 제어 (가장 중요!)

GOOSE가 사용자를 학습하지만, **사용자가 항상 통제권을 가진다**.

### 11.1 Transparency (투명성)

**학습 내용 조회**:
- "What have you learned about me?" 명령
- Identity Graph 시각화
- 패턴 발견 리스트
- LoRA 버전 히스토리

### 11.2 Edit (수정)

- 잘못 학습한 내용 수정
- "No, I don't like X" 같은 명령
- 특정 카테고리 삭제
- 특정 기간 데이터 롤백

### 11.3 Export (내보내기)

- 모든 개인 데이터 JSON export
- GDPR 준수
- 다른 GOOSE 인스턴스로 이전 가능
- Human-readable 형식

### 11.4 Delete (삭제)

- 전체 데이터 삭제 ("Factory Reset")
- 특정 카테고리 삭제
- 특정 기간 데이터 삭제
- 즉시 반영 (soft delete 없음)

### 11.5 Reset (초기화)

- 학습 초기화 (처음부터)
- Identity Graph 비우기
- LoRA 제거
- 페르소나 재선택

### 11.6 Opt-out (거부)

- 특정 학습 차원 비활성화
- Mood detection 끄기
- Pattern mining 끄기
- LoRA training 끄기

---

## 12. 적응 평가 메트릭

### 12.1 Personalization Success

| 메트릭 | 측정 방법 | 목표 |
|-------|---------|------|
| Response Edit Rate | 사용자가 응답 수정하는 빈도 | < 5% |
| Follow-up Question Rate | 이해 못해서 재질문 | < 10% |
| Conversation Length | 평균 대화 지속 시간 | 증가 추세 |
| Session Frequency | 일일 세션 수 | 증가 추세 |
| Satisfaction Rating | explicit 별점 | 4.5+/5 |
| NPS (Net Promoter Score) | 추천 의향 | 60+ |
| D30 Retention | 30일 유지율 | 50%+ |

### 12.2 Adaptation Speed

**학습 속도 측정**:
- Day 1 → Day 7: 기본 스타일 학습 (80%)
- Day 7 → Day 30: 패턴 인식 (70%)
- Day 30 → Day 90: 개인화 완성 (90%)
- Day 90+: 미세 조정

### 12.3 Preservation

**과거 지식 유지**:
- 이전 학습 덮어쓰지 않음
- Catastrophic forgetting 방지 (EWC, LwF)
- 롤백 가능
- 버전 관리

---

## 13. 로드맵

### Phase 1 (0-6개월): 기본 적응
- Persona Detection (명시적)
- Style Adaptation (formality, verbosity)
- Cultural (다국어)
- Time-based (기본 시간대)

### Phase 2 (6-12개월): 고급 적응
- Big-5 inference
- Mood detection
- Energy awareness
- Domain expertise 세분화

### Phase 3 (12-18개월): 완전 개인화
- Group dynamics (가족 모드)
- Voice ID
- User LoRA training
- Proactive engine

### Phase 4 (18-24개월): 지능 완성
- Federated Learning (opt-in)
- 감정적 지능 고도화
- 다단계 문맥 추론
- Life transition 대응

---

## 14. 연구 참조

### Big-5 Personality in AI
- arXiv: "LLMs as Personality Classifiers"
- Big-5 Inventory (standard test)

### Style Mirroring
- "Conversational Mirroring in Human-AI"
- Adaptive Dialogue Systems

### Cultural Context in AI
- "Cross-Cultural AI Design"
- Hofstede Cultural Dimensions

### Emotion Detection in Text
- GoEmotions dataset (Google)
- EmoBERT, RoBERTa-emotion
- VAD (Valence-Arousal-Dominance) model

### Multi-User / Family Mode
- Voice biometric papers
- Federated user profiles

---

## 15. Ethics & Guardrails

### 15.1 No Dark Patterns

- 사용자가 원하지 않는 학습 안 함
- 인위적 의존 조장 X
- 건강한 사용 권장

### 15.2 No Emotional Manipulation

- 감정 감지는 지원 목적만
- 광고/구매 유도에 이용 X
- 취약 상태 악용 X

### 15.3 Transparency First

- 항상 무엇을 학습하는지 알림
- 명시적 동의 필요한 항목 분명
- 학습 거부권 보장

### 15.4 Cultural Sensitivity

- 고정관념 방지
- 개인차 존중
- 편견 검증

---

## 16. 결론

GOOSE의 8가지 적응 차원은 단순한 기능이 아니다.
**사용자를 진정으로 이해하려는 시도**이다.

하지만 이해는 통제가 아니다.
사용자는 언제든 학습을 수정, 삭제, 거부할 수 있다.

**"Understanding without control is a trap. GOOSE gives you both understanding AND control."**

시간이 갈수록 GOOSE는:
- 더 잘 이해한다
- 더 잘 적응한다
- 더 잘 예측한다
- 더 잘 돕는다

그리고 **언제나 당신의 것**이다.

Version: 4.0.0
Created: 2026-04-21
Companion: learning-engine.md

> **"I learn YOU, not everyone."**
