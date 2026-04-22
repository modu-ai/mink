# SPEC-GOOSE-DESKTOP-001 — Research Notes

> 본 문서는 spec.md 의존 컨텍스트를 심층 분석한다. Tauri v2 채택 근거, Claude Code UI 패턴 흡수 범위, ROADMAP v4.0 Phase 6 재편 근거를 정리한다.

---

## 1. ROADMAP v4.0 Phase 6 재편 근거

### 1.1 변경 이전 (v3.0)

- Phase 6: Deep Personalization (IDENTITY-001, VECTOR-001, LORA-001) — 3 SPEC, P2, 디지털 쌍둥이.

### 1.2 변경 이후 (v4.0)

- **Phase 6 (신규)**: Cross-Platform Clients — 5 SPEC (DESKTOP-001, BRIDGE-001, RELAY-001, MOBILE-001, GATEWAY-001).
- **Phase 7**: 기존 Phase 5(Promotion & Safety) 유지.
- **Phase 8**: 기존 Phase 6(Deep Personalization) 이동.

### 1.3 재편 동기

사용자 최종 확정(2026-04-22):

> "CLI가 아닌 데스크탑 앱으로 모바일 앱으로 항상 함께 할 수 있도록 하자."

이 지시는 GOOSE의 **표면(Surface) 전략**을 근본적으로 바꾼다. Deep Personalization은 "내부(깊이)"이며, Cross-Platform Clients는 "표면(넓이)"이다. 표면 없이 깊이만 있으면 사용자는 GOOSE를 만날 수 없다. 따라서 표면 구축이 먼저 와야 한다.

### 1.4 기존 SPEC-GOOSE-CLI-001과의 관계

CLI-001은 폐기하지 **않는다**. 재정의한다:

| 이전 (v3.0) | 이후 (v4.0) |
|------------|------------|
| GOOSE의 **기본 UI** | GOOSE의 **개발자/헤드리스 UI** |
| 모든 사용자가 설치 | 개발자·CI·서버 환경만 설치 |
| Ink 기반 풀 기능 | 기능 축소 (core chat + debug) |

---

## 2. Claude Code bridge/ → GOOSE Desktop 매핑 (일부)

bridge/ 33 파일 중 **Desktop UI와 직접 관련된** 파일만 추려 매핑(나머지는 BRIDGE-001에서 전담):

| Claude Code bridge/ 파일 | GOOSE Desktop 매핑 | 비고 |
|----------------------|-------------------|-----|
| `bridgeUI.ts` | `packages/goose-desktop/src/bridge/BridgeStatusBadge.tsx` | 페어링 상태 표시 |
| `bridgeDebug.ts` | Preferences > Developer > Bridge Debug Panel | 개발자 모드에서만 노출 |
| `bridgePermissionCallbacks.ts` | `src/bridge/PermissionDialog.tsx` | Mobile이 권한 요청할 때 Desktop에서 승인 UI |

---

## 3. Claude Code src/ui/, src/screens/, src/panels/ 패턴 흡수

Claude Code의 146개 UI 컴포넌트 중 Desktop 1차 릴리스에 이식할 대상:

### 3.1 Screens (풀스크린 뷰)

| Claude Code | GOOSE 대응 |
|------------|-----------|
| `src/screens/ChatScreen.tsx` | `src/screens/ChatScreen.tsx` (React 19) |
| `src/screens/ProjectScreen.tsx` | `src/screens/ProjectScreen.tsx` |
| `src/screens/PreferencesScreen.tsx` | `src/screens/PreferencesScreen.tsx` |

### 3.2 Panels (영역 뷰)

| Claude Code | GOOSE 대응 |
|------------|-----------|
| `src/panels/MessageList.tsx` | `src/panels/MessageList.tsx` |
| `src/panels/InputArea.tsx` | `src/panels/InputArea.tsx` (멀티라인, 첨부) |
| `src/panels/ToolOutputPanel.tsx` | `src/panels/ToolOutputPanel.tsx` |

### 3.3 신규 (Claude Code에 없음)

- `src/panels/RitualDashboard.tsx` — GOOSE 고유. product.md 정의.
- `src/panels/GrowthMeters.tsx` — GOOSE 고유.
- `src/components/MoodIndicator.tsx` — 트레이 + 헤더 mood 표시.

---

## 4. Tauri v2 채택 근거

### 4.1 Electron vs Tauri v2 비교

