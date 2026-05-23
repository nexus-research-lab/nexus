"use client";

import {
  type CSSProperties,
  type KeyboardEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { Check, ChevronDown, Loader2, Search, X } from "lucide-react";

import { cn } from "@/lib/utils";

export interface UiSelectMenuOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface UiMultiSelectMenuOption extends UiSelectMenuOption {
  description?: ReactNode;
}

type UiSelectMenuPlacement = "auto" | "bottom" | "top";
type UiSelectMenuSize = "xs" | "sm" | "md";
type UiSelectMenuSurface = "surface" | "dialog";

interface UiSelectMenuProps {
  aria_label: string;
  button_class_name?: string;
  class_name?: string;
  disabled?: boolean;
  id?: string;
  label?: ReactNode;
  leading?: ReactNode;
  menu_class_name?: string;
  on_change: (value: string) => void;
  options: UiSelectMenuOption[];
  placement?: UiSelectMenuPlacement;
  placeholder?: string;
  size?: UiSelectMenuSize;
  surface?: UiSelectMenuSurface;
  value: string;
}

interface UiMultiSelectMenuProps {
  aria_label: string;
  button_class_name?: string;
  class_name?: string;
  disabled?: boolean;
  empty_text?: ReactNode;
  error_text?: ReactNode;
  id?: string;
  is_loading?: boolean;
  label?: ReactNode;
  leading?: ReactNode;
  loading_text?: ReactNode;
  menu_class_name?: string;
  on_change: (value: string[]) => void;
  on_query_change?: (value: string) => void;
  options: UiMultiSelectMenuOption[];
  placement?: UiSelectMenuPlacement;
  placeholder?: ReactNode;
  query?: string;
  search_placeholder?: string;
  size?: UiSelectMenuSize;
  surface?: UiSelectMenuSurface;
  value: string[];
}

interface UiSelectMenuPosition {
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
const SELECT_MENU_SEARCH_ROW_HEIGHT = 44;

function get_select_menu_size_config(size: UiSelectMenuSize) {
  return {
    height_class_name: size === "xs" ? "h-7" : size === "sm" ? "h-9" : "h-10",
    rounded_class_name: size === "xs" ? "rounded-[10px]" : size === "sm" ? "rounded-[12px]" : "rounded-[13px]",
    text_class_name: size === "xs" ? "text-[11px]" : size === "sm" ? "text-[12px]" : "text-[13px]",
    option_height_class_name: size === "xs" ? "min-h-7 text-[12px]" : "min-h-8 text-[13px]",
    estimated_option_height: size === "xs" ? 28 : 32,
  };
}

function estimate_select_menu_height(option_count: number, option_height: number, extra_height = 8): number {
  return Math.min(
    SELECT_MENU_MAX_HEIGHT,
    Math.max(option_height + 8, option_count * option_height + extra_height),
  );
}

function resolve_select_menu_position({
  button,
  estimated_height,
  estimated_option_height,
  placement,
}: {
  button: HTMLButtonElement;
  estimated_height: number;
  estimated_option_height: number;
  placement: UiSelectMenuPlacement;
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
  const width = Math.min(rect.width, viewport_width - SELECT_MENU_VIEWPORT_MARGIN * 2);
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

function get_select_menu_button_class_name({
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

function get_select_menu_panel_surface_class_name(surface: UiSelectMenuSurface): string {
  return surface === "dialog"
    ? "border-(--modal-card-border) bg-[color:color-mix(in_srgb,var(--background)_94%,white)] shadow-[0_16px_36px_rgba(15,23,42,0.14)]"
    : "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_96%,white)] shadow-[0_14px_32px_rgba(15,23,42,0.12)] backdrop-blur";
}

function get_select_menu_option_state_class_name(surface: UiSelectMenuSurface, is_active: boolean): string {
  if (is_active) {
    return surface === "dialog"
      ? "bg-[color:color-mix(in_srgb,var(--primary)_13%,transparent)] font-semibold text-(--text-strong) hover:bg-[color:color-mix(in_srgb,var(--primary)_16%,transparent)]"
      : "bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)] font-semibold text-(--text-strong) hover:bg-[color:color-mix(in_srgb,var(--primary)_14%,transparent)]";
  }

  return surface === "dialog"
    ? "text-(--text-default) hover:bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] hover:text-(--text-strong)"
    : "text-(--text-default) hover:bg-(--surface-interactive-hover-background)";
}

/** 共享自定义下拉菜单，避免业务侧重复实现原生 select 无法控制的弹层定位。 */
export function UiSelectMenu({
  aria_label,
  button_class_name,
  class_name,
  disabled = false,
  id,
  label,
  leading,
  menu_class_name,
  on_change,
  options,
  placement = "auto",
  placeholder = "请选择",
  size = "md",
  surface = "surface",
  value,
}: UiSelectMenuProps) {
  const [is_open, set_is_open] = useState(false);
  const [menu_position, set_menu_position] = useState<UiSelectMenuPosition | null>(null);
  const menu_id = useId();
  const root_ref = useRef<HTMLDivElement>(null);
  const button_ref = useRef<HTMLButtonElement>(null);
  const menu_ref = useRef<HTMLDivElement>(null);
  const enabled_options = useMemo(
    () => options.filter((option) => !option.disabled),
    [options],
  );
  const active_option = options.find((option) => option.value === value);
  const {
    estimated_option_height,
    height_class_name,
    option_height_class_name,
    rounded_class_name,
    text_class_name,
  } = get_select_menu_size_config(size);

  const update_menu_position = useCallback(() => {
    const button = button_ref.current;
    if (!button) {
      return;
    }
    set_menu_position(resolve_select_menu_position({
      button,
      estimated_height: estimate_select_menu_height(options.length, estimated_option_height),
      estimated_option_height,
      placement,
    }));
  }, [estimated_option_height, options.length, placement]);

  useEffect(() => {
    if (!is_open || disabled) {
      return;
    }

    const handle_pointer_down = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!root_ref.current?.contains(target) && !menu_ref.current?.contains(target)) {
        set_is_open(false);
      }
    };
    const handle_key_down = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        set_is_open(false);
        button_ref.current?.focus();
      }
    };

    document.addEventListener("pointerdown", handle_pointer_down, true);
    document.addEventListener("keydown", handle_key_down);
    window.addEventListener("resize", update_menu_position);
    window.addEventListener("scroll", update_menu_position, true);
    return () => {
      document.removeEventListener("pointerdown", handle_pointer_down, true);
      document.removeEventListener("keydown", handle_key_down);
      window.removeEventListener("resize", update_menu_position);
      window.removeEventListener("scroll", update_menu_position, true);
    };
  }, [disabled, is_open, update_menu_position]);

  useLayoutEffect(() => {
    if (is_open) {
      update_menu_position();
    }
  }, [is_open, update_menu_position]);

  const change_value = (next_value: string) => {
    if (disabled) {
      return;
    }
    on_change(next_value);
    set_is_open(false);
    button_ref.current?.focus();
  };

  const move_selection = (direction: 1 | -1) => {
    if (disabled || enabled_options.length === 0) {
      return;
    }
    const current_index = Math.max(
      0,
      enabled_options.findIndex((option) => option.value === value),
    );
    const next_index = (current_index + direction + enabled_options.length) % enabled_options.length;
    on_change(enabled_options[next_index].value);
    update_menu_position();
    set_is_open(true);
  };

  const handle_key_down = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (disabled) {
      return;
    }

    if (event.key === "Escape") {
      set_is_open(false);
      return;
    }

    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      set_is_open((open) => {
        if (!open) {
          update_menu_position();
        }
        return !open;
      });
      return;
    }

    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      move_selection(event.key === "ArrowDown" ? 1 : -1);
    }
  };

  const menu_style: CSSProperties = {
    bottom: menu_position?.bottom,
    left: menu_position?.left,
    maxHeight: menu_position?.max_height,
    top: menu_position?.top,
    visibility: menu_position ? "visible" : "hidden",
    width: menu_position?.width,
  };
  const portal_container = typeof document === "undefined"
    ? null
    : root_ref.current?.closest("[data-modal-root='true']") ?? document.body;
  const menu = is_open ? (
    <div
      ref={menu_ref}
      aria-label={aria_label}
      className={cn(
        "fixed z-[120] overflow-y-auto rounded-[14px] border p-1 animate-in fade-in-0 zoom-in-95 duration-(--motion-duration-fast) data-[placement=bottom]:slide-in-from-top-1 data-[placement=top]:slide-in-from-bottom-1",
        get_select_menu_panel_surface_class_name(surface),
        menu_class_name,
      )}
      data-placement={menu_position?.placement ?? "bottom"}
      data-state="open"
      data-surface={surface}
      data-ui-select-menu-open="true"
      id={menu_id}
      role="listbox"
      style={menu_style}
    >
      {options.map((option) => {
        const is_active = option.value === value;
        return (
          <button
            key={option.value}
            aria-selected={is_active}
            className={cn(
              "flex w-full items-center justify-between rounded-[10px] px-2.5 text-left transition-[background-color,color] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
              option_height_class_name,
              get_select_menu_option_state_class_name(surface, is_active),
            )}
            data-active={is_active ? "true" : undefined}
            disabled={option.disabled}
            onClick={() => change_value(option.value)}
            role="option"
            type="button"
          >
            <span className="truncate">{option.label}</span>
            {is_active ? <Check className="h-3.5 w-3.5 text-(--primary)" /> : null}
          </button>
        );
      })}
    </div>
  ) : null;

  return (
    <div
      ref={root_ref}
      className={cn("relative w-full", height_class_name, class_name)}
      data-ui-select-menu-open={is_open ? "true" : undefined}
    >
      <button
        ref={button_ref}
        aria-controls={is_open ? menu_id : undefined}
        aria-disabled={disabled}
        aria-expanded={is_open}
        aria-haspopup="listbox"
        aria-label={aria_label}
        className={get_select_menu_button_class_name({
          rounded_class_name,
          surface,
          text_class_name,
          class_name: button_class_name,
        })}
        disabled={disabled}
        id={id}
        onClick={() => {
          set_is_open((open) => {
            if (!open) {
              update_menu_position();
            }
            return !open;
          });
        }}
        onKeyDown={handle_key_down}
        type="button"
      >
        <span className="flex min-w-0 items-center gap-2">
          {leading ? <span className="shrink-0 text-(--icon-default)">{leading}</span> : null}
          {label ? (
            <>
              <span className="shrink-0 text-[12px] font-medium text-(--text-muted)">
                {label}
              </span>
              <span className="h-3.5 w-px shrink-0 bg-(--divider-subtle-color)" />
            </>
          ) : null}
          <span className="truncate font-semibold text-(--text-strong)">
            {active_option?.label ?? placeholder}
          </span>
        </span>
        <ChevronDown
          className={cn(
            "h-4 w-4 shrink-0 text-(--icon-muted) transition-transform",
            is_open && "rotate-180",
          )}
        />
      </button>

      {menu && portal_container ? createPortal(menu, portal_container) : null}
    </div>
  );
}

