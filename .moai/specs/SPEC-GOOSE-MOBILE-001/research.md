# SPEC-GOOSE-MOBILE-001 — Research Notes

> React Native 0.76+ New Architecture 선택, Whisper/Porcupine on-device, 한국 특화(카카오) 분리, 푸시 provider 전략.

---

## 1. React Native 0.76+ New Architecture 선택 근거

### 1.1 New Architecture 핵심

- **Fabric**: JSI(JavaScript Interface) 기반 렌더러. Bridge 제거, 동기 호출 가능
- **TurboModules**: 네이티브 모듈 lazy 로드. startup ~40% 단축
- **Codegen**: TypeScript 타입 → C++ 네이티브 인터페이스 자동 생성

### 1.2 vs Flutter

| 항목 | RN 0.76+ | Flutter |
|-----|---------|---------|
| 네이티브 look | 네이티브 위젯 | 자체 렌더 (Material/Cupertino 흉내) |
| 생태계 | npm 전체 + native modules | pub.dev |
| 팀 스킬 | TypeScript (Desktop 공유) | Dart (신규) |
| Desktop과 코드 공유 | 가능 (types.ts, 비즈니스 로직) | 불가 |
| Apple Silicon 최적화 | Hermes JS engine | AOT native |

**결론**: Desktop(TS) ↔ Mobile 코드 공유 용이성 및 팀 스킬로 RN 선택.

### 1.3 Expo vs Bare Workflow

- **Expo**: 빌드 서비스 간편, but 일부 native module 제약
- **Bare**: 유연성 최대, Porcupine/Whisper native 통합 편함

**결정**: Bare workflow (Porcupine, Whisper, vision-camera 등 네이티브 의존성 많음).

---

## 2. Wake word: Picovoice Porcupine 분석

### 2.1 왜 Porcupine인가

| 후보 | 장단점 |
|-----|-------|
| **Porcupine** | 커스텀 keyword 지원, on-device, 무료 개인용 tier | 상용 라이센스 별도 |
| Snowboy | 2022년 EOL |
| Vosk wake | 정확도 낮음 |
| OpenWakeWord (ESP) | 실험적, RN 바인딩 부재 |

### 2.2 커스텀 "Hey goose" 훈련

- Picovoice Console에서 "Hey goose" phrase 업로드
- .ppn 파일 다운로드 (iOS/Android 별도)
- 앱 번들에 포함

### 2.3 배터리 & 프라이버시

- 상시 대기 전력: iPhone 15 Pro ~0.8%/시간 측정치
- 오디오는 on-device 처리, 네트워크 송출 없음
- Wake 감지 이벤트만 JS로 전달 (privacy-by-design)

---

## 3. STT: Whisper.rn (whisper.cpp wrapper)

### 3.1 모델 선택

| 모델 | 크기 | RAM | 정확도 | 속도(iPhone 15) |
|-----|-----|-----|-------|----------------|
| tiny | 75MB | ~160MB | 낮음 | 실시간 |
| base | 150MB | ~210MB | 중간 | 실시간 |
| small | 470MB | ~640MB | 높음 | 1.5x realtime |

**1차 기본**: base 모델. 사용자 선택 옵션 제공.

### 3.2 언어 지원

- base 다언어 모델: en/ko/ja/zh 포함
- language detection 자동 + 수동 override

### 3.3 Core ML / Metal 가속

- iOS: Whisper.cpp가 Core ML backend 지원 → GPU 가속
- Android: Vulkan backend (Android 13+)

### 3.4 대안

- Apple Speech Framework (iOS): 무료, 정확도 준수. but 오프라인 지원 제한적
- Google SpeechRecognizer (Android): 오프라인 모델 별도 다운로드
- 결정: **Whisper 통일**. 크로스 플랫폼 일관성 + 오프라인 default + 다언어 커버.

---

## 4. 푸시 Provider 전략

### 4.1 iOS APNs

- iOS 17+ 기본
- Silent push(`content-available: 1`)로 capacityWake 용도
- Push Entitlement 필요, Apple Developer Program

### 4.2 Android FCM

- Firebase Cloud Messaging
- `priority: high` + `data-only` 메시지로 background wake
- google-services.json 포함

### 4.3 구현 경로

```
goosed (PC) --push event--> Bridge
                              |
                              v
                     FCM / APNs gateway
                              |
                              v
                     Mobile OS push service
                              |
                              v
                     Mobile app (wake → reconnect)
```

- Bridge가 APNs/FCM 직접 호출 vs 별도 gateway:
  - 1차: Bridge에 통합 (gateway 단순화)
  - 향후: GATEWAY-001이 multi-platform 통합

### 4.4 토큰 수명 관리

- Mobile이 토큰 갱신 시 Bridge에 재등록
- Bridge는 device_id + 최신 token 매핑

---

## 5. 한국 특화: 카카오톡 분리

### 5.1 왜 GATEWAY-001로 분리

카카오톡 통합은 **메신저 봇 패턴**이라 Mobile 앱이 아닌 별도 gateway 서비스에서 처리:

- 카카오 알림톡 API는 서버 → 사용자 phone으로 직접 (Mobile 앱 불필요)
- Mobile 앱은 **토글만** 노출 ("카카오톡으로 알림도 받기")
- 실제 메시지는 goosed → GATEWAY-001 → Kakao API → 사용자 카톡

### 5.2 Mobile 역할

- Settings 화면에서 "KakaoTalk Notifications" 스위치 제공
- 활성화 시 OAuth 로그인 (kakao SDK)
- 수신한 access token을 Bridge로 전달 → goosed가 GATEWAY-001에 등록

