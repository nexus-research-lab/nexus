// INPUT: 列表行的选择状态、交互开关、视觉 tone、密度与外部布局 class。
// OUTPUT: 由共享 token/recipe 组成的列表行样式、角色和焦点入口。
// POS: ListRow 展示模型；不渲染 DOM，也不判断业务选择或路由。

import { cn } from "@/shared/ui/class-name";
import { SIDEBAR_SELECTION_CLASS_NAME } from "@/shared/ui/sidebar/sidebar-selection";

interface UiListRowPresentation {
  className: string;
  role: "button" | undefined;
  tabIndex: 0 | undefined;
}

export type UiListRowDensity = "default" | "compact";

const LIST_ROW_DENSITY_CLASS_NAMES: Record<UiListRowDensity, string> = {
  default: "min-h-[64px] px-2.5 py-2",
  compact: "min-h-12 px-3 py-2",
};

const LIST_ROW_STATE_CLASS_NAMES = {
  active: "border-transparent bg-(--surface-interactive-active-background) text-(--text-strong) shadow-none",
  activeSidebar: cn(SIDEBAR_SELECTION_CLASS_NAME, "text-(--text-strong)"),
  idleDefault: "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
  idleMuted: "text-(--text-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
} as const;

export function getUiListRowPresentation({
  active,
  activeTone,
  className,
  density,
  inactiveTone,
  interactive,
}: {
  active: boolean;
  activeTone: "default" | "sidebar";
  className?: string;
  density: UiListRowDensity;
  inactiveTone: "default" | "muted";
  interactive: boolean;
}): UiListRowPresentation {
  const state = active
    ? activeTone === "sidebar" ? "activeSidebar" : "active"
    : inactiveTone === "muted"
      ? "idleMuted"
      : "idleDefault";
  return {
    className: cn(
      "group/item relative flex w-full items-center gap-3 radius-control-md border border-transparent text-left transition-[background,border-color,color,box-shadow] duration-(--motion-duration-fast)",
      LIST_ROW_DENSITY_CLASS_NAMES[density],
      interactive && "cursor-pointer",
      LIST_ROW_STATE_CLASS_NAMES[state],
      className,
    ),
    role: interactive ? "button" : undefined,
    tabIndex: interactive ? 0 : undefined,
  };
}
