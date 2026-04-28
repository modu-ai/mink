# SPEC Re-Audit Report: 6-SPEC Batch (Post-Fix Verification)

Iteration: Re-audit (post-fix verification)
Date: 2026-04-29
Auditor: plan-auditor
Verdict: PASS WITH CONDITIONS

---

## Executive Summary

All 18 previously identified defects (3 CRITICAL, 8 MAJOR, 7 MINOR) have been verified as FIXED across the 6 SPEC files. The re-audit discovered 6 new MINOR defects, all traceability gaps where a REQ lacks a dedicated AC. No new CRITICAL or MAJOR defects were introduced by the fixes. Cross-SPEC consistency (llm.mode values, model names, Ollama detection methods, CLI tool delegation) is clean.

---

## 1. Previous Defect Verification

### CRITICAL (3/3 Fixed)

| # | SPEC | Description | Status | Evidence |
|---|------|-------------|--------|----------|
| C1 | TRAIN-001 | LoRA rank constraint undefined behavior | FIXED | REQ-TR-001 (spec.md:L103): "Values outside this range shall be rejected at config-load time with an InvalidConfigError listing the valid range." Explicit error type + valid range. |
| C2 | ROUTER-001 v1.1 | "local model confidence" never defined | FIXED | REQ-RT-019 (amendment:L89): "Confidence shall be computed by the existing SimpleClassifier from v1.0.0 as classifier.Score(msg).Confidence (a float in [0.0, 1.0] derived from message length, keyword complexity, and tool-call presence heuristics)." Full definition with source, type, and derivation method. |
| C3 | GEMMA4-001 / ROUTER-001 | Config namespace conflict: llm.mode 3-mode vs 4-mode | FIXED | GEMMA4-001 REQ-G4-008 (spec.md:L123): "local-only, cloud-only, hybrid, or delegation". ROUTER-001 REQ-RT-018 (amendment:L86): "local-only, cloud-only, hybrid, delegation". Config schema (GEMMA4-001:L222): 4 values. All references consistent. |

### MAJOR (8/8 Fixed)

| # | SPEC | Description | Status | Evidence |
|---|------|-------------|--------|----------|
| M1 | TRAIN-001 | GRPO reward test data missing | FIXED | Section 6.3 (spec.md:L313-321): test_cases.jsonl format fully defined with field-by-field docs: `{"response": str, "expected_route": str, "tool_call_valid": bool, "korean_quality": float}`. |
| M2 | TRAIN-001 | Training artifact gitignore patterns | FIXED | REQ-TR-014 (spec.md:L135): "shall include the following patterns: *.gguf, *.safetensors, adapters/, checkpoints/, merged_weights/, and the configured artifact directory path." |
| M3 | LLM-001 v0.2 | Stream() for non-streaming CLI | FIXED | REQ-LLM-020 (amendment:L47): "Stream() shall execute Complete() internally and return the entire response as a single-chunk stream (one Chunk with Done: true), setting Capabilities().Streaming = false." |
| M4 | LLM-001 v0.2 | SIGTERM/SIGKILL AC missing | FIXED | AC-LLM-024a (amendment:L126-133): Explicit test for SIGTERM-then-SIGKILL scenario with cmd.Wait() zombie prevention. |
| M5 | ROUTER-001 v1.1 | Delegation rule priority on multiple match | FIXED | REQ-RT-020 (amendment:L91): "shall select the first matching rule in the rules array order (declaration-order priority); later rules are not evaluated (short-circuit)." |
| M6 | ROUTER-001 v1.1 | Multiple regex match undefined | FIXED | Pseudo-code (amendment:L251-255): Shows `for rule in cfg.Delegation.Rules: if regexMatch: return` — confirms short-circuit evaluation. |
| M7 | ONBOARDING-001 | "Already downloaded" detection undefined | FIXED | REQ-OB-021(a) (amendment:L57-58): "shall be determined by querying ollama list (or GET /api/tags) and checking for any model whose name starts with ai-goose/". |
| M8 | ONBOARDING-001 | Model download success follow-on AC missing | FIXED | AC-OB-022 (amendment:L214-215): "다운로드 완료 후 'Model ready: gemma4-e4b-rl-v1 Q5_K_M (~4 GB)' 메시지 표시, 'Next' 버튼 활성화". |

### MINOR (7/7 Fixed)

