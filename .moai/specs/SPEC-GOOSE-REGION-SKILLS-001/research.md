# SPEC-GOOSE-REGION-SKILLS-001 — Research Notes

> Companion to `./spec.md`. Catalogs cultural sources, per-country skill scopes, SKILLS-001 extension rationale, and edge cases.

---

## 1. SKILLS-001 확장 전략

### 1.1 Frontmatter 필드 추가 결정

옵션 비교:

| 옵션 | 장점 | 단점 | 결정 |
|------|-----|-----|------|
| (A) 신규 `locales:` 필드 | 명시적, 검증 쉬움 | SKILLS-001 스키마 bump | **채택** |
| (B) `paths:`에 country 인코딩 | 스키마 불변 | 의미 모호(paths = file path) | 제외 |
| (C) 별도 `region/*` 패키지 | 분리 명확 | 두 레지스트리 관리 복잡 | 제외 |

옵션 (A) 채택 근거: SKILLS-001 `SAFE_SKILL_PROPERTIES` allowlist가 이미 확장 가능한 구조. `locales` 추가는 semver minor bump.

### 1.2 TriggerMode 확장

SKILLS-001의 4 trigger(inline/fork/conditional/remote) 중:

- **Inline**: system prompt에 주입. region skill 대부분이 이 모드.
- **Fork**: sub-agent spawn. 예: `jondaetmal-etiquette`가 대화 스타일 조정 agent fork.
- **Conditional (paths)**: 파일 경로 기반. region skill과 무관.
- **Remote**: 외부 URL. region skill은 기본 로컬.

**결정**: region skill은 기본 Inline. 필요 시 Fork도 허용. 본 SPEC은 `locales:` 필드를 **추가 트리거**로 취급 (existing trigger와 AND 조건).

---

## 2. 10+ 국가별 문화 원천 자료

### 2.1 한국 (KR)

**공휴일 (2026년 기준)**:
- 신정(1/1), 삼일절(3/1), 어린이날(5/5), 부처님오신날(음 4/8), 현충일(6/6), 광복절(8/15), 개천절(10/3), 한글날(10/9), 크리스마스(12/25)
- 음력: 설날(3일), 추석(3일)
- 대체공휴일제: 2021년부터 주말 겹치면 다음 평일 대체

**기념일**:
- 어버이날(5/8), 스승의 날(5/15), 수능일(11월 셋째 목요일)
- 발렌타인(2/14), 화이트데이(3/14), 빼빼로데이(11/11)

**메신저**: 카카오톡(점유율 96%), 라인(부차), 텔레그램(개발자 커뮤니티)

**지역 서비스**: 배달의민족, 쿠팡, 네이버 지도, T머니

**존댓말 체계**:
- 하십시오체 (가장 격식): "안녕하십니까"
- 하오체 (격식): "안녕하오"
- 해요체 (일상 존댓말): "안녕해요"
- 해체 (반말): "안녕"

**법률**: PIPA (개인정보보호법), 주민등록번호 수집 금지

### 2.2 일본 (JP)

**祝日 (2026년 기준)**:
- 元日(1/1), 成人の日(1월 둘째 월), 建国記念の日(2/11), 天皇誕生日(2/23), 春分の日(3월 변동), 昭和の日(4/29), 憲法記念日(5/3), みどりの日(5/4), こどもの日(5/5), 海の日(7월 셋째 월), 山の日(8/11), 敬老の日(9월 셋째 월), 秋分の日(9월 변동), 体育の日(10월 둘째 월), 文化の日(11/3), 勤労感謝の日(11/23)

**기념일**: お盆(8월 중순), 七五三(11/15), 卒業式(3월)

**메신저**: LINE(점유율 80%+), Twitter, Discord

**敬語 체계**:
- 丁寧語 (ていねいご, 정중어): "です/ます"
- 尊敬語 (そんけいご, 존경어): "いらっしゃる"
- 謙譲語 (けんじょうご, 겸양어): "伺う"

**문화적 관용구**: お疲れ様です, よろしくお願いします, すみません

### 2.3 중국 (CN)

**法定节假日 (2026년 기준)**:
- 元旦(1/1), 春节(음력 정월, 7일), 清明节(4월 초), 劳动节(5/1~3), 端午节(음력 5/5), 中秋节(음력 8/15), 国庆节(10/1~7)
- 调休 제도: 공휴일 전후 주말 근무로 연휴 만들기 (문화적 특징)

**메신저**: WeChat(99%+), QQ(감소세), 钉钉(업무)

