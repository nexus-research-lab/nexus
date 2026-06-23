"use client";

import {
  type KeyboardEvent,
  type ReactNode,
  useCallback,
  useMemo,
} from "react";
import { createPortal } from "react-dom";
import { Check, ChevronDown, Loader2, Search, X } from "lucide-react";

import { cn } from "@/lib/utils";
import { useSelectMenuLayer } from "./select-menu-layer";
import {
  estimate_select_menu_height,
  get_select_menu_button_class_name,
  get_select_menu_option_state_class_name,
  get_select_menu_panel_surface_class_name,
  get_select_menu_size_config,
  resolve_select_menu_position,
  SELECT_MENU_SEARCH_ROW_HEIGHT,
  type UiSelectMenuPlacement,
  type UiSelectMenuSize,
  type UiSelectMenuSurface,
} from "./select-menu-model";

export interface UiMultiSelectMenuOption {
  value: string;
  label: string;
  disabled?: boolean;
  description?: ReactNode;
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

  const estimate_position = useCallback((button: HTMLButtonElement) => {
    return resolve_select_menu_position({
      button,
      estimated_height: estimate_select_menu_height(
        Math.max(options.length, 1),
        has_option_description ? 52 : estimated_option_height,
        has_search ? SELECT_MENU_SEARCH_ROW_HEIGHT + 8 : 8,
      ),
      estimated_option_height: has_option_description ? 52 : estimated_option_height,
      placement,
    });
  }, [estimated_option_height, has_option_description, has_search, options.length, placement]);

  const {
    button_ref,
    is_open,
    menu_id,
    menu_position,
    menu_ref,
    menu_style,
    portal_container,
    root_ref,
    set_is_open,
    update_menu_position,
  } = useSelectMenuLayer({ disabled, estimate_position });

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
