package briefing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/messaging/telegram"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// stubSender captures calls to Send for assertion in tests.
type stubSender struct {
	calls  []telegram.SendRequest
	respFn func(req telegram.SendRequest) (*telegram.SendResponse, error)
}

func (s *stubSender) Send(_ context.Context, req telegram.SendRequest) (*telegram.SendResponse, error) {
	s.calls = append(s.calls, req)
	if s.respFn != nil {
		return s.respFn(req)
	}
	return &telegram.SendResponse{MessageID: 1, ChatID: req.ChatID}, nil
}

func sampleBriefingForTelegram() *BriefingPayload {
	return &BriefingPayload{
		GeneratedAt: time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC),
		Status: map[string]string{
			"weather": "ok",
			"journal": "ok",
			"date":    "ok",
			"mantra":  "ok",
		},
		Weather: WeatherModule{
			Current: &WeatherCurrent{
				Temp:      18.5,
				FeelsLike: 17.0,
				Condition: "Clear",
				Location:  "Seoul",
			},
			AirQuality: &AirQuality{AQI: 45, Level: "good"},
		},
		JournalRecall: RecallModule{
			Anniversaries: []*AnniversaryEntry{
				{YearsAgo: 1, Date: "2025-05-14", EmojiMood: "🙂"},
			},
			MoodTrend: &MoodTrend{Period: "7 days", Trend: "stable"},
		},
		DateCalendar: DateModule{
			Today:     "2026-05-14",
			DayOfWeek: "목요일",
		},
		Mantra: MantraModule{Text: "오늘도 한 걸음"},
	}
}

// TestRenderTelegram_StructureAndChatID covers AC-007 (rendering parity).
func TestRenderTelegram_StructureAndChatID(t *testing.T) {
	p := sampleBriefingForTelegram()
	req := RenderTelegram(p, 123456)

	if req.ChatID != 123456 {
		t.Errorf("ChatID = %d, want 123456", req.ChatID)
	}
	if req.ParseMode != telegram.ParseModeMarkdownV2 {
		t.Errorf("ParseMode = %s, want %s", req.ParseMode, telegram.ParseModeMarkdownV2)
	}

	want := []string{
		"MORNING BRIEFING",
		"Weather",
		"18.5°C",
		"Clear",
		"Seoul",
		"AQI: 45",
		"Journal Recall",
		"2025-05-14",
		"stable",
		"Date",
		"2026-05-14",
		"목요일",
		"Mantra",
		"오늘도 한 걸음",
	}
	for _, w := range want {
		if !strings.Contains(req.Text, w) {
			t.Errorf("Text missing %q\nfull text: %s", w, req.Text)
		}
	}
}

// TestRenderTelegram_OfflineWeather covers AC-002 propagation to telegram.
func TestRenderTelegram_OfflineWeather(t *testing.T) {
	p := sampleBriefingForTelegram()
	p.Weather.Offline = true
	p.Status["weather"] = "offline"

	text := RenderTelegramText(p)
	if !strings.Contains(text, "offline") {
		t.Errorf("expected offline marker, got: %s", text)
	}
}

// TestRenderTelegram_NoMantra omits the mantra section when text is empty.
func TestRenderTelegram_NoMantra(t *testing.T) {
	p := sampleBriefingForTelegram()
	p.Mantra.Text = ""

	text := RenderTelegramText(p)
	if strings.Contains(text, "✨ Mantra") {
		t.Errorf("expected no mantra section, got: %s", text)
	}
}

// TestRenderTelegram_NilPayload returns empty string.
func TestRenderTelegram_NilPayload(t *testing.T) {
	if got := RenderTelegramText(nil); got != "" {
		t.Errorf("RenderTelegramText(nil) = %q, want empty", got)
	}
}

// TestSendBriefingTelegram_Ok validates a successful send returns "ok"
// and reaches the sender exactly once.
func TestSendBriefingTelegram_Ok(t *testing.T) {
	stub := &stubSender{}
	cfg := TelegramChannelConfig{Token: "tkn", ChatID: 42}
	status, err := SendBriefingTelegram(context.Background(), stub, cfg, sampleBriefingForTelegram(), zap.NewNop())
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if status != "ok" {
		t.Errorf("status = %s, want ok", status)
	}
	if len(stub.calls) != 1 {
		t.Errorf("calls = %d, want 1", len(stub.calls))
	}
	if stub.calls[0].ChatID != 42 {
		t.Errorf("call chat_id = %d, want 42", stub.calls[0].ChatID)
	}
}

