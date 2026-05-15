// Package onboarding — flow_test.go covers the OnboardingFlow state machine.
// Tests follow the TDD order from SPEC-MINK-ONBOARDING-001 §6.8 (RED #1, #3, #10 and extensions).
// All tests use stdlib testing only; no third-party assertion libraries.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2, §6.8
// AC: AC-OB-001 (partial), AC-OB-006 (partial), AC-OB-025 (partial)
package onboarding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
	"time"
)

// hexPattern matches a 32-character lowercase hex string.
var hexPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// --- §6.8 RED #1 — TestStartFlow_StartsAtStep1 ---

// TestStartFlow_StartsAtStep1 verifies AC-OB-001: a fresh flow starts at step 1
// with a valid SessionID, a recent StartedAt, and zero-valued OnboardingData.
func TestStartFlow_StartsAtStep1(t *testing.T) {
	before := time.Now().UTC()
	f, err := StartFlow(context.Background(), nil)
	after := time.Now().UTC()

	if err != nil {
		t.Fatalf("StartFlow returned error: %v", err)
	}
	if f.CurrentStep != 1 {
		t.Errorf("CurrentStep = %d, want 1", f.CurrentStep)
	}
	if !hexPattern.MatchString(f.SessionID) {
		t.Errorf("SessionID %q does not match 32-hex pattern", f.SessionID)
	}
	if f.StartedAt.Before(before) || f.StartedAt.After(after) {
		t.Errorf("StartedAt %v outside expected window [%v, %v]", f.StartedAt, before, after)
	}
	if f.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nil", f.CompletedAt)
	}
	// Data must be zero-valued (no locale prefill when locale arg is nil).
	// Direct comparison is not possible because LocaleChoice contains a slice field;
	// instead verify key scalar fields are at their zero values.
	if f.Data.Locale.Country != "" || f.Data.Persona.Name != "" ||
		f.Data.Model.OllamaInstalled || f.Data.Provider.Provider != "" {
		t.Errorf("Data is not zero-valued: %+v", f.Data)
	}
}

// TestStartFlow_WithLocale_PrefillsLocaleChoice verifies that passing a non-nil locale
// prefills Data.Locale from the caller's value (LOCALE-001 forward reference).
func TestStartFlow_WithLocale_PrefillsLocaleChoice(t *testing.T) {
	locale := &LocaleChoice{
		Country:    "KR",
		Language:   "ko",
		Timezone:   "Asia/Seoul",
		LegalFlags: []string{"PIPA"},
	}

	f, err := StartFlow(context.Background(), locale)
	if err != nil {
		t.Fatalf("StartFlow returned error: %v", err)
	}
	if f.Data.Locale.Country != "KR" {
		t.Errorf("Data.Locale.Country = %q, want %q", f.Data.Locale.Country, "KR")
	}
	if f.Data.Locale.Language != "ko" {
		t.Errorf("Data.Locale.Language = %q, want %q", f.Data.Locale.Language, "ko")
	}
	if f.Data.Locale.Timezone != "Asia/Seoul" {
		t.Errorf("Data.Locale.Timezone = %q, want %q", f.Data.Locale.Timezone, "Asia/Seoul")
	}
	if len(f.Data.Locale.LegalFlags) != 1 || f.Data.Locale.LegalFlags[0] != "PIPA" {
		t.Errorf("Data.Locale.LegalFlags = %v, want [PIPA]", f.Data.Locale.LegalFlags)
	}
}

// --- §6.8 RED #3 and extensions --- SubmitStep advancement ---

// TestSubmitStep_AdvancesCurrentStep verifies that a successful submit increments CurrentStep.
func TestSubmitStep_AdvancesCurrentStep(t *testing.T) {
	cases := []struct {
		name       string
		setupSteps int // how many steps to submit before the target
		data       any
		wantStep   int // expected CurrentStep after submit
	}{
		{
			name:       "step1_locale_advances_to_2",
			setupSteps: 0,
			data:       LocaleChoice{Country: "KR", Language: "ko"},
			wantStep:   2,
		},
		{
			name:       "step4_persona_advances_to_5",
			setupSteps: 3,
			data:       PersonaProfile{Name: "Alice", HonorificLevel: HonorificFormal},
			wantStep:   5,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFlowAt(t, tc.setupSteps)
			if err := f.SubmitStep(f.CurrentStep, tc.data); err != nil {
				t.Fatalf("SubmitStep(%d) error: %v", f.CurrentStep, err)
			}
			if f.CurrentStep != tc.wantStep {
				t.Errorf("CurrentStep = %d, want %d", f.CurrentStep, tc.wantStep)
			}
		})
	}
}