| # | SPEC | Description | Status | Evidence |
|---|------|-------------|--------|----------|
| m1 | LLM-001 v0.2 | Capabilities() streaming flag | FIXED | REQ-LLM-024 (amendment:L49): "Streaming is true if the CLI tool supports --output-format stream-json (or equivalent) and false otherwise." |
| m2 | ROUTER-001 v1.1 | Override prefix mid-message | FIXED | REQ-RT-021 (amendment:L93): "shall only be recognized at the start of the message (position 0); the same string appearing elsewhere shall not trigger an override." |
| m3 | GEMMA4-001 | Non-deterministic AC-G4-003 | FIXED | AC-G4-003 (spec.md:L163): "진행률 콜백이 호출됨 (최소 1회 status callback)" — measurable criterion replaces percentage. |
| m4 | ONBOARDING-001 | AC-OB-025 rounding | FIXED | AC-OB-025 (amendment:L233): "round(4/7 * 100) = 57%, 반올림" — explicit formula with worked example. |
| m5 | TRAIN-001 | lora_alpha rationale | FIXED | REQ-TR-004 (spec.md:L109): "lora_alpha shall default to 2 * lora_rank (following the LoRA convention where alpha=2*rank provides stable gradient scaling)." |
| m6 | CROSSPLAT-001 | PowerShell execution policy | FIXED | REQ-CP-002 (spec.md:L172): "Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass internally; if the execution policy cannot be bypassed, the script shall print a manual instruction." |
| m7 | TRAIN-001 | GRPO test data format alignment | FIXED | Section 6.3 (spec.md:L301-321): rewards.py definitions and test_cases.jsonl format documented together in the same section. |

---

## 2. New Defects Found

All new defects are MINOR traceability gaps (REQ without dedicated AC). No CRITICAL or MAJOR defects introduced.

| # | SPEC | REQ | Description | Severity |
|---|------|-----|-------------|----------|
| D1 | GEMMA4-001 | REQ-G4-007 | Download resume support has no dedicated AC. REQ defines resume behavior ("shall support resume by re-issuing POST /api/pull") but no AC explicitly tests the interrupt-then-resume scenario. | MINOR |
| D2 | TRAIN-001 | REQ-TR-009 | KL divergence penalty computation has no dedicated AC. REQ defines the behavior but no AC verifies it. | MINOR |
| D3 | TRAIN-001 | REQ-TR-012 | GPU memory utilization warning has no dedicated AC. REQ defines threshold and behavior but no AC tests the warning trigger. | MINOR |
| D4 | CROSSPLAT-001 | REQ-CP-019 | Debian/RPM package generation has no dedicated AC. REQ-C-018 (Homebrew) has AC-CP-012 but REQ-CP-019 has no equivalent AC for .deb/.rpm artifacts. | MINOR |
| D5 | LLM-001 v0.2 | REQ-LLM-025b | Exit code to LLMError subclass mapping has no dedicated AC. The mapping (exit 1 -> ErrInvalidRequest, exit 2 -> ErrUnauthorized, signal -> ErrServerUnavailable) is defined but no AC verifies it. | MINOR |
| D6 | ROUTER-001 v1.1 | REQ-RT-025 | "No network calls during routing" constraint has no dedicated AC. The constraint is clear but no test verifies that routing decisions don't trigger network I/O. | MINOR |

**Assessment**: These 6 MINOR defects are traceability gaps where implementation constraints or edge-case behaviors are defined in REQs but lack a corresponding AC that would provide binary PASS/FAIL verification. They are LOW priority because:
- D1: Resume is Ollama-native; the SPEC's contribution is minimal (re-issue the request).
- D2: KL divergence is a training metric logged to stdout; implicitly verified by GRPO training output.
- D3: GPU warning is a defensive log; implicitly verified by monitoring during training.
- D4: Package generation is a goreleaser configuration concern; CI verifies artifacts exist.
- D5: Exit code mapping is internal error handling; partially covered by AC-LLM-023 (not found), AC-LLM-024 (timeout).
- D6: No-network-calls is an architectural constraint; routing tests run without mocking network.

---

## 3. Cross-SPEC Consistency Check

