// Package install — tui_e2e_test.go validates the 7-step wizard end-to-end
// using huh's accessible mode (line-by-line stdin) instead of ANSI key
// sequences.  Every test drives RunWizard through a lineReader that wraps
// strings.NewReader and returns one byte at a time, preventing bufio.Scanner
// from consuming more than one line per field (each field's RunAccessible
// creates a fresh bufio.Scanner over the shared io.Reader).
//
// SPEC: SPEC-MINK-ONBOARDING-001 §6 Phase 2D — teatest end-to-end
package install

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/onboarding"
)

// lineReader wraps an io.Reader and returns exactly one byte per Read call.
// huh's accessible-mode fields each create a new bufio.Scanner over the shared
// io.Reader.  Without this wrapper the first Scanner consumes the entire input
// stream into its internal buffer, starving all subsequent fields.  By reading
// one byte at a time we ensure every Scanner can only see one line before the
// next field's Scanner takes over.
type lineReader struct{ r io.Reader }

func (b *lineReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return b.r.Read(p[:1])
}

// -----------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------

// e2eOpts builds a WizardOptions suitable for accessible-mode E2E tests.
// The caller provides the full input string (all newline-terminated field
// values for every form in the run) and a buffer that captures output.
// The input is wrapped in lineReader so that each huh field's bufio.Scanner
// reads only one line before handing control to the next field's scanner.
func e2eOpts(input string, output *bytes.Buffer, flowOpts ...onboarding.FlowOption) WizardOptions {
	return WizardOptions{
		Input:         &lineReader{strings.NewReader(input)},
		Output:        output,
		Accessible:    true,
		SkipPreflight: true,
		FlowOptions:   flowOpts,
	}
}

// dryRunFlowOpts returns FlowOption values with an in-memory keyring and
// DryRun=true (no disk writes).
func dryRunFlowOpts(kr onboarding.KeyringClient) []onboarding.FlowOption {
	return []onboarding.FlowOption{
		onboarding.WithKeyring(kr),
		onboarding.WithCompletionOptions(onboarding.CompletionOptions{DryRun: true}),
	}
}

// standardFlowOpts returns FlowOption with an in-memory keyring and normal
// (non-dry-run) completion options.
func standardFlowOpts(kr onboarding.KeyringClient) []onboarding.FlowOption {
	return []onboarding.FlowOption{
		onboarding.WithKeyring(kr),
		onboarding.WithCompletionOptions(onboarding.CompletionOptions{}),
	}
}

// pathExists reports whether the named filesystem path exists.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// -----------------------------------------------------------------------
// Input sequences
//
// All sequences are for accessible-mode huh forms where each field reads
// one line terminated by "\n".
//
// Select fields: "N\n" selects 1-based option N; "\n" accepts the default.
// Confirm fields: "Y\n" or "y\n" = yes; "n\n" = no; "\n" = accept default.
// Input fields:  "text\n" sets value; "\n" = accept default (pre-filled).
// Text (multi-line) fields: "\n" accepts default pre-filled value.
//
// Navigation form (runNavChoice, shown before steps 2-7):
//   Option 1: Continue with this step   → "1\n"
//   Option 2: Skip this step (if shown) → "2\n"
//   Option 3: Go back (if shown)        → "3\n"  (when Skip is also shown)
//
// -----------------------------------------------------------------------

