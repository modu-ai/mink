// locale-autodetect.spec.ts — Playwright E2E for Step 1 locale auto-detect (6 scenarios).
//
// Verifies the auto-detect flow added by SPEC-MINK-LOCALE-001 amendment-v0.2
// (AC-LC-020 / AC-LC-021 / AC-LC-024 / AC-LC-025) end-to-end through the real
// install server. The backend /install/api/locale/probe endpoint is intercepted
// per scenario via page.route() so we exercise frontend wiring without depending
// on external services (ipapi.co / Nominatim).
//
// Scenarios:
//   1. KR GPS high       — geolocation granted, probe returns KR/high
//   2. US GPS high       — geolocation granted (NYC coords), probe returns US/high
//   3. FR GPS high       — geolocation granted (Paris coords), probe returns FR/high
//   4. DE GPS high       — geolocation granted (Berlin coords), probe returns DE/high
//   5. Geo denied → IP fallback medium — permission denied, probe returns US/medium
//   6. All fail → manual — permission denied, probe returns manual
//
// SPEC: SPEC-MINK-LOCALE-001 amendment-v0.2 (AC-LC-020/021/024/025)
//       SPEC-MINK-ONBOARDING-001 amendment-v0.4 (AC-OB-021/022)
import { test, expect, Page, BrowserContext } from "@playwright/test";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

interface ProbeBody {
  country: string;
  language: string;
  timezone: string;
  accuracy: "high" | "medium" | "manual";
}

// Intercept POST /install/api/locale/probe and reply with the given body.
// Fulfils every request to the route until the test ends.
async function mockProbe(page: Page, body: ProbeBody): Promise<void> {
  await page.route("**/install/api/locale/probe", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(body),
    });
  });
}

// Make the probe endpoint fail (network / 5xx) — exercises the catch branch
// inside runGpsProbe and runIpFallback (treated as manual).
async function mockProbeFailure(page: Page): Promise<void> {
  await page.route("**/install/api/locale/probe", async (route) => {
    await route.fulfill({
      status: 500,
      contentType: "application/json",
      body: JSON.stringify({ error: { code: "probe_failed", message: "synthetic failure" } }),
    });
  });
}

// Navigate to /install and wait until Step 1 is interactive (header + privacy notice).
async function gotoStep1(page: Page): Promise<void> {
  await page.goto("/install");
  await expect(
    page.locator("header").filter({ hasText: "MINK" })
  ).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId("privacy-notice")).toBeVisible({ timeout: 10_000 });
}

// Grant geolocation permission and place the user at given coordinates.
async function grantGeo(
  context: BrowserContext,
  lat: number,
  lng: number
): Promise<void> {
  await context.grantPermissions(["geolocation"], { origin: process.env.MINK_WEB_BASE_URL ?? "http://127.0.0.1" });
  await context.setGeolocation({ latitude: lat, longitude: lng });
}

// Revoke all permissions so navigator.geolocation rejects with PERMISSION_DENIED.
async function denyGeo(context: BrowserContext): Promise<void> {
  await context.clearPermissions();
}

// Wait for the accuracy badge to settle to the expected level.
// The component flips from `detecting-indicator` to `accuracy-badge` once the
// probe resolves (typically < 200 ms with our mocked route).
async function expectAccuracy(page: Page, expected: "high" | "medium" | "manual"): Promise<void> {
  const badge = page.getByTestId("accuracy-badge");
  await expect(badge).toBeVisible({ timeout: 15_000 });
  await expect(badge).toHaveAttribute("data-accuracy", expected);
}

// ---------------------------------------------------------------------------
// Test suite — 6 scenarios
// ---------------------------------------------------------------------------

