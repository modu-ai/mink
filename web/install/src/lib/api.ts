// Typed fetch client for the MINK install wizard API.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.3 (Web UI HTTP API)
// All POST requests send the X-MINK-CSRF header (double-submit pattern) and
// credentials: 'include' so the SameSite=Strict cookie is forwarded.
import type {
  OnboardingState,
  ApiError,
  LocaleProbeRequest,
  LocaleProbeResponse,
} from "@/types/onboarding";

// @MX:ANCHOR: [AUTO] Central API client consumed by useOnboarding hook and all step components — fan_in >= 5.
// @MX:REASON: CSRF token lifecycle and session_id routing are managed here; changes break all callers.
export class InstallApi {
  private readonly baseUrl: string;
  private csrfToken: string = "";

  constructor(baseUrl = "/install/api") {
    this.baseUrl = baseUrl;
  }

  // setCsrfToken is called by the hook after /start succeeds.
  setCsrfToken(token: string): void {
    this.csrfToken = token;
  }

  // start creates a new onboarding session. No CSRF header needed (first request).
  async start(): Promise<OnboardingState> {
    const res = await fetch(`${this.baseUrl}/session/start`, {
      method: "POST",
      credentials: "include",
    });
    return this.handleResponse(res);
  }

  // getState retrieves the current session state.
  async getState(sessionId: string): Promise<OnboardingState> {
    const res = await fetch(`${this.baseUrl}/session/${sessionId}/state`, {
      credentials: "include",
    });
    return this.handleResponse(res);
  }

  // submitStep submits step-specific data and returns the updated state.
  async submitStep<T>(
    sessionId: string,
    stepNumber: number,
    body: T
  ): Promise<OnboardingState> {
    const res = await fetch(
      `${this.baseUrl}/session/${sessionId}/step/${stepNumber}/submit`,
      {
        method: "POST",
        headers: this.postHeaders(),
        credentials: "include",
        body: JSON.stringify(body),
      }
    );
    return this.handleResponse(res);
  }

  // skipStep skips the current step (not allowed for step 7).
  async skipStep(
    sessionId: string,
    stepNumber: number
  ): Promise<OnboardingState> {
    const res = await fetch(
      `${this.baseUrl}/session/${sessionId}/step/${stepNumber}/skip`,
      {
        method: "POST",
        headers: this.postHeaders(),
        credentials: "include",
      }
    );
    return this.handleResponse(res);
  }

  // back navigates to the previous step.
  async back(sessionId: string): Promise<OnboardingState> {
    const res = await fetch(`${this.baseUrl}/session/${sessionId}/back`, {
      method: "POST",
      headers: this.postHeaders(),
      credentials: "include",
    });
    return this.handleResponse(res);
  }

  // pullStreamUrl returns the SSE endpoint URL for ollama pull progress.
  // Used by usePullProgress; EventSource cannot set headers so no CSRF header is
  // sent — the backend accepts cookie auth + Origin check for this endpoint.
  pullStreamUrl(sessionId: string, model: string): string {
    return (
      `${this.baseUrl}/session/${encodeURIComponent(sessionId)}/pull/stream` +
      `?model=${encodeURIComponent(model)}`
    );
  }

  // complete finalises the onboarding session.
  async complete(sessionId: string): Promise<OnboardingState> {
    const res = await fetch(`${this.baseUrl}/session/${sessionId}/complete`, {
      method: "POST",
      headers: this.postHeaders(),
      credentials: "include",
    });
    return this.handleResponse(res);
  }

  // probeLocale calls POST /install/api/locale/probe to detect country/language/timezone.
  // Pass { lat, lng } for GPS-assisted detection; omit body or pass {} for IP-only fallback.
  // SPEC: SPEC-MINK-LOCALE-001 amendment-v0.2 (AC-LC-020, AC-LC-021)
  async probeLocale(
    sessionId: string,
    body: LocaleProbeRequest
  ): Promise<LocaleProbeResponse> {
    const res = await fetch(`${this.baseUrl}/locale/probe`, {
      method: "POST",
      headers: {
        ...this.postHeaders(),
        "X-MINK-Session": sessionId,
      },
      credentials: "include",
      body: JSON.stringify(body),
    });
    if (res.ok) {
      return res.json() as Promise<LocaleProbeResponse>;
    }
    let errBody: ApiError | undefined;
    try {
      errBody = (await res.json()) as ApiError;
    } catch {
      throw new Error(`HTTP ${res.status}: ${res.statusText}`);
    }
    const code = errBody?.error?.code ?? "unknown";
    const message = errBody?.error?.message ?? `HTTP ${res.status}`;
    throw Object.assign(new Error(message), { code });
  }

  private postHeaders(): Record<string, string> {
    return {
      "Content-Type": "application/json",
      "X-MINK-CSRF": this.csrfToken,
    };
  }

  // @MX:WARN: [AUTO] Async path without granular error typing — callers must handle ApiError.
  // @MX:REASON: Error body parsing can fail if backend returns non-JSON on 5xx.
  private async handleResponse(res: Response): Promise<OnboardingState> {
    if (res.ok) {
      return res.json() as Promise<OnboardingState>;
    }
    let errBody: ApiError | undefined;
    try {
      errBody = (await res.json()) as ApiError;
    } catch {
      throw new Error(`HTTP ${res.status}: ${res.statusText}`);
    }
    const code = errBody?.error?.code ?? "unknown";
    const message = errBody?.error?.message ?? `HTTP ${res.status}`;
    throw Object.assign(new Error(message), { code });
  }
}

// Singleton instance used by the useOnboarding hook.
export const installApi = new InstallApi();
