# SPEC-GOOSE-ONBOARDING-001 — Research Notes

> Companion to `./spec.md`. Analyzes competitor onboarding UX, 5-minute budget validation, GDPR/PIPA/PIPL consent text mapping, animation cost, and UX edge cases.

---

## 1. 경쟁사 Onboarding 벤치마크

### 1.1 분석 대상

| 제품 | 단계 수 | 평균 소요 | 완료율* | 주요 수집 |
|------|-------|---------|--------|---------|
| ChatGPT Desktop | 3 | ~2분 | 95% | 이름, API key |
| Claude Desktop | 2 | ~1분 | 97% | OAuth or API key |
| Notion | 5 | ~4분 | 85% | 이름, 사용처, 팀 |
| Linear | 6 | ~5분 | 80% | 이름, 조직, 관심 영역 |
| Raycast | 4 | ~3분 | 90% | 환경 설정, 단축키 |
| Superhuman | 8 | ~15분 | 60% | email 연결, 단축키 훈련, 자세한 개인화 |
| Duolingo | 7 | ~3분 | 93% | 목표 언어, 레벨, 일일 목표 |

*출처: 공개 블로그 포스트, 업계 평균 추정

### 1.2 핵심 인사이트

- **5-8단계가 달성 가능 영역**: Superhuman이 8단계임에도 완료율 60% 유지 (가치 명확)
- **Skip 옵션 중요**: Linear, Notion 모두 Skip 허용 → 완료율 상승
- **기본값 품질**: Duolingo는 기본값이 너무 공격적이라 조정률 낮음 → GOOSE는 보수적 기본값
- **각인 효과**: 이름을 묻는 단계는 "나를 기억한다"는 느낌을 줘 engagement 상승

### 1.3 GOOSE의 차별화

- **부화 애니메이션**: 경쟁사 대부분은 "완료!" 토스트만. GOOSE는 감성적 연결.
- **LOCALE 강조**: 대부분 영어 위주. GOOSE는 Step 2에서 사용자 언어로 전환.
- **Ritual 커스터마이징**: Daily companion의 DNA. Step 6에서 즉시 반려감 체감.

---

## 2. 5분 예산 검증

### 2.1 단계별 예상 소요 (median user)

| Step | Min | Max | 근거 |
|------|-----|-----|------|
| 1 Welcome | 10s | 30s | 읽기 + Next |
| 2 Locale | 20s | 60s | 확인 + 필요 시 수정 |
| 3 Identity | 30s | 90s | 이름 입력 + 옵션 |
| 4 Daily Pattern | 40s | 120s | 5개 시간 선택 (TimePicker UX 중요) |
| 5 Interests | 30s | 90s | 태그 클릭 |
| 6 Rituals | 20s | 60s | 3개 토글 |
| 7 LLM Provider | 60s | 180s | OAuth 대기 시간 포함 |
| 8 Privacy | 30s | 60s | 읽기 + 체크 |
| **총합** | 240s (4분) | 690s (11.5분) | — |

Median: ~6분. P75: ~8분. P95: ~12분.

### 2.2 5분 목표 달성 전략

- **Step 7 OAuth가 bottleneck**: Skip 허용 + "나중에" 배너로 우회 권장
- **Step 4 TimePicker 최적화**: 하나의 "평일 패턴" 추천 프리셋 3종 (early bird / standard / night owl)
- **Step 5 관심사 max 제한**: 직업 1 + 취미 3 + 도메인 2 선택 시 자동 전진 (optional)
- **Skip 사용률 목표**: Step 4~6은 50%+, Step 7은 30%+ Skip 예상

### 2.3 CI 자동 테스트

Playwright 스크립트로 full flow를 스크립트 자동 입력 → 측정:

```
describe('Onboarding 5-min target', () => {
  test('minimal path (all skips + name only)', async ({ page }) => {
    const t0 = Date.now();
    await startOnboarding(page);
    await skipToStep2(page);      // Step 1 → 2
    await acceptLocale(page);      // Step 2 → 3
    await fillName(page, "Test");  // Step 3 → 4
    for (const step of [4, 5, 6, 7]) {
      await skipStep(page);
    }
    await consentDefault(page);    // Step 8 → complete
    const elapsed = Date.now() - t0;
    expect(elapsed).toBeLessThan(60_000); // 자동화 1분 이내 목표
  });
});
```

---

## 3. GDPR/PIPA/PIPL Consent 문구 매핑

### 3.1 GDPR (EU)

