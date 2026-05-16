// App.tsx — root component for the MINK install wizard.
// Bootstraps the session on mount, then routes to the appropriate step component.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.3
import { useEffect } from "react";
import { useOnboarding } from "@/hooks/useOnboarding";
import { StepProgress } from "@/components/StepProgress";
import { Step1Locale } from "@/components/steps/Step1Locale";
import { StepPlaceholder } from "@/components/steps/StepPlaceholder";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import type { LocaleChoice } from "@/types/onboarding";

// Step metadata for placeholders 2-7.
const STEP_META: Record<number, { title: string; canSkip: boolean }> = {
  2: { title: "Model Setup / 모델 설정", canSkip: true },
  3: { title: "CLI Tools / CLI 도구", canSkip: true },
  4: { title: "Persona / 페르소나", canSkip: true },
  5: { title: "LLM Provider / LLM 공급자", canSkip: true },
  6: { title: "Messenger Channel / 메신저 채널", canSkip: true },
  7: { title: "Privacy & Consent / 개인정보 동의", canSkip: false },
};

// @MX:ANCHOR: [AUTO] Root routing component — all step navigation flows through here; fan_in >= 7.
// @MX:REASON: session_id, state.current_step, and hook callbacks are the central control plane.
export default function App() {
  const { state, loading, error, start, submitStep, skipStep, back, complete } =
    useOnboarding();

  // Bootstrap: call /session/start once on mount.
  useEffect(() => {
    void start();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Loading spinner while session initialises.
  if (state === null) {
    return (
      <div className="min-h-screen flex flex-col items-center justify-center bg-background p-4">
        <div className="flex flex-col items-center gap-4 text-muted-foreground">
          {loading && (
            <div
              className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent"
              aria-label="Loading"
            />
          )}
          {error != null && (
            <p className="text-sm text-destructive">
              연결 오류 / Connection error: {error}
            </p>
          )}
          {!loading && error != null && (
            <Button onClick={() => void start()} variant="outline" size="sm">
              재시도 / Retry
            </Button>
          )}
          {!loading && error === null && (
            <p className="text-sm">연결 중... / Connecting...</p>
          )}
        </div>
      </div>
    );
  }

  const { current_step, total_steps } = state;

  // Completion screen — shown when current_step > total_steps.
  const isComplete = current_step > total_steps || state.completed_at !== null;

  return (
    <div className="min-h-screen bg-background flex flex-col">
      {/* Header */}
      <header className="border-b bg-card/50 backdrop-blur-sm sticky top-0 z-10">
        <div className="container mx-auto px-4 py-3 flex items-center gap-3">
          <span className="text-2xl font-bold tracking-tight text-primary">
            MINK
          </span>
          <span className="text-sm text-muted-foreground">
            설치 마법사 / Install Wizard
          </span>
        </div>
      </header>

      <main className="flex-1 container mx-auto px-4 py-8 flex flex-col gap-6 max-w-2xl">
        {/* Progress bar — hidden on completion screen */}
        {!isComplete && (
          <StepProgress
            currentStep={current_step}
            totalSteps={total_steps}
          />
        )}

        {/* Step router */}
        {isComplete ? (
          <CompletionScreen
            completedAt={state.completed_at}
            loading={loading}
            error={error}
            onComplete={() => void complete()}
          />
        ) : (
          <StepRouter
            currentStep={current_step}
            loading={loading}
            error={error}
            onSubmitLocale={async (locale) => submitStep(1, locale)}
            onSkip={async (n) => skipStep(n)}
            onBack={async () => back()}
          />
        )}
      </main>
    </div>
  );
}

// StepRouter picks the right step component based on current_step.
interface StepRouterProps {
  currentStep: number;
  loading: boolean;
  error: string | null;
  onSubmitLocale: (locale: LocaleChoice) => Promise<void>;
  onSkip: (stepNumber: number) => Promise<void>;
  onBack: () => Promise<void>;
}

function StepRouter({
  currentStep,
  loading,
  error,
  onSubmitLocale,
  onSkip,
  onBack,
}: StepRouterProps) {
  if (currentStep === 1) {
    return (
      <Step1Locale
        loading={loading}
        error={error}
        onSubmit={onSubmitLocale}
      />
    );
  }

  const meta = STEP_META[currentStep];
  if (meta != null) {
    return (
      <StepPlaceholder
        stepNumber={currentStep}
        title={meta.title}
        loading={loading}
        error={error}
        canSkip={meta.canSkip}
        onSkip={meta.canSkip ? async () => onSkip(currentStep) : undefined}
        onBack={onBack}
      />
    );
  }

  // Fallback for unknown step numbers.
  return (
    <div className="text-sm text-muted-foreground text-center py-8">
      알 수 없는 단계 / Unknown step: {currentStep}
    </div>
  );
}

// CompletionScreen — shown when all 7 steps are done.
interface CompletionScreenProps {
  completedAt: string | null;
  loading: boolean;
  error: string | null;
  onComplete: () => void;
}

function CompletionScreen({
  completedAt,
  loading,
  error,
  onComplete,
}: CompletionScreenProps) {
  if (completedAt !== null) {
    // Session already persisted — show final screen.
    return (
      <Card className="w-full max-w-lg mx-auto text-center">
        <CardHeader>
          <CardTitle>온보딩 완료 / Onboarding Complete</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-muted-foreground text-sm">
            온보딩이 완료되었습니다. <code>mink</code> 명령으로 시작하세요.
            <br />
            Onboarding complete. Run <code>mink</code> to get started.
          </p>
        </CardContent>
      </Card>
    );
  }

  // Still needs the /complete call.
  return (
    <Card className="w-full max-w-lg mx-auto text-center">
      <CardHeader>
        <CardTitle>완료 / Complete</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-sm text-muted-foreground">
          모든 단계가 완료되었습니다. 설정을 저장하려면 마법사 종료를
          클릭하세요.
          <br />
          All steps complete. Click Finish to save your configuration.
        </p>
        {error != null && (
          <p className="text-xs text-destructive bg-destructive/10 border border-destructive/20 rounded px-3 py-2">
            {error}
          </p>
        )}
      </CardContent>
      <CardFooter className="justify-center">
        <Button onClick={onComplete} disabled={loading}>
          {loading ? "저장 중..." : "마법사 종료 / Finish"}
        </Button>
      </CardFooter>
    </Card>
  );
}
