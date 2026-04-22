---
id: SPEC-GOOSE-MOBILE-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 6
size: 대(L)
lifecycle: spec-anchored
---

# SPEC-GOOSE-MOBILE-001 — GOOSE Mobile Companion App (iOS + Android)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | ROADMAP v4.0 Phase 6 신규 SPEC. 사용자 최종 컨셉(2026-04-22)에 따라 PC 메인 + Mobile **원격 동반(companion) 앱**. React Native 0.76+ New Architecture, QR 페어링, Bridge/Relay 경유 원격 채팅, 아침 브리핑 푸시, 저녁 일기 모바일 입력. | manager-spec |

---

## 1. 개요 (Overview)

Mobile은 **PC에 종속된 동반 앱**이다. 첫 실행 시 Desktop App이 생성한 QR 코드를 스캔하여 페어링하면, 이후 언제 어디서든 PC의 `goosed` 세션에 원격 접속한다. PC가 오프라인이면 Mobile은 대부분의 기능이 제한된다(단, 오프라인 일기 저장 후 동기화는 허용).

주요 기능:

1. **QR 페어링**: Desktop이 생성한 QR 스캔 → Bridge/Relay 연결 수립
2. **원격 채팅**: Mobile에서 질문 → PC goosed가 처리 → 응답 스트리밍
3. **아침 브리핑 푸시**: PC가 발송하는 시간별 브리핑을 모바일 push 알림으로 수신
4. **저녁 일기 입력**: Mobile이 입력 편의상 최적(키보드 + 음성)
5. **음성 입력**: Whisper on-device + "Hey goose" wake word (Picovoice Porcupine)
6. **위치 기반 트리거** (옵션): 집·회사·학교 등 geofence로 맥락 알림

기술 기반: React Native 0.76+ (New Architecture - TurboModules, Fabric), TypeScript, BRIDGE-001/RELAY-001 소비. iOS(iPhone 12+, iOS 17+) + Android(Android 12+/API 31+).

한국 특화(옵션): 카카오 알림톡 수신 경로는 GATEWAY-001과 연계.

---

## 2. 배경 (Background)

### 2.1 왜 Mobile이 **동반**이지 **대체**가 아닌가

사용자 지시(2026-04-22):

> "기본 설치는 pc이지만 모바일 클라우드 연동으로 앱에서 pc를 제어 또는 지시를 할 수가 있다."

- PC가 고성능 연산(LLM 호출, LoRA 추론, tool 실행) 담당
- Mobile은 **원격 인터페이스** — 입력·알림·언제 어디서나 접근
- PC가 꺼져 있으면 기능 제한 (단, Trusted Device 1회 pairing이면 PC wake 후 자동 재연결)

### 2.2 왜 React Native 0.76+ New Architecture

- **Fabric** renderer: 네이티브 레이아웃 동기 실행, 60fps 안정
- **TurboModules**: lazy 로드, startup 단축
- **Codegen**: TypeScript → 네이티브 타입 안전 브리지
- **HarmonyOS 지원 확대**(중국 시장) 계획 시 유리
- Expo 52+ 호환

### 2.3 왜 Picovoice Porcupine

- On-device wake word (클라우드 불필요 → 프라이버시)
- iOS/Android 모두 지원
- 커스텀 wake word "Hey goose" 훈련 가능 (Picovoice Console)
- 낮은 전력: iPhone에서 상시 대기 시 ~1%/시간

### 2.4 왜 Whisper on-device

- `whisper.rn` (Community React Native binding of whisper.cpp)
- Tiny/Base 모델로 모바일에서 실시간 STT
- 한국어·일본어·중국어 포함 다언어 지원 (모델 선택)
- 클라우드 호출 없음 → E2EE 원칙 유지

### 2.5 structure.md와의 정합성

