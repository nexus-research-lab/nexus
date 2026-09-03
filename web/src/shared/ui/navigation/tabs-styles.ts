// INPUT: 选择条密度、active 状态与外部布局 class。
// OUTPUT: 使用统一 control radius 和交互 token、且标签始终单行的选择条样式。
// POS: UiTabs 视觉投影；不定义 DOM 语义、路由或内容面板。

import { cn } from "@/shared/ui/class-name";

export type UiTabsDensity = "default" | "compact";
export type UiTabsVariant = "soft" | "line";

interface UiTabStyleOptions {
  active?: boolean;
  density?: UiTabsDensity;
  variant?: UiTabsVariant;
}

export function getUiTabsNavClassName(className?: string): string {
  return cn(
    "soft-scrollbar scrollbar-hide flex min-w-0 items-center gap-1 overflow-x-auto",
    className,
  );
}

export function getUiTabDismissClassName(className?: string): string {
  return cn(
    "flex h-6 w-6 shrink-0 items-center justify-center radius-control-xs text-(--icon-muted) transition-[background-color,color,opacity] duration-(--motion-duration-fast) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive) hover:opacity-100 focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_32%,transparent)]",
    className,
  );
}

export function getUiTabClassName(
  options: UiTabStyleOptions = {},
  className?: string,
): string {
  const {
    active = false,
    density = "default",
    variant = "soft",
  } = options;

  return cn(
    "ui-navigation-tab inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap px-2.5 py-0 font-medium transition-[background,border-color,color] duration-(--motion-duration-fast) ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]",
    density === "compact" ? "h-8 text-xs" : "h-9 text-xs",
    variant === "line"
      ? "rounded-none border-x-0 border-t-0 border-b-2 bg-transparent"
      : "radius-control-sm border-0",
    variant === "line"
      ? active
        ? "border-(--text-strong) font-semibold text-(--text-strong)"
        : "border-transparent text-(--text-muted) hover:border-(--divider-strong-color) hover:text-(--text-default)"
      : active
        ? "bg-(--surface-interactive-active-background) font-semibold text-(--text-strong)"
        : "text-(--text-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-default)",
    className,
  );
}