// TestSubmitStep_WrongStep_Errors verifies that submitting a step that does not
// equal CurrentStep returns ErrStepOutOfOrder.
func TestSubmitStep_WrongStep_Errors(t *testing.T) {
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// CurrentStep is 1; submitting step 3 must return ErrStepOutOfOrder.
	err = f.SubmitStep(3, CLIToolsDetection{})
	if !errors.Is(err, ErrStepOutOfOrder) {
		t.Errorf("SubmitStep(3) on step 1 = %v, want ErrStepOutOfOrder", err)
	}
}

// TestSubmitStep_DataTypeMismatch_Errors verifies that submitting a PersonaProfile on
// step 2 (which expects ModelSetup) returns ErrStepDataMismatch.
func TestSubmitStep_DataTypeMismatch_Errors(t *testing.T) {
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// Advance to step 2.
	if err := f.SubmitStep(1, LocaleChoice{}); err != nil {
		t.Fatal(err)
	}
	// Submit wrong type for step 2.
	err = f.SubmitStep(2, PersonaProfile{Name: "Alice"})
	if !errors.Is(err, ErrStepDataMismatch) {
		t.Errorf("SubmitStep(2, PersonaProfile) = %v, want ErrStepDataMismatch", err)
	}
}

// TestSubmitStep_OutOfRange_Errors verifies that step 0 and step 8 both return ErrStepRange.
func TestSubmitStep_OutOfRange_Errors(t *testing.T) {
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, step := range []int{0, 8} {
		err := f.SubmitStep(step, nil)
		if !errors.Is(err, ErrStepRange) {
			t.Errorf("SubmitStep(%d) = %v, want ErrStepRange", step, err)
		}
	}
}

// --- Skip ---

// TestSkipStep_AdvancesWithoutData verifies AC-OB-006 (partial): skipping step 2
// leaves Data.Model at its zero value and advances CurrentStep to 3.
func TestSkipStep_AdvancesWithoutData(t *testing.T) {
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// Submit step 1 to reach step 2.
	if err := f.SubmitStep(1, LocaleChoice{}); err != nil {
		t.Fatal(err)
	}

	if err := f.SkipStep(2); err != nil {
		t.Fatalf("SkipStep(2) error: %v", err)
	}
	if f.CurrentStep != 3 {
		t.Errorf("CurrentStep = %d, want 3", f.CurrentStep)
	}
	if f.Data.Model != (ModelSetup{}) {
		t.Errorf("Data.Model is not zero-valued after skip: %+v", f.Data.Model)
	}
}

// --- Back ---

// TestBack_DecrementsCurrentStep verifies that Back after one submitted step
// returns CurrentStep to 1 and preserves Data.Locale.
func TestBack_DecrementsCurrentStep(t *testing.T) {
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	locale := LocaleChoice{Country: "DE", Language: "de", LegalFlags: []string{"GDPR"}}
	if err := f.SubmitStep(1, locale); err != nil {
		t.Fatal(err)
	}
	if f.CurrentStep != 2 {
		t.Fatalf("pre-Back CurrentStep = %d, want 2", f.CurrentStep)
	}

	if err := f.Back(); err != nil {
		t.Fatalf("Back() error: %v", err)
	}
	if f.CurrentStep != 1 {
		t.Errorf("CurrentStep = %d after Back, want 1", f.CurrentStep)
	}
	// Data.Locale must be preserved (not cleared) on Back.
	if f.Data.Locale.Country != "DE" {
		t.Errorf("Data.Locale.Country = %q after Back, want %q", f.Data.Locale.Country, "DE")
	}
}

// TestBack_AtStep1_Errors verifies §6.8 RED #10 (partial): calling Back on a fresh
// flow (CurrentStep == 1) returns ErrCannotGoBack.
func TestBack_AtStep1_Errors(t *testing.T) {
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	err = f.Back()
	if !errors.Is(err, ErrCannotGoBack) {
		t.Errorf("Back() at step 1 = %v, want ErrCannotGoBack", err)
	}
}

// --- Complete ---

// TestComplete_BeforeAllSteps_Errors verifies that Complete before all steps are done
// returns ErrNotReadyToComplete.
func TestComplete_BeforeAllSteps_Errors(t *testing.T) {
	cases := []struct {
		name       string
		setupSteps int // number of steps to submit before calling Complete
	}{
		{"step0_no_submit", 0},
		{"step3_partial", 3},
		{"step6_one_before_last", 6},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFlowAt(t, tc.setupSteps)
			_, err := f.Complete()
			if !errors.Is(err, ErrNotReadyToComplete) {
				t.Errorf("Complete() after %d steps = %v, want ErrNotReadyToComplete", tc.setupSteps, err)
			}
		})
	}
}

