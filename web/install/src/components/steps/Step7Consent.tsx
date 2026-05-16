// Step7Consent — Privacy & consent flags step.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2 Step 7
// Includes GDPR-aware conditional flow when LegalFlags contains "GDPR".
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
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import type { ConsentFlags } from "@/types/onboarding";
import type { StepProps } from "./StepProps";

interface ConsentItem {
  key: keyof Omit<ConsentFlags, "GDPRExplicitConsent">;
  label: string;
  description: string;
  defaultValue: boolean;
}

const CONSENT_ITEMS: ConsentItem[] = [
  {
    key: "ConversationStorageLocal",
    label: "로컬 저장 / Local storage only",
    description: "대화를 로컬에만 저장합니다. 외부 서버로 전송되지 않습니다. / Conversations are stored locally only. Nothing is sent to external servers.",
    defaultValue: true,
  },
  {
    key: "LoRATrainingAllowed",
    label: "LoRA 학습 허용 / Allow LoRA training",
    description: "대화 데이터를 로컬 LoRA 파인튜닝에 사용하도록 허용합니다. / Allow conversation data to be used for local LoRA fine-tuning.",
    defaultValue: false,
  },
  {
    key: "TelemetryEnabled",
    label: "익명 통계 / Anonymous telemetry",
    description: "익명 사용 통계를 MINK 개선에 활용합니다. / Send anonymous usage statistics to help improve MINK.",
    defaultValue: false,
  },
  {
    key: "CrashReportingEnabled",
    label: "크래시 리포트 / Crash reporting",
    description: "크래시 발생 시 자동으로 리포트를 전송합니다. / Automatically send crash reports when errors occur.",
    defaultValue: false,
  },
];

function hasGdpr(legalFlags: string[]): boolean {
  return legalFlags.some((f) => f.toUpperCase() === "GDPR");
}

