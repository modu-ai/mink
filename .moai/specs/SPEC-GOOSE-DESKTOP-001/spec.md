---
id: SPEC-GOOSE-DESKTOP-001
version: 0.1.0
status: planned
created_at: 2026-04-21
updated_at: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 6
size: 대(L)
lifecycle: spec-anchored
labels: []
---

# SPEC-GOOSE-DESKTOP-001 — GOOSE Desktop App (기본 UI, Tauri v2)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | ROADMAP v4.0 Phase 6 신규 Phase "Cross-Platform Clients" 추가에 따른 초안. 기존 Phase 6(Deep Personalization)은 Phase 8로 이동. 사용자 최종 확정 컨셉(2026-04-22) "PC가 메인, 모바일은 원격 클라이언트"에 따라 Desktop App을 GOOSE의 **기본 UI**로 정의. | manager-spec |

---

## 1. 개요 (Overview)

사용자 최종 지시(2026-04-22):

> "CLI가 아닌 데스크탑 앱으로 모바일 앱으로 항상 함께 할 수 있도록 하자. 기본 설치는 pc이지만 모바일 클라우드 연동으로 앱에서 pc를 제어 또는 지시를 할 수가 있다."

본 SPEC은 GOOSE의 **기본 사용자 인터페이스**를 CLI가 아닌 **Desktop App**으로 재정의한다. GOOSE는 설치 시 `goosed` daemon + Desktop App이 **기본 구성**이며, CLI(SPEC-GOOSE-CLI-001)는 개발/디버그/스크립팅 전용으로 유지된다. Desktop App은 사용자 PC에 상주하면서 채팅 UI, Rituals 대시보드, Growth Meters, 시스템 트레이 통합을 제공한다.

기술 기반은 Tauri v2(Rust backend + React/TypeScript frontend)이며, Claude Code의 146개 UI 컴포넌트 패턴(`src/ui/`, `src/screens/`, `src/panels/`)을 직접 흡수한다. 본 SPEC은 UI 껍데기(chrome)를 정의하고, 실제 세션·QueryEngine 연결은 TRANSPORT-001 gRPC를 경유한다.

---

## 2. 배경 (Background)

### 2.1 왜 Desktop App이 기본인가

- **상시 동반성**: GOOSE는 "평생 동반자" 콘셉트(product.md)로, 사용자와 항상 함께하려면 터미널이 아닌 OS 레벨에서 상주해야 한다. 시스템 트레이, 전역 단축키, 푸시 알림은 CLI로 제공 불가.
- **Claude Code 관찰**: Claude Code는 터미널 네이티브이지만, bridge/(33 파일) 패턴을 통해 모바일·웹 원격 세션을 명시적으로 지원. 즉 CLI만으로는 불충분하다는 업계 합의.
- **Rituals & Meters**: product.md가 정의한 아침 브리핑, 저녁 일기, Growth Meters는 그래픽 UI가 필수. 터미널로는 시각적 임팩트가 제한됨.
- **경쟁사 벤치마크**: ChatGPT Desktop, Claude Desktop, Raycast, Alfred 모두 OS 레벨 앱으로 진화. GOOSE가 CLI에 머물면 일반 사용자 접근성 결여.

### 2.2 왜 Tauri v2인가

- **Go ↔ UI 분리 원칙**: tech.md §1.2는 UI 레이어를 TypeScript로 고정. gRPC로 `goosed`와 통신.
- **경량성**: Electron 대비 번들 크기 ~10배 작음(수십 MB vs 수백 MB), 메모리 사용량 3-5배 적음.
- **네이티브 Rust backend**: 시스템 트레이, 전역 단축키, 자동 업데이트, 파일 시스템 권한 등을 Rust(tauri::api)로 처리. goose-crypto(Rust)와 동일 생태계.
- **크로스 플랫폼**: macOS(x86/arm), Linux(x86/arm), Windows — 5 target 자동 지원.
- **Auto-update**: Tauri updater plugin 내장(서명 검증 포함).

### 2.3 CLI와의 관계

- SPEC-GOOSE-CLI-001은 **유지하되 재정의**: Desktop이 기본, CLI는 개발자·CI·헤드리스 환경 전용.
- 둘 다 동일한 `goosed` gRPC 서버에 연결(TRANSPORT-001).
- Desktop App이 없을 때(서버, 도커, SSH)도 CLI로 동일 기능 접근 가능 → Desktop은 **필수 아닌 편의**.

### 2.4 기존 v4.0 structure.md와의 정합성