// TestComplete_AfterStep7_ReturnsSnapshot verifies the happy path: after all 7 steps
// Complete returns a non-nil *OnboardingData equal to f.Data, and CompletedAt is set.
func TestComplete_AfterStep7_ReturnsSnapshot(t *testing.T) {
	f := submitAllSteps(t)

	before := time.Now().UTC()
	snapshot, err := f.Complete()
	after := time.Now().UTC()

	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}
	if snapshot == nil {
		t.Fatal("Complete() returned nil snapshot")
	}
	// Direct equality is not possible because OnboardingData contains slice fields.
	// Verify that scalar fields in the snapshot match f.Data.
	if snapshot.Locale.Country != f.Data.Locale.Country ||
		snapshot.Persona.Name != f.Data.Persona.Name ||
		snapshot.Provider.Provider != f.Data.Provider.Provider ||
		snapshot.Messenger.Type != f.Data.Messenger.Type ||
		snapshot.Consent.ConversationStorageLocal != f.Data.Consent.ConversationStorageLocal {
		t.Errorf("snapshot does not match f.Data:\ngot  %+v\nwant %+v", *snapshot, f.Data)
	}
	if f.CompletedAt == nil {
		t.Fatal("CompletedAt is nil after Complete")
	}
	if f.CompletedAt.Before(before) || f.CompletedAt.After(after) {
		t.Errorf("CompletedAt %v outside window [%v, %v]", *f.CompletedAt, before, after)
	}
}

// --- Full happy path ---

// TestFullHappyPath_AllSeven verifies AC-OB-025 (partial): a sequential walk through
// all 7 steps advances CurrentStep from 1 to completedSentinel (8), and the final
// OnboardingData reflects every submitted field.
func TestFullHappyPath_AllSeven(t *testing.T) {
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	steps := []struct {
		step int
		data any
	}{
		{1, LocaleChoice{Country: "KR", Language: "ko", Timezone: "Asia/Seoul"}},
		{2, ModelSetup{OllamaInstalled: true, SelectedModel: "gemma4-e4b", ModelSizeBytes: 4_000_000_000, RAMBytes: 16_000_000_000}},
		{3, CLIToolsDetection{DetectedTools: []CLITool{{Name: "claude", Version: "1.2.3", Path: "/usr/local/bin/claude"}}}},
		{4, PersonaProfile{Name: "Bob", HonorificLevel: HonorificCasual, Pronouns: "he/him", SoulMarkdown: "A helpful assistant."}},
		{5, ProviderChoice{Provider: ProviderAnthropic, AuthMethod: AuthMethodAPIKey, APIKeyStored: false}},
		{6, MessengerChannel{Type: MessengerLocalTerminal}},
		{7, ConsentFlags{ConversationStorageLocal: true, LoRATrainingAllowed: false, TelemetryEnabled: false, CrashReportingEnabled: false}},
	}

	for _, s := range steps {
		if f.CurrentStep != s.step {
			t.Fatalf("before step %d: CurrentStep = %d", s.step, f.CurrentStep)
		}
		if err := f.SubmitStep(s.step, s.data); err != nil {
			t.Fatalf("SubmitStep(%d) error: %v", s.step, err)
		}
	}

	if f.CurrentStep != completedSentinel {
		t.Errorf("after step 7: CurrentStep = %d, want %d", f.CurrentStep, completedSentinel)
	}

	snapshot, err := f.Complete()
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}

	// Spot-check a few fields from different steps.
	if snapshot.Locale.Country != "KR" {
		t.Errorf("Locale.Country = %q, want %q", snapshot.Locale.Country, "KR")
	}
	if !snapshot.Model.OllamaInstalled {
		t.Error("Model.OllamaInstalled = false, want true")
	}
	if len(snapshot.CLITools.DetectedTools) != 1 {
		t.Errorf("CLITools.DetectedTools len = %d, want 1", len(snapshot.CLITools.DetectedTools))
	}
	if snapshot.Persona.Name != "Bob" {
		t.Errorf("Persona.Name = %q, want %q", snapshot.Persona.Name, "Bob")
	}
	if snapshot.Provider.Provider != ProviderAnthropic {
		t.Errorf("Provider.Provider = %q, want %q", snapshot.Provider.Provider, ProviderAnthropic)
	}
	if snapshot.Messenger.Type != MessengerLocalTerminal {
		t.Errorf("Messenger.Type = %q, want %q", snapshot.Messenger.Type, MessengerLocalTerminal)
	}
	if !snapshot.Consent.ConversationStorageLocal {
		t.Error("Consent.ConversationStorageLocal = false, want true")
	}
}

