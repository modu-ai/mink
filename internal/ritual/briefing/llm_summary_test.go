package briefing

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/llm"
)

// stubLLMProvider records every Complete call for invariant 5 verification.
type stubLLMProvider struct {
	calls    []llm.CompletionRequest
	respText string
	err      error
}

func (s *stubLLMProvider) Name() string { return "stub" }

func (s *stubLLMProvider) Complete(_ context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	s.calls = append(s.calls, req)
	if s.err != nil {
		return llm.CompletionResponse{}, s.err
	}
	return llm.CompletionResponse{Text: s.respText, Model: req.Model}, nil
}

func (s *stubLLMProvider) Stream(_ context.Context, _ llm.CompletionRequest) (<-chan llm.Chunk, error) {
	return nil, errors.New("not implemented")
}

func (s *stubLLMProvider) CountTokens(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (s *stubLLMProvider) Capabilities(_ context.Context, _ string) (llm.Capabilities, error) {
	return llm.Capabilities{}, nil
}

func sampleBriefingForLLM() *BriefingPayload {
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
				Temp:      18.7,
				FeelsLike: 17.0,
				Condition: "Clear",
				Location:  "Seoul",
			},
			AirQuality: &AirQuality{AQI: 45, Level: "good"},
		},
		JournalRecall: RecallModule{
			Anniversaries: []*AnniversaryEntry{
				{YearsAgo: 1, Date: "2025-05-14", Text: "맛있는 저녁을 먹었다"},
				{YearsAgo: 3, Date: "2023-05-14", Text: "친구와 영화를 봤다"},
			},
			MoodTrend: &MoodTrend{Period: "7 days", Trend: "improving"},
		},
		DateCalendar: DateModule{
			Today:     "2026-05-14",
			DayOfWeek: "목요일",
			SolarTerm: &SolarTerm{Name: "입하", NameHanja: "立夏"},
		},
		Mantra: MantraModule{Text: "오늘도 한 걸음"},
	}
}

// TestBuildLLMSummaryRequest_CategoricalOnly validates AC-009 invariant 5:
// the request struct exposes ONLY categorical fields. We assert no entry
// text / mantra text / precise coordinates can leak through.
func TestBuildLLMSummaryRequest_CategoricalOnly(t *testing.T) {
	p := sampleBriefingForLLM()
	req := BuildLLMSummaryRequest(p)
	if req == nil {
		t.Fatal("request nil")
	}

	// Marshal to JSON and inspect the encoded payload as the LLM would see it.
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	encoded := string(b)

	forbidden := []string{
		"맛있는 저녁을 먹었다", // entry text 1
		"친구와 영화를 봤다",  // entry text 2
		"오늘도 한 걸음",    // mantra text
		"18.7",        // raw float coord/temp
	}
	for _, f := range forbidden {
		if strings.Contains(encoded, f) {
			t.Errorf("LLM request leaked forbidden token %q (invariant 5 violation)\nencoded: %s", f, encoded)
		}
	}

	allowed := []string{
		"2026-05-14",
		"목요일",
		"Clear",
		"Seoul",
		"good",
		"\"anniversary_count\":2",
		"\"mood_trend_slope\":1",
		"\"mantra_present\":true",
		"\"solar_term_present\":true",
	}
	for _, a := range allowed {
		if !strings.Contains(encoded, a) {
			t.Errorf("LLM request missing categorical signal %q\nencoded: %s", a, encoded)
		}
	}
}

// TestBuildLLMSummaryRequest_NilPayload returns nil safely.
func TestBuildLLMSummaryRequest_NilPayload(t *testing.T) {
	if got := BuildLLMSummaryRequest(nil); got != nil {
		t.Errorf("BuildLLMSummaryRequest(nil) = %+v, want nil", got)
	}
}

// TestBuildLLMSummaryRequest_DegradedStatus handles missing modules.
func TestBuildLLMSummaryRequest_DegradedStatus(t *testing.T) {
	p := &BriefingPayload{
		Status: map[string]string{"weather": "error", "journal": "skipped"},
		DateCalendar: DateModule{
			Today:     "2026-05-14",
			DayOfWeek: "목요일",
		},
	}
	req := BuildLLMSummaryRequest(p)
	if req.WeatherStatus != "error" {
		t.Errorf("WeatherStatus = %s, want error", req.WeatherStatus)
	}
	if req.JournalStatus != "skipped" {
		t.Errorf("JournalStatus = %s, want skipped", req.JournalStatus)
	}
	if req.MantraPresent {
		t.Error("MantraPresent should be false when text is empty")
	}
	if req.SolarTermPresent || req.HolidayPresent {
		t.Error("calendar flags should be false when nil")
	}
}

