// Step2Model — Ollama + model selection step.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2 Step 2
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { ModelSetup } from "@/types/onboarding";
import type { StepProps } from "./StepProps";

// Format bytes into a human-readable string (e.g., "16 GB").
function humanizeBytes(bytes: number): string {
  if (bytes <= 0) return "알 수 없음 / Unknown";
  const gb = bytes / (1024 * 1024 * 1024);
  if (gb >= 1) return `${gb.toFixed(1)} GB`;
  const mb = bytes / (1024 * 1024);
  return `${mb.toFixed(0)} MB`;
}

export function Step2Model({
  data,
  loading,
  submitStep,
  skipStep,
  back,
  canBack,
  canSkip,
}: StepProps) {
  const model = data.Model;
  const [selectedModel, setSelectedModel] = useState(
    model.DetectedModel || ""
  );
  const [localError, setLocalError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function handleSubmit() {
    setPending(true);
    setLocalError(null);
    try {
      const body: ModelSetup = {
        OllamaInstalled: model.OllamaInstalled,
        DetectedModel: model.DetectedModel,
        SelectedModel: selectedModel.trim() || model.DetectedModel,
        ModelSizeBytes: model.ModelSizeBytes,
        RAMBytes: model.RAMBytes,
      };
      await submitStep<ModelSetup>(2, body);
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
      await skipStep(2);
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "오류가 발생했습니다.");
    } finally {
      setPending(false);
    }
  }

  async function handleBack() {
    await back();
  }

  const isBusy = loading || pending;

  return (
    <Card className="w-full max-w-lg mx-auto">
      <CardHeader>
        <CardTitle>Step 2: 모델 설정 / Model Setup</CardTitle>
        <CardDescription>
          MINK이 사용할 LLM 모델을 선택하세요.
          <br />
          Choose the LLM model MINK will use.
        </CardDescription>
      </CardHeader>

      <CardContent className="space-y-5">
        {/* Ollama status indicator */}
        <div className="flex items-center gap-2 rounded-lg border p-4">
          <span
            className={
              model.OllamaInstalled ? "text-success" : "text-destructive"
            }
            aria-hidden="true"
          >
            {model.OllamaInstalled ? "✓" : "✗"}
          </span>
          <span className="text-sm font-medium">
            {model.OllamaInstalled
              ? "Ollama 설치됨 / Ollama installed"
              : "Ollama 미설치 / Ollama not installed"}
          </span>
        </div>

        {/* Detected hardware info */}
        {model.OllamaInstalled && (
          <div className="rounded-lg bg-muted/50 px-4 py-3 space-y-1 text-sm">
            <div className="flex justify-between">
              <span className="text-muted-foreground">감지된 모델 / Detected model</span>
              <span className="font-mono text-xs">{model.DetectedModel || "—"}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">RAM</span>
              <span className="font-mono text-xs">{humanizeBytes(model.RAMBytes)}</span>
            </div>
          </div>
        )}

        {/* Model selection input */}
        <div className="space-y-2">
          <Label htmlFor="selected-model">
            선택 모델 / Selected model
          </Label>
          <Input
            id="selected-model"
            placeholder="ai-mink/gemma4-e4b-rl-v1:q5_k_m"
            value={selectedModel}
            onChange={(e) => setSelectedModel(e.target.value)}
            disabled={isBusy}
            aria-describedby={model.DetectedModel ? "model-hint" : undefined}
          />
          {model.DetectedModel && (
            <p id="model-hint" className="text-xs text-muted-foreground">
              비워두면 감지된 모델 사용 / Leave empty to use the detected model.
            </p>
          )}
        </div>

        {!model.OllamaInstalled && (
          <p className="text-xs text-amber-600 bg-amber-50 border border-amber-200 rounded px-3 py-2">
            Ollama가 설치되지 않았습니다. 나중에 설치하려면 건너뛰기를 누르세요.
            <br />
            Ollama is not installed. Press Skip to configure later.
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
          onClick={handleBack}
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
