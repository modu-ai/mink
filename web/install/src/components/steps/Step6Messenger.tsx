// Step6Messenger — Messenger channel configuration step.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2 Step 6
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { MessengerChannel, MessengerType } from "@/types/onboarding";
import type { StepProps } from "./StepProps";

interface MessengerOption {
  value: MessengerType;
  label: string;
  tokenHint?: string;
}

const MESSENGER_OPTIONS: MessengerOption[] = [
  {
    value: "local_terminal",
    label: "로컬 터미널 / Local Terminal",
  },
  {
    value: "telegram",
    label: "Telegram",
    tokenHint: "BotFather에서 발급한 Bot Token (@BotFather → /newbot)",
  },
  {
    value: "slack",
    label: "Slack",
    tokenHint:
      "Incoming Webhook URL (api.slack.com/messaging/webhooks)",
  },
  {
    value: "discord",
    label: "Discord",
    tokenHint: "Webhook URL (채널 설정 → 연동 → 웹후크 생성)",
  },
  {
    value: "custom",
    label: "Custom",
    tokenHint: "공급자별 Bot Token 또는 Webhook URL / Provider-specific token or URL",
  },
];

export function Step6Messenger({
  data,
  loading,
  submitStep,
  skipStep,
  back,
  canBack,
  canSkip,
}: StepProps) {
  const saved = data.Messenger;
  const [messengerType, setMessengerType] = useState<MessengerType>(
    saved.Type || "local_terminal"
  );
  const [botTokenKey, setBotTokenKey] = useState(saved.BotTokenKey || "");
  const [localError, setLocalError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const selectedOption = MESSENGER_OPTIONS.find(
    (o) => o.value === messengerType
  );
  const needsToken = messengerType !== "local_terminal";
  const isBusy = loading || pending;

  async function handleSubmit() {
    setPending(true);
    setLocalError(null);
    try {
      const body: MessengerChannel = {
        Type: messengerType,
        BotTokenKey: botTokenKey.trim(),
      };
      await submitStep<MessengerChannel>(6, body);
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
      await skipStep(6);
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "오류가 발생했습니다.");
    } finally {
      setPending(false);
    }
  }

  return (
    <Card className="w-full max-w-lg mx-auto">
      <CardHeader>
        <CardTitle>Step 6: 메신저 채널 / Messenger Channel</CardTitle>
        <CardDescription>
          MINK과 대화할 채널을 선택하세요.
          <br />
          Choose the channel you will use to interact with MINK.
        </CardDescription>
      </CardHeader>

      <CardContent className="space-y-5">
        {/* Messenger type select */}
        <div className="space-y-2">
          <Label htmlFor="messenger-type">채널 유형 / Channel Type</Label>
          <Select
            value={messengerType}
            onValueChange={(v) => {
              setMessengerType(v as MessengerType);
              setBotTokenKey("");
            }}
            disabled={isBusy}
          >
            <SelectTrigger id="messenger-type">
              <SelectValue placeholder="채널 선택..." />
            </SelectTrigger>
            <SelectContent>
              {MESSENGER_OPTIONS.map((opt) => (
                <SelectItem key={opt.value} value={opt.value}>
                  {opt.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {/* Conditional: token/webhook input */}
        {needsToken && (
          <div className="space-y-2">
            <Label htmlFor="bot-token">
              토큰 / Token or Webhook URL
            </Label>
            <Input
              id="bot-token"
              placeholder={
                selectedOption?.tokenHint ?? "토큰 또는 URL을 입력하세요..."
              }
              value={botTokenKey}
              onChange={(e) => setBotTokenKey(e.target.value)}
              disabled={isBusy}
              aria-describedby="token-hint"
            />
            {selectedOption?.tokenHint != null && (
              <p id="token-hint" className="text-xs text-muted-foreground">
                {selectedOption.tokenHint}
              </p>
            )}
          </div>
        )}

        {messengerType === "local_terminal" && (
          <p className="text-xs text-muted-foreground bg-muted/50 rounded px-3 py-2">
            로컬 터미널 모드는 별도 설정 없이 즉시 사용할 수 있습니다.
            <br />
            Local terminal mode requires no additional configuration.
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