// --- Helpers ---

// newFlowAt creates a flow and advances it by submitting n steps with zero-value data.
// It uses SkipStep so that validators (Phase 1B) do not interfere.
func newFlowAt(t *testing.T, n int) *OnboardingFlow {
	t.Helper()
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	for i := 1; i <= n; i++ {
		if err := f.SkipStep(i); err != nil {
			t.Fatalf("SkipStep(%d): %v", i, err)
		}
	}
	return f
}

// submitAllSteps submits all 7 steps with minimal valid data and returns the flow
// ready for Complete().
func submitAllSteps(t *testing.T) *OnboardingFlow {
	t.Helper()
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	steps := []struct {
		step int
		data any
	}{
		{1, LocaleChoice{Country: "US", Language: "en"}},
		{2, ModelSetup{}},
		{3, CLIToolsDetection{}},
		{4, PersonaProfile{Name: "TestUser", HonorificLevel: HonorificFormal}},
		{5, ProviderChoice{Provider: ProviderUnset}},
		{6, MessengerChannel{Type: MessengerLocalTerminal}},
		{7, ConsentFlags{ConversationStorageLocal: true}},
	}
	for _, s := range steps {
		if err := f.SubmitStep(s.step, s.data); err != nil {
			t.Fatalf("SubmitStep(%d): %v", s.step, err)
		}
	}
	return f
}

// submitAllStepsForPersist submits all 7 steps and is suitable for CompleteAndPersist tests
// that need a hermetic environment. Non-GDPR locale avoids consent complexity.
func submitAllStepsForPersist(t *testing.T, opts ...FlowOption) *OnboardingFlow {
	t.Helper()
	f, err := StartFlow(context.Background(), nil, opts...)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	steps := []struct {
		step int
		data any
	}{
		{1, LocaleChoice{Country: "US", Language: "en"}},
		{2, ModelSetup{}},
		{3, CLIToolsDetection{}},
		{4, PersonaProfile{Name: "TestUser", HonorificLevel: HonorificFormal}},
		{5, ProviderChoice{Provider: ProviderUnset}},
		{6, MessengerChannel{Type: MessengerLocalTerminal}},
		{7, ConsentFlags{ConversationStorageLocal: true}},
	}
	for _, s := range steps {
		if err := f.SubmitStep(s.step, s.data); err != nil {
			t.Fatalf("SubmitStep(%d): %v", s.step, err)
		}
	}
	return f
}

// --- Phase 1F — FlowOption and WithKeyring ---

// TestStartFlow_VariadicNoOptions_BackwardCompat verifies that StartFlow(ctx, nil)
// with no options compiles and behaves identically to Phase 1A (regression guard).
func TestStartFlow_VariadicNoOptions_BackwardCompat(t *testing.T) {
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatalf("StartFlow(ctx, nil) returned error: %v", err)
	}
	if f.CurrentStep != 1 {
		t.Errorf("CurrentStep = %d, want 1", f.CurrentStep)
	}
	if !hexPattern.MatchString(f.SessionID) {
		t.Errorf("SessionID %q does not match 32-hex pattern", f.SessionID)
	}
}

// TestStartFlow_WithKeyring_StoresClient verifies that WithKeyring injects the client
// by observing its effect: step 5 with AuthMethodAPIKey and a valid key results in
// APIKeyStored == true, which only happens when the keyring client is present.
func TestStartFlow_WithKeyring_StoresClient(t *testing.T) {
	kr := NewInMemoryKeyring()
	f, err := StartFlow(context.Background(), nil, WithKeyring(kr))
	if err != nil {
		t.Fatalf("StartFlow with WithKeyring: %v", err)
	}

	// Advance to step 5 via skips.
	for i := 1; i <= 4; i++ {
		if err := f.SkipStep(i); err != nil {
			t.Fatalf("SkipStep(%d): %v", i, err)
		}
	}

	// Submit step 5 via ProviderStepInput with a valid anthropic key.
	const validKey = "sk-ant-api03-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	input := ProviderStepInput{
		Choice: ProviderChoice{
			Provider:   ProviderAnthropic,
			AuthMethod: AuthMethodAPIKey,
		},
		APIKey: validKey,
	}
	if err := f.SubmitStep(5, input); err != nil {
		t.Fatalf("SubmitStep(5, ProviderStepInput): %v", err)
	}

	if !f.Data.Provider.APIKeyStored {
		t.Error("Provider.APIKeyStored = false, want true (keyring client was injected)")
	}
}

