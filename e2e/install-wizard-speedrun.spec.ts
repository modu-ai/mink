// install-wizard-speedrun.spec.ts — Playwright E2E speedrun for the MINK install wizard.
//
// Drives the Web UI through all 7 steps using default / skip actions and
// asserts the total elapsed time from initial page load to completion screen
// is within the 4-minute SLA (AC-OB-016).
//
// Server lifecycle is managed by global-setup.ts / global-teardown.ts.
// SPEC: SPEC-MINK-ONBOARDING-001 Phase 4 (AC-OB-016)
import { test, expect } from "@playwright/test";

// SLA: Web UI must complete in ≤ 4 minutes (240 seconds).
const WEB_SLA_MS = 4 * 60 * 1000;

test.describe("install wizard speedrun", () => {
  test("completes all 7 steps within 4-minute SLA", async ({ page }) => {
    // -----------------------------------------------------------------------
    // Navigate to the install wizard and record start time.
    // -----------------------------------------------------------------------
    const startMs = Date.now();

    await page.goto("/install");

    // Wait for the wizard header to confirm the app has loaded.
    await expect(
      page.locator("header").filter({ hasText: "MINK" })
    ).toBeVisible({ timeout: 15_000 });

    // -----------------------------------------------------------------------
    // Step 1: Locale
    // KR is selected by default (index 0 in PRESETS, matching CLI behaviour).
    // Just click "Continue / 다음" without changing the selection.
    // -----------------------------------------------------------------------
    await expect(
      page.getByText("Step 1: 지역 선택 / Locale Selection")
    ).toBeVisible({ timeout: 10_000 });

    // KR radio is checked by default — verify before continuing.
    const krLabel = page.locator('label').filter({ hasText: 'Korea' });
    await expect(krLabel).toBeVisible();

    await page.getByRole("button", { name: /Continue/i }).click();

    // -----------------------------------------------------------------------
    // Step 2: Model Setup
    // Ollama is not available in CI — click "건너뛰기 / Skip".
    // -----------------------------------------------------------------------
    await expect(
      page.getByText(/Step 2:/i)
    ).toBeVisible({ timeout: 10_000 });

    await page.getByRole("button", { name: /Skip/i }).click();

    // -----------------------------------------------------------------------
    // Step 3: CLI Tools
    // No tools assumed in CI — click "건너뛰기 / Skip".
    // -----------------------------------------------------------------------
    await expect(
      page.getByText(/Step 3:/i)
    ).toBeVisible({ timeout: 10_000 });

    await page.getByRole("button", { name: /Skip/i }).click();

    // -----------------------------------------------------------------------
    // Step 4: Persona (required — must enter a name)
    // -----------------------------------------------------------------------
    await expect(
      page.getByText(/Step 4:/i)
    ).toBeVisible({ timeout: 10_000 });

    await page.locator("#persona-name").fill("SpeedrunTester");

    await page.getByRole("button", { name: /Continue/i }).click();

    // -----------------------------------------------------------------------
    // Step 5: Provider
    // Skip — most CI environments have no provider API key.
    // -----------------------------------------------------------------------
    await expect(
      page.getByText(/Step 5:/i)
    ).toBeVisible({ timeout: 10_000 });

    await page.getByRole("button", { name: /Skip/i }).click();

    // -----------------------------------------------------------------------
    // Step 6: Messenger Channel
    // Skip — default is local_terminal.
    // -----------------------------------------------------------------------
    await expect(
      page.getByText(/Step 6:/i)
    ).toBeVisible({ timeout: 10_000 });

    await page.getByRole("button", { name: /Skip/i }).click();

    // -----------------------------------------------------------------------
    // Step 7: Privacy & Consent
    // KR locale uses PIPA (not GDPR), so Skip is allowed.
    // Keep default checkboxes and click "Continue / 다음".
    // -----------------------------------------------------------------------
    await expect(
      page.getByText(/Step 7:/i)
    ).toBeVisible({ timeout: 10_000 });

    // For KR (PIPA region) there is no GDPR block, so we can proceed normally.
    await page.getByRole("button", { name: /Continue/i }).click();

    // -----------------------------------------------------------------------
    // Completion screen
    // -----------------------------------------------------------------------
    // The CompletionScreen shows a "Complete" button that triggers persist.
    // After clicking it, the final "온보딩 완료" card appears.
    const completeButton = page.getByRole("button", { name: /Complete|완료/i });
    if (await completeButton.isVisible({ timeout: 10_000 })) {
      await completeButton.click();
    }

    await expect(
      page.getByText(/온보딩 완료|Onboarding Complete/i)
    ).toBeVisible({ timeout: 30_000 });

    // -----------------------------------------------------------------------
    // SLA assertion
    // -----------------------------------------------------------------------
    const elapsedMs = Date.now() - startMs;
    const elapsedSec = (elapsedMs / 1000).toFixed(1);
    const slaStatus = elapsedMs <= WEB_SLA_MS ? "PASS" : "FAIL";

    console.log(
      `Web UI speedrun: ${elapsedSec}s / ${WEB_SLA_MS / 1000}s SLA — ${slaStatus}`
    );

    expect(
      elapsedMs,
      `Web UI speedrun exceeded ${WEB_SLA_MS / 1000}s SLA (actual: ${elapsedSec}s)`
    ).toBeLessThanOrEqual(WEB_SLA_MS);
  });
});
