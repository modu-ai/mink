// StepProgress renders the top-level wizard progress bar.
// current_step values: 1-7 = active steps, 8 = completion sentinel.
import { Progress } from "@/components/ui/progress";

interface StepProgressProps {
  currentStep: number;
  totalSteps: number;
}

export function StepProgress({ currentStep, totalSteps }: StepProgressProps) {
  // On the completion sentinel (step 8), show 100%.
  const percent =
    currentStep > totalSteps
      ? 100
      : Math.round(((currentStep - 1) / totalSteps) * 100);

  const label =
    currentStep > totalSteps
      ? "완료 / Complete"
      : `Step ${currentStep} / ${totalSteps}`;

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>{label}</span>
        <span>{percent}%</span>
      </div>
      <Progress value={percent} aria-label={`Onboarding progress: ${label}`} />
    </div>
  );
}