export function UiMultiSelectMenu({
  aria_label,
  button_class_name,
  class_name,
  disabled = false,
  empty_text = "暂无选项",
  error_text,
  id,
  is_loading = false,
  label,
  leading,
  loading_text = "加载中...",
  menu_class_name,
  on_change,
  on_query_change,
  options,
  placement = "auto",
  placeholder = "请选择",
  query = "",
  search_placeholder = "搜索",
  size = "md",
  surface = "surface",
  value,
}: UiMultiSelectMenuProps) {
  const [is_open, set_is_open] = useState(false);
  const [menu_position, set_menu_position] = useState<UiSelectMenuPosition | null>(null);
  const menu_id = useId();
  const root_ref = useRef<HTMLDivElement>(null);
  const button_ref = useRef<HTMLButtonElement>(null);
  const menu_ref = useRef<HTMLDivElement>(null);
  const selected_value_set = useMemo(() => new Set(value), [value]);
  const selected_options = useMemo(
    () => value.map((item) => options.find((option) => option.value === item) ?? { value: item, label: item }),
    [options, value],
  );
  const has_option_description = options.some((option) => Boolean(option.description));
  const {
    estimated_option_height,
    height_class_name,
    option_height_class_name,
    rounded_class_name,
    text_class_name,
  } = get_select_menu_size_config(size);
  const has_search = Boolean(on_query_change);

  const update_menu_position = useCallback(() => {
    const button = button_ref.current;
    if (!button) {
      return;
    }
    set_menu_position(resolve_select_menu_position({
      button,
      estimated_height: estimate_select_menu_height(
        Math.max(options.length, 1),
        has_option_description ? 52 : estimated_option_height,
        has_search ? SELECT_MENU_SEARCH_ROW_HEIGHT + 8 : 8,
      ),
      estimated_option_height: has_option_description ? 52 : estimated_option_height,
      placement,
    }));
  }, [estimated_option_height, has_option_description, has_search, options.length, placement]);

  useEffect(() => {
    if (!is_open || disabled) {
      return;
    }

    const handle_pointer_down = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!root_ref.current?.contains(target) && !menu_ref.current?.contains(target)) {
        set_is_open(false);
      }
    };
    const handle_key_down = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        set_is_open(false);
        button_ref.current?.focus();
      }
    };

    document.addEventListener("pointerdown", handle_pointer_down, true);
    document.addEventListener("keydown", handle_key_down);
    window.addEventListener("resize", update_menu_position);
    window.addEventListener("scroll", update_menu_position, true);
    return () => {
      document.removeEventListener("pointerdown", handle_pointer_down, true);
      document.removeEventListener("keydown", handle_key_down);
      window.removeEventListener("resize", update_menu_position);
      window.removeEventListener("scroll", update_menu_position, true);
    };
  }, [disabled, is_open, update_menu_position]);

  useLayoutEffect(() => {
    if (is_open) {
      update_menu_position();
    }
  }, [is_open, update_menu_position]);

  const toggle_open = () => {
    if (disabled) {
      return;
    }
    set_is_open((open) => {
      if (!open) {
        update_menu_position();
      }
      return !open;
    });
  };

  const toggle_value = (next_value: string) => {
    if (disabled) {
      return;
    }
    const next_values = selected_value_set.has(next_value)
      ? value.filter((item) => item !== next_value)
      : [...value, next_value];
    on_change(next_values);
    update_menu_position();
  };

  const remove_value = (next_value: string) => {
    on_change(value.filter((item) => item !== next_value));
    update_menu_position();
  };

  const handle_key_down = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (disabled) {
      return;
    }
    if (event.key === "Escape") {
      set_is_open(false);
      return;
    }
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      toggle_open();
    }
  };

  const menu_style: CSSProperties = {
    bottom: menu_position?.bottom,
    left: menu_position?.left,
    maxHeight: menu_position?.max_height,
    top: menu_position?.top,
    visibility: menu_position ? "visible" : "hidden",
    width: menu_position?.width,
  };
  const portal_container = typeof document === "undefined"
    ? null
    : root_ref.current?.closest("[data-modal-root='true']") ?? document.body;
  const menu = is_open ? (
    <div
      ref={menu_ref}
      aria-label={aria_label}
      className={cn(
        "fixed z-[120] flex flex-col overflow-hidden rounded-[14px] border animate-in fade-in-0 zoom-in-95 duration-(--motion-duration-fast) data-[placement=bottom]:slide-in-from-top-1 data-[placement=top]:slide-in-from-bottom-1",
        get_select_menu_panel_surface_class_name(surface),
        menu_class_name,
      )}
      data-placement={menu_position?.placement ?? "bottom"}
      data-state="open"
      data-surface={surface}
      data-ui-select-menu-open="true"
      id={menu_id}
      role="listbox"
      style={menu_style}
    >
      {has_search ? (
        <label className="flex h-11 items-center gap-2 border-b border-(--divider-subtle-color) px-3">
          <Search className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
          <input
            className="min-w-0 flex-1 bg-transparent text-[13px] font-medium text-(--text-strong) outline-none placeholder:text-(--text-soft)"
            onChange={(event) => on_query_change?.(event.target.value)}
            placeholder={search_placeholder}
            type="search"
            value={query}
          />
        </label>
      ) : null}

      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto p-1">
        {is_loading ? (
          <div className="flex min-h-10 items-center gap-2 px-2.5 text-[13px] text-(--text-muted)">
            <Loader2 className="h-4 w-4 animate-spin" />
            {loading_text}
          </div>
        ) : error_text ? (
          <div className="m-1 rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_18%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--destructive)_7%,transparent)] px-2.5 py-2 text-[13px] leading-5 text-(--destructive)">
            {error_text}
          </div>
        ) : options.length === 0 ? (
          <div className="flex min-h-10 items-center px-2.5 text-[13px] text-(--text-muted)">
            {empty_text}
          </div>
        ) : (
          options.map((option) => {
            const is_active = selected_value_set.has(option.value);
            return (
              <button
                key={option.value}
                aria-selected={is_active}
                className={cn(
                  "flex w-full items-center gap-2 rounded-[10px] px-2.5 text-left transition-[background-color,color] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
                  option.description ? "py-2 text-[13px]" : option_height_class_name,
                  get_select_menu_option_state_class_name(surface, is_active),
                )}
                data-active={is_active ? "true" : undefined}
                disabled={option.disabled}
                onClick={() => toggle_value(option.value)}
                role="option"
                type="button"
              >
                <span className="min-w-0 flex-1">
                  <span className="block truncate">{option.label}</span>
                  {option.description ? (
                    <span className="mt-0.5 block truncate text-[11px] font-normal text-(--text-muted)">
                      {option.description}
                    </span>
                  ) : null}
                </span>
                <span className="flex h-4 w-4 shrink-0 items-center justify-center text-(--primary)">
                  {is_active ? <Check className="h-3.5 w-3.5" /> : null}
                </span>
              </button>
            );
          })
        )}
      </div>
    </div>
  ) : null;

  return (
    <div
      ref={root_ref}
      className={cn("relative w-full", value.length > 0 ? "min-h-10" : height_class_name, class_name)}
      data-ui-select-menu-open={is_open ? "true" : undefined}
    >
      <button
        ref={button_ref}
        aria-controls={is_open ? menu_id : undefined}
        aria-disabled={disabled}
        aria-expanded={is_open}
        aria-haspopup="listbox"
        aria-label={aria_label}
        className={get_select_menu_button_class_name({
          rounded_class_name,
          surface,
          text_class_name,
          class_name: cn(value.length > 0 && "min-h-10 py-1.5", button_class_name),
        })}
        disabled={disabled}
        id={id}
        onClick={toggle_open}
        onKeyDown={handle_key_down}
        type="button"
      >
        <span className="flex min-w-0 flex-1 items-center gap-2">
          {leading ? <span className="shrink-0 text-(--icon-default)">{leading}</span> : null}
          {label ? (
            <>
              <span className="shrink-0 text-[12px] font-medium text-(--text-muted)">
                {label}
              </span>
              <span className="h-3.5 w-px shrink-0 bg-(--divider-subtle-color)" />
            </>
          ) : null}
          <span className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
            {selected_options.length > 0 ? (
              selected_options.map((option) => {
                const accessible_label = typeof option.label === "string" || typeof option.label === "number"
                  ? String(option.label)
                  : option.value;
                return (
                  <span
                    key={option.value}
                    className="inline-flex max-w-[11rem] items-center gap-1 rounded-[6px] border border-(--divider-subtle-color) bg-transparent py-0.5 pl-2 pr-1 text-[11px] font-medium text-(--text-strong)"
                  >
                    <span className="min-w-0 truncate">{option.label}</span>
                    <span
                      aria-label={`移除 ${accessible_label}`}
                      className="flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-(--icon-muted) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
                      onClick={(event) => {
                        event.stopPropagation();
                        remove_value(option.value);
                      }}
                      onKeyDown={(event) => event.stopPropagation()}
                      role="button"
                      tabIndex={-1}
                    >
                      <X className="h-2.5 w-2.5" />
                    </span>
                  </span>
                );
              })
            ) : (
              <span className="truncate font-semibold text-(--text-muted)">
                {placeholder}
              </span>
            )}
          </span>
        </span>
        <ChevronDown
          className={cn(
            "h-4 w-4 shrink-0 text-(--icon-muted) transition-transform",
            is_open && "rotate-180",
          )}
        />
      </button>

      {menu && portal_container ? createPortal(menu, portal_container) : null}
    </div>
  );
}