- structure.md §1 `packages/goose-mobile/` 존재. 본 SPEC이 **행동 계약**을 정의.
- structure.md §4.3이 이미 기술 스택 명시(React Native, Picovoice, Whisper, ExecuTorch, Stripe).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `packages/goose-mobile/` React Native 0.76+ 프로젝트.
2. **QR 페어링 화면**:
   - 카메라 접근 권한 요청
   - QR 스캔 (react-native-vision-camera + vision-camera-code-scanner)
   - Desktop에서 생성한 `{pairingToken, relayEndpoint, serverPublicKey, deviceDisplayName}` 디코드
   - Bridge `/pair` 호출 → JWT 수신 → 보안 저장(iOS Keychain, Android EncryptedSharedPreferences)
3. **원격 채팅 화면**:
   - 메시지 리스트 (가상화, FlashList)
   - 입력 영역 (멀티라인, 첨부, 음성 토글)
   - 스트리밍 청크 렌더링 (≤50ms 간격)
   - 오프라인 배너 (연결 실패 시)
4. **저녁 일기 화면** (Journal Entry):
   - 날짜 헤더, 큰 텍스트 입력
   - 음성 입력 토글 (Whisper on-device)
   - 저장 버튼 → PC로 동기화, 오프라인 시 로컬 큐
5. **푸시 알림**:
   - iOS: APNs 토큰 등록 → Bridge에 report
   - Android: FCM 토큰 등록 → Bridge에 report
   - 알림 탭 시 해당 컨텍스트로 앱 딥링크
6. **Wake word (옵션 ON/OFF)**:
   - Picovoice Porcupine "Hey goose" 커스텀 모델
   - 감지 시 음성 입력 모드 자동 진입
7. **생체 인증 잠금**:
   - 앱 실행 시 Face ID / Touch ID / Android Biometric
   - 로컬 캐시(저장된 메시지, 일기 초안)는 잠금 해제 후에만 접근
8. **설정**:
   - Trusted Device 관리 (페어링 해제)
   - 언어 (ko/en/ja/zh, 기본은 OS 설정 추적)
   - 다크 모드
   - Wake word on/off, 음성 입력 모델 선택
9. **위치 트리거 (옵션)**:
   - Geofence 등록 (회사 도착 시 알림 등)
   - 기본 OFF, 사용자 명시적 enable 필수
10. **i18n**: ko/en/ja/zh (Desktop과 동일)
11. **배포**: iOS는 TestFlight → App Store, Android는 Play Store Internal → Production

### 3.2 OUT OF SCOPE

- **Offline-first 풀 AI**: Mobile에서 LLM을 로컬 구동하지 않음. Whisper(STT)만 on-device. 향후 ExecuTorch LoRA는 별도 SPEC.
- **Video call / screen share**: v2+.
- **푸시 provider 서버 구현(APNs/FCM gateway)**: Bridge가 APNs/FCM에 직접 보내거나 GATEWAY-001이 중계. Mobile은 토큰 등록만.
- **Desktop 페어링 UI**: DESKTOP-001.
- **Bridge/Relay 내부 구현**: BRIDGE-001, RELAY-001.
- **Kakao / WeChat 통합**: GATEWAY-001.
- **위젯 / App Clips / Instant Apps**: v2+.
- **Wear OS / Apple Watch**: v2+.

---

## 4. 의존성 (Dependencies)

- **상위 의존**: SPEC-GOOSE-BRIDGE-001(프로토콜), SPEC-GOOSE-RELAY-001(전송), SPEC-GOOSE-DESKTOP-001(pairing 상대 + QR 생성).
- **느슨한 연계**: SPEC-GOOSE-GATEWAY-001(메신저 봇 옵션).
- **라이브러리 (RN 0.76+)**:
  - `react-native` 0.76+, `react` 19.x, `typescript` 5.x
  - `@connectrpc/connect` 2.x (gRPC-Web for Bridge `/pair` REST endpoint와 병용)
  - `react-native-vision-camera` 4.x + `react-native-vision-camera-code-scanner`
  - `@picovoice/porcupine-react-native` 3.x (wake word)
  - `whisper.rn` 0.4+ (on-device STT)
  - `@notifee/react-native` 7.x + `@react-native-firebase/messaging` (push)
  - `react-native-keychain` 8.x (iOS Keychain, Android Keystore wrapper)
  - `react-native-biometrics` 3.x
  - `react-native-background-geolocation` 4.x (옵션)
  - `zustand` 5.x, `@shopify/flash-list` 1.x
  - `i18next` + `react-i18next`