**결제**: 支付宝(알리페이), 微信支付(위챗페이). 신용카드 점유율 낮음, 모바일 QR 지배.

**법률**: PIPL(个人信息保护法 2021-), 网络安全法, 数据安全法. 데이터 현지화 엄격.

**문화**: 체면(面子, miànzi), 관시(关系, guānxi), 겸손(谦虚, qiānxū).

### 2.4 미국 (US)

**연방 공휴일**:
- New Year's Day(1/1), MLK Day(1월 셋째 월), Presidents' Day(2월 셋째 월), Memorial Day(5월 마지막 월), Juneteenth(6/19), Independence Day(7/4), Labor Day(9월 첫째 월), Columbus Day(10월 둘째 월), Veterans Day(11/11), Thanksgiving(11월 넷째 목), Christmas(12/25)

**팁 문화**: 레스토랑 15~22%, 테이크아웃 0~10%, 우버/리프트 10~15%

**결제**: Venmo(개인간), Zelle(은행 간), Apple Pay, Cash App. 체크 지불 잔존.

**세금 시즌**: 4월 15일 연방 소득세 마감. 1월 말부터 W-2 도착.

### 2.5 EU (region:eu)

**공통 규제**:
- **GDPR** (2018-): 개인정보 처리 동의, 삭제권, 이동권. 벌금 최대 연매출 4%.
- **SEPA**: 유로존 계좌이체 표준
- **메트릭 단위**: km, °C, kg 일관

**국가별 추가**:
- DE: Feiertage(국가별 다름), Sonntagsruhe(일요일 상점 휴무), Siezen/Duzen
- FR: Fêtes, 공휴일 대체 없음 주말 겹쳐도
- IT: Ferragosto(8/15), 지역별 성인일

### 2.6 베트남 (VN)

**공휴일**:
- 元旦(1/1), Tết(음력 정월, 5~7일), Hùng Kings(음 3/10), Reunification Day(4/30), Labour Day(5/1), National Day(9/2)

**메신저**: Zalo(점유율 90%+), Facebook Messenger, Viber

**결제**: Momo, ZaloPay, VNPay

**호칭**: anh(형/오빠), chị(누나), em(동생). 나이 서열 엄격.

### 2.7 인도 (IN)

**축제**:
- Republic Day(1/26), Holi(3월), Ram Navami(3~4월), Eid al-Fitr(변동), Independence Day(8/15), Gandhi Jayanti(10/2), Diwali(10~11월, 5일간)
- 지역별: Durga Puja(벵갈), Pongal(타밀), Onam(케랄라)

**결제**: UPI(전국 표준), Paytm, PhonePe

**언어**: 공식 22개, 영어 + 힌디 + 지역어 code-switching 일상

**카스트 중립**: 카스트 언급 회피, 이름만으로 카스트 추측 금지

### 2.8 아랍권 (region:ar-world)

**종교 기반 공휴일**:
- Ramadan 1개월 (금식, 업무시간 단축)
- Eid al-Fitr (라마단 종료 후 3일)
- Eid al-Adha (4일)
- Islamic New Year, Prophet's Birthday

**히즈라 달력**: 음력 기반 354일/년. 그레고리력과 매년 11일 차이.

**주말**: 금/토 (이슬람 전통). UAE는 2022년 토/일로 변경.

**Halal 식단**: 돼지고기 · 알코올 금지, 도축 방식 규정

**RTL**: 아랍어는 우→좌 읽기

### 2.9 브라질 (BR)

**공휴일**:
- Confraternização Universal(1/1), Carnaval(2~3월, 4일), Sexta-feira Santa, Tiradentes(4/21), Dia do Trabalho(5/1), Corpus Christi(변동), Independência(9/7), Nossa Senhora Aparecida(10/12), Finados(11/2), Proclamação da República(11/15), Natal(12/25)

**Carnaval**: 국가적 축제, 리우/사우바도르 대표

**결제**: Pix(2020-, 실시간 이체 국가 표준), 현금 사용률 급감

**포르투갈어 철자**: BR과 PT 차이 존재 (cotidiano vs quotidiano)

### 2.10 독일 (DE)

**Feiertage (연방 + 주별)**:
- Neujahr(1/1), Karfreitag(변동), Ostermontag(변동), Tag der Arbeit(5/1), Christi Himmelfahrt(변동), Pfingstmontag(변동), Tag der Deutschen Einheit(10/3), Weihnachten(12/25-26)
- 주별: Heilige Drei Könige(BY/BW), Allerheiligen(가톨릭주)

