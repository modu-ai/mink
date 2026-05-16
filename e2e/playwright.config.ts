// playwright.config.ts — Playwright configuration for the MINK install wizard speedrun.
// Single project (Chromium), 1 worker, headless, SLA: 4 minutes total.
// SPEC: SPEC-MINK-ONBOARDING-001 Phase 4 (AC-OB-016)
import { defineConfig, devices } from "@playwright/test";

// The mink server URL is written by globalSetup and read by tests via env var.
// Default fallback is used for local runs where the server was started externally.
export default defineConfig({
  testDir: ".",
  testMatch: "**/*.spec.ts",

  // SLA budget: 4 minutes per test (Web UI ≤ 4 min).
  timeout: 4 * 60 * 1000,
  expect: {
    timeout: 10_000,
  },

  // One worker: the mink server is a single process — no parallelism needed.
  workers: 1,
  fullyParallel: false,

  // Retry once on CI to tolerate transient port binding delays.
  retries: process.env.CI ? 1 : 0,

  reporter: process.env.CI
    ? [["line"], ["html", { open: "never" }]]
    : [["line"]],

  use: {
    // Browser URL is populated by globalSetup into MINK_WEB_BASE_URL.
    baseURL: process.env.MINK_WEB_BASE_URL ?? "http://127.0.0.1:18080",
    headless: true,
    screenshot: "only-on-failure",
    video: "off",
    trace: "off",
  },

  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],

  // globalSetup spawns the mink server; globalTeardown kills it.
  globalSetup: "./global-setup.ts",
  globalTeardown: "./global-teardown.ts",
});
