// Step3CLI — CLI tools detection and selection step.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2 Step 3
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
import type { CLITool, CLIToolsDetection } from "@/types/onboarding";
import type { StepProps } from "./StepProps";

// Known CLI tools displayed in a fixed order for consistency.
const KNOWN_TOOLS = ["claude", "gemini", "codex"] as const;

export function Step3CLI({
  data,
  loading,
  submitStep,
  skipStep,
  back,
  canBack,
  canSkip,
}: StepProps) {
  // Build initial checked state from detected tools.
  const detectedMap = new Map<string, CLITool>(
    data.CLITools.DetectedTools.map((t) => [t.Name, t])
  );

  const [checked, setChecked] = useState<Record<string, boolean>>(
    Object.fromEntries(KNOWN_TOOLS.map((name) => [name, detectedMap.has(name)]))
  );
  const [localError, setLocalError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  function toggleTool(name: string) {
    setChecked((prev) => ({ ...prev, [name]: !prev[name] }));
  }

  async function handleSubmit() {
    setPending(true);
    setLocalError(null);
    try {
      // Only include tools that the user kept checked AND were originally detected.
      const selectedTools = data.CLITools.DetectedTools.filter(
        (t) => checked[t.Name]
      );
      const body: CLIToolsDetection = { DetectedTools: selectedTools };
      await submitStep<CLIToolsDetection>(3, body);
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "오류가 발생했습니다.");
    } finally {
      setPending(false);
    }
  }

  async function handleSkip() {
    setPending(true);
    setLocalError(null);
    try {
      await skipStep(3);
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "오류가 발생했습니다.");
    } finally {
      setPending(false);
    }
  }

  const isBusy = loading || pending;

  return (
    <Card className="w-full max-w-lg mx-auto">
      <CardHeader>
        <CardTitle>Step 3: CLI 도구 / CLI Tools</CardTitle>
        <CardDescription>
          감지된 AI CLI 도구를 확인하세요. 필요 없는 항목의 체크를 해제할 수
          있습니다.
          <br />
          Review detected AI CLI tools. Uncheck any you do not want MINK to
          use.
        </CardDescription>
      </CardHeader>

      <CardContent className="space-y-4">
        {KNOWN_TOOLS.map((name) => {
          const tool = detectedMap.get(name);
          const isDetected = tool != null;
          return (
            <div
              key={name}
              className={[
                "flex items-start gap-3 rounded-lg border p-4",
                isDetected ? "border-border" : "border-dashed opacity-50",
              ].join(" ")}
            >
              <Checkbox
                id={`tool-${name}`}
                checked={checked[name] ?? false}
                onCheckedChange={() => toggleTool(name)}
                disabled={!isDetected || isBusy}
                aria-label={name}
              />
              <div className="flex-1 min-w-0">
                <Label
                  htmlFor={`tool-${name}`}
                  className="font-medium capitalize cursor-pointer"
                >
                  {name}
                </Label>
                {isDetected && tool != null ? (
                  <div className="mt-1 space-y-0.5">
                    <p className="text-xs font-mono text-muted-foreground truncate">
                      {tool.Path}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      v{tool.Version}
                    </p>
                  </div>
                ) : (
                  <p className="mt-1 text-xs text-muted-foreground">
                    감지되지 않음 / Not detected
                  </p>
                )}
              </div>
            </div>
          );
        })}

        {data.CLITools.DetectedTools.length === 0 && (
          <p className="text-xs text-muted-foreground text-center py-2">
            감지된 CLI 도구가 없습니다. 건너뛰기를 눌러 계속하세요.
            <br />
            No CLI tools detected. Press Skip to continue.
          </p>
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
          {canSkip && (
            <Button variant="ghost" onClick={handleSkip} disabled={isBusy}>
              건너뛰기 / Skip
            </Button>
          )}
          <Button onClick={handleSubmit} disabled={isBusy}>
            {isBusy ? "처리 중..." : "Continue / 다음"}
          </Button>
        </div>
      </CardFooter>
    </Card>
  );
}