**필수 요소**:
- 처리 목적 명시
- 법적 근거 (consent 선택 시)
- 데이터 주체 권리 (access, rectification, erasure, portability, object)
- 철회 권리 명시
- 국가감독기관 정보

**문구 템플릿 (영어)**:

```
GOOSE processes your data to provide the Daily Companion service.

Legal basis: Your explicit consent.

Your rights:
- Access your data at any time
- Correct or delete your data
- Withdraw consent at any time (results in limited features)
- Export all your data in machine-readable format
- Lodge a complaint with your national data protection authority

☐ I explicitly consent to the processing of my personal data as described.
   This consent is freely given and can be withdrawn at any time.
```

### 3.2 PIPA (한국)

**필수 요소**:
- 수집 목적
- 수집 항목
- 보유 기간
- 동의 거부권 + 거부 시 불이익

**문구 템플릿 (한국어)**:

```
개인정보 수집·이용 동의

1. 수집 항목: 이름, 거주 국가, 생활 패턴, 관심사, 대화 기록
2. 수집 목적: GOOSE Daily Companion 서비스 제공, 개인화
3. 보유 기간: 사용자가 탈퇴 또는 삭제 요청 시까지
4. 동의 거부 권리: 거부할 경우 일부 기능 제한됨

※ GOOSE는 주민등록번호, 금융정보, 생체정보 등 민감정보를 수집하지 않습니다.
※ 모든 개인정보는 사용자 기기에 로컬로 저장됩니다.

☐ 개인정보 수집·이용에 동의합니다.
```

### 3.3 PIPL (중국)

**필수 요소**:
- 个人信息处理者名称 (처리자 명칭)
- 处理目的 (처리 목적)
- 处理方式 (처리 방식)
- 保存期限 (보관 기간)
- 个人信息权利行使方式 (권리 행사 방법)

**문구 템플릿 (简体中文)**:

```
个人信息处理告知书

处理者: GOOSE 项目
处理目的: 提供 Daily Companion 服务，个性化体验
处理方式: 本地存储，不上传至境外服务器
保存期限: 至用户请求删除
权利行使: 随时通过应用内设置访问、更正、删除

※ 本应用不收集境外数据，不涉及跨境传输。

☐ 我已阅读并同意以上个人信息处理说明。
```

### 3.4 CCPA (미국 CA)

**필수 요소**:
- 수집 카테고리
- "Do not sell my personal information" 옵션 (GOOSE는 판매 안 함)

**문구 템플릿 (English)**:

```
California Consumer Privacy Notice

Categories of data collected: Identifiers (name), Internet activity (app usage).

GOOSE does not sell your personal information.

Your rights:
- Right to know
- Right to delete
- Right to opt-out (N/A for GOOSE)

☐ I acknowledge the privacy practices of GOOSE.
```

### 3.5 LGPD (브라질), FZ-152 (러시아), APPI (일본)

- LGPD: GDPR 유사, 포르투갈어
- FZ-152: 데이터 현지화 조항, 러시아 영토 내 저장 권고 (GOOSE는 로컬 저장이므로 자연 준수)
- APPI: 처리 목적 + 제3자 제공 여부 (GOOSE는 제3자 제공 없음)

각 국가 문구는 LOCALE-001의 `legal_flags` + I18N-001의 `locales/{lang}/consent.yaml`에서 조합.

---

## 4. 애니메이션 비용 측정

### 4.1 부화 애니메이션 (Step 8 → main UI 전환)

- 재생 시간: 3초
- Framer Motion: 6 keyframe 전환
- 예상 비용: macOS M1 기준 60fps 안정, Windows i5-8세대 45~60fps
- `prefers-reduced-motion` 시: fade only, 0.5초

### 4.2 GPU 가속

- CSS `transform` + `opacity` 위주 → GPU 가속
- `filter: blur(...)` 는 GPU 비용 높음, 회피
- 이미지 자산은 WebP + 256x256 이하

### 4.3 저사양 PC 대응

- 저사양 감지: `navigator.hardwareConcurrency < 4` → 애니메이션 축약
- "Skip animation" 버튼: 1초 내 완료 표시

---

## 5. 입력 사니타이징 규칙

### 5.1 이름 필드 (Step 3)

허용 문자 regex:
```
^[\p{L}\p{M}\p{N}\s\-.,'ㆍ・]{1,100}$
```

- `\p{L}` 모든 언어 문자
- `\p{M}` 결합 다이어크리틱 (é, ñ, ü)
- `\p{N}` 숫자 (이름에 "2세" 등)
- `\s`, `-`, `.`, `,`, `'`, `ㆍ`, `・` (구두점)
- HTML 태그, shell metachar 차단

