// Step5Provider — LLM provider + authentication step.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2 Step 5
import { useState } from "react";
import { Eye, EyeOff } from "lucide-react";
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
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type {
  AuthMethod,
  Provider,
  ProviderStepInput,
} from "@/types/onboarding";
import type { StepProps } from "./StepProps";

const PROVIDERS: { value: Provider; label: string }[] = [
  { value: "anthropic", label: "Anthropic (Claude)" },
  { value: "openai", label: "OpenAI (GPT)" },
  { value: "google", label: "Google (Gemini)" },
  { value: "ollama", label: "Ollama (Local)" },
  { value: "deepseek", label: "DeepSeek" },
  { value: "custom", label: "Custom endpoint" },
];

export function Step5Provider({
  data,
  loading,
  submitStep,
  skipStep,
  back,
  canBack,
  canSkip,
}: StepProps) {
  const saved = data.Provider;
  const [provider, setProvider] = useState<Provider>(
    saved.Provider !== "unset" ? saved.Provider : "anthropic"
  );
  const [authMethod, setAuthMethod] = useState<AuthMethod>(
    saved.AuthMethod || "api_key"
  );
  const [apiKey, setApiKey] = useState("");
  const [showKey, setShowKey] = useState(false);
  const [customEndpoint, setCustomEndpoint] = useState(
    saved.CustomEndpoint || ""
  );
  const [preferredModel, setPreferredModel] = useState(
    saved.PreferredModel || ""
  );
  const [localError, setLocalError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  // OAuth is only supported for Google in Phase 4.
  const oauthDisabled = provider !== "google";

  // When provider changes, reset oauth selection if it becomes disabled.
  function handleProviderChange(v: Provider) {
    setProvider(v);
    if (v !== "google" && authMethod === "oauth") {
      setAuthMethod("api_key");
    }
  }

  const isBusy = loading || pending;

  async function handleSubmit() {
    setPending(true);
    setLocalError(null);
    try {
      const body: ProviderStepInput = {
        Choice: {
          Provider: provider,
          AuthMethod: authMethod,
          APIKeyStored: false,
          CustomEndpoint: customEndpoint.trim(),
          PreferredModel: preferredModel.trim(),
        },
        APIKey: apiKey,
      };
      await submitStep<ProviderStepInput>(5, body);
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
      await skipStep(5);
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "오류가 발생했습니다.");
    } finally {
      setPending(false);
    }
  }

  return (
    <Card className="w-full max-w-lg mx-auto">
      <CardHeader>
        <CardTitle>Step 5: LLM 공급자 / LLM Provider</CardTitle>
        <CardDescription>
          MINK이 사용할 LLM 서비스와 인증 방법을 선택하세요.
          <br />
          Select the LLM service and authentication method for MINK.
        </CardDescription>
      </CardHeader>

      <CardContent className="space-y-5">
        {/* Provider select */}
        <div className="space-y-2">
          <Label htmlFor="provider-select">공급자 / Provider</Label>
          <Select
            value={provider}
            onValueChange={(v) => handleProviderChange(v as Provider)}
            disabled={isBusy}
          >
            <SelectTrigger id="provider-select">
              <SelectValue placeholder="공급자 선택..." />
            </SelectTrigger>
            <SelectContent>
              {PROVIDERS.map((p) => (
                <SelectItem key={p.value} value={p.value}>
                  {p.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {/* Auth method radio */}
        <div className="space-y-2">
          <Label>인증 방법 / Auth Method</Label>
          <RadioGroup
            value={authMethod}
            onValueChange={(v) => setAuthMethod(v as AuthMethod)}
            disabled={isBusy}
            className="space-y-2"
          >
            {[
              { value: "api_key" as AuthMethod, label: "API 키 / API Key" },
              {
                value: "oauth" as AuthMethod,
                label: "OAuth",
                disabled: oauthDisabled,
                hint: oauthDisabled
                  ? "Phase 4 implementation — Google 전용 / Google only"
                  : undefined,
              },
              { value: "env" as AuthMethod, label: "환경변수 / Environment variable" },
            ].map((opt) => (
              <div
                key={opt.value}
                className={[
                  "flex items-center gap-2",
                  opt.disabled ? "opacity-50" : "",
                ].join(" ")}
              >
                <RadioGroupItem
                  value={opt.value}
                  id={`auth-${opt.value}`}
                  disabled={opt.disabled}
                />
                <Label
                  htmlFor={`auth-${opt.value}`}
                  className="cursor-pointer"
                >
                  {opt.label}
                  {opt.hint != null && (
                    <span className="ml-2 text-xs text-muted-foreground">
                      ({opt.hint})
                    </span>
                  )}
                </Label>
              </div>
            ))}
          </RadioGroup>
        </div>

        {/* Conditional: API key input */}
        {authMethod === "api_key" && (
          <div className="space-y-2">
            <Label htmlFor="api-key">API 키 / API Key</Label>
            <div className="relative">
              <Input
                id="api-key"
                type={showKey ? "text" : "password"}
                placeholder="sk-..."
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                disabled={isBusy}
                className="pr-10"
                aria-label="API key input"
              />
              <button
                type="button"
                onClick={() => setShowKey((s) => !s)}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                aria-label={showKey ? "키 숨기기 / Hide key" : "키 보기 / Show key"}
              >
                {showKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>
          </div>
        )}

        {/* Conditional: custom endpoint */}
        {provider === "custom" && (
          <div className="space-y-2">
            <Label htmlFor="custom-endpoint">커스텀 엔드포인트 / Custom Endpoint</Label>
            <Input
              id="custom-endpoint"
              type="url"
              placeholder="https://your-endpoint.example/v1"
              value={customEndpoint}
              onChange={(e) => setCustomEndpoint(e.target.value)}
              disabled={isBusy}
            />
          </div>
        )}

        {/* Preferred model (optional) */}
        <div className="space-y-2">
          <Label htmlFor="preferred-model">
            선호 모델 / Preferred Model{" "}
            <span className="text-muted-foreground font-normal text-xs">(선택 / optional)</span>
          </Label>
          <Input
            id="preferred-model"
            placeholder="e.g., claude-sonnet-4-6"
            value={preferredModel}
            onChange={(e) => setPreferredModel(e.target.value)}
            disabled={isBusy}
          />
        </div>

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
