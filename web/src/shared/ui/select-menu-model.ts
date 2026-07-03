import { cn } from "@/lib/utils";

export type UiSelectMenuPlacement = "auto" | "bottom" | "top";
export type UiSelectMenuSize = "xs" | "sm" | "md";
export type UiSelectMenuSurface = "surface" | "dialog";

export interface UiSelectMenuPosition {
  bottom?: number;
  left: number;
  maxHeight: number;
  placement: "bottom" | "top";
  top?: number;
  width: number;
}

const SELECT_MENU_GAP = 6;
const SELECT_MENU_VIEWPORT_MARGIN = 12;
const SELECT_MENU_MAX_HEIGHT = 280;

export const SELECT_MENU_SEARCH_ROW_HEIGHT = 44;

export function getSelectMenuSizeConfig(size: UiSelectMenuSize) {
  return {
    heightClassName: size === "xs" ? "h-7" : size === "sm" ? "h-9" : "h-10",
    roundedClassName: size === "xs" ? "rounded-[10px]" : size === "sm" ? "rounded-[12px]" : "rounded-[13px]",
    textClassName: size === "xs" ? "text-[11px]" : size === "sm" ? "text-[12px]" : "text-[13px]",
    optionHeightClassName: size === "xs" ? "min-h-7 text-[12px]" : "min-h-8 text-[13px]",
    estimatedOptionHeight: size === "xs" ? 28 : 32,
  };
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
}): UiSelectMenuPosition {
  const rect = button.getBoundingClientRect();
  const viewportWidth = window.innerWidth;
  const viewportHeight = window.innerHeight;
  const availableAbove = Math.max(0, rect.top - SELECT_MENU_VIEWPORT_MARGIN);
  const availableBelow = Math.max(0, viewportHeight - rect.bottom - SELECT_MENU_VIEWPORT_MARGIN);
  const shouldPlaceTop = placement === "top"
    || (placement === "auto" && availableBelow < estimatedHeight && availableAbove > availableBelow);
  const availableSpace = shouldPlaceTop ? availableAbove : availableBelow;
  const maxHeight = Math.min(
    SELECT_MENU_MAX_HEIGHT,
    estimatedHeight,
    Math.max(estimatedOptionHeight + 8, availableSpace - SELECT_MENU_GAP),
  );
  const width = Math.min(
    Math.max(rect.width, menuMinWidth ?? 0),
    viewportWidth - SELECT_MENU_VIEWPORT_MARGIN * 2,
  );
  const left = Math.min(
    Math.max(SELECT_MENU_VIEWPORT_MARGIN, rect.left),
    Math.max(SELECT_MENU_VIEWPORT_MARGIN, viewportWidth - width - SELECT_MENU_VIEWPORT_MARGIN),
  );

  return {
    left,
    width,
    maxHeight,
    placement: shouldPlaceTop ? "top" : "bottom",
    ...(shouldPlaceTop
      ? { bottom: Math.max(SELECT_MENU_VIEWPORT_MARGIN, viewportHeight - rect.top + SELECT_MENU_GAP) }
      : { top: Math.min(rect.bottom + SELECT_MENU_GAP, viewportHeight - SELECT_MENU_VIEWPORT_MARGIN - maxHeight) }),
  };
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
    surface === "dialog"
      ? "dialog-input shadow-none hover:border-[color:color-mix(in_srgb,var(--primary)_24%,var(--modal-input-border))] hover:bg-[color:color-mix(in_srgb,var(--modal-input-focus-background)_72%,transparent)] focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_14%,transparent)]"
      : "border border-[color:color-mix(in_srgb,var(--primary)_22%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--background)_94%,white)] shadow-[0_1px_2px_rgba(15,23,42,0.04)] hover:border-[color:color-mix(in_srgb,var(--primary)_38%,var(--divider-subtle-color))] focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_18%,transparent)]",
    roundedClassName,
    textClassName,
    className,
  );
}

export function getSelectMenuPanelSurfaceClassName(surface: UiSelectMenuSurface): string {
  return surface === "dialog"
    ? "border-(--modal-card-border) bg-[color:color-mix(in_srgb,var(--background)_94%,white)] shadow-[0_16px_36px_rgba(15,23,42,0.14)]"
    : "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_96%,white)] shadow-[0_14px_32px_rgba(15,23,42,0.12)] backdrop-blur";
}

export function getSelectMenuOptionStateClassName(surface: UiSelectMenuSurface, isActive: boolean): string {
  if (isActive) {
    return surface === "dialog"
      ? "bg-[color:color-mix(in_srgb,var(--primary)_13%,transparent)] font-semibold text-(--text-strong) hover:bg-[color:color-mix(in_srgb,var(--primary)_16%,transparent)]"
      : "bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)] font-semibold text-(--text-strong) hover:bg-[color:color-mix(in_srgb,var(--primary)_14%,transparent)]";
  }

  return surface === "dialog"
    ? "text-(--text-default) hover:bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] hover:text-(--text-strong)"
    : "text-(--text-default) hover:bg-(--surface-interactive-hover-background)";
}
