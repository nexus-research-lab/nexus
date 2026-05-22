"use client";

import {
  type KeyboardEvent,
  type ReactNode,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";
import { Check, ChevronDown } from "lucide-react";

import { cn } from "@/lib/utils";

export interface UiSelectMenuOption {
  value: string;
  label: string;
  disabled?: boolean;
}

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
  placeholder?: string;
  size?: "xs" | "sm" | "md";
  value: string;
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
  placeholder = "请选择",
  size = "md",
  value,
}: UiSelectMenuProps) {
  const [is_open, set_is_open] = useState(false);
  const menu_id = useId();
  const root_ref = useRef<HTMLDivElement>(null);
  const button_ref = useRef<HTMLButtonElement>(null);
  const enabled_options = useMemo(
    () => options.filter((option) => !option.disabled),
    [options],
  );
  const active_option = options.find((option) => option.value === value);
  const height_class_name = size === "xs" ? "h-7" : size === "sm" ? "h-9" : "h-10";
  const rounded_class_name = size === "xs" ? "rounded-[10px]" : size === "sm" ? "rounded-[12px]" : "rounded-[13px]";
  const text_class_name = size === "xs" ? "text-[11px]" : size === "sm" ? "text-[12px]" : "text-[13px]";
  const option_height_class_name = size === "xs" ? "h-7 text-[12px]" : "h-8 text-[13px]";

  useEffect(() => {
    if (!is_open || disabled) {
      return;
    }

    const handle_pointer_down = (event: PointerEvent) => {
      if (!root_ref.current?.contains(event.target as Node)) {
        set_is_open(false);
      }
    };
    const handle_key_down = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        set_is_open(false);
        button_ref.current?.focus();
      }
    };

    document.addEventListener("pointerdown", handle_pointer_down);
    document.addEventListener("keydown", handle_key_down);
    return () => {
      document.removeEventListener("pointerdown", handle_pointer_down);
      document.removeEventListener("keydown", handle_key_down);
    };
  }, [disabled, is_open]);

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
      set_is_open((open) => !open);
      return;
    }

    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      move_selection(event.key === "ArrowDown" ? 1 : -1);
    }
  };

  return (
    <div ref={root_ref} className={cn("relative w-full", height_class_name, class_name)}>
      <button
        ref={button_ref}
        aria-controls={is_open ? menu_id : undefined}
        aria-disabled={disabled}
        aria-expanded={is_open}
        aria-haspopup="listbox"
        aria-label={aria_label}
        className={cn(
          "flex h-full w-full items-center justify-between gap-2 border border-[color:color-mix(in_srgb,var(--primary)_22%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--background)_94%,white)] px-3 shadow-[0_1px_2px_rgba(15,23,42,0.04)] transition-[border-color,box-shadow] hover:border-[color:color-mix(in_srgb,var(--primary)_38%,var(--divider-subtle-color))] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
          rounded_class_name,
          text_class_name,
          button_class_name,
        )}
        disabled={disabled}
        id={id}
        onClick={() => set_is_open((open) => !open)}
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

      {is_open ? (
        <div
          aria-label={aria_label}
          className={cn(
            "absolute left-0 right-0 top-[calc(100%+6px)] z-50 overflow-hidden rounded-[14px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_96%,white)] p-1 shadow-[0_14px_32px_rgba(15,23,42,0.12)] backdrop-blur",
            menu_class_name,
          )}
          id={menu_id}
          role="listbox"
        >
          {options.map((option) => {
            const is_active = option.value === value;
            return (
              <button
                key={option.value}
                aria-selected={is_active}
                className={cn(
                  "flex w-full items-center justify-between rounded-[10px] px-2.5 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
                  option_height_class_name,
                  is_active
                    ? "bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)] font-semibold text-(--text-strong)"
                    : "text-(--text-default) hover:bg-(--surface-interactive-hover-background)",
                )}
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
      ) : null}
    </div>
  );
}