### 5.2 Custom Tags (Step 5)

- 최대 10개
- 각 태그 최대 40자
- 허용 문자: 이름 필드와 동일 + `#`
- URL 포맷 거부 (tag 공간 남용 방지)

### 5.3 Custom LLM Endpoint (Step 7)

- URL 스키마 강제: `https://` only (HTTP 거부)
- localhost/127.0.0.1/192.168.x/10.x 예외 (Ollama 등)
- IP 리터럴은 localhost 예외 외 거부
- Port range 1024~65535

---

## 6. Draft Resume 메커니즘

### 6.1 저장 포맷

`~/.goose/onboarding-draft.yaml`:

```yaml
session_id: abc-def-12345
started_at: "2026-04-22T10:30:00+09:00"
last_updated: "2026-04-22T10:32:15+09:00"
current_step: 5
data:
  locale:
    country: KR
    primary_language: ko-KR
    timezone: Asia/Seoul
  identity:
    name: "홍길동"
  daily_pattern:
    wake_time: "06:30"
    # ...
  # step 5 in progress
```

### 6.2 원자적 쓰기

- `os.Rename` 기반 atomic write
- 임시 파일 `~/.goose/.onboarding-draft.yaml.tmp` → rename

### 6.3 재개 플로우

```
1. 앱 실행
2. CONFIG 읽기: onboarding_completed == false
3. draft 파일 존재 확인
4. draft.last_updated가 24시간 이내 → 재개 프롬프트
   > "이전에 중단하신 온보딩을 계속하시겠어요? (Step 5부터)"
   > [처음부터 다시] [계속하기]
5. 24시간 초과 → draft 삭제, Step 1부터
```

### 6.4 손상 복구

- YAML schema 검증 실패 → draft 삭제, Step 1부터
- partial data 허용 (빈 필드는 기본값)

---

## 7. UX 접근성 (WCAG AA)

### 7.1 필수 항목

- 모든 버튼 키보드 접근
- Tab 순서 논리적 (진행 방향)
- Focus 표시 가시 (aria-ring 2px)
- 대비 4.5:1 이상
- alt 텍스트 (부화 애니메이션)
- `prefers-reduced-motion`, `prefers-color-scheme`

### 7.2 RTL 지원

- Step 진행 방향은 시각적으로 RTL에서 반대 (Next 버튼이 좌측)
- 진행 바의 "채움" 방향도 RTL 반전

### 7.3 Screen Reader

- 각 Step에 `aria-live="polite"` 진행 알림
- 에러 메시지는 `role="alert"` 즉시 읽힘

---

## 8. 각 Step 데이터 → 후속 SPEC 매핑 매트릭스

| 데이터 필드 | 저장 위치 | 소비 SPEC | 소비 방식 |
|-----------|---------|---------|---------|
| country | CONFIG `locale.override.country` | LOCALE-001, REGION-SKILLS-001, SCHEDULER-001 | 런타임 로드 |
| primary_language | CONFIG `locale.override.primary_language` | I18N-001, ADAPTER-001 | UI + prompt |
| timezone | CONFIG `locale.override.timezone` | SCHEDULER-001, BRIEFING-001 | cron scheduling |
| name | CONFIG `user.name` + Identity Graph seed | ADAPTER-001, BRIEFING-001, JOURNAL-001 | prompt injection |
| preferred_honorific | CONFIG `user.honorific` | ADAPTER-001 persona | prompt 말투 힌트 |
| wake_time | CONFIG `schedule.wake` | SCHEDULER-001 | cron |
| breakfast/lunch/dinner | CONFIG `schedule.meals.*` | SCHEDULER-001, HEALTH-001 | cron |
| sleep_time | CONFIG `schedule.sleep` | SCHEDULER-001, JOURNAL-001 | quiet hours |
| interests | CONFIG `user.interests` + Identity Graph | VECTOR-001, BRIEFING-001 | 추천 topic |
| rituals enabled | CONFIG `rituals.*.enabled` | RITUAL-001 | on/off |
| llm_provider | CONFIG `llm.default_provider` | CREDPOOL-001, ROUTER-001 | 프로바이더 선택 |
| api_key | OS Keychain | CREDPOOL-001 | lookup at use |
| consent.conversation_storage | CONFIG `consent.storage` | MEMORY-001, JOURNAL-001 | persist or not |
| consent.lora_training | CONFIG `consent.lora` | LORA-001 | train or skip |
| consent.telemetry | CONFIG `consent.telemetry` | 전역 | emit or not |

