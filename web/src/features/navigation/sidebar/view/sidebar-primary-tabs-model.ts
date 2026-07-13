import { cn } from "@/shared/ui/class-name";

export type SidebarPrimaryTabsVariant = "rail" | "panel";

interface SidebarPrimaryTabVariantPresentation {
  badgeClassName: string;
  buttonActiveClassName: string;
  buttonBaseClassName: string;
  buttonInactiveClassName: string;
  containerClassName: string;
  iconBaseClassName: string;
  iconFrameClassName: string;
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
  showLabel: boolean;
}

const ACTIVE_ICON_CLASS_NAME = "fill-(--primary) stroke-(--primary)";

const SIDEBAR_PRIMARY_TAB_VARIANTS = {
  panel: {
    badgeClassName: "absolute -right-2.5 -top-2 h-4 min-w-4 px-1 text-[10px] shadow-[0_2px_6px_rgba(255,76,84,0.28)]",
    buttonActiveClassName: "bg-[color:color-mix(in_srgb,var(--primary)_14%,var(--surface-elevated-background))] text-(--primary) shadow-[0_8px_22px_color-mix(in_srgb,var(--primary)_10%,transparent)]",
    buttonBaseClassName: "flex h-9 items-center justify-center gap-1.5 rounded-[11px] text-[13px] font-medium transition-[background,color,box-shadow] duration-(--motion-duration-fast)",
    buttonInactiveClassName: "text-(--text-muted) hover:text-(--text-strong)",
    containerClassName: "grid grid-cols-3 gap-1 rounded-[14px] bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_58%,transparent)] p-1",
    iconBaseClassName: "h-3.5 w-3.5",
    iconFrameClassName: "relative flex h-4 w-4 items-center justify-center",
    showLabel: true,
    useAriaLabel: false,
  },
  rail: {
    badgeClassName: "absolute -right-1 -top-1 h-4 min-w-4 px-1 text-[10px] shadow-[0_2px_6px_rgba(255,76,84,0.28)]",
    buttonActiveClassName: "bg-(--surface-interactive-active-background) text-(--primary)",
    buttonBaseClassName: "relative flex h-9 w-9 items-center justify-center rounded-full text-(--icon-default) transition-(background,color,transform) duration-(--motion-duration-fast) hover:-translate-y-px hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
    buttonInactiveClassName: "",
    containerClassName: "mt-1 flex flex-col items-center gap-1.5",
    iconBaseClassName: "h-4 w-4",
    iconFrameClassName: "contents",
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
    showLabel: presentation.showLabel,
  };
}
