# Research — SPEC-GOOSE-HEALTH-001

## 1. 법적·윤리적 배경

### 1.1 의료기기법 (한국)

본 앱은 **의료기기 아님** 으로 분류되어야 규제 대상 제외. 조건:
- 진단 기능 없음
- 처방 기능 없음
- 치료 효과 주장 없음
- 복약 **로그·리마인더**만 제공 (client-side 관리)

애매한 영역: "이 약 먹어도 될까요?" 같은 질문에 답변하면 의료기기 판정 리스크.
→ 모든 답변은 **"의사/약사 상담"** 으로 redirect.

### 1.2 FDA Mobile Medical Apps Guidance (참고)

유사 카테고리: "Medication reminder apps with dose tracking" → low-risk, FDA 비규제.
단, "algorithm-based dose adjustment" 는 고위험 → 본 SPEC은 이를 엄격히 배제.

## 2. 식약처 DUR API 상세

### 2.1 DUR 정의

Drug Utilization Review — 약물 사용 평가. 병용금기·특정연령군금기·용량주의 등 데이터.

### 2.2 API 목록

- 병용금기: `/getUsjntTabooInfoList` (성분-성분)
- 특정연령금기: `/getSpcifyAgrdeTabooInfoList`
- 임부금기: `/getPwnmTabooInfoList`
- 용량주의: `/getCpctyAtentInfoList`
- 투여기간주의: `/getMdctnPdAtentInfoList`
- 효능군중복: `/getEfcyDplctInfoList`

본 SPEC v0.1은 **병용금기만** 사용. 나머지는 v0.2+.

### 2.3 Response 예 (병용금기)

```xml
<item>
  <INGR_CODE_A>500000</INGR_CODE_A>
  <INGR_NAME_A>warfarin</INGR_NAME_A>
  <INGR_CODE_B>110701</INGR_CODE_B>
  <INGR_NAME_B>aspirin</INGR_NAME_B>
  <PROHBT_CONTENT>출혈 위험 증가</PROHBT_CONTENT>
</item>
```

Severity 매핑 (DUR는 "금기"만 제공, severe로 매핑).
→ moderate/minor 구분은 DrugBank 또는 static DB 필요.

## 3. Static Fallback DB

주요 100개 상호작용 goldenfile:

```json
[
  {
    "med_a": "warfarin",
    "med_b": "aspirin",
    "severity": "severe",
    "mechanism": "platelet inhibition + anticoagulation → 출혈 위험",
    "recommendation": "병용 금지, 의사 상담",
    "source": "korean_dur_2024"
  },
  {
    "med_a": "mao_inhibitor",
    "med_b": "ssri",
    "severity": "severe",
    "mechanism": "serotonin syndrome",
    "recommendation": "절대 병용 금지",
    "source": "who_drugbank"
  },
  ...
]
```

출처: WHO Essential Medicines Interactions, 식약처 2024 DUR 공지.

## 4. 알레르기 교차반응

주요 그룹:

| 그룹 | 대표 약물 | 교차반응 가능 |
|------|---------|-----------|
| Penicillin계 | amoxicillin, ampicillin | Cephalosporin 1세대 (5-10%) |
| Sulfa계 | sulfamethoxazole | thiazide 이뇨제, sulfonylurea |
| NSAID | aspirin, ibuprofen | Cox-2 inhibitor (아스피린 과민) |
| Aminoglycoside | gentamicin | streptomycin (호환 low) |

Goldenfile: `testdata/allergy_cross_reactivity.json`

## 5. 복약 순응률 계산

### 5.1 기본 공식

```
adherence = taken_count / scheduled_count
```

### 5.2 시간대별 가중치 (v0.2+)

- "식후 30분" 등 precise timing 과 실제 intake time 비교
- ±1시간 이내 → full credit
- ±2시간 이내 → 0.7 credit
- 그 이상 → 0.3 credit

v0.1은 단순 taken/scheduled.

### 5.3 Adherence 레벨