// TestFormatLLMPrompt_StructureAndAbsence verifies the prompt template can
// never include entry text or mantra raw text.
func TestFormatLLMPrompt_StructureAndAbsence(t *testing.T) {
	p := sampleBriefingForLLM()
	prompt := FormatLLMPrompt(BuildLLMSummaryRequest(p))

	if prompt == "" {
		t.Fatal("prompt empty")
	}

	forbidden := []string{
		"맛있는 저녁을 먹었다",
		"친구와 영화를 봤다",
		"오늘도 한 걸음",
		"18.7",
	}
	for _, f := range forbidden {
		if strings.Contains(prompt, f) {
			t.Errorf("prompt leaked %q: %s", f, prompt)
		}
	}
}

func TestFormatLLMPrompt_NilRequest(t *testing.T) {
	if got := FormatLLMPrompt(nil); got != "" {
		t.Errorf("FormatLLMPrompt(nil) = %q, want empty", got)
	}
}

// TestGenerateLLMSummary_DisabledByConfig validates M1/M2 default behavior:
// when cfg.LLMSummary is false, no provider call occurs.
func TestGenerateLLMSummary_DisabledByConfig(t *testing.T) {
	stub := &stubLLMProvider{respText: "should not be called"}
	cfg := &Config{Mantra: "ok", LLMSummary: false}

	got, err := GenerateLLMSummary(context.Background(), stub, sampleBriefingForLLM(), cfg, "model-a")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "" {
		t.Errorf("got = %q, want empty when LLMSummary=false", got)
	}
	if len(stub.calls) != 0 {
		t.Errorf("expected 0 provider calls, got %d", len(stub.calls))
	}
}

// TestGenerateLLMSummary_NilProvider returns empty without error (degraded).
func TestGenerateLLMSummary_NilProvider(t *testing.T) {
	cfg := &Config{Mantra: "ok", LLMSummary: true}
	got, err := GenerateLLMSummary(context.Background(), nil, sampleBriefingForLLM(), cfg, "model-a")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "" {
		t.Errorf("got = %q, want empty when provider=nil", got)
	}
}

// TestGenerateLLMSummary_HappyPath_RequestInvariant5 is the integrated
// AC-009 invariant 5 check: end-to-end provider call must never see
// forbidden tokens.
func TestGenerateLLMSummary_HappyPath_RequestInvariant5(t *testing.T) {
	stub := &stubLLMProvider{respText: "오늘은 맑고 가볍게 시작하기 좋은 목요일.\n천천히 호흡하며 한 걸음씩."}
	cfg := &Config{Mantra: "ok", LLMSummary: true}

	got, err := GenerateLLMSummary(context.Background(), stub, sampleBriefingForLLM(), cfg, "qwen2.5:3b")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got == "" {
		t.Fatal("got empty summary")
	}
	if len(stub.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(stub.calls))
	}

	call := stub.calls[0]
	if call.Model != "qwen2.5:3b" {
		t.Errorf("model = %s, want qwen2.5:3b", call.Model)
	}
	if len(call.Messages) < 2 {
		t.Fatalf("messages len = %d, want >= 2", len(call.Messages))
	}

	all := ""
	for _, m := range call.Messages {
		all += m.Content + "\n"
	}
	forbidden := []string{
		"맛있는 저녁을 먹었다",
		"친구와 영화를 봤다",
		"오늘도 한 걸음",
		"18.7",
	}
	for _, f := range forbidden {
		if strings.Contains(all, f) {
			t.Errorf("LLM messages leaked %q (invariant 5 violation):\n%s", f, all)
		}
	}
}

// TestGenerateLLMSummary_ProviderError surfaces the error.
func TestGenerateLLMSummary_ProviderError(t *testing.T) {
	stub := &stubLLMProvider{err: errors.New("upstream boom")}
	cfg := &Config{Mantra: "ok", LLMSummary: true}

	got, err := GenerateLLMSummary(context.Background(), stub, sampleBriefingForLLM(), cfg, "m")
	if err == nil {
		t.Fatal("expected provider error")
	}
	if got != "" {
		t.Errorf("got = %q on error, want empty", got)
	}
}