---

## 5. 요구사항 (EARS Requirements)

### 5.1 Ubiquitous

- **REQ-MB-001**: The Mobile app **shall** store Bridge JWT and refresh tokens only in the OS secure enclave (iOS Keychain, Android Keystore).
- **REQ-MB-002**: The Mobile app **shall** request biometric authentication at app launch whenever the last successful unlock was more than 15 minutes ago.
- **REQ-MB-003**: The Mobile app **shall** externalize all user-facing strings to ko / en / ja / zh language packs and follow the OS language as the default.
- **REQ-MB-004**: The Mobile app **shall** implement a pairing flow that requires an explicit QR scan followed by biometric confirmation before registering a Trusted Device.

### 5.2 Event-Driven

- **REQ-MB-005**: **When** the user scans a valid pairing QR, the app **shall** decode the payload, generate an ed25519 keypair, call Bridge `/pair`, and store the returned tokens upon success.
- **REQ-MB-006**: **When** the Bridge WebSocket disconnects while the user is in foreground, the app **shall** attempt reconnection with exponential backoff (1s, 2s, 4s, capped at 30s) and display a reconnecting indicator.
- **REQ-MB-007**: **When** the OS delivers a silent push notification from Bridge, the app **shall** wake and attempt a WebSocket reconnect within 3 seconds of foreground.
- **REQ-MB-008**: **When** the wake word "Hey goose" is detected (if enabled), the app **shall** open the voice input modal without requiring the user to unlock the phone.
- **REQ-MB-009**: **When** the user taps a push notification, the app **shall** deep-link to the contextual screen (briefing, chat message, ritual reminder) indicated by the notification payload.

### 5.3 State-Driven

- **REQ-MB-010**: **While** the app is offline, the app **shall** queue journal entries locally in an encrypted SQLite DB and synchronize them in FIFO order on reconnect.
- **REQ-MB-011**: **While** the phone is in background and battery saver is on, the app **shall** rely on push notifications for updates rather than maintaining a WebSocket.
- **REQ-MB-012**: **While** a chat response is streaming, the app **shall** render partial chunks in the UI at intervals not exceeding 50ms.

### 5.4 Optional

- **REQ-MB-013**: **Where** the user enables location-based triggers, the app **shall** register background geofences for up to 20 user-defined locations.
- **REQ-MB-014**: **Where** the user enables wake word, the app **shall** run Porcupine with the custom "Hey goose" keyword and respect the battery saver preference.
- **REQ-MB-015**: **Where** the user selects a regional locale (ko-KR), the app **shall** offer the optional KakaoTalk notification path via GATEWAY-001 during settings.

### 5.5 Unwanted Behavior

- **REQ-MB-016**: **If** biometric authentication fails three consecutive times, **then** the app **shall** lock down for 60 seconds and require fallback passcode on the fourth attempt.
- **REQ-MB-017**: **If** a QR payload signature does not match the expected Bridge server public key, **then** the app **shall not** proceed with pairing and **shall** display a warning dialog.
- **REQ-MB-018**: **If** the Bridge returns `session_revoked` on reconnect, **then** the app **shall** clear local tokens, notify the user, and redirect to the pairing screen.

### 5.6 Complex

- **REQ-MB-019**: **While** the phone is locked and the wake word feature is enabled, **when** the user says "Hey goose", the app **shall** play a subtle confirmation tone and queue the voice input for delivery once the user unlocks — voice input **shall not** be sent to PC while the phone is locked.

---

## 6. 핵심 타입 시그니처 (TypeScript)

