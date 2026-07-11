import { cn } from "@/shared/ui/class-name";

import {
  resolveAnchoredOverlayPosition,
  type UiAnchoredOverlayPlacement,
  type UiAnchoredOverlayPosition,
} from "../overlay/anchored-overlay-model";

export type UiSelectMenuPlacement = UiAnchoredOverlayPlacement;
export type UiSelectMenuSize = "xs" | "sm" | "md";
export type UiSelectMenuSurface = "surface" | "dialog";
const SELECT_MENU_MAX_HEIGHT = 280;

export const SELECT_MENU_SEARCH_ROW_HEIGHT = 44;

const SELECT_MENU_SIZE_CONFIG: Record<UiSelectMenuSize, {
  estimatedOptionHeight: number;
  heightClassName: string;
  optionHeightClassName: string;
  roundedClassName: string;
  textClassName: string;
}> = {
  md: {
    estimatedOptionHeight: 32,
    heightClassName: "h-10",
    optionHeightClassName: "min-h-8 text-[13px]",
    roundedClassName: "rounded-[13px]",
    textClassName: "text-[13px]",
  },
  sm: {
    estimatedOptionHeight: 32,
    heightClassName: "h-9",
    optionHeightClassName: "min-h-8 text-[13px]",
    roundedClassName: "rounded-[12px]",
    textClassName: "text-[12px]",
  },
  xs: {
    estimatedOptionHeight: 28,
    heightClassName: "h-7",
    optionHeightClassName: "min-h-7 text-[12px]",
    roundedClassName: "rounded-[10px]",
    textClassName: "text-[11px]",
  },
};

const SELECT_MENU_BUTTON_SURFACE_CLASS_NAMES: Record<UiSelectMenuSurface, string> = {
  dialog: "dialog-input shadow-none hover:border-[color:color-mix(in_srgb,var(--primary)_24%,var(--modal-input-border))] hover:bg-[color:color-mix(in_srgb,var(--modal-input-focus-background)_72%,transparent)] focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_14%,transparent)]",
  surface: "border border-[color:color-mix(in_srgb,var(--primary)_22%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--background)_94%,white)] shadow-[0_1px_2px_rgba(15,23,42,0.04)] hover:border-[color:color-mix(in_srgb,var(--primary)_38%,var(--divider-subtle-color))] focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_18%,transparent)]",
};

const SELECT_MENU_PANEL_SURFACE_CLASS_NAMES: Record<UiSelectMenuSurface, string> = {
  dialog: "border-(--modal-card-border) bg-[color:color-mix(in_srgb,var(--background)_94%,white)] shadow-[0_16px_36px_rgba(15,23,42,0.14)]",
  surface: "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_96%,white)] shadow-[0_14px_32px_rgba(15,23,42,0.12)] backdrop-blur",
};

const SELECT_MENU_OPTION_STATE_CLASS_NAMES: Record<
  UiSelectMenuSurface,
  Record<"active" | "inactive", string>
> = {
  dialog: {
    active: "bg-[color:color-mix(in_srgb,var(--primary)_13%,transparent)] font-semibold text-(--text-strong) hover:bg-[color:color-mix(in_srgb,var(--primary)_16%,transparent)]",
    inactive: "text-(--text-default) hover:bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] hover:text-(--text-strong)",
  },
  surface: {
    active: "bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)] font-semibold text-(--text-strong) hover:bg-[color:color-mix(in_srgb,var(--primary)_14%,transparent)]",
    inactive: "text-(--text-default) hover:bg-(--surface-interactive-hover-background)",
  },
};

export function getSelectMenuSizeConfig(size: UiSelectMenuSize) {
  return SELECT_MENU_SIZE_CONFIG[size];
}

export function estimateSelectMenuHeight(optionCount: number, optionHeight: number, extraHeight = 8): number {
  return Math.min(
    SELECT_MENU_MAX_HEIGHT,
    Math.max(optionHeight + 8, optionCount * optionHeight + extraHeight),
  );
}

export function resolveSelectMenuPosition({
  button,
  estimatedHeight,
  estimatedOptionHeight,
  menuMinWidth,
  placement,
}: {
  button: HTMLButtonElement;
  estimatedHeight: number;
  estimatedOptionHeight: number;
  placement: UiSelectMenuPlacement;
  menuMinWidth?: number;
}): UiAnchoredOverlayPosition {
  return resolveAnchoredOverlayPosition({
    anchor: button,
    estimatedHeight,
    maxHeight: SELECT_MENU_MAX_HEIGHT,
    minHeight: estimatedOptionHeight + 8,
    minWidth: menuMinWidth,
    placement,
  });
}

export function getSelectMenuButtonClassName({
  roundedClassName,
  surface,
  textClassName,
  className,
}: {
  roundedClassName: string;
  surface: UiSelectMenuSurface;
  textClassName: string;
  className?: string;
}) {
  return cn(
    "flex h-full w-full items-center justify-between gap-2 px-3 transition-[background,border-color,box-shadow] duration-(--motion-duration-fast) focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
    SELECT_MENU_BUTTON_SURFACE_CLASS_NAMES[surface],
    roundedClassName,
    textClassName,
    className,
  );
}

export function getSelectMenuPanelSurfaceClassName(surface: UiSelectMenuSurface): string {
  return SELECT_MENU_PANEL_SURFACE_CLASS_NAMES[surface];
}

export function getSelectMenuOptionStateClassName(surface: UiSelectMenuSurface, isActive: boolean): string {
  return SELECT_MENU_OPTION_STATE_CLASS_NAMES[surface][
    isActive ? "active" : "inactive"
  ];
}