- structure.md §1 `packages/goose-desktop/` 존재(Tauri v2). 본 SPEC은 그 패키지의 **행동 계약**을 확정.
- structure.md §4.2 "Desktop App (Tauri v2)" 명시. 본 SPEC은 그 설계를 요구사항 수준으로 구체화.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `packages/goose-desktop/` TypeScript 패키지(Tauri v2 프로젝트 스캐폴드).
2. Tauri Rust backend(`src-tauri/`): window manager, system tray, global shortcut, auto-updater, IPC command invocations.
3. React 19 + TypeScript frontend:
   - 메인 창(MainWindow): 채팅 UI(Claude Code `src/screens/` 참고), 좌측 Rituals 사이드바, 우측 Growth Meters 패널.
   - 시스템 트레이 메뉴(열기/숨기기/종료/GOOSE mood 표시).
   - 전역 단축키(⌘K/Ctrl+K): 창 토글.
   - 멀티 창 지원: 프로젝트별 별도 창, Preferences 창.
   - 설정 화면(Preferences): LLM provider, 언어, 테마, 자동시작.
4. `goosed` gRPC 클라이언트 통합(@connectrpc/connect 2.x), proto는 TRANSPORT-001 재사용.
5. 자동 daemon 부트스트랩: Desktop 실행 시 `goosed` 프로세스 spawn(이미 실행 중이면 skip), 종료 시 graceful stop.
6. 다크 모드(시스템 설정 추적 + 수동 오버라이드).
7. i18n: ko/en/ja/zh 4개 언어, i18next 기반.
8. 앱 아이콘 + 시스템 트레이 아이콘(GOOSE mood 4 상태: calm/active/learning/alert).
9. 패키징: macOS `.dmg` + 코드사인, Linux `.deb`/`.rpm`/AppImage, Windows `.msi` + 코드사인.
10. Auto-update: Tauri updater plugin, GitHub Releases feed, 서명 검증(ed25519).

### 3.2 OUT OF SCOPE

- Mobile 페어링 QR 코드 생성(BRIDGE-001 §3 참조. Desktop은 QR 표시만, 생성 로직은 BRIDGE-001).
- 실제 채팅 스트리밍 구현(QUERY-001 계약 소비만).
- 플러그인 마켓플레이스 UI(PLUGIN-001 별도 SPEC).
- Voice input / wake word (Desktop 1차 릴리스 out; MOBILE-001이 우선).
- Store 등록(MAS/MS Store/Snap): 릴리스 자동화는 v1.0 이후.

---

## 4. 의존성 (Dependencies)

- **Phase 6(신규) 상위 의존**: SPEC-GOOSE-TRANSPORT-001(gRPC proto), SPEC-GOOSE-QUERY-001(채팅 세션 스트리밍), SPEC-GOOSE-CLI-001(참조: 동일 gRPC 계약).
- **동일 Phase 6 하위 의존**: SPEC-GOOSE-BRIDGE-001(Mobile pairing QR 코드 수신), SPEC-GOOSE-MOBILE-001(pairing 상대).
- **라이브러리**:
  - `tauri` 2.x (Rust backend framework)
  - `tauri-plugin-updater` 2.x, `tauri-plugin-global-shortcut` 2.x, `tauri-plugin-notification` 2.x
  - `react` 19.x, `@connectrpc/connect` 2.x, `zustand` 5.x (state), `tailwindcss` 4.x, `shadcn/ui`, `framer-motion` 11.x
  - `i18next` 23.x + `react-i18next` 14.x

---

## 5. 요구사항 (EARS Requirements)

### 5.1 Ubiquitous

- **REQ-DK-001**: The Desktop App **shall** bundle a `goosed` daemon binary and start it automatically on launch when no daemon is detected on the configured gRPC port.
- **REQ-DK-002**: The Desktop App **shall** display a system tray icon that reflects GOOSE's current mood (calm, active, learning, alert).
- **REQ-DK-003**: All user-facing strings **shall** be externalized and available in ko / en / ja / zh language packs.
- **REQ-DK-004**: The Desktop App **shall** support dark mode following the OS preference, with a manual override stored in user preferences.

### 5.2 Event-Driven

- **REQ-DK-005**: **When** the user presses the configured global shortcut (default ⌘K on macOS, Ctrl+K elsewhere), the main window **shall** toggle between focused-front and hidden-to-tray states.
- **REQ-DK-006**: **When** the Desktop App receives a notification event from `goosed` over gRPC (morning briefing, ritual reminder, proactive suggestion), it **shall** dispatch an OS-native notification via `tauri-plugin-notification`.
- **REQ-DK-007**: **When** the auto-updater detects a newer signed release on the configured feed, the Desktop App **shall** prompt the user with release notes before downloading.
- **REQ-DK-008**: **When** the main window closes, the Desktop App **shall** minimize to system tray rather than exiting the process (unless the user explicitly selects "Quit").

