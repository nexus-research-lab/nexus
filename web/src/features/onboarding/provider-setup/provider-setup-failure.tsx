// INPUT: Provider 向导当前阶段已经确认的问题、数据影响、下一步与严重程度。
// OUTPUT: 不改变向导阶段或自动重放操作的紧凑恢复提示。
// POS: Provider 向导的纯展示组件；阶段判断仍由 provider-setup-dialog 负责。
import { CircleAlert } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { RecoverySummary } from "@/shared/ui/feedback/recovery-summary";

export function ProviderSetupFailureView({
  impact,
  nextStep,
  problem,
  tone = "danger",
}: {
  impact: string;
  nextStep: string;
  problem: string;
  tone?: "danger" | "warning";
}) {
  return (
    <div
      aria-live="polite"
      className={cn(
        "flex items-start gap-2.5 border-l-2 py-1 pl-3 pr-1",
        tone === "warning"
          ? "border-[color:color-mix(in_srgb,var(--warning)_42%,transparent)]"
          : "border-[color:color-mix(in_srgb,var(--destructive)_38%,transparent)]",
      )}
      role="status"
    >
      <CircleAlert
        aria-hidden="true"
        className={cn(
          "mt-0.5 h-3.5 w-3.5 shrink-0",
          tone === "warning" ? "text-(--warning)" : "text-(--destructive)",
        )}
      />
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium leading-5 text-(--text-strong)">
          {problem}
        </p>
        <RecoverySummary className="mt-0.5" impact={impact} nextStep={nextStep} />
      </div>
    </div>
  );
}
