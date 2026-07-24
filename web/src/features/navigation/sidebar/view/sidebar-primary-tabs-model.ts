import { cn } from "@/shared/ui/class-name";
import { SIDEBAR_SELECTION_CLASS_NAME } from "@/shared/ui/sidebar/sidebar-selection";

export type SidebarPrimaryTabsVariant = "dock" | "focus" | "rail";

interface SidebarPrimaryTabVariantPresentation {
  badgeClassName: string;
  buttonActiveClassName: string;
  buttonBaseClassName: string;
  buttonInactiveClassName: string;
  containerClassName: string;
  iconBaseClassName: string;
  iconFrameClassName: string;
  labelClassName: string;
  showLabel: boolean;
  useAriaLabel: boolean;
}

interface SidebarPrimaryTabPresentation {
  ariaCurrent: "page" | undefined;
  ariaLabel: string | undefined;
  badgeClassName: string;
  buttonClassName: string;
  iconClassName: string;
  iconFrameClassName: string;
  labelClassName: string;
  showLabel: boolean;
}

const ACTIVE_ICON_CLASS_NAME = "fill-(--primary) stroke-(--primary)";

const SIDEBAR_PRIMARY_TAB_VARIANTS = {
  dock: {
    badgeClassName: "absolute -right-1.5 -top-1.5 h-4 min-w-4 px-1 text-[10px]",
    buttonActiveClassName: cn(SIDEBAR_SELECTION_CLASS_NAME, "text-(--text-strong)"),
    buttonBaseClassName: "relative flex h-[54px] w-[52px] flex-col items-center justify-center gap-1 rounded-[10px] text-[10px] font-medium transition-[background,color] duration-(--motion-duration-fast)",
    buttonInactiveClassName: "text-(--text-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
    containerClassName: "flex flex-col items-center gap-1.5 px-1 py-2",
    iconBaseClassName: "h-[18px] w-[18px]",
    iconFrameClassName: "relative flex h-5 w-5 items-center justify-center",
    labelClassName: "max-w-full truncate px-1 leading-none",
    showLabel: true,
    useAriaLabel: false,
  },
  focus: {
    badgeClassName: "absolute -right-2 -top-2 h-4 min-w-4 px-1 text-[10px]",
    buttonActiveClassName: cn(SIDEBAR_SELECTION_CLASS_NAME, "text-(--text-strong)"),
    buttonBaseClassName: "relative flex h-10 w-[192px] items-center gap-2 rounded-[10px] px-3 text-[12px] font-medium transition-[background,color] duration-(--motion-duration-fast)",
    buttonInactiveClassName: "text-(--text-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
    containerClassName: "flex flex-col items-center gap-1 px-2 py-2",
    iconBaseClassName: "h-[18px] w-[18px]",
    iconFrameClassName: "relative flex h-5 w-5 items-center justify-center",
    labelClassName: "min-w-0 truncate leading-none",
    showLabel: true,
    useAriaLabel: false,
  },
  rail: {
    badgeClassName: "absolute -right-1 -top-1 h-4 min-w-4 px-1 text-[10px]",
    buttonActiveClassName: cn(SIDEBAR_SELECTION_CLASS_NAME, "text-(--text-strong)"),
    buttonBaseClassName: "relative flex h-9 w-9 items-center justify-center rounded-full text-(--icon-default) transition-[background,color] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
    buttonInactiveClassName: "",
    containerClassName: "mt-1 flex flex-col items-center gap-1.5",
    iconBaseClassName: "h-4 w-4",
    iconFrameClassName: "contents",
    labelClassName: "",
    showLabel: false,
    useAriaLabel: true,
  },
} as const satisfies Record<
  SidebarPrimaryTabsVariant,
  SidebarPrimaryTabVariantPresentation
>;

export function getSidebarPrimaryTabsClassName(
  variant: SidebarPrimaryTabsVariant,
): string {
  return SIDEBAR_PRIMARY_TAB_VARIANTS[variant].containerClassName;
}

export function resolveSidebarPrimaryTabPresentation({
  active,
  label,
  variant,
}: {
  active: boolean;
  label: string;
  variant: SidebarPrimaryTabsVariant;
}): SidebarPrimaryTabPresentation {
  const presentation = SIDEBAR_PRIMARY_TAB_VARIANTS[variant];
  return {
    ariaCurrent: active ? "page" : undefined,
    ariaLabel: presentation.useAriaLabel ? label : undefined,
    badgeClassName: presentation.badgeClassName,
    buttonClassName: cn(
      presentation.buttonBaseClassName,
      active
        ? presentation.buttonActiveClassName
        : presentation.buttonInactiveClassName,
    ),
    iconClassName: cn(
      presentation.iconBaseClassName,
      active && ACTIVE_ICON_CLASS_NAME,
    ),
    iconFrameClassName: presentation.iconFrameClassName,
    labelClassName: presentation.labelClassName,
    showLabel: presentation.showLabel,
  };
}
