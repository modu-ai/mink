// Step1Locale.test.tsx — Vitest + RTL test suite for locale auto-detection flow.
// SPEC: SPEC-MINK-LOCALE-001 amendment-v0.2
// Covers: AC-LC-020 (GPS→probe→high badge), AC-LC-021 (denied→IP fallback),
//         AC-LC-024 (manual fallback), AC-LC-025 (privacy notice DOM),
//         GPS timeout path, submit callback.

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Step1Locale } from "../Step1Locale";

// ---------------------------------------------------------------------------
// Helpers to mock navigator.geolocation
// ---------------------------------------------------------------------------

type GeoSuccessCallback = (pos: GeolocationPosition) => void;
type GeoErrorCallback = (err: GeolocationPositionError) => void;

function mockGeoSuccess(lat: number, lng: number) {
  Object.defineProperty(navigator, "geolocation", {
    configurable: true,
    value: {
      getCurrentPosition: vi.fn((success: GeoSuccessCallback) => {
        success({
          coords: { latitude: lat, longitude: lng, accuracy: 10 },
          timestamp: Date.now(),
        } as GeolocationPosition);
      }),
    },
  });
}

function mockGeoDenied() {
  Object.defineProperty(navigator, "geolocation", {
    configurable: true,
    value: {
      getCurrentPosition: vi.fn(
        (_success: GeoSuccessCallback, error: GeoErrorCallback) => {
          error({
            code: 1, // PERMISSION_DENIED
            message: "Permission denied",
            PERMISSION_DENIED: 1,
            POSITION_UNAVAILABLE: 2,
            TIMEOUT: 3,
          } as GeolocationPositionError);
        }
      ),
    },
  });
}

function mockGeoTimeout(timeoutMs = 6000) {
  // getCurrentPosition never calls either callback — the component's own timer fires first.
  Object.defineProperty(navigator, "geolocation", {
    configurable: true,
    value: {
      getCurrentPosition: vi.fn(() => {
        // Intentionally silent — useGeolocation's manual timeout will fire at 5000 ms.
        return;
      }),
    },
  });
  return timeoutMs;
}

// ---------------------------------------------------------------------------
// Helpers to mock fetch for /probe endpoint
// ---------------------------------------------------------------------------

function mockProbeResponse(body: object, status = 200) {
  global.fetch = vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? "OK" : "Error",
    json: () => Promise.resolve(body),
  } as Response);
}

// ---------------------------------------------------------------------------
// Default props
// ---------------------------------------------------------------------------

