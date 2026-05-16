// useGeolocation — Web Geolocation API wrapper with timeout and error handling.
// SPEC: SPEC-MINK-LOCALE-001 amendment-v0.2 (AC-LC-020, AC-LC-021)
//
// Provides a controlled geolocation request lifecycle:
//   - "idle": initial state, request not yet issued
//   - "prompting": navigator.geolocation.getCurrentPosition in flight
//   - "success": coords resolved within timeoutMs
//   - "denied": PERMISSION_DENIED error from browser
//   - "timeout": request exceeded timeoutMs
//   - "unsupported": navigator.geolocation not available
//   - "error": any other GeolocationPositionError

// @MX:NOTE: [AUTO] useGeolocation wraps browser Geolocation API — the request() call triggers
//   a browser permission prompt that cannot be suppressed.
// @MX:REASON: Callers must display a privacy notice before invoking request() per PIPA/GDPR/CCPA
//   consent-before-access requirement (AC-LC-025).

import { useState, useCallback } from "react";

export interface GeolocationResult {
  lat: number;
  lng: number;
}

export interface UseGeolocationState {
  status:
    | "idle"
    | "prompting"
    | "success"
    | "denied"
    | "timeout"
    | "unsupported"
    | "error";
  coords: GeolocationResult | null;
  error: string | null;
}

export interface UseGeolocationOptions {
  timeoutMs?: number;
}

// @MX:ANCHOR: [AUTO] useGeolocation — consumed by Step1Locale and test harness; fan_in >= 3.
// @MX:REASON: Status transitions must stay stable — callers branch on status values directly.
export function useGeolocation(
  opts?: UseGeolocationOptions
): UseGeolocationState & { request: () => void } {
  const timeoutMs = opts?.timeoutMs ?? 5000;

  const [state, setState] = useState<UseGeolocationState>({
    status: "idle",
    coords: null,
    error: null,
  });

  const request = useCallback(() => {
    if (!navigator.geolocation) {
      setState({ status: "unsupported", coords: null, error: "Geolocation not supported" });
      return;
    }

    setState({ status: "prompting", coords: null, error: null });

    // Manual timeout via setTimeout because the Geolocation API timeout option
    // only counts from when the device starts acquiring — not from call time.
    let settled = false;
    const timer = setTimeout(() => {
      if (!settled) {
        settled = true;
        setState({ status: "timeout", coords: null, error: "Geolocation request timed out" });
      }
    }, timeoutMs);

    navigator.geolocation.getCurrentPosition(
      (pos) => {
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        setState({
          status: "success",
          coords: { lat: pos.coords.latitude, lng: pos.coords.longitude },
          error: null,
        });
      },
      (err) => {
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        if (err.code === err.PERMISSION_DENIED) {
          setState({ status: "denied", coords: null, error: "Location permission denied" });
        } else if (err.code === err.TIMEOUT) {
          setState({ status: "timeout", coords: null, error: "Geolocation request timed out" });
        } else {
          setState({ status: "error", coords: null, error: err.message });
        }
      },
      // Pass a generous timeout to the browser API as a backstop; our timer fires first.
      { timeout: timeoutMs + 500, enableHighAccuracy: false }
    );
  }, [timeoutMs]);

  return { ...state, request };
}