// happyPathKRInput is the full input sequence for a KR-locale happy-path run:
//   - No Ollama (SkipPreflight=true → zero-value OllamaStatus)
//   - No CLI tools detected (zero-value cliTools slice)
//   - Persona name "TestUser", formal honorific, no pronouns
//   - Ollama provider (no auth/key form needed)
//   - Local terminal messenger
//   - All 4 consent confirms accepted at their defaults
//
// Field-by-field:
//
//	Step 1  locale select (KR is option 1, default):   "\n"
//	Step 2  nav (1=Continue, 2=Skip, 3=Back):          "1\n"
//	Step 2  confirm continue without Ollama (dflt Y):   "\n"
//	Step 3  nav:                                        "1\n"
//	 (no step 3 form — empty tools list → Println + SubmitStep)
//	Step 4  nav:                                        "1\n"
//	Step 4  Input name:                                 "TestUser\n"
//	Step 4  Select honor (1=Formal):                    "1\n"
//	Step 4  Input pronouns (empty):                     "\n"
//	Step 4  Text soul (accept pre-filled default):      "\n"
//	Step 5  nav:                                        "1\n"
//	Step 5  Select provider (4=Ollama, no key needed):  "4\n"
//	Step 6  nav:                                        "1\n"
//	Step 6  Select messenger (1=local terminal):        "1\n"
//	Step 7  nav (KR/PIPA: skipAllowed, 3 opts):         "1\n"
//	 (no GDPR form)
//	Step 7  Consent 4 Confirms (all at defaults):       "\n\n\n\n"
const happyPathKRInput = "" +
	"\n" + // step 1: locale (default = KR)
	"1\n" + // step 2 nav: Continue
	"\n" + // step 2: confirm "Continue without Ollama" (default = yes)
	"1\n" + // step 3 nav: Continue
	// no step-3 form (zero cliTools)
	"1\n" + // step 4 nav: Continue
	"TestUser\n" + // step 4: persona name
	"1\n" + // step 4: honorific (formal, option 1)
	"\n" + // step 4: pronouns (empty)
	"\n" + // step 4: soul text (accept default)
	"1\n" + // step 5 nav: Continue
	"4\n" + // step 5: provider = Ollama (4th option)
	// Ollama is local — no auth or api-key form
	"1\n" + // step 6 nav: Continue
	"1\n" + // step 6: messenger = local terminal
	"1\n" + // step 7 nav: Continue
	// no GDPR form (KR locale)
	"\n\n\n\n" // step 7: 4 consent Confirms at defaults

// gdprFRInput drives a complete FR-locale run, accepting GDPR consent.
//
// FR locale is the 3rd option in localePresets (0-based index 2 → 1-based "3").
// Step 7 nav for GDPR locale: skipAllowed=false → only Continue(1) and Back(2).
// GDPR confirm: "Y\n" = accept.
const gdprFRInput = "" +
	"3\n" + // step 1: locale = FR (3rd option)
	"1\n" + // step 2 nav: Continue
	"\n" + // step 2: confirm without Ollama
	"1\n" + // step 3 nav: Continue
	// no step-3 form
	"1\n" + // step 4 nav: Continue
	"TestFR\n" + // step 4: persona name
	"1\n" + // step 4: honorific
	"\n" + // step 4: pronouns
	"\n" + // step 4: soul
	"1\n" + // step 5 nav: Continue
	"4\n" + // step 5: provider = Ollama
	"1\n" + // step 6 nav: Continue
	"1\n" + // step 6: messenger
	"1\n" + // step 7 nav: Continue (GDPR: only Continue(1) and Back(2))
	"Y\n" + // step 7: GDPR explicit consent accepted
	"\n\n\n\n" // step 7: regular consent form

// gdprFRRejectInput is identical to gdprFRInput except the user rejects
// the GDPR consent confirm ("n\n").
const gdprFRRejectInput = "" +
	"3\n" + // step 1: locale = FR
	"1\n" + // step 2 nav: Continue
	"\n" + // step 2: confirm without Ollama
	"1\n" + // step 3 nav: Continue
	// no step-3 form
	"1\n" + // step 4 nav: Continue
	"TestFR\n" + "1\n" + "\n" + "\n" + // step 4: persona
	"1\n" + // step 5 nav: Continue
	"4\n" + // step 5: provider = Ollama
	"1\n" + // step 6 nav: Continue
	"1\n" + // step 6: messenger
	"1\n" + // step 7 nav: Continue
	"n\n" // step 7: GDPR consent REJECTED

// backStep4Input navigates steps 1-3 normally, selects Back at the step-4
// navigation form, then re-submits steps 3 and 4 before completing 5-7.
//
// Step 4 nav options (showSkip=true, showBack=true → 3 options):
//
//	1 = Continue, 2 = Skip, 3 = Back
//
// After Back: flow.CurrentStep reverts to 3.
// Step 3 re-runs (nav + no-form path again).
// Step 4 re-runs with Continue this time, completing normally.
const backStep4Input = "" +
	"\n" + // step 1: locale KR
	"1\n" + // step 2 nav: Continue
	"\n" + // step 2: confirm
	"1\n" + // step 3 nav: Continue (first pass)
	// no step-3 form (first pass)
	"3\n" + // step 4 nav: Back (option 3)
	// flow.CurrentStep = 3 now
	"1\n" + // step 3 nav: Continue (second pass)
	// no step-3 form (second pass)
	"1\n" + // step 4 nav: Continue (second pass)
	"BackUser\n" + // step 4: persona name
	"1\n" + // step 4: honorific
	"\n" + // step 4: pronouns
	"\n" + // step 4: soul
	"1\n" + // step 5 nav: Continue
	"4\n" + // step 5: provider Ollama
	"1\n" + // step 6 nav: Continue
	"1\n" + // step 6: messenger
	"1\n" + // step 7 nav: Continue
	"\n\n\n\n" // step 7: consent defaults