**Sie vs Du**:
- Sie: 공식, 처음 만난 사람, 직장 상사
- Du: 친구, 가족, 10대 청소년간

**Datenschutz**: GDPR 최엄격 적용국, Bundesdatenschutzgesetz 연방법

---

## 3. region: 그룹 별칭 설계 근거

### 3.1 왜 그룹 별칭이 필요한가

- EU 27개국 각각에 `gdpr-strict` skill을 복사하면 유지보수 폭탄
- `region:eu` 하나로 선언하면 추가 국가(크로아티아 2013 가입, 불가리아 2007) 자동 포함
- 그룹 멤버십은 정치적 변경 가능성 有 (Brexit, EU 확장)

### 3.2 Brexit 케이스 (GB)

- 2020-01-31 EU 탈퇴, 2024-현재 비회원
- 본 SPEC의 `region:eu`는 **현재 EU 27개국만** 포함
- GB는 `region:uk-commonwealth`(신규) 또는 독립 `gb/*` skill로 처리

### 3.3 기타 그룹 후보 (v0.5+)

- `region:nordic`: DK, SE, NO, FI, IS
- `region:balkan`: RS, HR, BA, ME, MK, AL, XK
- `region:gulf`: SA, AE, QA, KW, BH, OM
- `region:commonwealth`: GB, CA, AU, NZ, IE, IN(부분)
- `region:francophone`: FR, BE, CH, CA-QC, 아프리카 다수

초기 v0.1은 `region:eu`, `region:ar-world`, `region:latam`, `region:sea`, `region:east-asia` 5개만. 나머지는 요구 발생 시 추가.

---

## 4. Skill 콘텐츠 작성 가이드라인

### 4.1 언어

- country skill은 해당 국가 언어로 작성 (kr skill은 한국어)
- group skill은 영어로 작성 (`region:eu`의 gdpr-strict는 영어)
- 혼합 국가(스위스 de/fr/it)는 영어 + 예시는 해당 언어

### 4.2 구조

```markdown
---
frontmatter (10 lines)
---

# 제목

(한 문단 개요)

## 핵심 데이터
- 사실 나열 (공휴일, 날짜, 통화, etc)

## 행동 규칙
- 언제 어떻게 반응해야 하는가
- 피해야 할 것

## 예시 대화 (선택)
- Good: "..."
- Bad: "..."
```

### 4.3 금지 사항

- 정치적 판단 (특정 정당 지지, 분쟁 지역 영유권)
- 종교 편향 (종교간 우월성)
- 성별/인종 stereotype
- 의료/법률 단정 (disclaimer 필수)

### 4.4 권장 사항

- 로컬 검토자 확보
- 정치 민감 주제는 "중립 안내" 패턴 ("이 주제는 사용자별로 의견이 다양합니다")
- 숫자는 ISO 표준 (날짜는 YYYY-MM-DD, 시간은 24h + IANA TZ)

---

## 5. 공휴일 DB 통합 (`rickar/cal/v2`)

### 5.1 지원 국가 (라이브러리 내장)

`rickar/cal/v2`는 30+ 국가 공휴일 패키지 제공:

- US, CA, MX, BR, GB, DE, FR, IT, ES, NL, BE, AT, CH, IE, PT, SE, NO, DK, FI, PL, CZ, HU, SK, GR, TR, RU, UA, JP, KR(부분), AU, NZ, IN(부분), ZA

### 5.2 누락 국가

- CN (중국): 음력 기반, 조휴 제도 복잡 → 커스텀 resolver 필요
- VN (베트남): Tết 음력 → 자체 구현
- SA/AE (아랍권): Hijri 달력 → `go-calendar-hijri` 외부 의존

### 5.3 본 SPEC의 접근

- rickar/cal/v2 지원 국가: skill이 `calendar.query(country)` tool 사용
- 미지원 국가: 해당 country skill 내부에 공휴일 테이블 인라인 포함
- 결과는 SCHEDULER-001이 cron 조건으로 소비

---

## 6. CODEOWNERS 제안

`.github/CODEOWNERS` 초안:

```
# Region skills — language/culture native reviewers
.claude/skills/region/kr/         @goose-team/kr-reviewers
.claude/skills/region/jp/         @goose-team/jp-reviewers
.claude/skills/region/cn/         @goose-team/cn-reviewers
.claude/skills/region/us/         @goose-team/us-reviewers
.claude/skills/region/vn/         @goose-team/vn-reviewers
.claude/skills/region/in/         @goose-team/in-reviewers
.claude/skills/region/br/         @goose-team/br-reviewers
.claude/skills/region/de/         @goose-team/de-reviewers
.claude/skills/region/_groups/eu/      @goose-team/eu-reviewers
.claude/skills/region/_groups/ar-world/ @goose-team/ar-reviewers

# Core region engine — SPEC owner
internal/skill/region/            @goose-team/skills-maintainers
```