```typescript
// packages/goose-mobile/src/types.ts

// Mobile 애플리케이션 진입
export interface MobileApp {
  bootstrap(): Promise<void>;              // 토큰 로드, Bridge 재연결
  shutdown(): Promise<void>;
}

// QR 페어링 페이로드 (Desktop이 생성)
export interface PairingPayload {
  pairingToken: string;          // 1회용, 5분 유효
  relayEndpoint: {               // RELAY-001 참조
    url: string;
    serverPublicKey: string;     // base64 (ed25519/x25519)
    preSharedKey?: string;       // 옵션 PSK
  };
  serverDisplayName: string;     // "Goos의 MacBook Pro"
  issuedAt: number;              // unix epoch
  signature: string;             // Desktop 정적 키로 서명
}

export interface PairingScreen {
  scan(): Promise<PairingPayload>;               // 카메라 → QR → 파싱
  verify(payload: PairingPayload): boolean;       // 서명 & 만료 검증
  complete(payload: PairingPayload): Promise<TrustedDeviceHandle>;
}

// 원격 채팅 화면
export interface RemoteChatScreen {
  send(text: string, attachments?: Attachment[]): Promise<void>;
  cancelStreaming(): void;
  scrollToBottom(): void;
}

// 저녁 일기 화면
export interface JournalEntryScreen {
  startNewEntry(date: Date): JournalDraft;
  save(draft: JournalDraft): Promise<"synced" | "queued">;  // 오프라인 큐
}

export interface JournalDraft {
  id: string;
  date: string;                  // YYYY-MM-DD
  body: string;
  attachments: Attachment[];
  createdAt: number;
  updatedAt: number;
}

// 음성 입력
export interface VoiceInput {
  start(): Promise<void>;                  // Whisper 녹음 시작
  stop(): Promise<string>;                  // 전사 결과 반환
  cancel(): void;
}

// 푸시 알림
export interface PushNotification {
  registerToken(): Promise<string>;         // APNs/FCM 토큰 획득
  reportToBridge(token: string): Promise<void>;
  onReceive(handler: (payload: PushPayload) => void): Unsubscribe;
  onTap(handler: (payload: PushPayload) => void): Unsubscribe;
}

export interface PushPayload {
  kind: "briefing" | "ritual" | "chat" | "control";
  sessionId?: string;
  deepLink?: string;
}

// Wake word
export interface WakeWord {
  enable(): Promise<void>;                  // Porcupine 시작
  disable(): Promise<void>;
  onDetected(handler: () => void): Unsubscribe;
}

// Trusted Device 핸들 (pairing 성공 후)
export interface TrustedDeviceHandle {
  deviceId: string;
  serverDisplayName: string;
  pairedAt: number;
}
```

---

## 7. 수락 기준 (Acceptance Criteria)

### 7.1 AC-MB-001 — QR 페어링

**Given** Desktop이 QR 표시 **When** Mobile이 카메라로 스캔 **Then** 10초 이내 서명 검증·Bridge `/pair` 호출·Trusted Device 등록 완료. 이후 메인 화면 진입.

### 7.2 AC-MB-002 — 조작된 QR 거부

**Given** 서명이 잘못된 QR **When** Mobile이 스캔 **Then** 경고 다이얼로그 표시, 페어링 미진행. 재시도 허용.

### 7.3 AC-MB-003 — 생체 인증 잠금

**Given** 앱을 16분 후 재실행 **When** 앱 열기 **Then** Face/Touch ID 프롬프트 표시. 성공 시 이전 화면 복원. 3연속 실패 시 60초 lockout.

### 7.4 AC-MB-004 — 원격 채팅 스트리밍

**Given** 페어링된 상태 + 활성 세션 **When** 메시지 전송 **Then** 1초 이내 PC가 처리 시작, 응답 청크가 50ms 간격으로 UI 업데이트.

### 7.5 AC-MB-005 — 오프라인 일기 큐

