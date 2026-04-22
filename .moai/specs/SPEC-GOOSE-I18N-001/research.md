# SPEC-GOOSE-I18N-001 — Research Notes

> Companion to `./spec.md`. Documents library comparison, ICU MessageFormat case studies, RTL case studies, Tier 3 LLM pipeline cost analysis, and authoring conventions.

---

## 1. Hermes Agent 다국어 패턴 상세 분석

Hermes의 `agent/gateway/` 7개 플랫폼(Telegram/Discord/Slack/Matrix/KakaoTalk/WeChat/Webhook) 메시지는 **플랫폼별 번역 엔진이 다름**:

| 플랫폼 | 번역 방식 | 규모 |
|-------|---------|-----|
| Telegram | LocalizedString via library | 90+ 언어 |
| Discord | `discord.js` locale | 30+ 언어 |
| Slack | webhook에서 en 고정 | — |
| Matrix | `matrix.to` locale | 15+ 언어 |

GOOSE는 위 플랫폼별 번역 책임을 **GATEWAY-001**에 위임하고, 본 SPEC은 Desktop/Mobile/CLI 본체의 i18n만 담당.

### 1.1 Hermes가 잘한 부분

- 플루럴 처리(`i18next` 기본값 사용)
- 언어별 fallback 체인 (`fr-CA → fr → en`)

### 1.2 Hermes의 한계

- ICU MessageFormat 미사용 → 성별/서수/복합 플루럴 처리 불가
- RTL 지원 없음(Telegram이 자체 제공)
- 번역 품질 검증 자동화 없음

GOOSE는 이 한계를 메운다: ICU 채택 + RTL 공식 지원 + `goose i18n lint` 자동화.

---

## 2. ICU MessageFormat vs gettext vs 단순 치환

| 패러다임 | 플루럴 | 성별 | 네스팅 | 학습 비용 | 채택 |
|---------|-------|-----|-------|---------|------|
| 단순 치환 `{name}` | 수동 분기 | 수동 | 수동 | 낮음 | 제외(한계) |
| gettext `.po` | 3-6 form | 매크로 | 어려움 | 중간 | 제외(레거시) |
| ICU MessageFormat | CLDR 기반 | `select` | 자유 | 중~고 | **채택** |
| Fluent(Mozilla) | 강력 | 강력 | 자유 | 고 | 후보 제외(Go 지원 약함) |

ICU 학습 비용은 있으나 한 번 배우면 모든 언어 커버. `i18next-icu`가 파싱 엔진 제공.

---

## 3. CLDR Plural Rules 상세

각 언어의 플루럴 form 개수:

| 언어 | Form | 예시 |
|------|-----|------|
| ja, ko, zh, th, vi, id | 1 (other) | "5개" |
| en, de, es, it, pt, nl, sv, da | 2 (one, other) | "1 item" / "5 items" |
| fr | 2 (one, other) | 단, 0과 1 모두 one |
| ru, uk, pl, hr | 4 (one, few, many, other) | "21 сообщение" / "23 сообщения" / "25 сообщений" |
| ar | 6 (zero, one, two, few, many, other) | "0 رسائل" / "1 رسالة" / "2 رسالتان" / ... |
| cy (웰시) | 6 | 가장 복잡 |
| hi | 2 (one, other) | 단, 0과 1 모두 one |

CLDR 플루럴 데이터는 `go-i18n/v2`와 `i18next` 내장. 별도 다운로드 불필요.

### 3.1 러시아어 플루럴 결정 규칙 예시

```
one: n mod 10 = 1 AND n mod 100 != 11
few: n mod 10 = 2..4 AND n mod 100 != 12..14
many: n mod 10 = 0 OR n mod 10 = 5..9 OR n mod 100 = 11..14
other: everything else (사실상 소수점, 큰 수)
```

| n | form | 메시지 |
|---|------|-------|
| 1 | one | "1 сообщение" |
| 2 | few | "2 сообщения" |
| 5 | many | "5 сообщений" |
| 11 | many | "11 сообщений" (11은 one 예외) |
| 21 | one | "21 сообщение" |
| 22 | few | "22 сообщения" |

