// INPUT: 标签关闭名称、关闭命令与父级提供的位置/可见性 class。
// OUTPUT: 保持原生 title、固定关闭图标与既有命中区的独立关闭按钮。
// POS: Workspace 标签的关闭 DOM owner；不选择标签或管理标签集合。

import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

const TAB_DISMISS_CLASS_NAME =
  "flex h-6 w-6 shrink-0 items-center justify-center radius-control-xs text-(--icon-muted) transition-[background-color,color,opacity] duration-(--motion-duration-fast) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive) hover:opacity-100 focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_32%,transparent)]";

export function UiTabDismissButton({
  className,
  label,
  onDismiss,
}: {
  className?: string;
  label: string;
  onDismiss: () => void;
}) {
  return (
    <UiTooltip label={label}><button
      aria-label={label}
      className={cn(TAB_DISMISS_CLASS_NAME, className)}
      onClick={(event) => {
        event.stopPropagation();
        onDismiss();
      }}

      type="button"
    >
      <X className="h-3 w-3" />
    </button></UiTooltip>
  );
}