**Given** Wi-Fi/셀룰러 오프라인 **When** 일기 작성 후 저장 **Then** "오프라인 저장됨" 표시, 로컬 DB에 암호화 저장. 네트워크 복구 시 자동 동기화, UI에 "동기화 완료" 토스트.

### 7.6 AC-MB-006 — 푸시 알림 수신

**Given** 앱이 background + APNs 토큰 등록 완료 **When** PC가 브리핑 발송 **Then** 3초 이내 알림 도착. 탭 시 브리핑 화면으로 딥링크.

### 7.7 AC-MB-007 — Wake word

**Given** Wake word ON **When** 사용자가 "Hey goose" 말하기 **Then** 1초 이내 감지되어 음성 입력 모달 오픈 (앱 foreground 시). Porcupine 배터리 영향 ≤ 2%/시간.

### 7.8 AC-MB-008 — 잠금 상태에서 wake word

**Given** 잠금 + Wake word ON **When** "Hey goose" **Then** 확인 tone 재생, 입력은 큐. 잠금 해제 전까지 PC로 전송되지 않음.

### 7.9 AC-MB-009 — 세션 revoke 처리

**Given** Desktop이 해당 Mobile device revoke **When** Mobile이 재연결 시도 **Then** `session_revoked` 응답 수신, 로컬 토큰 삭제, 페어링 화면으로 복귀.

### 7.10 AC-MB-010 — 푸시 wake 후 재연결

**Given** Mobile background, WebSocket 끊김 **When** Bridge가 silent push 발송 **Then** 앱이 wake 후 3초 이내 WebSocket 재연결, 대기 중 outbound 메시지 replay.

### 7.11 AC-MB-011 — 백그라운드 geofence (옵션)

**Given** 위치 트리거 활성, "회사" geofence 등록 **When** Mobile이 회사 반경 진입 **Then** Bridge에 이벤트 전송, PC가 맞춤 알림 발송.

### 7.12 AC-MB-012 — iOS + Android 빌드

**Given** CI 매트릭스 **When** Fastlane + Gradle 빌드 **Then** iOS `.ipa` + Android `.aab` 아티팩트가 생성되고 TestFlight / Play Internal에 업로드.

---

## 8. TDD 전략

- **RED (RN 단위)**:
  - `__tests__/pairing.test.ts`: PairingScreen의 verify() 실패 케이스 (signature mismatch, expired)
  - `__tests__/journal-queue.test.ts`: 오프라인 → 온라인 FIFO 순서
  - `__tests__/bridge-client.test.ts`: reconnect 지수 백오프
- **RED (네이티브)**:
  - iOS: Keychain 저장/조회 유닛 (XCTest)
  - Android: Keystore + Biometric (JUnit)
- **GREEN**:
  - Bridge 호출은 `@connectrpc/connect` 클라이언트 mock으로 격리.
  - Push는 `@notifee/react-native` mock.
- **REFACTOR**:
  - zustand store 단일 진실원 (session, user, pending journals)
  - Suspense boundary로 로딩 상태 단순화
- **E2E (Detox)**:
  - 페어링 full flow
  - 메시지 전송 → 응답 수신
  - 오프라인 → 온라인 일기 동기화
- **커버리지**: TypeScript 85%+, 네이티브 코드 70%+.

---

## 9. 제외 항목 (Exclusions)

- **Mobile LLM 실행**: 1차 OFF. ExecuTorch LoRA는 별도 SPEC.
- **비디오 통화 / 스크린 공유**: v2+.
- **카카오톡 / 위챗 알림톡 실제 통합**: GATEWAY-001 전담. Mobile은 UI 토글만.
- **iPad / Android Tablet 전용 UI**: 1차는 phone layout만. 태블릿 최적화는 v2+.
- **Wear OS / Apple Watch**: v2+.
- **Offline-first full 기능**: Mobile은 PC 의존. Journal만 offline 허용.
- **Chromecast / AirPlay 미디어 송출**: scope 제외.
- **Desktop-less 단독 사용 모드**: 본 제품은 PC가 메인. Mobile 단독 실행 시 페어링 화면만 허용.