### 5.3 Opt-in 원칙

기본 OFF. 사용자가 한국 거주/사용자 계정일 때만 UI 노출.

---

## 6. Secure Storage 전략

### 6.1 iOS

- Keychain Services via `react-native-keychain`
- `kSecAttrAccessibleWhenUnlockedThisDeviceOnly`
- Biometric gate 옵션 (`accessControl: BIOMETRY_CURRENT_SET`)

### 6.2 Android

- Android Keystore via `react-native-keychain`
- EncryptedSharedPreferences (AndroidX Security)
- StrongBox 우선 (Pixel 3+)

### 6.3 저장 대상

| 항목 | 저장 방법 |
|-----|---------|
| Bridge access JWT | Keychain/Keystore |
| Bridge refresh token | Keychain/Keystore + biometric |
| 사용자 ed25519 private key | Keychain/Keystore + biometric |
| Journal draft (암호화) | SQLCipher + DB encryption key in Keychain |
| Settings | UserDefaults / SharedPreferences (암호화 불필요) |

---

## 7. 오프라인 Journal 큐 설계

### 7.1 로컬 DB

- SQLCipher (암호화 SQLite)
- 테이블 `journal_drafts`:

```sql
CREATE TABLE journal_drafts (
  id TEXT PRIMARY KEY,
  date TEXT NOT NULL,
  body TEXT NOT NULL,
  attachments_json TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  sync_state TEXT NOT NULL  -- pending | syncing | synced | failed
);
```

### 7.2 Sync 로직

1. 네트워크 복구 이벤트 감지 (NetInfo)
2. `sync_state = pending` 레코드 FIFO 조회
3. Bridge로 순차 전송, 성공 시 `synced`
4. 실패 시 지수 백오프 후 재시도, max 5회

### 7.3 충돌 해결

- 일기는 Mobile이 원본(user는 mobile에서만 작성)
- PC도 일기 보여주지만 read-only
- 충돌 불가 (단방향)

---

## 8. Hermes gateway/ vs BRIDGE/MOBILE 관계

사용자가 제공한 Hermes gateway/ 패턴은 **BRIDGE-001이 아니라 GATEWAY-001의 참조**임을 재확인:

| 패턴 | 대상 |
|-----|-----|
| Telegram/Discord/Slack/Matrix/Kakao/WeChat Bot | GATEWAY-001 |
| Claude Code bridge/ (iPhone companion) | BRIDGE-001 + MOBILE-001 |

MOBILE-001은 **퍼스트파티 RN 앱**이고, GATEWAY-001은 **서드파티 메신저**에서 goose 호출하는 봇이다.

---

## 9. iOS / Android 최소 요구사항

### 9.1 iOS

- iOS 17.0+
- iPhone 12 이상 (Neural Engine for Whisper Core ML)
- Push 권한 필수
- Face ID 또는 Touch ID 권장
- Camera (QR 스캔)

### 9.2 Android

- Android 12+ (API 31+)
- 4GB+ RAM (Whisper base 모델)
- Google Play Services (FCM)
- BiometricPrompt API
- Camera2 API

---

## 10. TDD 전략 상세

### 10.1 단위 테스트

- Jest + `@testing-library/react-native`
- Bridge client, zustand stores, 페어링 검증 로직, 오프라인 큐

### 10.2 네이티브 유닛

- iOS: XCTest로 Keychain wrapper, Biometric wrapper
- Android: Robolectric + JUnit for Keystore

### 10.3 E2E

- Detox: pairing full flow (mock Desktop QR), message send/receive, offline → online
- 5개 디바이스 매트릭스: iPhone 15 Pro, iPhone SE, Galaxy S25, Pixel 8, Redmi

### 10.4 Visual regression

- Storybook + Chromatic (optional)

### 10.5 커버리지 목표

- TS: 85%+
- iOS native: 70%+
- Android native: 70%+

---

## 11. 오픈 이슈

1. **iOS Push 비용**: Apple Developer Program $99/년 필수 (조직 계정 고려).
2. **Porcupine 상용 라이센스**: GOOSE가 오픈소스 but Porcupine 코어는 상용. 개인 사용자 무료 tier + 기업용 라이센스 필요.
3. **Whisper 모델 배포**: 앱 번들에 포함(150MB)? → 초기 다운로드 UX 고려. 1차는 앱 번들, 2차는 on-demand.
4. **Background 지속 연결**: iOS BGProcessingTask로 ~30분 간격 heartbeat? 아니면 push 의존? 1차는 push.
5. **한국 앱스토어**: App Store KR 심사 이슈(추적 투명성). 기본 off가 안전.
6. **재난 경보 / Do Not Disturb**: 브리핑 푸시가 Do Not Disturb 모드에서 무음? Category 설정 필요.
7. **위치 트리거 배터리**: geofence는 iOS/Android OS 수준이라 영향 적으나 20개 한계 존재.
8. **QR 재사용 방지**: pairingToken 1회용이지만 스크린샷 후 공유 차단 불가. 5분 TTL로 mitigate.

---

## 12. 참조

- React Native 0.76 release notes (New Architecture GA)
- Picovoice Porcupine docs: <https://picovoice.ai/docs/porcupine/>
- whisper.cpp: <https://github.com/ggerganov/whisper.cpp>
- whisper.rn: <https://github.com/mybigday/whisper.rn>
- Apple APNs programming guide
- Firebase FCM best practices
- Expo 52 vs bare RN 비교 (2026.02)
- structure.md §4.3, tech.md §4.3