// resumeFromStep4Input is the input for the second RunWizard call in
// TestE2E_Resume_FromStep3.  The draft was saved at step 4 (after submitting
// steps 1-3), so this input covers only steps 4-7.
const resumeFromStep4Input = "" +
	"1\n" + // step 4 nav: Continue
	"ResumedUser\n" + // step 4: persona name
	"1\n" + // step 4: honorific
	"\n" + // step 4: pronouns
	"\n" + // step 4: soul
	"1\n" + // step 5 nav: Continue
	"4\n" + // step 5: provider Ollama
	"1\n" + // step 6 nav: Continue
	"1\n" + // step 6: messenger
	"1\n" + // step 7 nav: Continue
	"\n\n\n\n" // step 7: consent defaults

// -----------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------

// TestE2E_HappyPath_FullSubmit verifies the 7-step happy-path end-to-end.
// Locale: KR.  No Ollama, no CLI tools.  Persona "TestUser".
// Provider: Ollama (no API key).  Messenger: local terminal.
// All consent confirms at their secure defaults.
func TestE2E_HappyPath_FullSubmit(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MINK_PROJECT_DIR", tmpDir)
	t.Setenv("MINK_HOME", tmpDir)

	kr := onboarding.NewInMemoryKeyring()
	var out bytes.Buffer

	opts := e2eOpts(happyPathKRInput, &out, standardFlowOpts(kr)...)

	err := RunWizard(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunWizard returned error: %v\noutput:\n%s", err, out.String())
	}

	// The welcome line must mention the persona name.
	if !strings.Contains(out.String(), "TestUser") {
		t.Errorf("expected 'TestUser' in wizard output, got:\n%s", out.String())
	}
}

// TestE2E_GDPR_ConsentRejection verifies that a user who selects FR locale
// and then rejects the GDPR consent confirm receives an error containing
// "GDPR consent is required".
func TestE2E_GDPR_ConsentRejection(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MINK_PROJECT_DIR", tmpDir)
	t.Setenv("MINK_HOME", tmpDir)

	kr := onboarding.NewInMemoryKeyring()
	var out bytes.Buffer

	opts := e2eOpts(gdprFRRejectInput, &out, dryRunFlowOpts(kr)...)

	err := RunWizard(context.Background(), opts)
	if err == nil {
		t.Fatal("RunWizard expected error for GDPR rejection, got nil")
	}
	if !strings.Contains(err.Error(), "GDPR consent is required") {
		t.Errorf("error = %q, want to contain 'GDPR consent is required'", err.Error())
	}
}

// TestE2E_GDPR_ConsentAccepted verifies a complete FR-locale run where the
// user accepts GDPR consent.  RunWizard must return nil and the output must
// contain the persona name "TestFR".
func TestE2E_GDPR_ConsentAccepted(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MINK_PROJECT_DIR", tmpDir)
	t.Setenv("MINK_HOME", tmpDir)

	kr := onboarding.NewInMemoryKeyring()
	var out bytes.Buffer

	opts := e2eOpts(gdprFRInput, &out, standardFlowOpts(kr)...)

	err := RunWizard(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunWizard (FR GDPR accepted) returned error: %v\noutput:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "TestFR") {
		t.Errorf("expected 'TestFR' in output, got:\n%s", out.String())
	}
}

// TestE2E_Back_Step4ToStep3 verifies that selecting Back at the step-4
// navigation form returns the wizard to step 3, which re-runs cleanly, and
// the full flow completes normally after re-entering step 4.
func TestE2E_Back_Step4ToStep3(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MINK_PROJECT_DIR", tmpDir)
	t.Setenv("MINK_HOME", tmpDir)

	kr := onboarding.NewInMemoryKeyring()
	var out bytes.Buffer

	opts := e2eOpts(backStep4Input, &out, standardFlowOpts(kr)...)

	err := RunWizard(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunWizard returned error: %v\noutput:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "BackUser") {
		t.Errorf("expected 'BackUser' in output after back+re-enter, got:\n%s", out.String())
	}
}