// --- Phase 1F — Step 4 persona validation ---

// TestSubmitStep_Step4_ValidatesPersonaName_Empty verifies that an empty Name returns
// a wrapped ErrNameEmpty.
func TestSubmitStep_Step4_ValidatesPersonaName_Empty(t *testing.T) {
	f := newFlowAt(t, 3)
	err := f.SubmitStep(4, PersonaProfile{Name: "", HonorificLevel: HonorificFormal})
	if !errors.Is(err, ErrNameEmpty) {
		t.Errorf("SubmitStep(4, empty name) = %v, want errors.Is ErrNameEmpty", err)
	}
}

// TestSubmitStep_Step4_ValidatesPersonaName_Injection verifies that a name containing
// injection characters returns a wrapped ErrNameInjection.
func TestSubmitStep_Step4_ValidatesPersonaName_Injection(t *testing.T) {
	f := newFlowAt(t, 3)
	err := f.SubmitStep(4, PersonaProfile{Name: "<script>alert(1)</script>", HonorificLevel: HonorificFormal})
	if !errors.Is(err, ErrNameInjection) {
		t.Errorf("SubmitStep(4, injection name) = %v, want errors.Is ErrNameInjection", err)
	}
}

// TestSubmitStep_Step4_AcceptsEmptyHonorificLevel verifies that a valid name with an
// empty HonorificLevel returns nil (the default is applied at completion time).
func TestSubmitStep_Step4_AcceptsEmptyHonorificLevel(t *testing.T) {
	f := newFlowAt(t, 3)
	err := f.SubmitStep(4, PersonaProfile{Name: "Alice", HonorificLevel: ""})
	if err != nil {
		t.Errorf("SubmitStep(4, empty honorific) = %v, want nil", err)
	}
}

// TestSubmitStep_Step4_RejectsInvalidHonorificLevel verifies that a non-empty but
// invalid HonorificLevel value returns a wrapped ErrInvalidHonorificLevel.
func TestSubmitStep_Step4_RejectsInvalidHonorificLevel(t *testing.T) {
	f := newFlowAt(t, 3)
	err := f.SubmitStep(4, PersonaProfile{Name: "Alice", HonorificLevel: "rude"})
	if !errors.Is(err, ErrInvalidHonorificLevel) {
		t.Errorf("SubmitStep(4, invalid honorific) = %v, want errors.Is ErrInvalidHonorificLevel", err)
	}
}

// --- Phase 1F — Step 5 ProviderStepInput ---

// TestSubmitStep_Step5_ProviderStepInput_ValidatesAPIKey_Invalid verifies that an
// anthropic key with wrong format returns a wrapped ErrInvalidAPIKeyFormat.
func TestSubmitStep_Step5_ProviderStepInput_ValidatesAPIKey_Invalid(t *testing.T) {
	f := newFlowAt(t, 4)
	input := ProviderStepInput{
		Choice: ProviderChoice{
			Provider:   ProviderAnthropic,
			AuthMethod: AuthMethodAPIKey,
		},
		APIKey: "wrong",
	}
	err := f.SubmitStep(5, input)
	if !errors.Is(err, ErrInvalidAPIKeyFormat) {
		t.Errorf("SubmitStep(5, bad key) = %v, want errors.Is ErrInvalidAPIKeyFormat", err)
	}
}

// TestSubmitStep_Step5_ProviderStepInput_StoresToKeyring_AuthMethodAPIKey verifies
// the happy path: valid key + InMemoryKeyring + AuthMethodAPIKey → keyring has entry,
// Provider.APIKeyStored == true.
func TestSubmitStep_Step5_ProviderStepInput_StoresToKeyring_AuthMethodAPIKey(t *testing.T) {
	kr := NewInMemoryKeyring()
	f, err := StartFlow(context.Background(), nil, WithKeyring(kr))
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	for i := 1; i <= 4; i++ {
		if err := f.SkipStep(i); err != nil {
			t.Fatalf("SkipStep(%d): %v", i, err)
		}
	}

	const validKey = "sk-ant-api03-BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	input := ProviderStepInput{
		Choice: ProviderChoice{
			Provider:   ProviderAnthropic,
			AuthMethod: AuthMethodAPIKey,
		},
		APIKey: validKey,
	}
	if err := f.SubmitStep(5, input); err != nil {
		t.Fatalf("SubmitStep(5): %v", err)
	}

	if !f.Data.Provider.APIKeyStored {
		t.Error("Provider.APIKeyStored = false, want true")
	}

	// Verify the key is actually in the keyring.
	stored, err := GetProviderAPIKey(kr, "anthropic")
	if err != nil {
		t.Fatalf("GetProviderAPIKey: %v", err)
	}
	if stored != validKey {
		t.Errorf("keyring stored %q, want %q", stored, validKey)
	}
}