### 5.3 State-Driven

- **REQ-DK-009**: **While** the gRPC connection to `goosed` is disconnected, the Desktop App **shall** show a reconnecting banner at the top of the main window and retry with exponential backoff (1s, 2s, 4s, capped at 30s).
- **REQ-DK-010**: **While** `goosed` is streaming a chat response, the Desktop App **shall** render partial chunks progressively and display a stop-generation control in the input area.

### 5.4 Optional

- **REQ-DK-011**: **Where** the host OS supports biometric authentication (Touch ID / Windows Hello), the Desktop App **shall** offer to gate Preferences and Trusted Device management behind biometrics.
- **REQ-DK-012**: **Where** the host OS provides a native menu bar (macOS), the Desktop App **shall** expose File / Edit / View / Window / Help menus with standard shortcuts.

### 5.5 Unwanted Behavior

- **REQ-DK-013**: **If** the bundled `goosed` binary signature does not match the expected public key, **then** the Desktop App **shall not** launch the daemon and **shall** display a tamper-warning dialog.
- **REQ-DK-014**: **If** an auto-update package fails signature verification, **then** the Desktop App **shall not** apply the update and **shall** report the failure to the user without rollback of the running version.

### 5.6 Complex

- **REQ-DK-015**: **While** the user is in an active chat session, **when** the user presses ⌘W (macOS) or Ctrl+W (other), the Desktop App **shall** prompt the user to confirm that the in-progress response will be cancelled before closing the window.

---

## 6. 핵심 타입 (Go/TypeScript 시그니처)

본 SPEC은 UI 계층이므로 타입은 주로 TypeScript + Rust(Tauri command). Go 시그니처는 TRANSPORT-001에서 이미 정의된 gRPC 스텁을 소비.

### 6.1 TypeScript (frontend)

```typescript
// packages/goose-desktop/src/types.ts

// 메인 애플리케이션 상태 컨트롤러
export interface DesktopApp {
  bootstrap(): Promise<void>;              // daemon 부트스트랩 + gRPC 연결
  shutdown(): Promise<void>;                // graceful 종료
  getMood(): GooseMood;                     // 현재 mood (트레이 아이콘용)
}

// 메인 창
export interface MainWindow {
  show(): Promise<void>;
  hide(): Promise<void>;
  toggle(): Promise<void>;
  focus(): Promise<void>;
  openProject(projectId: string): Promise<void>;   // 프로젝트별 창
}

// 시스템 트레이
export interface SystemTray {
  setMood(mood: GooseMood): Promise<void>;         // 아이콘 갱신
  setMenu(items: TrayMenuItem[]): Promise<void>;   // 동적 메뉴
}

// GOOSE mood (트레이 아이콘 4상태)
export type GooseMood = "calm" | "active" | "learning" | "alert";

// Rituals 사이드바
export interface RitualDashboard {
  listTodayRituals(): Promise<Ritual[]>;
  markComplete(ritualId: string): Promise<void>;
}

// Growth Meters (성장 지표 패널)
export interface GrowthMeters {
  getCurrentMetrics(): Promise<MeterSnapshot>;
  subscribe(cb: (snap: MeterSnapshot) => void): Unsubscribe;
}

// 전역 단축키
export interface GlobalShortcut {
  register(accelerator: string, handler: () => void): Promise<void>;
  unregister(accelerator: string): Promise<void>;
}
```

### 6.2 Rust (Tauri backend)

```rust
// packages/goose-desktop/src-tauri/src/lib.rs
// GOOSE Desktop Tauri 진입점. goosed 부트스트랩과 OS 통합 담당.

#[tauri::command]
async fn bootstrap_daemon(app: tauri::AppHandle) -> Result<u32, String> {
    // goosed 프로세스 spawn, PID 반환
}

#[tauri::command]
async fn verify_daemon_signature(binary_path: String) -> Result<bool, String> {
    // ed25519 서명 검증 (REQ-DK-013)
}
```

---

## 7. 수락 기준 (Acceptance Criteria)

### 7.1 AC-DK-001 — Daemon 자동 부트스트랩

