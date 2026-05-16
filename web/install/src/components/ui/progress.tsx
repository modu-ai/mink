import * as React from "react";
import { cn } from "@/lib/utils";

// Minimal Progress component without @radix-ui/react-progress dependency.
// Uses native HTML progress semantics via div + aria attributes.
export interface ProgressProps extends React.HTMLAttributes<HTMLDivElement> {
  value?: number; // 0-100
}

const Progress = React.forwardRef<HTMLDivElement, ProgressProps>(
  ({ className, value = 0, ...props }, ref) => {
    const clamped = Math.min(100, Math.max(0, value));
    return (
      <div
        ref={ref}
        role="progressbar"
        aria-valuenow={clamped}
        aria-valuemin={0}
        aria-valuemax={100}
        className={cn(
          "relative h-2 w-full overflow-hidden rounded-full bg-primary/20",
          className
        )}
        {...props}
      >
        <div
          className="h-full bg-primary transition-all duration-300 ease-in-out"
          style={{ width: `${clamped}%` }}
        />
      </div>
    );
  }
);
Progress.displayName = "Progress";

export { Progress };
