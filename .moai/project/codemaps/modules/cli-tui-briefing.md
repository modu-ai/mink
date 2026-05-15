# cli/tui/briefing_* — TUI Briefing Slash Wiring (SPEC-MINK-BRIEFING-001 M4)

**위치**: internal/cli/tui/briefing_*.go
**파일**: 5 production + 3 test
**최근 변경**: SPEC-MINK-BRIEFING-001 v0.3.1 M4 (PR #182 패널, PR #186 dispatch full-wire)
**상태**: ✅ Implemented (AC-008/013/014/015 GREEN)

---

## 목적

TUI 슬래시 `/briefing` 명령으로 BRIEFING-001 morning briefing을 비동기 실행 후 결과를 BriefingPanel로 화면에 렌더. CLI 채널과 동일한 4 module 데이터 + crisis prepend를 사용한다.

---

## 컴포넌트 맵

| 파일 | 역할 |
|------|------|
| `briefing_dispatch.go` | `BriefingRunner` interface + `briefingRunCmd` tea.Cmd (비동기 호출 wrap) + `BriefingResultMsg` 메시지 |
| `briefing_model_opts.go` | `Model.WithBriefingRunner` / `WithUserID` 함수형 옵션 + `handleBriefingResult` 메시지 핸들러 |
| `briefing_panel.go` | `BriefingPanel` 뷰 + `Render()` 메서드 (`internal/ritual/briefing.RenderTUI` 호출) |
| `slash.go` (브리핑 case) | `/briefing` 슬래시 → `briefingRunCmd` 디스패치 (v0.3.1 M4 추가) |
| `model.go` (BriefingResultMsg) | Update loop에서 `handleBriefingResult` 분기 |

테스트:
- `briefing_panel_test.go` — snapshot test (AC-008 GREEN)
- `briefing_panel_crisis_test.go` — crisis prefix 보장 (AC-015 TUI portion)
- `briefing_slash_test.go` — `/briefing` slash 디스패치 + `mockBriefingRunner` 기반 E2E

---

## 공개 API

### BriefingRunner (briefing_dispatch.go)

```go
// TUI 측이 briefing 패키지에 직접 의존하지 않도록 좁힌 interface.
// @MX:ANCHOR: BriefingRunner is the TUI-side interface for the briefing pipeline.
type BriefingRunner interface {
    Run(ctx context.Context, userID string, today time.Time) (*briefing.BriefingPayload, error)
}

type BriefingResultMsg struct {
    Payload *briefing.BriefingPayload
    Err     error
}

func briefingRunCmd(runner BriefingRunner, userID string) tea.Cmd
// 30s timeout, time.Now을 today로 전달, BriefingResultMsg로 응답
```

### Model Options (briefing_model_opts.go)

```go
func (m *Model) WithBriefingRunner(runner BriefingRunner) *Model
func (m *Model) WithUserID(userID string) *Model

// Update 분기 — 결과 도착 시 BriefingPanel 생성 후 m.briefingPanel에 보관
func (m *Model) handleBriefingResult(msg BriefingResultMsg) (tea.Model, tea.Cmd)
```

### BriefingPanel (briefing_panel.go)

```go
type BriefingPanel struct { payload *briefing.BriefingPayload }

func NewBriefingPanel(payload *briefing.BriefingPayload) *BriefingPanel
func (p *BriefingPanel) Render() string
// briefing.RenderTUI 호출 + PrependCrisisResponseIfDetected 적용
```

---

## 디스패치 흐름 (M4 wiring)

```
사용자 입력: /briefing
   │
   ▼
slash.go HandleSlashCmd
   │   case "briefing": return briefingRunCmd(m.briefingRunner, m.userID)
   ▼
briefingRunCmd (tea.Cmd, 비동기)
   │   ctx, cancel := context.WithTimeout(30s)
   │   payload, err := runner.Run(ctx, userID, time.Now())
   │   return BriefingResultMsg{Payload, Err}
   ▼
Model.Update receives BriefingResultMsg
   │   → handleBriefingResult
   ▼
NewBriefingPanel(payload).Render()
   │   (PrependCrisisResponseIfDetected 적용)
   ▼
View()에 panel 출력
```

---

## production wiring 진입점

```go
// cmd/mink 측 wiring 예시 (M4)
import "github.com/modu-ai/mink/internal/ritual/briefing"

orch := briefing.NewOrchestrator(weather, journal, date, mantra,
    briefing.WithConfig(cfg),
    briefing.WithLLMProvider(provider),
    briefing.WithLLMModel("gpt-4o-mini"),
)

m := tui.NewModel(...).
    WithBriefingRunner(orch).    // *briefing.Orchestrator implements BriefingRunner
    WithUserID(currentUserID)
```

Concrete `*briefing.Orchestrator`는 별도 어댑터 없이 `BriefingRunner` 시그너처를 충족한다 — Go의 구조적 타입 시스템이 자연스럽게 매핑.

---

## 핵심 invariants

1. **TUI는 briefing 패키지 internal 함수에 직접 의존 X** — `BriefingRunner` interface 통해서만 접근. test는 `mockBriefingRunner`로 격리.
2. **30s timeout**: TUI freeze 방지. 초과 시 BriefingResultMsg{Err: context.DeadlineExceeded} 도달, 패널이 에러 메시지로 fallback.
3. **Crisis prefix always**: BriefingPanel.Render는 PrependCrisisResponseIfDetected를 거치지 않고 호출되는 경로 없음 — `render_tui.go` 또는 panel 내부 어디서든 prepend 보장.
4. **Async only**: synchronous Runner 호출 금지. tea.Cmd로 wrap한 경로만 사용.

---

## 테스트 시나리오

| 테스트 | 보장 |
|--------|------|
| `TestBriefingPanel_Render` (panel_test.go) | snapshot equality — AC-008 GREEN |
| `TestBriefingPanel_CrisisPrepend` | 위기 키워드 포함 payload → hotline prefix 출력 |
| `TestBriefingPanel_CrisisAbsent` | 위기 키워드 없음 → prefix 없음 |
| `TestSlashBriefing_DispatchOK` (slash_test.go) | `/briefing` 입력 → BriefingResultMsg 수신 → panel 노출 |
| `TestSlashBriefing_DispatchError` | runner error → 에러 메시지 패널 |

---

## 참고

- 본 모듈 wiring은 `briefing` 패키지 자체에는 영향 없음 — 양방향 변경 비대칭 (briefing 패키지는 stable, TUI 측만 진화)
- briefing 패키지 본체: [ritual-briefing.md](./ritual-briefing.md)
- 자매 슬래시: `/journal` (ritual/journal), `/weather` 등은 향후 동일 패턴으로 확장 예정
