// INPUT: 列表行的选择状态、交互开关、tone、密度、表面与外部布局 class。
// OUTPUT: 由共享 token/recipe 组成的列表行样式、角色和焦点入口。
// POS: ListRow 内部展示配方；不渲染 DOM，也不判断业务选择或路由。

import { cn } from "@/shared/ui/class-name";
import { SIDEBAR_SELECTION_CLASS_NAME } from "@/shared/ui/sidebar/sidebar-selection";

interface UiListRowPresentation {
  className: string;
  role: "button" | undefined;
  tabIndex: 0 | undefined;
}

export type UiListRowDensity = "default" | "compact" | "dense" | "sidebar" | "sidebarCompact";
export type UiListRowVariant = "plain" | "flush" | "outlined";

const LIST_ROW_DENSITY_CLASS_NAMES: Record<UiListRowDensity, string> = {
  default: "min-h-[64px] px-2.5 py-2",
  compact: "min-h-12 px-3 py-2",
  dense: "min-h-10 px-2.5 py-1.5",
  sidebar: "min-h-[60px] gap-2.5 px-2 py-2 max-[559px]:min-h-[80px] max-[559px]:gap-3 max-[559px]:rounded-(--radius-lg) max-[559px]:px-3 max-[559px]:py-3",
  sidebarCompact: "min-h-[54px] gap-2.5 py-1.5 pl-2 pr-[3px] max-[559px]:min-h-[72px] max-[559px]:gap-3 max-[559px]:rounded-(--radius-lg) max-[559px]:px-3 max-[559px]:py-2.5",
};

const LIST_ROW_STATE_CLASS_NAMES = {
  active: "border-transparent bg-(--surface-interactive-active-background) text-(--text-strong) shadow-none",
  activeSidebar: cn(SIDEBAR_SELECTION_CLASS_NAME, "text-(--text-strong)"),
  disabled: "cursor-not-allowed text-(--text-muted) opacity-(--disabled-opacity)",
  idleDefault: "text-(--text-default)",
  idleMuted: "text-(--text-muted)",
} as const;

export function getUiListRowPresentation({
  active,
  activeTone,
  className,
  density,
  disabled,
  inactiveTone,
  interactive,
  muted = false,
  variant = "plain",
}: {
  active: boolean;
  activeTone: "default" | "sidebar";
  className?: string;
  density: UiListRowDensity;
  disabled: boolean;
  inactiveTone: "default" | "muted";
  interactive: boolean;
  muted?: boolean;
  variant?: UiListRowVariant;
}): UiListRowPresentation {
  const state = disabled
    ? "disabled"
    : active
    ? activeTone === "sidebar" ? "activeSidebar" : "active"
    : inactiveTone === "muted"
      ? "idleMuted"
      : "idleDefault";
  return {
    className: cn(
      "group/item relative flex w-full items-center gap-3 radius-control-md border border-transparent text-left transition-[background,border-color,color,box-shadow] duration-(--motion-duration-fast)",
      LIST_ROW_DENSITY_CLASS_NAMES[density],
      variant === "flush" && "rounded-none",
      variant === "outlined" && "border-(--divider-subtle-color) bg-transparent",
      interactive && !disabled && "cursor-pointer",
      LIST_ROW_STATE_CLASS_NAMES[state],
      interactive && !disabled && !active && "hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
      variant === "outlined" && interactive && !disabled && !active && "hover:border-(--surface-interactive-active-border)",
      muted && !disabled && "opacity-70",
      className,
    ),
    role: interactive ? "button" : undefined,
    tabIndex: interactive && !disabled ? 0 : undefined,
  };
}