// TestSubmitStep_Step5_ProviderStepInput_SkipsKeyringWhenAuthMethodEnv verifies that
// when AuthMethod is "env", the keyring is NOT written even if a valid key is provided,
// and APIKeyStored remains false.
func TestSubmitStep_Step5_ProviderStepInput_SkipsKeyringWhenAuthMethodEnv(t *testing.T) {
	kr := NewInMemoryKeyring()
	f, err := StartFlow(context.Background(), nil, WithKeyring(kr))
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	for i := 1; i <= 4; i++ {
		if err := f.SkipStep(i); err != nil {
			t.Fatalf("SkipStep(%d): %v", i, err)
		}
	}

	const validKey = "sk-ant-api03-CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"
	input := ProviderStepInput{
		Choice: ProviderChoice{
			Provider:   ProviderAnthropic,
			AuthMethod: AuthMethodEnv,
		},
		APIKey: validKey,
	}
	if err := f.SubmitStep(5, input); err != nil {
		t.Fatalf("SubmitStep(5, env auth): %v", err)
	}

	if f.Data.Provider.APIKeyStored {
		t.Error("Provider.APIKeyStored = true, want false (env path skips keyring)")
	}

	// Keyring must not have any entry for anthropic.
	_, keyErr := GetProviderAPIKey(kr, "anthropic")
	if !errors.Is(keyErr, ErrKeyNotFound) {
		t.Errorf("keyring entry should be absent for env path, got err=%v", keyErr)
	}
}

// TestSubmitStep_Step5_ProviderStepInput_SkipsKeyringWhenNilKeyring verifies that
// when no keyring is injected, valid key + AuthMethodAPIKey proceeds without panic
// and APIKeyStored remains false (key validation still runs).
func TestSubmitStep_Step5_ProviderStepInput_SkipsKeyringWhenNilKeyring(t *testing.T) {
	f, err := StartFlow(context.Background(), nil) // no WithKeyring
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	for i := 1; i <= 4; i++ {
		if err := f.SkipStep(i); err != nil {
			t.Fatalf("SkipStep(%d): %v", i, err)
		}
	}

	const validKey = "sk-ant-api03-DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD"
	input := ProviderStepInput{
		Choice: ProviderChoice{
			Provider:   ProviderAnthropic,
			AuthMethod: AuthMethodAPIKey,
		},
		APIKey: validKey,
	}
	if err := f.SubmitStep(5, input); err != nil {
		t.Fatalf("SubmitStep(5, no keyring): %v", err)
	}

	if f.Data.Provider.APIKeyStored {
		t.Error("Provider.APIKeyStored = true, want false (nil keyring path)")
	}
}

// TestSubmitStep_Step5_ProviderStepInput_OllamaNoKeyRequired verifies that ollama
// with an empty key returns nil (ollama has no key requirement per validator).
func TestSubmitStep_Step5_ProviderStepInput_OllamaNoKeyRequired(t *testing.T) {
	f := newFlowAt(t, 4)
	input := ProviderStepInput{
		Choice: ProviderChoice{
			Provider:   ProviderOllama,
			AuthMethod: AuthMethodAPIKey,
		},
		APIKey: "",
	}
	if err := f.SubmitStep(5, input); err != nil {
		t.Errorf("SubmitStep(5, ollama empty key) = %v, want nil", err)
	}
}

// TestSubmitStep_Step5_LegacyProviderChoice_BackwardCompat verifies that passing
// a bare ProviderChoice (no APIKey) assigns it directly without validation and without
// calling the keyring (Phase 1A regression guard).
func TestSubmitStep_Step5_LegacyProviderChoice_BackwardCompat(t *testing.T) {
	kr := NewInMemoryKeyring()
	f, err := StartFlow(context.Background(), nil, WithKeyring(kr))
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	for i := 1; i <= 4; i++ {
		if err := f.SkipStep(i); err != nil {
			t.Fatalf("SkipStep(%d): %v", i, err)
		}
	}

	pc := ProviderChoice{Provider: ProviderAnthropic, AuthMethod: AuthMethodAPIKey}
	if err := f.SubmitStep(5, pc); err != nil {
		t.Fatalf("SubmitStep(5, legacy ProviderChoice): %v", err)
	}

	if f.Data.Provider.Provider != ProviderAnthropic {
		t.Errorf("Provider.Provider = %q, want %q", f.Data.Provider.Provider, ProviderAnthropic)
	}
	// APIKeyStored must remain false — legacy path does not touch keyring.
	if f.Data.Provider.APIKeyStored {
		t.Error("Provider.APIKeyStored = true, want false (legacy path)")
	}
	// Keyring must be untouched.
	_, keyErr := GetProviderAPIKey(kr, "anthropic")
	if !errors.Is(keyErr, ErrKeyNotFound) {
		t.Errorf("keyring should be empty for legacy path, got err=%v", keyErr)
	}
}

