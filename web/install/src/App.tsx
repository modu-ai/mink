// App.tsx — root component for the MINK install wizard.
// Bootstraps the session on mount, then routes to the appropriate step component.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.3
import { useEffect } from "react";
import { useOnboarding } from "@/hooks/useOnboarding";
import { StepProgress } from "@/components/StepProgress";
import { Step1Locale } from "@/components/steps/Step1Locale";
import { Step2Model } from "@/components/steps/Step2Model";
import { Step3CLI } from "@/components/steps/Step3CLI";
import { Step4Persona } from "@/components/steps/Step4Persona";
import { Step5Provider } from "@/components/steps/Step5Provider";
import { Step6Messenger } from "@/components/steps/Step6Messenger";
import { Step7Consent } from "@/components/steps/Step7Consent";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import type { LocaleChoice, OnboardingData } from "@/types/onboarding";

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
            sessionId={state.session_id}
            data={state.data}
            loading={loading}
            error={error}
            onSubmitLocale={async (locale) => submitStep(1, locale)}
            onSubmitStep={submitStep}
            onSkip={skipStep}
            onBack={back}
          />
        )}
      </main>
    </div>
  );
}

// StepRouter picks the right step component based on current_step.
interface StepRouterProps {
  currentStep: number;
  sessionId: string;
  data: OnboardingData;
  loading: boolean;
  error: string | null;
  onSubmitLocale: (locale: LocaleChoice) => Promise<void>;
  onSubmitStep: <T>(stepNumber: number, body: T) => Promise<void>;
  onSkip: (stepNumber: number) => Promise<void>;
  onBack: () => Promise<void>;
}

function StepRouter({
  currentStep,
  sessionId,
  data,
  loading,
  error,
  onSubmitLocale,
  onSubmitStep,
  onSkip,
  onBack,
}: StepRouterProps) {
  // Shared props for step components 2-7.
  const sharedProps = {
    data,
    sessionId,
    loading,
    submitStep: onSubmitStep,
    skipStep: onSkip,
    back: onBack,
    canBack: currentStep > 1,
  };

  switch (currentStep) {
    case 1:
      return (
        <Step1Locale
          loading={loading}
          error={error}
          onSubmit={onSubmitLocale}
        />
      );
    case 2:
      return <Step2Model {...sharedProps} canSkip={true} />;
    case 3:
      return <Step3CLI {...sharedProps} canSkip={true} />;
    case 4:
      // Persona is required — canSkip=false; backend will reject empty name.
      return <Step4Persona {...sharedProps} canSkip={false} />;
    case 5:
      return <Step5Provider {...sharedProps} canSkip={true} />;
    case 6:
      return <Step6Messenger {...sharedProps} canSkip={true} />;
    case 7:
      // canSkip is conditionally false for GDPR regions (Step7Consent handles internally).
      return <Step7Consent {...sharedProps} canSkip={true} />;
    default:
      return (
        <div className="text-sm text-muted-foreground text-center py-8">
          알 수 없는 단계 / Unknown step: {currentStep}
        </div>
      );
  }
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
