// Shared props interface consumed by all step components (Step2-Step7).
// SPEC: SPEC-MINK-ONBOARDING-001 §6.3
import type { OnboardingData } from "@/types/onboarding";

export interface StepProps {
  data: OnboardingData;
  loading: boolean;
  submitStep: <T>(stepNumber: number, body: T) => Promise<void>;
  skipStep: (stepNumber: number) => Promise<void>;
  back: () => Promise<void>;
  canBack: boolean;
  canSkip: boolean;
}
