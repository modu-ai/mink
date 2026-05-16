// Step1Locale — locale selection with auto-detection (GPS → IP → manual).
// SPEC: SPEC-MINK-LOCALE-001 amendment-v0.2 (AC-LC-020, AC-LC-021, AC-LC-024, AC-LC-025)
//
// Detection flow:
//   1. Show privacy notice (AC-LC-025: consent before access).
//   2. Request GPS via navigator.geolocation (useGeolocation hook, 5 s timeout).
//   3. On GPS success → POST /probe {lat, lng} → pre-select matching preset, badge = "high".
//   4. On GPS fail/deny/timeout → POST /probe {} (IP fallback) → pre-select, badge = "medium".
//   5. On /probe returning accuracy = "manual" → show failure notice, default KR, manual select.

// @MX:ANCHOR: [AUTO] Step1Locale — entry point for locale wizard step; fan_in >= 3 (App, tests, storybook).
// @MX:REASON: Auto-detection lifecycle (GPS → IP → manual) is the canonical implementation of
//   AC-LC-020/021/024; changes here break the full detection chain.

import { useState, useEffect, useCallback } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import type { LocaleChoice } from "@/types/onboarding";
import { useGeolocation } from "@/hooks/useGeolocation";
import { installApi } from "@/lib/api";

// Mirrors localePresets in internal/cli/install/tui.go — keep in sync.
interface LocalePreset extends LocaleChoice {
  _display: string;
}

const PRESETS: LocalePreset[] = [
  {
    Country: "KR",
    Language: "ko",
    Timezone: "Asia/Seoul",
    LegalFlags: ["PIPA"],
    _display: "Korea (한국어, Asia/Seoul)",
  },
  {
    Country: "US",
    Language: "en",
    Timezone: "America/New_York",
    LegalFlags: [],
    _display: "United States (English, America/New_York)",
  },
  {
    Country: "FR",
    Language: "fr",
    Timezone: "Europe/Paris",
    LegalFlags: ["GDPR"],
    _display: "France (français, Europe/Paris)",
  },
  {
    Country: "DE",
    Language: "de",
    Timezone: "Europe/Berlin",
    LegalFlags: ["GDPR"],
    _display: "Germany (Deutsch, Europe/Berlin)",
  },
];

type AccuracyLevel = "high" | "medium" | "manual" | null;

// Find the preset index that best matches the probe response country.
// Falls back to KR (index 0) when no match found.
function matchPreset(country: string): number {
  const idx = PRESETS.findIndex(
    (p) => p.Country.toLowerCase() === country.toLowerCase()
  );
  return idx >= 0 ? idx : 0;
}

function hasGdpr(preset: LocalePreset): boolean {
  return preset.LegalFlags.includes("GDPR");
}

interface Step1LocaleProps {
  loading: boolean;
  error: string | null;
  sessionId: string;
  onSubmit: (locale: LocaleChoice) => Promise<void>;
}

// @MX:NOTE: [AUTO] sessionId is required for /probe endpoint CSRF session correlation.
//   App.tsx must pass state.session_id as sessionId prop after /start succeeds.

