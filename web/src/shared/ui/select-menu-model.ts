import { cn } from "@/lib/utils";

export type UiSelectMenuPlacement = "auto" | "bottom" | "top";
export type UiSelectMenuSize = "xs" | "sm" | "md";
export type UiSelectMenuSurface = "surface" | "dialog";

export interface UiSelectMenuPosition {
  bottom?: number;
  left: number;
  max_height: number;
  placement: "bottom" | "top";
  top?: number;
  width: number;
}

const SELECT_MENU_GAP = 6;
const SELECT_MENU_VIEWPORT_MARGIN = 12;
const SELECT_MENU_MAX_HEIGHT = 280;

export const SELECT_MENU_SEARCH_ROW_HEIGHT = 44;

export function get_select_menu_size_config(size: UiSelectMenuSize) {
  return {
    height_class_name: size === "xs" ? "h-7" : size === "sm" ? "h-9" : "h-10",
    rounded_class_name: size === "xs" ? "rounded-[10px]" : size === "sm" ? "rounded-[12px]" : "rounded-[13px]",
    text_class_name: size === "xs" ? "text-[11px]" : size === "sm" ? "text-[12px]" : "text-[13px]",
    option_height_class_name: size === "xs" ? "min-h-7 text-[12px]" : "min-h-8 text-[13px]",
    estimated_option_height: size === "xs" ? 28 : 32,
  };
}

export function estimate_select_menu_height(option_count: number, option_height: number, extra_height = 8): number {
  return Math.min(
    SELECT_MENU_MAX_HEIGHT,
    Math.max(option_height + 8, option_count * option_height + extra_height),
  );
}

export function resolve_select_menu_position({
  button,
  estimated_height,
  estimated_option_height,
  menu_min_width,
  placement,
}: {
  button: HTMLButtonElement;
  estimated_height: number;
  estimated_option_height: number;
  placement: UiSelectMenuPlacement;
  menu_min_width?: number;
}): UiSelectMenuPosition {
  const rect = button.getBoundingClientRect();
  const viewport_width = window.innerWidth;
  const viewport_height = window.innerHeight;
  const available_above = Math.max(0, rect.top - SELECT_MENU_VIEWPORT_MARGIN);
  const available_below = Math.max(0, viewport_height - rect.bottom - SELECT_MENU_VIEWPORT_MARGIN);
  const should_place_top = placement === "top"
    || (placement === "auto" && available_below < estimated_height && available_above > available_below);
  const available_space = should_place_top ? available_above : available_below;
  const max_height = Math.min(
    SELECT_MENU_MAX_HEIGHT,
    estimated_height,
    Math.max(estimated_option_height + 8, available_space - SELECT_MENU_GAP),
  );
  const width = Math.min(
    Math.max(rect.width, menu_min_width ?? 0),
    viewport_width - SELECT_MENU_VIEWPORT_MARGIN * 2,
  );
  const left = Math.min(
    Math.max(SELECT_MENU_VIEWPORT_MARGIN, rect.left),
    Math.max(SELECT_MENU_VIEWPORT_MARGIN, viewport_width - width - SELECT_MENU_VIEWPORT_MARGIN),
  );

  return {
    left,
    width,
    max_height,
    placement: should_place_top ? "top" : "bottom",
    ...(should_place_top
      ? { bottom: Math.max(SELECT_MENU_VIEWPORT_MARGIN, viewport_height - rect.top + SELECT_MENU_GAP) }
      : { top: Math.min(rect.bottom + SELECT_MENU_GAP, viewport_height - SELECT_MENU_VIEWPORT_MARGIN - max_height) }),
  };
}

export function get_select_menu_button_class_name({
  rounded_class_name,
  surface,
  text_class_name,
  class_name,
}: {
  rounded_class_name: string;
  surface: UiSelectMenuSurface;
  text_class_name: string;
  class_name?: string;
}) {
  return cn(
    "flex h-full w-full items-center justify-between gap-2 px-3 transition-[background,border-color,box-shadow] duration-(--motion-duration-fast) focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
    surface === "dialog"
      ? "dialog-input shadow-none hover:border-[color:color-mix(in_srgb,var(--primary)_24%,var(--modal-input-border))] hover:bg-[color:color-mix(in_srgb,var(--modal-input-focus-background)_72%,transparent)] focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_14%,transparent)]"
      : "border border-[color:color-mix(in_srgb,var(--primary)_22%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--background)_94%,white)] shadow-[0_1px_2px_rgba(15,23,42,0.04)] hover:border-[color:color-mix(in_srgb,var(--primary)_38%,var(--divider-subtle-color))] focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_18%,transparent)]",
    rounded_class_name,
    text_class_name,
    class_name,
  );
}

export function get_select_menu_panel_surface_class_name(surface: UiSelectMenuSurface): string {
  return surface === "dialog"
    ? "border-(--modal-card-border) bg-[color:color-mix(in_srgb,var(--background)_94%,white)] shadow-[0_16px_36px_rgba(15,23,42,0.14)]"
    : "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_96%,white)] shadow-[0_14px_32px_rgba(15,23,42,0.12)] backdrop-blur";
}

export function get_select_menu_option_state_class_name(surface: UiSelectMenuSurface, is_active: boolean): string {
  if (is_active) {
    return surface === "dialog"
      ? "bg-[color:color-mix(in_srgb,var(--primary)_13%,transparent)] font-semibold text-(--text-strong) hover:bg-[color:color-mix(in_srgb,var(--primary)_16%,transparent)]"
      : "bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)] font-semibold text-(--text-strong) hover:bg-[color:color-mix(in_srgb,var(--primary)_14%,transparent)]";
  }

  return surface === "dialog"
    ? "text-(--text-default) hover:bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] hover:text-(--text-strong)"
    : "text-(--text-default) hover:bg-(--surface-interactive-hover-background)";
}
