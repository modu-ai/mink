// useOnboarding — React hook providing the single source of truth for wizard state.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.3
import { useState, useCallback } from "react";
import { installApi } from "@/lib/api";
import type { OnboardingState } from "@/types/onboarding";

export interface UseOnboardingResult {
  state: OnboardingState | null;
  loading: boolean;
  error: string | null;
  start: () => Promise<void>;
  submitStep: <T>(stepNumber: number, body: T) => Promise<void>;
  skipStep: (stepNumber: number) => Promise<void>;
  back: () => Promise<void>;
  complete: () => Promise<void>;
}

// @MX:ANCHOR: [AUTO] Single-instance hook consumed by App.tsx and all step components — fan_in >= 6.
// @MX:REASON: session_id and CSRF token are stored here; sharing via props would scatter state.
export function useOnboarding(): UseOnboardingResult {
  const [state, setState] = useState<OnboardingState | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const withLoading = useCallback(
    async (fn: () => Promise<OnboardingState>) => {
      setLoading(true);
      setError(null);
      try {
        const next = await fn();
        // Persist CSRF token only when the response actually carries one (start endpoint).
        // Mid-flow responses (submit / skip / back / complete / state) omit the field and
        // would otherwise clobber the stored token with undefined, breaking every
        // subsequent POST via the X-MINK-CSRF header mismatch.
        if (next.csrf_token) {
          installApi.setCsrfToken(next.csrf_token);
        }
        setState((prev) =>
          prev != null
            ? { ...prev, ...next, csrf_token: prev.csrf_token }
            : next
        );
        // Auto-complete: when all steps are done but persist hasn't happened yet,
        // call /complete immediately so the completion card appears without requiring
        // a separate user click. This keeps the E2E speedrun path synchronous.
        if (
          next.current_step > next.total_steps &&
          next.completed_at == null
        ) {
          const completed = await installApi.complete(next.session_id);
          setState((prev) =>
            prev != null
              ? { ...prev, ...completed, csrf_token: prev.csrf_token }
              : completed
          );
        }
      } catch (err) {
        const msg = err instanceof Error ? err.message : "Unknown error";
        setError(msg);
      } finally {
        setLoading(false);
      }
    },
    []
  );

  const start = useCallback(async () => {
    await withLoading(() => installApi.start());
  }, [withLoading]);

  const submitStep = useCallback(
    async <T>(stepNumber: number, body: T) => {
      if (!state) return;
      await withLoading(() =>
        installApi.submitStep(state.session_id, stepNumber, body)
      );
    },
    [state, withLoading]
  );

  const skipStep = useCallback(
    async (stepNumber: number) => {
      if (!state) return;
      await withLoading(() =>
        installApi.skipStep(state.session_id, stepNumber)
      );
    },
    [state, withLoading]
  );

  const back = useCallback(async () => {
    if (!state) return;
    await withLoading(() => installApi.back(state.session_id));
  }, [state, withLoading]);

  const complete = useCallback(async () => {
    if (!state) return;
    await withLoading(() => installApi.complete(state.session_id));
  }, [state, withLoading]);

  return { state, loading, error, start, submitStep, skipStep, back, complete };
}
