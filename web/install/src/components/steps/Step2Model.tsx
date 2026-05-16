// Step2Model — Ollama + model selection step with live pull progress UI.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.3 Phase 3C
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
import { Progress } from "@/components/ui/progress";
import { usePullProgress } from "@/hooks/usePullProgress";
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

// Map Go phase strings to Korean labels.
function phaseLabel(phase: string): string {
  const p = phase.toLowerCase();
  if (p.includes("manifest") && p.includes("pull")) return "매니페스트 가져오는 중";
  if (p.includes("manifest") && p.includes("writ")) return "매니페스트 쓰는 중";
  if (p.includes("download") || p.includes("layer")) return "레이어 다운로드 중";
  if (p.includes("verif")) return "검증 중";
  return phase; // Fallback to raw phase string.
}

// Display a bytes progress counter, e.g., "6.5 MB / 12 MB".
function bytesCounter(done: number, total: number): string {
  if (total <= 0) return "";
  return `${humanizeBytes(done)} / ${humanizeBytes(total)}`;
}

// @MX:NOTE: [AUTO] Ollama pull progress UI — Step2 is the only step that uses sessionId for SSE streaming.
export function Step2Model({
  data,
  sessionId,
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

  const pull = usePullProgress();

  // Effective model name for pull: prefer user input, fall back to detected.
  const effectiveModel =
    selectedModel.trim() || model.DetectedModel || "";

  // Show pull button when Ollama is installed and a model name is available.
  const canPull = model.OllamaInstalled && effectiveModel.length > 0;

  // Block navigation while streaming to prevent accidental step change.
  const isNavigationBlocked = pull.isStreaming;
  const isBusy = loading || pending || pull.isStreaming;

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

  function handlePull() {
    pull.start(sessionId, effectiveModel);
  }

  // Progress bar value — clamp -1 (unknown) to 0.
  const progressValue = pull.latest
    ? Math.max(0, pull.latest.PercentDone)
    : 0;

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

        {/* Pull progress section — visible when Ollama is installed */}
        {canPull && (
          <div className="space-y-3">
            {/* Pull button — hidden during streaming */}
            {!pull.isStreaming && !pull.done && (
              <Button
                variant="outline"
                size="sm"
                onClick={handlePull}
                disabled={loading || pending}
                className="w-full"
              >
                모델 지금 다운로드 / Pull model now
              </Button>
            )}

            {/* Live streaming progress */}
            {pull.isStreaming && (
              <div
                className="space-y-2 rounded-lg border p-4 bg-muted/30"
                role="status"
                aria-live="polite"
                aria-label="모델 다운로드 진행 중"
              >
                <div className="flex items-center justify-between text-sm">
                  <span className="text-muted-foreground font-medium">
                    {pull.latest ? phaseLabel(pull.latest.Phase) : "연결 중..."}
                  </span>
                  <span className="text-xs text-muted-foreground tabular-nums">
                    {progressValue > 0 ? `${progressValue.toFixed(0)}%` : ""}
                  </span>
                </div>
                <Progress value={progressValue} aria-label="다운로드 진행률" />
                {pull.latest &&
                  pull.latest.BytesTotal > 0 && (
                    <p className="text-xs text-muted-foreground tabular-nums">
                      {bytesCounter(
                        pull.latest.BytesDone,
                        pull.latest.BytesTotal
                      )}
                    </p>
                  )}
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => pull.cancel()}
                  className="text-destructive hover:text-destructive/80 px-0 h-auto"
                >
                  취소 / Cancel
                </Button>
              </div>
            )}

            {/* Success state */}
            {pull.done && !pull.isStreaming && (
              <div
                className="flex items-center gap-2 rounded-lg border border-success/30 bg-success/10 px-4 py-3"
                role="status"
                aria-live="polite"
              >
                <span className="text-success font-semibold" aria-hidden="true">
                  ✓
                </span>
                <span className="text-sm text-success font-medium">
                  모델 다운로드 완료 / Download complete
                </span>
              </div>
            )}

            {/* Pull-specific error state */}
            {pull.error !== null && !pull.isStreaming && (
              <div
                className="space-y-2 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3"
                role="alert"
              >
                <p className="text-xs text-destructive">
                  ✗ 다운로드 실패 / Download failed: {pull.error}
                </p>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handlePull}
                  disabled={loading || pending}
                >
                  다시 시도 / Retry
                </Button>
              </div>
            )}
          </div>
        )}

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
          disabled={!canBack || isBusy || isNavigationBlocked}
        >
          뒤로 / Back
        </Button>
        <div className="flex gap-2">
          {canSkip && (
            <Button
              variant="ghost"
              onClick={handleSkip}
              disabled={isBusy || isNavigationBlocked}
            >
              건너뛰기 / Skip
            </Button>
          )}
          <Button
            onClick={handleSubmit}
            disabled={isBusy || isNavigationBlocked}
          >
            {loading || pending ? "처리 중..." : "Continue / 다음"}
          </Button>
        </div>
      </CardFooter>
    </Card>
  );
}