각 언어 리뷰 그룹은 최소 2인, PR 1 LGTM으로 merge.

---

## 7. Country 원격 전송 방지 상세

### 7.1 위협 모델

사용자 country는 개인정보가 아니지만 **행동 패턴 식별자**로 활용 가능:

- 광고 타게팅
- 언어/문화 프로파일링
- 정치적 환경 추론

### 7.2 방어

REQ-RS-013: remote skill(`_canonical_*`)에는 country 기본 미전송. 사용자가 `skills.region.share_country_remote = true`로 명시 opt-in해야 전송.

### 7.3 테스트

AC-RS-010: mock HTTP 서버로 remote skill 호출, 요청 헤더와 바디에 `country`/`kr` 등 키워드 부재 검증.

---

## 8. 테스트 매트릭스

| Test ID | Skill 수 | Country 수 | 목적 |
|---------|---------|-----------|------|
| AC-RS-001 | 2 | 1 (KR) | 기본 매칭 |
| AC-RS-002 | 1 | 1 (DE via region:eu) | 그룹 별칭 |
| AC-RS-003 | 2 | 2 (KR→JP) | 전환 |
| AC-RS-004 | 2 | 1 | User disabled |
| AC-RS-005 | 2 | 1 | User authored override |
| AC-RS-006 | 1 | any | Country-agnostic |
| AC-RS-007 | 2 | 0 (empty) | Wildcard fallback |
| AC-RS-008 | 1 | invalid | Reject |
| AC-RS-011 | — | 27 | EU 그룹 확장 |
| AC-RS-012 | 30+ | 8+2groups | 번들 스켈레톤 |

---

## 9. 재사용 vs 재작성 LoC

| 모듈 | 재사용 | 재작성 | 총 Go LoC |
|------|-------|-------|----------|
| SKILLS-001 스키마 확장 | — | `locales` 필드 추가 + allowlist | ~30 |
| Matcher | — | 매칭 로직 | ~120 |
| Group alias 테이블 | — | 정적 map (5 그룹) | ~80 |
| Activator | SKILLS-001 registry | 필터 래퍼 | ~100 |
| Skill 스켈레톤 파일 × 30 | — | SKILL.md 마크다운 | N/A |
| Tests | — | 매칭 + 활성화 | ~500 |
| **합계** | — | — | **~830 Go LoC + 30 skill files** |

SPEC 크기 `M` (1500~4000 LoC) 내 하한. skill 콘텐츠 추가 기여로 총량 확장 예상.

---

## 10. 오픈 이슈

1. **Country vs Language 충돌**: 스위스(DE-CH, FR-CH, IT-CH) 사용자는 4개 언어 혼용. country 단독으로 skill 매칭 불충분. 향후 `primary_language` 조건 추가 고려.
2. **region:eu Brexit 대응**: UK skill 경로 미확정. v0.1은 `gb/*` 개별 디렉토리, v0.5 `region:uk-commonwealth` 그룹 신설 검토.
3. **분쟁 지역**: 대만(TW) / 홍콩(HK) / 티베트 / 팔레스타인 코드 처리 정치 민감. 초기 번들 포함 여부 보류, v0.5+ 별도 논의.
4. **음력 공휴일 정확성**: 중국/베트남의 음력 → 그레고리력 변환 라이브러리 신뢰도 검증. `rickar/cal/v2`는 CN 부분 지원, 대안으로 `go-lunar` 평가.
5. **카스트/계급 민감 skill**: 인도 `caste-neutral-etiquette`는 존재 자체가 정치적. 콘텐츠는 "caste 언급 회피" 가이드로 제한, 세부 역사는 외부 링크.
6. **다문화 이슬람 국가**: 말레이시아(MY)는 이슬람 + 중국계 + 인도계 혼재. 단일 `ar-world` 그룹 부적절. MY는 독립 skill로 다룸.

---

## 11. 참고 문헌

- ISO 3166-1 alpha-2
- UN M.49 regional composition
- EU official member states
- CIA World Factbook (public domain)
- CLDR territory metadata
- `rickar/cal/v2` README
- `biter777/countries` README
- branding.md §3, adaptation.md §4 (프로젝트 내부)

---

**End of research.md for SPEC-GOOSE-REGION-SKILLS-001**