---

## 4. RTL 구현 케이스 스터디

### 4.1 Twitter(X) RTL

Twitter는 아랍어 RTL 지원 10년 이상. 핵심 원칙:

- **CSS logical properties 전면 사용**: `margin-inline-start/end`, `padding-inline-*`
- **Unicode bidi isolate**: 이메일/URL/해시태그를 `<bdi>`로 감싸 LTR 유지
- **아이콘 대칭화**: 방향성 아이콘(좌/우 화살표)만 `scaleX(-1)`
- **숫자 표기**: 아랍어 숫자(٠١٢٣) vs 서양 숫자 선택 설정 가능

### 4.2 WhatsApp Mobile RTL

React Native 0.71+ 기준:

- `I18nManager.forceRTL(true)` 호출 후 **앱 재시작 필수**
- 기존 `marginLeft/paddingRight` 등은 자동 반전됨(RN Yoga 엔진)
- 단, `position: absolute`에서 `left/right`는 자동 반전 안 됨 → `start/end` 사용 필요
- iOS/Android 모두 동일 동작

### 4.3 GOOSE 결정

- Desktop은 Tailwind 4.x `rtl:` variant + logical properties 혼용
- Mobile은 `start/end` 스타일 규칙 + `I18nManager.forceRTL` + 재시작 프롬프트
- 방향성 아이콘만 별도 RTL 자산(Figma에서 수동 export)
- 숫자는 서양 숫자 유지(아랍어 표기 여부는 v1.5 사용자 옵션)

---

## 5. Tier 3 LLM 자동번역 파이프라인 비용 분석

### 5.1 번역 볼륨 추정

| 지표 | 값 |
|------|----|
| Tier 1 en 번들 총 키 수(v0.1) | ~800개 |
| 키당 평균 문자 수 | ~40자 |
| 총 source 문자 수 | ~32,000자 |
| 토큰 환산(영어 기준 1 token = 4자) | ~8,000 tokens source |
| 번역 결과 토큰(1.3배 팽창 가정) | ~10,400 tokens output |

### 5.2 프로바이더별 비용 (2026-04 기준 추정)

| 프로바이더 | Input $/M tokens | Output $/M tokens | 전체 번역 1회 |
|----------|-----------------|-------------------|-------------|
| Claude Sonnet 4.7 | 3 | 15 | ~$0.18 |
| GPT-4o | 2.5 | 10 | ~$0.13 |
| DeepSeek v3 | 0.14 | 0.28 | ~$0.005 |
| Gemini 2.5 Flash | 0.15 | 0.6 | ~$0.009 |

**권장**: DeepSeek 또는 Gemini Flash(저가 + 다국어 우수). Claude는 Tier 1 수동 검수 단계에서만.

### 5.3 파이프라인 단계

```
1. 사용자가 Tier 3 언어 요청
    ↓
2. AutoTranslate 서비스가 en/common.yaml 전체 로드
    ↓
3. 청크 분할(키 50개씩 batch)
    ↓
4. ADAPTER-001 → DeepSeek 호출
    (프롬프트: "Translate to sw-TZ, keep ICU placeholders, preserve English technical terms")
    ↓
5. 응답 YAML 파싱, 플레이스홀더 검증(영어 대비 일치)
    ↓
6. 실패 케이스는 재시도(최대 2회)
    ↓
7. 각 키에 _machine_translated: true 메타 추가
    ↓
8. locales/{lang}/*.yaml 저장
    ↓
9. UI 알림 + "Improve" 버튼
```

예상 시간: 800 키 × 5초/청크 / 50 키 = ~80초.

---

## 6. 20+ 언어 Tier 분류 근거

### 6.1 Tier 1 (4): 수동 검수 필수

| 언어 | L1 인구 | 프로젝트 내부 역량 | 이유 |
|------|--------|----------------|------|
| en-US | 400M+ | 네이티브 | 원본 언어 |
| ko-KR | 77M | GOOSE 팀 네이티브 | 주 사용자 |
| ja-JP | 125M | 검수 가능 | branding.md 명시 |
| zh-CN | 1B+ | 검수 가능 | 시장 규모 |