export function Step1Locale({
  loading,
  error,
  sessionId,
  onSubmit,
}: Step1LocaleProps) {
  // Default KR (index 0) — matches CLI behaviour; overridden by auto-detection.
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [accuracy, setAccuracy] = useState<AccuracyLevel>(null);
  const [detecting, setDetecting] = useState(false);
  const [detectFailed, setDetectFailed] = useState(false);

  const geo = useGeolocation({ timeoutMs: 5000 });

  // runIpFallback: POST /probe without coordinates → medium accuracy.
  const runIpFallback = useCallback(async () => {
    setDetecting(true);
    try {
      const result = await installApi.probeLocale(sessionId, {});
      if (result.accuracy === "manual") {
        setDetectFailed(true);
        setAccuracy("manual");
        setSelectedIndex(0); // KR default
      } else {
        setAccuracy(result.accuracy);
        setSelectedIndex(matchPreset(result.country));
      }
    } catch {
      // probe itself failed — treat as manual
      setDetectFailed(true);
      setAccuracy("manual");
      setSelectedIndex(0);
    } finally {
      setDetecting(false);
    }
  }, [sessionId]);

  // runGpsProbe: POST /probe with GPS coordinates → high accuracy.
  const runGpsProbe = useCallback(
    async (lat: number, lng: number) => {
      setDetecting(true);
      try {
        const result = await installApi.probeLocale(sessionId, { lat, lng });
        if (result.accuracy === "manual") {
          setDetectFailed(true);
          setAccuracy("manual");
          setSelectedIndex(0);
        } else {
          setAccuracy(result.accuracy);
          setSelectedIndex(matchPreset(result.country));
        }
      } catch {
        // GPS probe network error — fall through to IP fallback
        await runIpFallback();
      } finally {
        setDetecting(false);
      }
    },
    [sessionId, runIpFallback]
  );

  // React to geolocation status changes.
  // @MX:NOTE: [AUTO] Effect runs whenever geo.status transitions; GPS success triggers /probe with coords,
  //   any failure triggers IP fallback.
  useEffect(() => {
    if (geo.status === "success" && geo.coords) {
      runGpsProbe(geo.coords.lat, geo.coords.lng);
    } else if (
      geo.status === "denied" ||
      geo.status === "timeout" ||
      geo.status === "unsupported" ||
      geo.status === "error"
    ) {
      runIpFallback();
    }
  }, [geo.status, geo.coords, runGpsProbe, runIpFallback]);

  // On mount, request GPS automatically (privacy notice is shown before the request fires
  // because the component renders synchronously before the effect runs).
  useEffect(() => {
    geo.request();
    // Only run once on mount; geo.request is stable (useCallback).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const isDetecting = detecting || geo.status === "prompting";
  const selected = PRESETS[selectedIndex];

  async function handleSubmit() {
    const locale: LocaleChoice = {
      Country: selected.Country,
      Language: selected.Language,
      Timezone: selected.Timezone,
      LegalFlags: selected.LegalFlags,
    };
    await onSubmit(locale);
  }

  // Accuracy badge rendering.
  function AccuracyBadge() {
    if (accuracy === null) return null;
    const cfg =
      accuracy === "high"
        ? {
            label: "자동 감지 (정밀) / Auto-detected (high)",
            cls: "bg-green-100 text-green-800 border-green-300",
          }
        : accuracy === "medium"
          ? {
              label: "자동 감지 (IP) / Auto-detected (medium)",
              cls: "bg-amber-100 text-amber-800 border-amber-300",
            }
          : {
              label: "수동 선택 / Manual",
              cls: "bg-gray-100 text-gray-700 border-gray-300",
            };
    return (
      <span
        data-testid="accuracy-badge"
        data-accuracy={accuracy}
        className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold ${cfg.cls}`}
      >
        {cfg.label}
      </span>
    );
  }

  return (
    <Card className="w-full max-w-lg mx-auto">
      <CardHeader>
        <CardTitle>Step 1: 지역 선택 / Locale Selection</CardTitle>
        <CardDescription>
          MINK이 사용할 언어와 시간대를 선택하세요.
          <br />
          Select the language and timezone MINK will use.
        </CardDescription>
      </CardHeader>

      <CardContent className="space-y-4">
        {/* Privacy notice — rendered before any GPS prompt fires (AC-LC-025). */}
        <div
          data-testid="privacy-notice"
          className="rounded-md border border-blue-200 bg-blue-50 px-4 py-3 text-xs text-blue-800"
        >
          <p className="font-medium mb-1">
            위치 정보 사용 안내 / Location Usage Notice
          </p>
          <p>
            위치 정보는 지역 자동 감지에만 사용되며 저장되지 않습니다.
            <br />
            Location is used only for region auto-detection and is not stored.
          </p>
          <p className="mt-1 text-blue-600">
            PIPA / GDPR / CCPA 준수 · Compliant
          </p>
        </div>

        {/* Detection status area */}
        {isDetecting && (
          <p
            data-testid="detecting-indicator"
            className="text-sm text-muted-foreground animate-pulse"
          >
            지역 자동 감지 중... / Detecting your region...
          </p>
        )}

        {/* Auto-detection failure notice (AC-LC-024) */}
        {detectFailed && (
          <p
            data-testid="detect-failed-notice"
            className="text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded px-3 py-2"
          >
            자동 감지 실패 — 아래에서 직접 선택해주세요.
            <br />
            Auto-detection failed. Please select your region manually.
          </p>
        )}

        {/* Accuracy badge */}
        {accuracy !== null && !isDetecting && (
          <div className="flex items-center gap-2">
            <AccuracyBadge />
          </div>
        )}

        {/* Preset radio list */}
        <div className="space-y-2" role="radiogroup" aria-label="Locale preset">
          {PRESETS.map((preset, idx) => (
            <label
              key={preset.Country}
              className={[
                "flex items-center gap-3 rounded-lg border p-4 cursor-pointer transition-colors",
                selectedIndex === idx
                  ? "border-primary bg-primary/5"
                  : "border-border hover:bg-accent",
              ].join(" ")}
            >
              <input
                type="radio"
                name="locale"
                value={idx}
                checked={selectedIndex === idx}
                onChange={() => setSelectedIndex(idx)}
                className="h-4 w-4 accent-primary"
                aria-label={preset._display}
              />
              <span className="text-sm font-medium">{preset._display}</span>
            </label>
          ))}
        </div>

        {/* GDPR notice for EU presets */}
        {hasGdpr(selected) && (
          <p className="text-xs text-amber-600 bg-amber-50 border border-amber-200 rounded px-3 py-2">
            GDPR 동의는 Step 7에서 필수입니다. GDPR consent is required in Step
            7.
          </p>
        )}

        {/* Error from parent (submit error) */}
        {error != null && (
          <p className="text-xs text-destructive bg-destructive/10 border border-destructive/20 rounded px-3 py-2">
            {error}
          </p>
        )}
      </CardContent>

      <CardFooter className="justify-end">
        <Button onClick={handleSubmit} disabled={loading || isDetecting}>
          {loading ? "처리 중..." : "Continue / 다음"}
        </Button>
      </CardFooter>
    </Card>
  );
}