**Given** 시스템에 `goosed`가 실행 중이지 않고 **When** 사용자가 Desktop App을 실행 **Then** 5초 이내에 `goosed`가 spawn되고 gRPC `Ping`이 성공하며 메인 창이 표시된다.

### 7.2 AC-DK-002 — 시스템 트레이 mood 반영

**Given** Desktop이 실행 중 **When** 세션 상태가 learning → active로 변경 **Then** 트레이 아이콘이 1초 이내에 대응 mood 아이콘으로 전환된다.

### 7.3 AC-DK-003 — 전역 단축키 토글

**Given** 메인 창이 숨김 상태 **When** 사용자가 ⌘K 누름 **Then** 메인 창이 최상단 포커스로 표시된다. **When** 다시 ⌘K 누름 **Then** 트레이로 숨겨진다.

### 7.4 AC-DK-004 — 다국어 전환

**Given** 현재 언어가 en **When** Preferences에서 ko로 변경 **Then** 창 재시작 없이 모든 UI 문자열이 ko로 갱신된다.

### 7.5 AC-DK-005 — 다크 모드 추적

**Given** OS가 다크 모드 **When** Desktop 실행 **Then** 즉시 다크 테마가 적용된다. **When** 사용자가 라이트 오버라이드 설정 **Then** OS 상태와 무관하게 라이트 유지.

### 7.6 AC-DK-006 — gRPC 재연결

**Given** Desktop이 `goosed`와 연결됨 **When** daemon을 강제 종료 **Then** 연결 배너가 즉시 표시되고 1s → 2s → 4s 재시도 순서로 재연결을 시도한다.

### 7.7 AC-DK-007 — 스트리밍 응답 렌더링

**Given** 사용자가 질문 입력 **When** `goosed`가 chunk를 스트리밍 **Then** 청크 도착 간격 ≤50ms 로 UI가 점진적으로 업데이트되고 stop 버튼이 노출된다.

### 7.8 AC-DK-008 — 창 닫기 → 트레이

**Given** 메인 창이 표시됨 **When** 사용자가 창 닫기 버튼 클릭 **Then** 창이 숨겨지고 트레이 아이콘은 유지된다. 프로세스는 종료되지 않는다.

### 7.9 AC-DK-009 — Auto-update 서명 검증

**Given** 조작된 업데이트 패키지가 서버에서 전달 **When** Desktop이 다운로드 후 검증 **Then** 서명 실패 메시지가 표시되고 업데이트가 적용되지 않으며 현재 버전은 계속 실행된다.

### 7.10 AC-DK-010 — 변조된 daemon 거부

**Given** `goosed` 바이너리가 교체되어 서명 불일치 **When** Desktop 실행 **Then** tamper-warning 다이얼로그가 표시되고 daemon은 spawn되지 않는다.

### 7.11 AC-DK-011 — 생성 중 창 닫기 확인

**Given** 채팅 응답이 스트리밍 중 **When** 사용자가 ⌘W 누름 **Then** "응답 생성이 취소됩니다. 계속할까요?" 확인 다이얼로그가 표시된다.

### 7.12 AC-DK-012 — 크로스 플랫폼 빌드

**Given** CI 매트릭스가 실행 **When** `tauri build` 완료 **Then** macOS(x64/arm64), Linux(x64/arm64), Windows(x64) 5개 플랫폼 아티팩트가 모두 생성되고 서명된다.

---

## 8. TDD 전략

- **RED**: Tauri command mock + Vitest로 상태 관리(zustand store)와 gRPC 클라이언트 스텁부터 실패 테스트.
- **GREEN**: Rust backend는 `cargo test`로 sign verification/shortcut register 단위 테스트. Frontend는 Playwright로 E2E.
- **REFACTOR**: shadcn/ui 컴포넌트로 중복 제거. Storybook 도입(선택).
- 커버리지: TypeScript 85%+, Rust 80%+.

---

## 9. 제외 항목 (Exclusions)

- **Voice input / wake word**: MOBILE-001 우선 적용. Desktop은 1차 out.
- **QR 코드 생성 로직**: BRIDGE-001 §4에서 정의. Desktop은 렌더링만 담당.
- **Store 등록(MAS/MS Store/Snap)**: v1.0 이후 별도 릴리스 자동화 SPEC.
- **플러그인 설치 UI**: PLUGIN-001이 manifest 로더 제공. Desktop은 통합 후속.
- **Agency 파이프라인 UI**: Agency Constitution 별도 ROADMAP 소관.
- **실시간 음성 대화(full-duplex)**: Phase 8+ 이후 별도 SPEC.