// --- Phase 1F — Step 7 GDPR consent validation ---

// TestSubmitStep_Step7_GDPRConsentRequired_Blocks_NilExplicitConsent verifies that
// submitting step 7 with GDPR locale but nil GDPRExplicitConsent returns ErrGDPRConsentRequired.
func TestSubmitStep_Step7_GDPRConsentRequired_Blocks_NilExplicitConsent(t *testing.T) {
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	// Step 1 with GDPR locale.
	if err := f.SubmitStep(1, LocaleChoice{Country: "DE", Language: "de", LegalFlags: []string{"GDPR"}}); err != nil {
		t.Fatalf("SubmitStep(1): %v", err)
	}
	for i := 2; i <= 6; i++ {
		if err := f.SkipStep(i); err != nil {
			t.Fatalf("SkipStep(%d): %v", i, err)
		}
	}

	err = f.SubmitStep(7, ConsentFlags{GDPRExplicitConsent: nil})
	if !errors.Is(err, ErrGDPRConsentRequired) {
		t.Errorf("SubmitStep(7, nil GDPR consent) = %v, want errors.Is ErrGDPRConsentRequired", err)
	}
}

// TestSubmitStep_Step7_GDPRConsentRequired_Allows_True verifies that a GDPR locale
// with GDPRExplicitConsent set to true passes step 7 validation.
func TestSubmitStep_Step7_GDPRConsentRequired_Allows_True(t *testing.T) {
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	if err := f.SubmitStep(1, LocaleChoice{Country: "FR", Language: "fr", LegalFlags: []string{"GDPR"}}); err != nil {
		t.Fatalf("SubmitStep(1): %v", err)
	}
	for i := 2; i <= 6; i++ {
		if err := f.SkipStep(i); err != nil {
			t.Fatalf("SkipStep(%d): %v", i, err)
		}
	}

	trueVal := true
	if err := f.SubmitStep(7, ConsentFlags{GDPRExplicitConsent: &trueVal}); err != nil {
		t.Errorf("SubmitStep(7, GDPR=true) = %v, want nil", err)
	}
}

// TestSubmitStep_Step7_NonGDPR_AllowsNilExplicitConsent verifies that a non-GDPR locale
// with nil GDPRExplicitConsent passes step 7 validation.
func TestSubmitStep_Step7_NonGDPR_AllowsNilExplicitConsent(t *testing.T) {
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	if err := f.SubmitStep(1, LocaleChoice{Country: "US", Language: "en", LegalFlags: []string{}}); err != nil {
		t.Fatalf("SubmitStep(1): %v", err)
	}
	for i := 2; i <= 6; i++ {
		if err := f.SkipStep(i); err != nil {
			t.Fatalf("SkipStep(%d): %v", i, err)
		}
	}

	if err := f.SubmitStep(7, ConsentFlags{GDPRExplicitConsent: nil}); err != nil {
		t.Errorf("SubmitStep(7, non-GDPR nil consent) = %v, want nil", err)
	}
}

// --- Phase 1F — SkipStep GDPR enforcement ---

// TestSkipStep_Step7_GDPRRegion_Blocked verifies that SkipStep(7) in a GDPR region
// returns ErrGDPRConsentRequired.
func TestSkipStep_Step7_GDPRRegion_Blocked(t *testing.T) {
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	if err := f.SubmitStep(1, LocaleChoice{Country: "DE", Language: "de", LegalFlags: []string{"GDPR"}}); err != nil {
		t.Fatalf("SubmitStep(1): %v", err)
	}
	for i := 2; i <= 6; i++ {
		if err := f.SkipStep(i); err != nil {
			t.Fatalf("SkipStep(%d): %v", i, err)
		}
	}

	err = f.SkipStep(7)
	if !errors.Is(err, ErrGDPRConsentRequired) {
		t.Errorf("SkipStep(7) in GDPR region = %v, want errors.Is ErrGDPRConsentRequired", err)
	}
}