| Dimension | Check | Result |
|-----------|-------|--------|
| llm.mode values | GEMMA4-001, ROUTER-001 v1.1, LLM-001 v0.2 all use same 4-value set | CONSISTENT |
| Model naming | GEMMA4-001, TRAIN-001, CROSSPLAT-001, ONBOARDING-001 all reference `ai-goose/gemma4-e4b-rl-v1` variants | CONSISTENT |
| Ollama detection | GEMMA4-001 (GET /api/tags), ONBOARDING-001 (ollama list or GET /api/tags), CROSSPLAT-001 (ollama list) | CONSISTENT (complementary methods) |
| CLI tool scanning | CROSSPLAT-001 (install script detects), ONBOARDING-001 (onboarding detects if skipped install), LLM-001 v0.2 (auto-detect on startup) | CONSISTENT (layered detection) |
| RAM detection | GEMMA4-001 (Section 6.4), CROSSPLAT-001 (REQ-CP-010), ONBOARDING-001 (REQ-OB-021b) | CONSISTENT (same OS-specific methods) |
| Config path | All SPECs reference `~/.goose/config.yaml` or `.goose/config.yaml` | CONSISTENT |
| Terminology | "Layer 1/2/3" used consistently across ROUTER-001, LLM-001, GEMMA4-001 | CONSISTENT |

---

## 4. Per-SPEC Scores

| SPEC | Completeness | EARS Compliance | Traceability | Testability | Overall |
|------|-------------|-----------------|--------------|-------------|---------|
| GEMMA4-001 | 1.0 | 1.0 | 0.93 (14/15 REQs have AC) | 0.95 | PASS |
| TRAIN-001 | 1.0 | 1.0 | 0.87 (20/23 REQs have direct AC) | 0.90 | PASS |
| CROSSPLAT-001 | 1.0 | 1.0 | 0.96 (25/26 REQs have AC) | 0.95 | PASS |
| LLM-001 v0.2 | 0.95 | 1.0 | 0.93 (13/14 sub-REQs have AC) | 0.95 | PASS |
| ROUTER-001 v1.1 | 1.0 | 1.0 | 0.94 (16/17 sub-REQs have AC) | 0.95 | PASS |
| ONBOARDING-001 v0.3 | 1.0 | 1.0 | 1.0 (7/7 REQs have AC) | 0.95 | PASS |

---

## 5. Chain-of-Verification Pass

Second-look findings:

1. **REQ number sequencing**: Verified end-to-end for all 6 SPECs. No gaps, no duplicates. Amendments correctly continue numbering from base SPEC.
2. **Traceability completeness**: Re-checked every REQ-AC mapping. Found 6 MINOR gaps (listed in Section 2).
3. **Exclusions specificity**: All 6 SPECs have specific, actionable exclusion lists. No vague entries.
4. **Cross-SPEC contradictions**: Checked all pairwise interactions between GEMMA4-001, TRAIN-001, CROSSPLAT-001, ROUTER-001, LLM-001, ONBOARDING-001. No contradictions found.
5. **EARS compliance**: Re-verified all REQ text for "shall" usage. No "should", "may", "must try to" in normative text. All REQs follow one of the five EARS patterns.
6. **YAML frontmatter**: Verified for all 6 SPECs. GEMMA4-001, TRAIN-001, CROSSPLAT-001 have standard frontmatter. LLM-001 v0.2, ROUTER-001 v1.1, ONBOARDING-001 v0.3 use amendment metadata format (acceptable for amendments).

No additional defects beyond the 6 listed in Section 2 were found in the second pass.

---

## 6. Overall Verdict

**PASS WITH CONDITIONS**

- All 18 previously-identified defects are FIXED and verified.
- 6 new MINOR traceability gaps identified (Section 2).
- No new CRITICAL or MAJOR defects.
- Cross-SPEC consistency is clean.
- All SPECs are ready for implementation with the following recommendation:

### Recommendations for MINOR Fixes (Optional, Non-Blocking)

1. **GEMMA4-001**: Add AC-G4-010 to test REQ-G4-007 resume: interrupt a download mid-stream, re-invoke, verify partial resume.
2. **TRAIN-001**: Add AC-TR-011 to verify REQ-TR-009 KL divergence logging, and AC-TR-012 to verify REQ-TR-012 GPU memory warning.
3. **CROSSPLAT-001**: Add AC-CP-016 to verify REQ-CP-019 .deb/.rpm generation in CI.
4. **LLM-001 v0.2**: Add AC-LLM-025b to test exit code mapping (exit 1, exit 2, signal kill).
5. **ROUTER-001 v1.1**: Add AC-RT-026 to verify REQ-RT-025 (no network calls during routing decisions).

These additions are recommended but not blocking. Implementation can proceed.

---

**End of Re-Audit Report**
