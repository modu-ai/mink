// Step1Locale — fully implemented locale selection step.
// Matches localePresets from internal/cli/install/tui.go exactly.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2 Step 1
import { useState } from "react";
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

function hasGdpr(preset: LocalePreset): boolean {
  return preset.LegalFlags.includes("GDPR");
}

interface Step1LocaleProps {
  loading: boolean;
  error: string | null;
  onSubmit: (locale: LocaleChoice) => Promise<void>;
}

export function Step1Locale({ loading, error, onSubmit }: Step1LocaleProps) {
  // Default to KR (index 0) matching CLI behaviour.
  const [selectedIndex, setSelectedIndex] = useState(0);

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

      <CardContent className="space-y-3">
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
            />
            <span className="text-sm font-medium">{preset._display}</span>
          </label>
        ))}

        {hasGdpr(selected) && (
          <p className="text-xs text-amber-600 bg-amber-50 border border-amber-200 rounded px-3 py-2">
            GDPR 동의는 Step 7에서 필수입니다. GDPR consent is required in Step
            7.
          </p>
        )}

        {error != null && (
          <p className="text-xs text-destructive bg-destructive/10 border border-destructive/20 rounded px-3 py-2">
            {error}
          </p>
        )}
      </CardContent>

      <CardFooter className="justify-end">
        <Button onClick={handleSubmit} disabled={loading}>
          {loading ? "처리 중..." : "Continue / 다음"}
        </Button>
      </CardFooter>
    </Card>
  );
}