const defaultProps = {
  loading: false,
  error: null,
  sessionId: "test-session-id",
  onSubmit: vi.fn().mockResolvedValue(undefined),
};

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("Step1Locale", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  // -------------------------------------------------------------------------
  // Scenario 1: GPS success → response country="KR" → pre-select KR + high badge
  // AC-LC-020
  // -------------------------------------------------------------------------
  it("GPS success → country KR → KR radio selected + high badge visible", async () => {
    mockGeoSuccess(37.5665, 126.978); // Seoul coordinates
    mockProbeResponse({
      country: "KR",
      language: "ko",
      timezone: "Asia/Seoul",
      accuracy: "high",
    });

    render(<Step1Locale {...defaultProps} />);

    // Wait for the probe response to be processed.
    await waitFor(() => {
      const badge = screen.getByTestId("accuracy-badge");
      expect(badge).toBeInTheDocument();
      expect(badge).toHaveAttribute("data-accuracy", "high");
    });

    // KR radio should be checked.
    const radios = screen.getAllByRole("radio");
    // KR is index 0 in PRESETS.
    expect(radios[0]).toBeChecked();
  });

  // -------------------------------------------------------------------------
  // Scenario 2: Permission denied → IP fallback → medium badge
  // AC-LC-021
  // -------------------------------------------------------------------------
  it("Geolocation permission denied → IP fallback called → medium badge visible", async () => {
    mockGeoDenied();
    mockProbeResponse({
      country: "KR",
      language: "ko",
      timezone: "Asia/Seoul",
      accuracy: "medium",
    });

    render(<Step1Locale {...defaultProps} />);

    await waitFor(() => {
      const badge = screen.getByTestId("accuracy-badge");
      expect(badge).toHaveAttribute("data-accuracy", "medium");
    });

    // fetch should have been called once (IP-only, no lat/lng in body).
    expect(global.fetch).toHaveBeenCalledTimes(1);
    const callArgs = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    const body = JSON.parse(callArgs[1].body as string);
    // IP fallback sends empty object — no lat/lng keys.
    expect(body).not.toHaveProperty("lat");
    expect(body).not.toHaveProperty("lng");
  });

  // -------------------------------------------------------------------------
  // Scenario 3: GPS timeout (> 5 s) → IP fallback
  // AC-LC-021 (timeout path)
  // Strategy: use fake timers to advance past the 5000 ms threshold, then
  // flush all pending promises so the IP fallback completes.
  // -------------------------------------------------------------------------
  it(
    "GPS timeout → IP fallback triggered",
    async () => {
      vi.useFakeTimers({ shouldAdvanceTime: false });

      mockGeoTimeout();
      mockProbeResponse({
        country: "US",
        language: "en",
        timezone: "America/New_York",
        accuracy: "medium",
      });

      render(<Step1Locale {...defaultProps} />);

      // Advance past the 5000 ms timeout in useGeolocation and flush microtasks.
      await act(async () => {
        vi.advanceTimersByTime(5100);
        // Flush pending microtasks/promises so the state updates propagate.
        await Promise.resolve();
        await Promise.resolve();
      });

      // Restore real timers before waitFor so its internal polling works.
      vi.useRealTimers();

      await waitFor(() => {
        const badge = screen.getByTestId("accuracy-badge");
        expect(badge).toHaveAttribute("data-accuracy", "medium");
      });

      // US preset (index 1) should be selected.
      const radios = screen.getAllByRole("radio");
      expect(radios[1]).toBeChecked();
    },
    15000 // generous test timeout: fake timer advance + async probe
  );

  // -------------------------------------------------------------------------
  // Scenario 4: All detection failed (accuracy="manual") → failure notice + KR default
  // AC-LC-024
  // -------------------------------------------------------------------------
  it("probe returns accuracy=manual → failure notice shown + KR default selected", async () => {
    mockGeoDenied();
    mockProbeResponse({
      country: "XX",
      language: "xx",
      timezone: "UTC",
      accuracy: "manual",
    });

    render(<Step1Locale {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByTestId("detect-failed-notice")).toBeInTheDocument();
    });

    // Accuracy badge should show manual.
    const badge = screen.getByTestId("accuracy-badge");
    expect(badge).toHaveAttribute("data-accuracy", "manual");

    // KR (index 0) should be the default.
    const radios = screen.getAllByRole("radio");
    expect(radios[0]).toBeChecked();
  });

  // -------------------------------------------------------------------------
  // Scenario 5: Privacy notice DOM presence
  // AC-LC-025
  // -------------------------------------------------------------------------
  it("privacy notice is rendered in DOM before any GPS prompt", () => {
    // Use a never-resolving geolocation to ensure notice renders synchronously.
    Object.defineProperty(navigator, "geolocation", {
      configurable: true,
      value: {
        getCurrentPosition: vi.fn(() => {
          // Never calls back — simulates waiting.
        }),
      },
    });

    render(<Step1Locale {...defaultProps} />);

    const notice = screen.getByTestId("privacy-notice");
    expect(notice).toBeInTheDocument();
    // Verify key privacy text is present.
    expect(notice).toHaveTextContent("저장되지 않습니다");
    expect(notice).toHaveTextContent("not stored");
    expect(notice).toHaveTextContent("PIPA");
    expect(notice).toHaveTextContent("GDPR");
    expect(notice).toHaveTextContent("CCPA");
  });

  // -------------------------------------------------------------------------
  // Scenario 6: Submit → onSubmit called with selected LocaleChoice
  // -------------------------------------------------------------------------
  it("clicking Continue calls onSubmit with the selected locale", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    mockGeoDenied();
    mockProbeResponse({
      country: "FR",
      language: "fr",
      timezone: "Europe/Paris",
      accuracy: "medium",
    });

    render(<Step1Locale {...defaultProps} onSubmit={onSubmit} />);

    // Wait for detection to finish so the button is enabled.
    await waitFor(() => {
      expect(screen.getByTestId("accuracy-badge")).toBeInTheDocument();
    });

    // Select FR (index 2).
    const radios = screen.getAllByRole("radio");
    await userEvent.click(radios[2]);

    // Click Continue.
    const continueBtn = screen.getByRole("button", { name: /continue/i });
    await userEvent.click(continueBtn);

    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        Country: "FR",
        Language: "fr",
        Timezone: "Europe/Paris",
      })
    );
  });
});
