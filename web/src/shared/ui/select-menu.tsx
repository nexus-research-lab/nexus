"use client";

import {
  type KeyboardEvent,
  type ReactNode,
  useCallback,
  useMemo,
} from "react";
import { createPortal } from "react-dom";
import { Check, ChevronDown } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  estimate_select_menu_height,
  get_select_menu_button_class_name,
  get_select_menu_option_state_class_name,
  get_select_menu_panel_surface_class_name,
  get_select_menu_size_config,
  resolve_select_menu_position,
  type UiSelectMenuPlacement,
  type UiSelectMenuSize,
  type UiSelectMenuSurface,
} from "./select-menu-model";
import { useSelectMenuLayer } from "./select-menu-layer";
export { UiMultiSelectMenu } from "./multi-select-menu";
export type { UiMultiSelectMenuOption } from "./multi-select-menu";

export interface UiSelectMenuOption {
  value: string;
  label: string;
  disabled?: boolean;
}

interface UiSelectMenuProps {
  aria_label: string;
  allow_label_wrap?: boolean;
  button_class_name?: string;
  class_name?: string;
  disabled?: boolean;
  id?: string;
  label?: ReactNode;
  leading?: ReactNode;
  menu_class_name?: string;
  menu_min_width?: number;
  on_change: (value: string) => void;
  options: UiSelectMenuOption[];
  placement?: UiSelectMenuPlacement;
  placeholder?: string;
  size?: UiSelectMenuSize;
  surface?: UiSelectMenuSurface;
  value: string;
}

/** 共享自定义下拉菜单，避免业务侧重复实现原生 select 无法控制的弹层定位。 */
export function UiSelectMenu({
  aria_label,
  allow_label_wrap = false,
  button_class_name,
  class_name,
  disabled = false,
  id,
  label,
  leading,
  menu_class_name,
  menu_min_width,
  on_change,
  options,
  placement = "auto",
  placeholder = "请选择",
  size = "md",
  surface = "surface",
  value,
}: UiSelectMenuProps) {
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

  const estimate_position = useCallback((button: HTMLButtonElement) => {
    const resolved_option_height = allow_label_wrap
      ? Math.max(estimated_option_height, 46)
      : estimated_option_height;
    return resolve_select_menu_position({
      button,
      estimated_height: estimate_select_menu_height(options.length, resolved_option_height),
      estimated_option_height: resolved_option_height,
      menu_min_width,
      placement,
    });
  }, [allow_label_wrap, estimated_option_height, menu_min_width, options.length, placement]);

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
              "flex w-full justify-between gap-2 rounded-[10px] px-2.5 text-left transition-[background-color,color] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
              allow_label_wrap ? "items-start py-2" : "items-center",
              option_height_class_name,
              get_select_menu_option_state_class_name(surface, is_active),
            )}
            data-active={is_active ? "true" : undefined}
            disabled={option.disabled}
            onClick={() => change_value(option.value)}
            role="option"
            type="button"
          >
            <span
              className={cn(
                "min-w-0 flex-1",
                allow_label_wrap ? "whitespace-normal break-words leading-snug" : "truncate",
              )}
            >
              {option.label}
            </span>
            {is_active ? <Check className="mt-0.5 h-3.5 w-3.5 shrink-0 text-(--primary)" /> : null}
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
          <span
            className={cn(
              "min-w-0 font-semibold text-(--text-strong)",
              allow_label_wrap ? "whitespace-normal break-words text-left leading-snug" : "truncate",
            )}
          >
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
