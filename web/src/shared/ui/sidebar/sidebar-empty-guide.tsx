// INPUT: 侧栏空状态或读取失败的标题、影响、下一步和可选动作。
// OUTPUT: 符合侧栏密度的引导卡；失败态以 polite 方式完整播报。
// POS: 侧栏紧凑状态视图；不判断资源或修改结果。

import type { LucideIcon } from "lucide-react";
import { cn } from "@/shared/ui/class-name";

interface SidebarEmptyGuideProps {
  icon: LucideIcon;
  title: string;
  description: string;
  impact?: string;
  nextStep?: string;
  actionLabel?: string;
  onAction?: () => void;
  className?: string;
}

export function SidebarEmptyGuide({
  icon: Icon,
  title,
  description,
  impact,
  nextStep,
  actionLabel: actionLabel,
  onAction: onAction,
  className: className,
}: SidebarEmptyGuideProps) {
  return (
    <div
      aria-atomic={impact || nextStep ? "true" : undefined}
      aria-live={impact || nextStep ? "polite" : undefined}
      className={cn(
        "flex flex-col gap-1 rounded-[12px] border border-(--divider-subtle-color) px-2.5 py-2",
        className,
      )}
      role={impact || nextStep ? "status" : undefined}
    >
      <div className="flex items-center gap-1.5 text-(--text-muted)">
        <Icon className="h-3.5 w-3.5 shrink-0" />
        <span className="text-xs font-semibold">{title}</span>
      </div>
      <p className="text-xs leading-relaxed text-(--text-soft)">
        {description}
      </p>
      {impact ? (
        <p className="text-xs leading-relaxed text-(--text-muted)">{impact}</p>
      ) : null}
      {nextStep ? (
        <p className="text-xs font-medium leading-relaxed text-(--text-default)">
          {nextStep}
        </p>
      ) : null}
      {actionLabel && onAction ? (
        <button
          className="mt-0.5 inline-flex w-fit items-center gap-1 rounded-[8px] bg-(--surface-interactive-hover-background) px-2 py-[3px] text-xs font-semibold text-(--primary) transition-[background,color] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-active-background)"
          onClick={onAction}
          type="button"
        >
          {actionLabel}
        </button>
      ) : null}
    </div>
  );
}