// TestSkipStep_Step7_NonGDPRRegion_Allowed verifies that SkipStep(7) with no GDPR
// legal flag succeeds and CurrentStep advances to 8.
func TestSkipStep_Step7_NonGDPRRegion_Allowed(t *testing.T) {
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	if err := f.SubmitStep(1, LocaleChoice{Country: "US", Language: "en"}); err != nil {
		t.Fatalf("SubmitStep(1): %v", err)
	}
	for i := 2; i <= 6; i++ {
		if err := f.SkipStep(i); err != nil {
			t.Fatalf("SkipStep(%d): %v", i, err)
		}
	}

	if err := f.SkipStep(7); err != nil {
		t.Errorf("SkipStep(7) non-GDPR = %v, want nil", err)
	}
	if f.CurrentStep != 8 {
		t.Errorf("CurrentStep = %d, want 8 after SkipStep(7)", f.CurrentStep)
	}
}

// --- Phase 1F — CompleteAndPersist ---

// TestCompleteAndPersist_HappyPath verifies that CompleteAndPersist writes the global
// config, project config, and the onboarding-completed marker to a hermetic temp directory.
func TestCompleteAndPersist_HappyPath(t *testing.T) {
	tempHome := t.TempDir()
	tempProject := t.TempDir()

	globalPath := filepath.Join(tempHome, ".mink", "config.yaml")
	projectPath := filepath.Join(tempProject, ".mink", "config.yaml")
	markerPath := filepath.Join(tempHome, ".mink", "onboarding-completed")

	fixedTime := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	opts := CompletionOptions{
		GlobalConfigPathOverride:  globalPath,
		ProjectConfigPathOverride: projectPath,
		CompletedMarkerOverride:   markerPath,
		Now:                       func() time.Time { return fixedTime },
	}

	kr := NewInMemoryKeyring()
	f := submitAllStepsForPersist(t, WithKeyring(kr), WithCompletionOptions(opts))

	data, err := f.CompleteAndPersist()
	if err != nil {
		t.Fatalf("CompleteAndPersist() error: %v", err)
	}
	if data == nil {
		t.Fatal("CompleteAndPersist() returned nil data")
	}

	// Verify global config file exists.
	if _, statErr := os.Stat(globalPath); statErr != nil {
		t.Errorf("global config file not created: %v", statErr)
	}
	// Verify project config file exists.
	if _, statErr := os.Stat(projectPath); statErr != nil {
		t.Errorf("project config file not created: %v", statErr)
	}
	// Verify onboarding marker exists and contains an RFC3339 timestamp.
	markerBytes, readErr := os.ReadFile(markerPath)
	if readErr != nil {
		t.Fatalf("marker file not readable: %v", readErr)
	}
	markerContent := string(markerBytes)
	if len(markerContent) < 20 {
		t.Errorf("marker content looks wrong: %q", markerContent)
	}
	// Ensure CompletedAt is set on the flow.
	if f.CompletedAt == nil {
		t.Error("CompletedAt is nil after CompleteAndPersist")
	}
}

// TestCompleteAndPersist_NotReady_ReturnsErrNotReadyToComplete verifies that calling
// CompleteAndPersist without finishing all 7 steps returns ErrNotReadyToComplete.
func TestCompleteAndPersist_NotReady_ReturnsErrNotReadyToComplete(t *testing.T) {
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	// Only submit 3 steps.
	for i := 1; i <= 3; i++ {
		if err := f.SkipStep(i); err != nil {
			t.Fatalf("SkipStep(%d): %v", i, err)
		}
	}

	_, err = f.CompleteAndPersist()
	if !errors.Is(err, ErrNotReadyToComplete) {
		t.Errorf("CompleteAndPersist() not-ready = %v, want errors.Is ErrNotReadyToComplete", err)
	}
}

// TestCompleteAndPersist_PersistFailureWrapsErrPersistFailed verifies that a write
// failure in WriteCompletionConfig results in an error wrapping ErrPersistFailed.
// Skipped on Windows because /dev/null semantics differ.
func TestCompleteAndPersist_PersistFailureWrapsErrPersistFailed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: /dev/null directory semantics differ")
	}

	opts := CompletionOptions{
		// This path has a non-existent parent component that cannot be created.
		GlobalConfigPathOverride:  "/dev/null/impossible/x.yaml",
		ProjectConfigPathOverride: filepath.Join(t.TempDir(), "project.yaml"),
		CompletedMarkerOverride:   filepath.Join(t.TempDir(), "marker"),
	}

	f := submitAllStepsForPersist(t, WithCompletionOptions(opts))

	_, err := f.CompleteAndPersist()
	if !errors.Is(err, ErrPersistFailed) {
		t.Errorf("CompleteAndPersist() persist fail = %v, want errors.Is ErrPersistFailed", err)
	}
}
