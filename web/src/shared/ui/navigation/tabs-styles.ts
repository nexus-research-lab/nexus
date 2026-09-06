// INPUT: 选择条密度、active 状态与外部布局 class。
// OUTPUT: 使用共享 Typography、中性底线和交互 token 且标签始终单行的选择条样式。
// POS: UiTabs 唯一视觉投影；不定义 DOM 语义、路由或内容面板。

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export type UiTabsDensity = "default" | "compact";

interface UiTabStyleOptions {
  active?: boolean;
  density?: UiTabsDensity;
}

export function getUiTabsNavClassName(className?: string): string {
  return cn(
    "soft-scrollbar scrollbar-hide flex min-w-0 items-center gap-1 overflow-x-auto",
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
  } = options;

  return cn(
    "ui-navigation-tab inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap px-2.5 py-0 transition-[background,border-color,color] duration-(--motion-duration-fast) ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]",
    density === "compact" ? "h-8" : "h-9",
    getUiTypographyClassName({ role: "metadata", tone: active ? "strong" : "muted", weight: active ? "semibold" : "medium" }),
    "rounded-none border-x-0 border-t-0 border-b-2 bg-transparent",
    active
      ? "border-(--text-strong)"
      : "border-transparent hover:border-(--divider-strong-color) hover:text-(--text-default)",
    className,
  );
}