// TestE2E_Resume_FromStep3 verifies the Resume flow:
//  1. Use the onboarding API to submit steps 1-3 directly and save a draft
//     (simulates the user pausing the wizard after step 3).
//  2. Run RunWizard with Resume:true using only steps 4-7 input.
//  3. Assert RunWizard returns nil and emits "ResumedUser" in the output.
//  4. Assert the draft file is deleted after successful completion.
func TestE2E_Resume_FromStep3(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MINK_PROJECT_DIR", tmpDir)
	t.Setenv("MINK_HOME", tmpDir)

	kr := onboarding.NewInMemoryKeyring()

	// Phase A: build a draft at step 4 by submitting steps 1-3 directly.
	flow, err := onboarding.StartFlow(context.Background(), nil,
		onboarding.WithKeyring(kr),
	)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	if err := flow.SubmitStep(1, onboarding.LocaleChoice{
		Country:    "KR",
		Language:   "ko",
		Timezone:   "Asia/Seoul",
		LegalFlags: []string{"PIPA"},
	}); err != nil {
		t.Fatalf("SubmitStep 1: %v", err)
	}
	if err := flow.SubmitStep(2, onboarding.ModelSetup{}); err != nil {
		t.Fatalf("SubmitStep 2: %v", err)
	}
	if err := flow.SubmitStep(3, onboarding.CLIToolsDetection{}); err != nil {
		t.Fatalf("SubmitStep 3: %v", err)
	}
	// After SubmitStep(3) the flow advances to step 4.
	if err := onboarding.SaveDraft(onboarding.DraftFromFlow(flow)); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}

	// Phase B: resume from the draft, providing only steps 4-7 input.
	var out bytes.Buffer
	resumeOpts := WizardOptions{
		Input:         &lineReader{strings.NewReader(resumeFromStep4Input)},
		Output:        &out,
		Accessible:    true,
		SkipPreflight: true,
		Resume:        true,
		FlowOptions: []onboarding.FlowOption{
			onboarding.WithKeyring(kr),
			onboarding.WithCompletionOptions(onboarding.CompletionOptions{}),
		},
	}

	err = RunWizard(context.Background(), resumeOpts)
	if err != nil {
		t.Fatalf("RunWizard (Resume) returned error: %v\noutput:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "ResumedUser") {
		t.Errorf("expected 'ResumedUser' in output, got:\n%s", out.String())
	}

	// Draft must be deleted after successful completion.
	_, loadErr := onboarding.LoadDraft()
	if !errors.Is(loadErr, onboarding.ErrDraftNotFound) {
		t.Errorf("draft should be deleted after successful resume, LoadDraft = %v", loadErr)
	}
}

// TestE2E_DryRun_NoDiskWrite verifies that DryRun:true suppresses all disk
// writes: no config files and no onboarding-complete markers are created.
func TestE2E_DryRun_NoDiskWrite(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MINK_PROJECT_DIR", tmpDir)
	t.Setenv("MINK_HOME", tmpDir)

	kr := onboarding.NewInMemoryKeyring()
	var out bytes.Buffer

	opts := e2eOpts(happyPathKRInput, &out, dryRunFlowOpts(kr)...)

	err := RunWizard(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunWizard (DryRun) returned error: %v\noutput:\n%s", err, out.String())
	}

	// No draft should exist (DryRun suppresses autoSaveDraft).
	_, draftErr := onboarding.LoadDraft()
	if !errors.Is(draftErr, onboarding.ErrDraftNotFound) {
		t.Errorf("DryRun=true: draft file should not exist, LoadDraft = %v", draftErr)
	}

	// Neither the global config nor the project config should be written.
	if globalPath, pathErr := onboarding.GlobalConfigPath(); pathErr == nil {
		if pathExists(globalPath) {
			t.Errorf("DryRun=true: global config written to %s", globalPath)
		}
	}
	if projectPath, pathErr := onboarding.ProjectConfigPath(); pathErr == nil {
		if pathExists(projectPath) {
			t.Errorf("DryRun=true: project config written to %s", projectPath)
		}
	}
}