// TestSendBriefingTelegram_DisabledMissingToken covers EC-004
// (T-105 graceful disable).
func TestSendBriefingTelegram_DisabledMissingToken(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	stub := &stubSender{}
	cfg := TelegramChannelConfig{Token: "", ChatID: 42}
	status, err := SendBriefingTelegram(context.Background(), stub, cfg, sampleBriefingForTelegram(), logger)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if status != "disabled" {
		t.Errorf("status = %s, want disabled", status)
	}
	if len(stub.calls) != 0 {
		t.Errorf("expected no sender calls, got %d", len(stub.calls))
	}
	if recorded.Len() == 0 {
		t.Error("expected at least one warn log")
	}
	for _, e := range recorded.All() {
		for _, f := range e.Context {
			// chat_id raw value must never appear in log fields (REQ-BR-050).
			if f.Key == "chat_id" {
				t.Errorf("chat_id leaked in log: %+v", f)
			}
			if strings.Contains(f.String, "42") {
				t.Errorf("chat_id value 42 leaked in log: %+v", f)
			}
		}
	}
}

// TestSendBriefingTelegram_DisabledZeroChatID covers the zero-chat_id case.
func TestSendBriefingTelegram_DisabledZeroChatID(t *testing.T) {
	stub := &stubSender{}
	cfg := TelegramChannelConfig{Token: "tkn", ChatID: 0}
	status, err := SendBriefingTelegram(context.Background(), stub, cfg, sampleBriefingForTelegram(), zap.NewNop())
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if status != "disabled" {
		t.Errorf("status = %s, want disabled", status)
	}
}

// TestSendBriefingTelegram_DisabledNilSender covers the nil sender case.
func TestSendBriefingTelegram_DisabledNilSender(t *testing.T) {
	cfg := TelegramChannelConfig{Token: "tkn", ChatID: 42}
	status, err := SendBriefingTelegram(context.Background(), nil, cfg, sampleBriefingForTelegram(), zap.NewNop())
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if status != "disabled" {
		t.Errorf("status = %s, want disabled", status)
	}
}

// TestSendBriefingTelegram_NilPayload returns an error before invoking the sender.
func TestSendBriefingTelegram_NilPayload(t *testing.T) {
	stub := &stubSender{}
	cfg := TelegramChannelConfig{Token: "tkn", ChatID: 42}
	_, err := SendBriefingTelegram(context.Background(), stub, cfg, nil, zap.NewNop())
	if err == nil {
		t.Error("expected error for nil payload")
	}
	if len(stub.calls) != 0 {
		t.Errorf("expected no sender calls, got %d", len(stub.calls))
	}
}

// TestSendBriefingTelegram_SendError validates that sender errors are
// reported as status="error" and the raw chat_id is not leaked in logs.
func TestSendBriefingTelegram_SendError(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	stub := &stubSender{
		respFn: func(_ telegram.SendRequest) (*telegram.SendResponse, error) {
			return nil, errors.New("simulated timeout")
		},
	}
	cfg := TelegramChannelConfig{Token: "tkn", ChatID: 42}
	status, err := SendBriefingTelegram(context.Background(), stub, cfg, sampleBriefingForTelegram(), logger)
	if err == nil {
		t.Fatal("expected error from sender")
	}
	if status != "error" {
		t.Errorf("status = %s, want error", status)
	}
	for _, e := range recorded.All() {
		for _, f := range e.Context {
			if strings.Contains(f.String, "42") {
				t.Errorf("chat_id value 42 leaked in log: %+v", f)
			}
		}
	}
}

// TestClassifyError exercises the small classifier helper.
func TestClassifyError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{errors.New("telegram: unauthorized chat_id"), "unauthorized"},
		{errors.New("dial tcp: timeout"), "timeout"},
		{errors.New("context canceled"), "context"},
		{errors.New("boom"), "other"},
	}
	for _, c := range cases {
		got := classifyError(c.err)
		if got != c.want {
			t.Errorf("classifyError(%v) = %s, want %s", c.err, got, c.want)
		}
	}
}