---

## 9. 테스트 매트릭스

| Test ID | Step | Type | 목적 |
|---------|------|------|------|
| AC-OB-001 | 1 | integration | First launch triggers onboarding |
| AC-OB-002 | any | ui | Progress bar 렌더링 |
| AC-OB-003 | 2 | integration | Locale 수정 → I18N 전환 |
| AC-OB-004 | 2 | integration | Conflict UI 표시 |
| AC-OB-005 | 3 | unit | Name empty 검증 |
| AC-OB-006 | 4 | unit | Skip → defaults |
| AC-OB-007 | 7 | integration | Keychain 저장 |
| AC-OB-008 | 8 | integration | GDPR consent |
| AC-OB-009 | complete | integration | Region skills 활성화 |
| AC-OB-010 | complete | ui | 부화 애니메이션 |
| AC-OB-011 | any | integration | Draft resume |
| AC-OB-012 | 7 | integration | Skip LLM |
| AC-OB-013 | complete | ui | Mobile pairing 제안 |
| AC-OB-014 | 8 | integration | GDPR skip 거부 |
| AC-OB-015 | 3 | security | Name injection reject |
| AC-OB-016 | full | e2e | 5분 이내 완료 |

---

## 10. 재사용 vs 재작성 LoC

| 모듈 | 재사용 | 재작성 | 총 LoC |
|------|-------|-------|-------|
| Frontend: 8 Step components | shadcn/ui + react-hook-form | Step logic + i18n | ~1200 TS |
| Frontend: zustand store + types | zustand | state machine | ~300 TS |
| Frontend: Animation | framer-motion | 부화 시퀀스 | ~150 TS |
| Backend: OnboardingFlow | — | state machine | ~350 Go |
| Backend: Step validators | zod-like | 8 step × 검증 | ~250 Go |
| Backend: Completion handler | 후속 SPEC API | orchestration | ~200 Go |
| Rust Tauri commands | tauri-plugin-keyring | wrapper | ~180 Rust |
| Tests (Go + TS + E2E) | — | matrix | ~1200 |
| i18n 번역 (20+ 언어 × ~80 키) | I18N-001 | YAML 콘텐츠 | — |
| **합계** | — | — | **~3830 LoC** |

SPEC 크기 `M` (1500~4000 LoC) 상한 근접. 스켈레톤 중심 개발 후 iteration으로 세부 UX 개선.

---

## 11. 오픈 이슈

1. **OAuth 리다이렉트 대기 UX**: Step 7에서 OAuth 클릭 후 브라우저 이동. 사용자가 돌아오지 않으면 Step 7에 영원히 머무름. Timeout + "Skip" fallback 필요(30초 제안).
2. **Ollama 모델 드롭다운**: Step 7 Ollama 선택 시 `GET http://localhost:11434/api/tags` 호출. 실패 시 "Ollama 미실행" 안내.
3. **개인정보 처리방침 문구 법률 검토**: `docs/privacy-policy.md`가 초안이며, 프로덕션 배포 전 지역별 법률 전문가 검토 필요.
4. **Identity Graph seed의 시간 절약 효과**: 초기 노드가 너무 적으면 POLE+O 확장 시간이 늘어남. Step 5 관심사 태그를 Identity Graph "interest" 노드로 직접 변환 최적화.
5. **Mobile 페어링 제안 타이밍**: 완료 직후 제안 vs 첫 채팅 후 제안. 전자는 engagement ↑, 후자는 부담 ↓. A/B 테스트 필요(v1.0+).
6. **Skip 과다 사용 시 "반쪽" 온보딩**: 모든 step을 Skip하면 데이터가 거의 없음. Incentive 필요("3개 관심사 추가하면 개인화 향상" 메시지).
7. **재온보딩 권한**: Preferences > Re-run onboarding. 재실행 시 기존 데이터 유지 vs 초기화 선택.

---

## 12. 참고 문헌

- GDPR official text (EU)
- PIPA (개인정보보호법, 한국)
- PIPL (中华人民共和国个人信息保护法)
- CCPA Final Regulations (California AG)
- LGPD (Lei Geral de Proteção de Dados, Brasil)
- Nielsen Norman Group: Onboarding UX research
- Superhuman Onboarding case study
- Duolingo streak/commitment research

---

**End of research.md for SPEC-GOOSE-ONBOARDING-001**
