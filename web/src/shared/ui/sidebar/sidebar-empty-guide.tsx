// INPUT: 侧栏空状态或读取失败的标题、影响、必要说明和可选动作。
// OUTPUT: 符合侧栏密度、语义排版与共享动作规范的引导卡；失败态不重复恢复说明。
// POS: 侧栏紧凑状态视图；不拥有 Button/排版 recipe，也不判断资源或修改结果。

import type { LucideIcon } from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

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
        "surface-radius-md flex flex-col gap-1 border border-(--divider-subtle-color) px-2.5 py-2",
        className,
      )}
      role={impact || nextStep ? "status" : undefined}
    >
      <div className="flex items-center gap-1.5 text-(--text-muted)">
        <Icon className="h-3.5 w-3.5 shrink-0" />
        <span className={getUiTypographyClassName({
          role: "caption",
          tone: "muted",
          weight: "semibold",
        })}>
          {title}
        </span>
      </div>
      {!impact ? (
        <p className={getUiTypographyClassName({ role: "caption", tone: "soft" })}>
          {description}
        </p>
      ) : null}
      {impact ? (
        <p className={getUiTypographyClassName({ role: "caption", tone: "muted" })}>
          {impact}
        </p>
      ) : null}
      {nextStep && !(actionLabel && onAction) ? (
        <p className={getUiTypographyClassName({
          role: "caption",
          tone: "default",
          weight: "medium",
        })}>
          {nextStep}
        </p>
      ) : null}
      {actionLabel && onAction ? (
        <UiButton
          className="mt-0.5 w-fit"
          onClick={onAction}
          size="xs"
          tone="primary"
          variant="ghost"
        >
          {actionLabel}
        </UiButton>
      ) : null}
    </div>
  );
}