### 6.2 Tier 2 (12): LLM + 커뮤니티 리뷰

| 언어 | L1 인구 | 선정 근거 |
|------|--------|---------|
| es | 500M | 히스패닉/라틴 시장 |
| fr | 300M | 서유럽 + 아프리카 |
| de | 130M | DACH 개발자 시장 |
| pt-BR | 260M | 브라질 시장 |
| ru | 260M | 동유럽 + 중앙아시아 |
| vi | 90M | 동남아 주요 |
| th | 70M | 동남아 주요 |
| id | 270M | 동남아 최대 |
| ar | 420M | MENA + RTL 대표 |
| hi | 600M+ | 인도 시장 |
| tr | 85M | 터키 + 키프로스 |
| pl | 45M | 중동부 유럽 |

### 6.3 Tier 3 (open-ended)

사용자가 요청하는 모든 BCP 47 코드. 초기 커버되지 않는 예시: sw(스와힐리), uk(우크라이나), cs(체코), nl(네덜란드), el(그리스), fil(타갈로그) 등. Tier 2로 승격 기준: 커뮤니티 리뷰어 2인 이상 확보.

---

## 7. 번역 키 네이밍 규칙

### 7.1 네임스페이스

`{namespace}:{keyPath}` 형식. 네임스페이스는 파일 단위(`common.yaml` → `common`).

### 7.2 키 경로

snake_case + dot-separated 계층:

- `onboarding.step_1.title`
- `settings.theme.dark`
- `errors.auth.token_expired`

### 7.3 플레이스홀더

- 변수: `{name}` (snake_case)
- 플루럴: `{count, plural, ...}`
- 성별: `{gender, select, masculine {...} feminine {...} other {...}}`

### 7.4 금지 패턴

- HTML in translation values (XSS 위험)
- 하드코딩된 날짜 포맷(`"4월 22일"` 대신 `formatDate(d, "long")`)
- 문장 fragments concatenation (번역자가 전체 맥락 필요)

---

## 8. `goose i18n lint` 설계

### 8.1 검사 항목

```
goose i18n lint
 ├─ [ERROR] missing_key_tier1  → Tier 1 언어에서 en 대비 누락
 ├─ [ERROR] invalid_yaml       → YAML 구문 오류
 ├─ [ERROR] invalid_icu        → ICU 문법 오류
 ├─ [ERROR] placeholder_mismatch → en과 번역본의 {var} 목록 불일치
 ├─ [WARN]  missing_key_tier2  → Tier 2 언어 누락
 ├─ [WARN]  unused_key         → 소스 코드에서 사용 안 된 키
 ├─ [WARN]  bom_present        → BOM 제거 필요
 └─ [INFO]  machine_translated_ratio → Tier 3 자동번역 비율
```

### 8.2 출력 포맷

```
packages/goose-desktop/locales/ko/common.yaml:
  ERROR [missing_key_tier1] key "common:farewell" missing (en has value)
  ERROR [placeholder_mismatch] key "onboarding.step_2": en has {name}, ko has {nama}

packages/goose-desktop/locales/ar/common.yaml:
  WARN [machine_translated_ratio] 78% keys are _machine_translated (Tier 3)

Result: 2 errors, 1 warning. Exit code 1.
```

### 8.3 CI 통합

`.github/workflows/ci.yml`의 pre-commit 훅으로 설치 권장:

```yaml
- name: i18n Lint
  run: |
    go run ./cmd/goose i18n lint --strict-tier1
```

---

## 9. Hot Reload 메커니즘 상세 (개발)

개발 모드에서 YAML 수정 → UI 갱신 플로우:

```
1. 개발자가 common.yaml 수정
   ↓
2. chokidar(Vite) 파일 변경 감지
   ↓
3. i18next-http-backend의 reload endpoint 호출
   ↓
4. i18next.reloadResources([lng])
   ↓
5. react-i18next가 useTranslation hook 소비자에게 re-render 신호
   ↓
6. 5초 이내 UI 갱신
```

프로덕션에서는 `i18next-http-backend` 미사용, `import.meta.glob('./locales/**/*.yaml', { eager: true })`로 빌드 타임 번들링. 네트워크 I/O 없음.