export function Step7Consent({
  data,
  loading,
  submitStep,
  skipStep,
  back,
  canBack,
}: StepProps) {
  const saved = data.Consent;
  const gdprRequired = hasGdpr(data.Locale.LegalFlags);

  const [flags, setFlags] = useState<Record<string, boolean>>(
    Object.fromEntries(
      CONSENT_ITEMS.map((item) => [
        item.key,
        saved[item.key] !== undefined ? (saved[item.key] as boolean) : item.defaultValue,
      ])
    )
  );
  // null = not selected yet (GDPR only)
  const [gdprConsent, setGdprConsent] = useState<boolean | null>(
    saved.GDPRExplicitConsent
  );
  const [localError, setLocalError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  function toggleFlag(key: string) {
    setFlags((prev) => ({ ...prev, [key]: !prev[key] }));
  }

  const isBusy = loading || pending;
  // Continue is blocked if GDPR is required but not yet selected.
  const canContinue = !gdprRequired || gdprConsent !== null;
  // Skip is blocked when GDPR is required.
  const canSkipStep = !gdprRequired;

  async function handleSubmit() {
    if (gdprRequired && gdprConsent === null) {
      setLocalError("GDPR 동의 여부를 선택해 주세요. / Please select your GDPR consent.");
      return;
    }
    setPending(true);
    setLocalError(null);
    try {
      const body: ConsentFlags = {
        ConversationStorageLocal: flags.ConversationStorageLocal ?? true,
        LoRATrainingAllowed: flags.LoRATrainingAllowed ?? false,
        TelemetryEnabled: flags.TelemetryEnabled ?? false,
        CrashReportingEnabled: flags.CrashReportingEnabled ?? false,
        GDPRExplicitConsent: gdprRequired ? gdprConsent : null,
      };
      await submitStep<ConsentFlags>(7, body);
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "오류가 발생했습니다.");
    } finally {
      setPending(false);
    }
  }

  async function handleSkip() {
    if (!canSkipStep) return;
    setPending(true);
    setLocalError(null);
    try {
      await skipStep(7);
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "오류가 발생했습니다.");
    } finally {
      setPending(false);
    }
  }

  return (
    <Card className="w-full max-w-lg mx-auto">
      <CardHeader>
        <CardTitle>Step 7: 개인정보 동의 / Privacy & Consent</CardTitle>
        <CardDescription>
          데이터 사용 방식을 선택하세요. 언제든지 설정에서 변경할 수 있습니다.
          <br />
          Choose how your data is used. You can change these settings anytime.
        </CardDescription>
      </CardHeader>

      <CardContent className="space-y-5">
        {/* GDPR notice */}
        {gdprRequired && (
          <div className="rounded-lg border border-amber-300 bg-amber-50 px-4 py-3">
            <p className="text-sm font-semibold text-amber-800">
              GDPR 지역 — 동의 절차 필수
            </p>
            <p className="text-xs text-amber-700 mt-1">
              귀하의 지역은 GDPR이 적용됩니다. 아래 GDPR 명시적 동의 항목을
              반드시 선택해야 합니다.
              <br />
              Your region is subject to GDPR. You must select your explicit
              GDPR consent below.
            </p>
          </div>
        )}

        {/* Consent checkboxes */}
        <div className="space-y-3">
          {CONSENT_ITEMS.map((item) => (
            <div key={item.key} className="flex items-start gap-3">
              <Checkbox
                id={`consent-${item.key}`}
                checked={flags[item.key] ?? false}
                onCheckedChange={() => toggleFlag(item.key)}
                disabled={isBusy}
                className="mt-0.5"
              />
              <div>
                <Label
                  htmlFor={`consent-${item.key}`}
                  className="cursor-pointer font-medium"
                >
                  {item.label}
                </Label>
                <p className="text-xs text-muted-foreground mt-0.5">
                  {item.description}
                </p>
              </div>
            </div>
          ))}
        </div>

        {/* GDPR explicit consent radio — only when in GDPR region */}
        {gdprRequired && (
          <div className="rounded-lg border border-primary/30 bg-primary/5 px-4 py-4 space-y-3">
            <p className="text-sm font-semibold text-primary">
              GDPR 명시적 동의 / GDPR Explicit Consent
            </p>
            <RadioGroup
              value={gdprConsent === null ? "" : String(gdprConsent)}
              onValueChange={(v) => setGdprConsent(v === "true")}
              disabled={isBusy}
              className="space-y-2"
            >
              <div className="flex items-center gap-2">
                <RadioGroupItem value="true" id="gdpr-accept" />
                <Label htmlFor="gdpr-accept" className="cursor-pointer">
                  동의합니다 / I consent
                </Label>
              </div>
              <div className="flex items-center gap-2">
                <RadioGroupItem value="false" id="gdpr-reject" />
                <Label htmlFor="gdpr-reject" className="cursor-pointer">
                  거부합니다 / I do not consent
                </Label>
              </div>
            </RadioGroup>
          </div>
        )}

        {localError != null && (
          <p className="text-xs text-destructive bg-destructive/10 border border-destructive/20 rounded px-3 py-2">
            {localError}
          </p>
        )}
      </CardContent>

      <CardFooter className="flex justify-between gap-2">
        <Button
          variant="outline"
          onClick={back}
          disabled={!canBack || isBusy}
        >
          뒤로 / Back
        </Button>
        <div className="flex gap-2">
          {canSkipStep && (
            <Button variant="ghost" onClick={handleSkip} disabled={isBusy}>
              건너뛰기 / Skip
            </Button>
          )}
          {gdprRequired && !canSkipStep && (
            <Button
              variant="ghost"
              disabled
              title="GDPR 지역에서는 Skip이 차단됩니다 / Skip is disabled in GDPR regions"
            >
              건너뛰기 불가 / Skip disabled
            </Button>
          )}
          <Button
            onClick={handleSubmit}
            disabled={!canContinue || isBusy}
          >
            {isBusy ? "처리 중..." : "Continue / 다음"}
          </Button>
        </div>
      </CardFooter>
    </Card>
  );
}