- ≥ 0.95: "우수"
- 0.85-0.95: "양호"
- 0.70-0.85: "보통"
- < 0.70: "주의 필요 — 의사 상담 권장"

## 6. 리마인더 UX 패턴

### 6.1 Escalation

```
T+0 (식후 15분): "💊 [약이름] 드실 시간이에요"
    [드셨어요] [아직이에요] [나중에]
T+60: "💊 아직 안 드셨어요? 중요한 약이라 까먹으면 안 돼요"
T+120: "💊 오늘 [약] 아직 못 드셨어요. 지금이라도 드시거나 주치의에게 문의하세요"
T+240: 포기 (skip 기록)
```

### 6.2 사용자 응답 매칭

자연어 파싱 (간단):
- "먹었어" / "yes" / "ok" / "✓" → taken=true
- "아직" / "나중" / "later" → snooze
- "안 먹을래" / "skip" / "패스" → skipped=true

## 7. 식사 로그 프라이버시

### 7.1 민감도

"무엇을 먹었는지"는 민감 정보 (섭식장애, 당뇨 관리 등 함의).

### 7.2 저장 정책

- MEMORY-001 session_id="health"
- 파일 권한 0600
- Export 시 per-op 사용자 확인
- A2A 전송 금지
- LLM 프롬프트에 포함 시 "dietary pattern" 요약만 (구체 메뉴 X)

## 8. Hydration Tracker

### 8.1 기본 목표

성인 남성: 2500ml, 여성: 2000ml (WHO)
사용자 개별 조정 가능.

### 8.2 시간별 expected

```
expected_at_hour(h) = goal_ml × max(0, (h - 6) / 16)
  // 06시 = 0, 22시 = goal, 그 외는 비례
```

Reminder: `actual < expected - 200ml` 시 "물 한 잔 어떠세요?"

## 9. 테스트 전략

### 9.1 DUR Mock

`httptest.NewServer` 로 식약처 XML response 모의. 주요 상호작용 5종 fixture.

### 9.2 Interaction 코퍼스

100+ 상호작용 goldenfile 전수 검증.

### 9.3 Canned Response 검증

"머리 아파", "감기 걸린 것 같아" 등 20+ 증상 표현 → canned response만 반환하는지 verify.

## 10. 한국 고령 사용자 특화

1. **단일 약 vs 복합 처방**: 고령자 평균 5-7종 복용 → severity 체크 성능 중요.
2. **UI 접근성**: 큰 글씨, 음성 출력 (TTS via BRIEFING-001 재사용).
3. **가족 모니터링** (opt-in): 가족에게 순응률 리포트 공유 가능? → 프라이버시 고민, v0.2+.
4. **의료보험 EDI 연동**: 처방전 자동 import? → 기술·법적 복잡, 범위 외.

## 11. 오픈 이슈

1. **Severity 세분화**: DUR는 이진 (금기/주의). DrugBank은 minor/moderate/severe. 두 소스 혼용 전략.
2. **Ingredient 정규화**: 같은 성분 다른 브랜드명 (예: "타이레놀" = acetaminophen) 매핑 테이블 유지 부담.
3. **식약처 API 인증키 관리**: 프로젝트 공유 키 vs 사용자 개별 발급. 개별 발급이 안전하나 UX 불편.
4. **복약 history 장기 보관**: 년 단위 누적 → 의료 기록 준함. 사용자가 export 가능하지만 저장 기간 정책 미정.
5. **여행 시 TZ 처리**: SCHEDULER-001과 동일, 24시간 pause 전략 상속.

## 12. 참고

- 한국 DUR 지침: https://www.hira.or.kr/
- Medication Reminder Apps evidence: Cochrane review
- FDA Mobile Medical Apps: https://www.fda.gov/medical-devices/digital-health-center-excellence/mobile-medical-applications
- HL7 FHIR MedicationRequest (미래 연동): https://www.hl7.org/fhir/medicationrequest.html