| 항목 | Electron | Tauri v2 |
|-----|----------|----------|
| 번들 크기 | ~150MB (Chromium 포함) | ~10MB (OS WebView 사용) |
| 메모리 (idle) | ~200MB | ~40MB |
| 시작 시간 | ~1.5s | ~0.3s |
| 보안 | Node 풀 액세스 | Rust IPC 경계 + capability |
| 생태계 | 성숙 | v2 안정화 완료 (2025.10+) |
| auto-update | 타사 플러그인 | 내장 `tauri-plugin-updater` |

tech.md §1.2는 UI 레이어를 TypeScript + Tauri로 고정. 일관성 유지.

### 4.2 Tauri v2 핵심 기능 사용

- **Window Plugin**: 멀티 창, 창 상태 저장/복원.
- **Tray Plugin**: 시스템 트레이 아이콘, 동적 메뉴.
- **Global Shortcut Plugin**: OS 레벨 단축키 등록.
- **Notification Plugin**: OS 네이티브 알림.
- **Updater Plugin**: 서명 기반 자동 업데이트(ed25519).
- **IPC Commands**: Rust backend ↔ JS frontend 타입 안전 호출.

### 4.3 Rust backend 책임

- `goosed` 프로세스 spawn/kill (tokio::process)
- ed25519 서명 검증 (ring crate)
- OS accessibility API 직접 접근(Tauri의 Rust 생태계와 goose-desktop Rust crate 공유)
- 안전한 secret 저장 (Keychain / Credential Manager / Secret Service)

---

## 5. 다국어(i18n) 전략

| 언어 | 우선순위 | 담당 번역 |
|-----|--------|----------|
| en | 기본 | 코드 작성자 |
| ko | 1차 | 한국어 네이티브 리뷰 |
| ja | 2차 | 커뮤니티 기여 |
| zh | 2차 | 커뮤니티 기여 |

i18next + react-i18next. 리소스는 `packages/goose-desktop/src/locales/{lang}/*.json`.

---

## 6. TDD 전략

### 6.1 RED (실패 테스트 먼저)

```typescript
// desktop/tests/bootstrap.test.ts
describe("DesktopApp.bootstrap", () => {
  it("spawns goosed if not running", async () => {
    const app = new DesktopApp({ portCheck: () => false });
    await app.bootstrap();
    expect(mockSpawn).toHaveBeenCalledWith("goosed", expect.any(Array));
  });
  it("skips spawn if goosed already running", async () => { /* ... */ });
});
```

### 6.2 GREEN (최소 구현)

Tauri command `bootstrap_daemon`에서 port 스캔 후 필요시 spawn. 검증은 `verify_daemon_signature`로 gate.

### 6.3 REFACTOR

- 상태 관리: zustand store로 단일 진실원 (mood, connection state, current chat).
- 에러 경계: React ErrorBoundary + Tauri event bus.

### 6.4 커버리지 목표

- TypeScript: 85%+ (Vitest)
- Rust backend: 80%+ (cargo test)
- E2E: Playwright 핵심 플로우 5종 (launch, chat, shortcut, tray, update)

---

## 7. 오픈 이슈

1. **Tauri v2 updater ed25519 공개키 분배 방식**: 소스 임베드 vs 빌드 시 주입 — CI 서명 워크플로우와 조율 필요.
2. **Linux AppImage vs Flatpak 우선순위**: AppImage는 배포 간편, Flatpak은 샌드박스 강력. 1차 AppImage만.
3. **Windows 코드사인**: EV 인증서 구매 여부 — 초기 커뮤니티 서명으로 시작, SmartScreen warning은 릴리스 노트 고지.
4. **macOS notarization**: Apple Developer Program 필수. 비용과 시간 리스크.
5. **`goosed` 바이너리 번들 전략**: Desktop 앱 안에 포함 vs 별도 다운로드. 1차는 포함(사용자 경험 우선), 업데이트 시 분리 고려.
6. **채팅 히스토리 저장 위치**: Desktop 로컬 SQLite vs `goosed` 중앙 저장. 2차는 goosed 중앙 + Desktop 캐시.

---

## 8. 참조

- ROADMAP v4.0 §4 Phase 6 (본 변경 포함)
- structure.md §1 `packages/goose-desktop/`, §4.2
- tech.md §1.2 (TypeScript UI 10%), §4.2 (Tauri v2)
- product.md (Rituals, Growth Meters, GOOSE mood)
- Claude Code `src/ui/`, `src/screens/`, `src/panels/` (146 components)
- Tauri v2 docs: <https://v2.tauri.app>
