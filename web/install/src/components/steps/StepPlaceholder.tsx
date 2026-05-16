// StepPlaceholder — generic placeholder for steps 2-7.
// Steps 2-6: skip + back allowed. Step 7 (Consent): implementation-blocked.
// SPEC: SPEC-MINK-ONBOARDING-001 Phase 3A
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";

interface StepPlaceholderProps {
  stepNumber: number;
  title: string;
  loading: boolean;
  error: string | null;
  canSkip: boolean;
  onSkip?: () => Promise<void>;
  onBack: () => Promise<void>;
}

export function StepPlaceholder({
  stepNumber,
  title,
  loading,
  error,
  canSkip,
  onSkip,
  onBack,
}: StepPlaceholderProps) {
  const isConsentStep = stepNumber === 7;

  return (
    <Card className="w-full max-w-lg mx-auto">
      <CardHeader>
        <CardTitle>
          Step {stepNumber}: {title}
        </CardTitle>
        <CardDescription>
          Phase 3B에서 구현 예정 — This step will be implemented in Phase 3B.
        </CardDescription>
      </CardHeader>

      <CardContent className="space-y-3">
        <div className="rounded-lg border border-dashed border-muted-foreground/30 p-6 text-center text-sm text-muted-foreground">
          <p className="font-medium">⚙ Phase 3B 구현 예정</p>
          <p className="mt-1 text-xs">
            This step is a shell for Phase 3A. Full functionality arrives in
            Phase 3B.
          </p>
        </div>

        {isConsentStep && (
          <p className="text-xs text-muted-foreground bg-muted rounded px-3 py-2">
            Phase 3B 완성 시까지 완료 불가. Consent step cannot be completed
            until Phase 3B is implemented.
          </p>
        )}

        {error != null && (
          <p className="text-xs text-destructive bg-destructive/10 border border-destructive/20 rounded px-3 py-2">
            {error}
          </p>
        )}
      </CardContent>

      <CardFooter className="flex justify-between gap-2">
        <Button variant="outline" onClick={onBack} disabled={loading}>
          Back / 이전
        </Button>

        <div className="flex gap-2">
          {canSkip && !isConsentStep && onSkip != null && (
            <Button variant="ghost" onClick={onSkip} disabled={loading}>
              Skip / 건너뛰기
            </Button>
          )}

          {isConsentStep && (
            <Button disabled title="Phase 3B 완성 시까지 완료 불가">
              Continue / 다음
            </Button>
          )}
        </div>
      </CardFooter>
    </Card>
  );
}