test.describe("Step 1 locale auto-detect (6 scenarios)", () => {
  // -------------------------------------------------------------------------
  // Scenario 1 — KR GPS high
  // -------------------------------------------------------------------------
  test("1. KR GPS high — Seoul coords + probe high → KR pre-selected", async ({
    page,
    context,
  }) => {
    await mockProbe(page, {
      country: "KR",
      language: "ko",
      timezone: "Asia/Seoul",
      accuracy: "high",
    });
    await grantGeo(context, 37.5665, 126.978); // Seoul
    await gotoStep1(page);

    await expectAccuracy(page, "high");

    // The KR preset (index 0) radio should be checked.
    const krLabel = page.locator("label").filter({ hasText: /Korea/ });
    await expect(krLabel).toBeVisible();
    const krRadio = krLabel.locator('input[type="radio"]');
    await expect(krRadio).toBeChecked();
  });

  // -------------------------------------------------------------------------
  // Scenario 2 — US GPS high
  // -------------------------------------------------------------------------
  test("2. US GPS high — NYC coords + probe high → US pre-selected", async ({
    page,
    context,
  }) => {
    await mockProbe(page, {
      country: "US",
      language: "en",
      timezone: "America/New_York",
      accuracy: "high",
    });
    await grantGeo(context, 40.7128, -74.006); // New York City
    await gotoStep1(page);

    await expectAccuracy(page, "high");

    const usLabel = page.locator("label").filter({ hasText: /United States/ });
    const usRadio = usLabel.locator('input[type="radio"]');
    await expect(usRadio).toBeChecked();
  });

  // -------------------------------------------------------------------------
  // Scenario 3 — FR GPS high
  // -------------------------------------------------------------------------
  test("3. FR GPS high — Paris coords + probe high → FR pre-selected (GDPR)", async ({
    page,
    context,
  }) => {
    await mockProbe(page, {
      country: "FR",
      language: "fr",
      timezone: "Europe/Paris",
      accuracy: "high",
    });
    await grantGeo(context, 48.8566, 2.3522); // Paris
    await gotoStep1(page);

    await expectAccuracy(page, "high");

    const frLabel = page.locator("label").filter({ hasText: /France/ });
    const frRadio = frLabel.locator('input[type="radio"]');
    await expect(frRadio).toBeChecked();
  });

  // -------------------------------------------------------------------------
  // Scenario 4 — DE GPS high
  // -------------------------------------------------------------------------
  test("4. DE GPS high — Berlin coords + probe high → DE pre-selected (GDPR)", async ({
    page,
    context,
  }) => {
    await mockProbe(page, {
      country: "DE",
      language: "de",
      timezone: "Europe/Berlin",
      accuracy: "high",
    });
    await grantGeo(context, 52.52, 13.405); // Berlin
    await gotoStep1(page);

    await expectAccuracy(page, "high");

    const deLabel = page.locator("label").filter({ hasText: /Germany/ });
    const deRadio = deLabel.locator('input[type="radio"]');
    await expect(deRadio).toBeChecked();
  });

  // -------------------------------------------------------------------------
  // Scenario 5 — Geo denied → IP fallback medium
  // -------------------------------------------------------------------------
  test("5. Geo denied → IP fallback medium — probe returns US/medium", async ({
    page,
    context,
  }) => {
    await mockProbe(page, {
      country: "US",
      language: "en",
      timezone: "America/New_York",
      accuracy: "medium",
    });
    await denyGeo(context);
    await gotoStep1(page);

    // After permission denial the useGeolocation hook transitions to "denied"
    // immediately; runIpFallback fires the probe → medium badge.
    await expectAccuracy(page, "medium");

    const usLabel = page.locator("label").filter({ hasText: /United States/ });
    const usRadio = usLabel.locator('input[type="radio"]');
    await expect(usRadio).toBeChecked();
  });

  // -------------------------------------------------------------------------
  // Scenario 6 — All fail → manual fallback (KR default)
  // -------------------------------------------------------------------------
  test("6. All fail → manual — denied geo + probe failure → manual badge", async ({
    page,
    context,
  }) => {
    await mockProbeFailure(page);
    await denyGeo(context);
    await gotoStep1(page);

    await expectAccuracy(page, "manual");

    // detect-failed-notice should be visible.
    await expect(page.getByTestId("detect-failed-notice")).toBeVisible();

    // KR (index 0) is the manual default per Step1Locale.tsx.
    const krLabel = page.locator("label").filter({ hasText: /Korea/ });
    const krRadio = krLabel.locator('input[type="radio"]');
    await expect(krRadio).toBeChecked();
  });
});
