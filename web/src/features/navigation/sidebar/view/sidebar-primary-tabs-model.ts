/**
 * INPUT: 一级导航选中态与固定布局规则。
 * OUTPUT: 保留紧凑密度且不裁剪下伸字符的导航展示投影。
 * POS: 宽侧边栏一级导航的唯一样式模型。
 */
import { cn } from "@/shared/ui/class-name";
import { SIDEBAR_SELECTION_CLASS_NAME } from "@/shared/ui/sidebar/sidebar-selection";

interface SidebarPrimaryTabDefinition {
  badgeClassName: string;
  buttonActiveClassName: string;
  buttonBaseClassName: string;
  buttonInactiveClassName: string;
  containerClassName: string;
  iconBaseClassName: string;
  iconFrameActiveClassName: string;
  iconFrameClassName: string;
  iconFrameInactiveClassName: string;
  labelClassName: string;
}

interface SidebarPrimaryTabPresentation {
  ariaCurrent: "page" | undefined;
  badgeClassName: string;
  buttonClassName: string;
  iconClassName: string;
  iconFrameClassName: string;
  labelClassName: string;
  showLabel: boolean;
}

const SIDEBAR_PRIMARY_TAB_DEFINITION = {
  badgeClassName: "absolute -right-1.5 -top-1.5 h-4 min-w-4 px-1 text-2xs",
  buttonActiveClassName: "text-(--text-strong)",
  buttonBaseClassName: "group/sidebar-tab relative flex h-[50px] w-10 flex-col items-center justify-center gap-0.5 text-2xs font-medium transition-colors duration-(--motion-duration-fast)",
  buttonInactiveClassName: "text-(--text-muted) hover:text-(--text-strong)",
  containerClassName: "flex flex-col items-center gap-1.5 px-1 py-2",
  iconBaseClassName: "h-[18px] w-[18px]",
  iconFrameActiveClassName: SIDEBAR_SELECTION_CLASS_NAME,
  iconFrameClassName: "relative flex h-8 w-8 shrink-0 items-center justify-center rounded-[10px] transition-[background,color] duration-(--motion-duration-fast)",
  iconFrameInactiveClassName: "group-hover/sidebar-tab:bg-(--surface-interactive-hover-background)",
  labelClassName: "max-w-full truncate px-1 leading-tight",
} as const satisfies SidebarPrimaryTabDefinition;

export function getSidebarPrimaryTabsClassName(): string {
  return SIDEBAR_PRIMARY_TAB_DEFINITION.containerClassName;
}

export function resolveSidebarPrimaryTabPresentation({
  active,
}: {
  active: boolean;
}): SidebarPrimaryTabPresentation {
  const presentation = SIDEBAR_PRIMARY_TAB_DEFINITION;
  return {
    ariaCurrent: active ? "page" : undefined,
    badgeClassName: presentation.badgeClassName,
    buttonClassName: cn(
      presentation.buttonBaseClassName,
      active
        ? presentation.buttonActiveClassName
        : presentation.buttonInactiveClassName,
    ),
    iconClassName: presentation.iconBaseClassName,
    iconFrameClassName: cn(
      presentation.iconFrameClassName,
      active
        ? presentation.iconFrameActiveClassName
        : presentation.iconFrameInactiveClassName,
    ),
    labelClassName: presentation.labelClassName,
    showLabel: true,
  };
}