---

## 10. 테스트 매트릭스

| Test | 언어 | Type | 목적 |
|------|-----|------|------|
| Plural-OneOther | en, de | unit | 2 form |
| Plural-Russian4 | ru, uk, pl | unit | 4 form |
| Plural-Arabic6 | ar | unit | 6 form |
| Plural-KoreanNone | ko, ja, zh | unit | other only |
| RTL-Switch | ar, he | integration | dir 속성 |
| DateFormat | ko, en, ja | unit | Intl.DateTimeFormat |
| CurrencyFormat | ko, en, ja | unit | ISO 4217 |
| RelativeTime | en, ko, ja, fr | unit | "an hour ago" |
| ListFormat | en, ko, es | unit | Oxford comma |
| ICURejectCode | all | security | eval reject |
| YAMLRejectBOM | all | loader | BOM 감지 |
| LintMissingTier1 | — | lint CLI | ERROR on missing key |

---

## 11. 재사용 vs 재작성 예상 LoC

| 모듈 | 재사용 | 재작성 | 총 LoC 추정 |
|------|-------|-------|-----------|
| Backend Translator | go-i18n/v2 | wrapper + RTL 로직 | ~300 |
| Backend Formatter | x/text/{date,number} | wrapper | ~200 |
| Backend Loader | yaml.v3 | validator | ~150 |
| Backend CLI (lint) | cobra | rules | ~400 |
| Frontend i18n init | i18next + plugins | config + hooks | ~150 TS |
| Frontend RTL util | — | 간단 | ~50 TS |
| Frontend LLM auto-translate UI | — | 배지 + PR 링크 | ~200 TS |
| Tier 3 LLM pipeline | ADAPTER-001 | prompt template + orchestration | ~300 |
| 번역 YAML 초기(20 언어 × ~800 키) | — | — | ~16,000 YAML 엔트리 |
| Tests (Go + TS) | — | CLDR matrix | ~1500 |
| **합계** | — | — | **~3250 LoC + 16k YAML** |

SPEC 크기 `M` (1500~4000 LoC) 범위 준수. YAML 콘텐츠는 LoC 산정 대상 외.

---

## 12. 오픈 이슈 (본 SPEC 내에서 미결)

1. **Tier 2 커뮤니티 리뷰어 모집**: 12 언어 × 평균 2인 = 24인 확보 경로 미정. 초기 제품 출시 후 Discord(branding §9.2)에서 공개 모집 예정.
2. **서법 혼용 언어**: 중국어 simplified vs traditional, 세르비아어 Cyrillic vs Latin 처리. 본 SPEC은 zh-CN만 Tier 1 포함, zh-TW/zh-HK는 Tier 2 지정.
3. **성 중립 언어 처리**: 한국어/일본어/중국어는 성별 구분 없는 반면 스페인어/포르투갈어는 성별 필수. `neutral` fallback 패턴이 UX 측면에서 자연스러운지 미검증.
4. **ICU `selectordinal`**: 영어 1st/2nd/3rd/4th. 아직 요구 케이스 없음. v0.1은 미지원, 필요시 v0.5 추가.
5. **번역 메모리 캐싱**: LLM 자동번역 재호출 시 동일 키 중복 호출 방지. 추후 `~/.goose/i18n-cache/`에 해시 기반 캐시 추가 검토.
6. **RTL 모바일 재시작 UX**: `I18nManager.forceRTL` 후 재시작 강제는 RN 제약. 사용자 프릭션 큰 편. 대안: 모달 안내 + 자동 재시작(Firebase `CodePush`)은 v1.5+.

---

## 13. 참고 문헌

- ICU User Guide: https://unicode-org.github.io/icu/
- CLDR Plural Rules: https://cldr.unicode.org/index/cldr-spec/plural-rules
- Unicode UAX #9 (Bidi Algorithm)
- `nicksnyder/go-i18n/v2` README
- `i18next` documentation
- Mozilla Fluent (대안 조사용)
- W3C Internationalization Activity best practices

---

**End of research.md for SPEC-GOOSE-I18N-001**
