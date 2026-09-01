// INPUT: 业务已经确认的数据影响与安全下一步。
// OUTPUT: 一句当前影响，以及没有可执行动作时才需要的恢复说明。
// POS: 异常文案的视觉组合层；不补写、不截断，也不推断任何恢复事实。
import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

export function RecoverySummary({
  className,
  impact,
  nextStep,
}: {
  className?: string;
  impact: ReactNode;
  nextStep?: ReactNode;
}) {
  return (
    <p
      className={cn(
        "break-words text-xs leading-5 text-(--text-muted) [overflow-wrap:anywhere]",
        className,
      )}
    >
      <span data-recovery-impact>{impact}</span>
      {nextStep ? (
        <>{" "}<span className="text-(--text-default)" data-recovery-next-step>
          {nextStep}
        </span></>
      ) : null}
    </p>
  );
}
