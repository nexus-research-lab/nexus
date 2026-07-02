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
  aria_label: ariaLabel,
  allow_label_wrap: allowLabelWrap = false,
  button_class_name: buttonClassName,
  class_name: className,
  disabled = false,
  id,
  label,
  leading,
  menu_class_name: menuClassName,
  menu_min_width: menuMinWidth,
  on_change: onChange,
  options,
  placement = "auto",
  placeholder = "请选择",
  size = "md",
  surface = "surface",
  value,
}: UiSelectMenuProps) {
  const enabledOptions = useMemo(
    () => options.filter((option) => !option.disabled),
    [options],
  );
  const activeOption = options.find((option) => option.value === value);
  const {
    estimated_option_height: estimatedOptionHeight,
    height_class_name: heightClassName,
    option_height_class_name: optionHeightClassName,
    rounded_class_name: roundedClassName,
    text_class_name: textClassName,
  } = get_select_menu_size_config(size);

  const estimatePosition = useCallback((button: HTMLButtonElement) => {
    const resolvedOptionHeight = allowLabelWrap
      ? Math.max(estimatedOptionHeight, 46)
      : estimatedOptionHeight;
    return resolve_select_menu_position({
      button,
      estimated_height: estimate_select_menu_height(options.length, resolvedOptionHeight),
      estimated_option_height: resolvedOptionHeight,
      menu_min_width: menuMinWidth,
      placement,
    });
  }, [allowLabelWrap, estimatedOptionHeight, menuMinWidth, options.length, placement]);

  const {
    button_ref: buttonRef,
    is_open: isOpen,
    menu_id: menuId,
    menu_position: menuPosition,
    menu_ref: menuRef,
    menu_style: menuStyle,
    portal_container: portalContainer,
    root_ref: rootRef,
    set_is_open: setIsOpen,
    update_menu_position: updateMenuPosition,
  } = useSelectMenuLayer({ disabled, estimate_position: estimatePosition });

  const changeValue = (nextValue: string) => {
    if (disabled) {
      return;
    }
    onChange(nextValue);
    setIsOpen(false);
    buttonRef.current?.focus();
  };

  const moveSelection = (direction: 1 | -1) => {
    if (disabled || enabledOptions.length === 0) {
      return;
    }
    const currentIndex = Math.max(
      0,
      enabledOptions.findIndex((option) => option.value === value),
    );
    const nextIndex = (currentIndex + direction + enabledOptions.length) % enabledOptions.length;
    onChange(enabledOptions[nextIndex].value);
    updateMenuPosition();
    setIsOpen(true);
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (disabled) {
      return;
    }

    if (event.key === "Escape") {
      setIsOpen(false);
      return;
    }

    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      setIsOpen((open) => {
        if (!open) {
          updateMenuPosition();
        }
        return !open;
      });
      return;
    }

    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      moveSelection(event.key === "ArrowDown" ? 1 : -1);
    }
  };

  const menu = isOpen ? (
    <div
      ref={menuRef}
      aria-label={ariaLabel}
      className={cn(
        "fixed z-[120] overflow-y-auto rounded-[14px] border p-1 animate-in fade-in-0 zoom-in-95 duration-(--motion-duration-fast) data-[placement=bottom]:slide-in-from-top-1 data-[placement=top]:slide-in-from-bottom-1",
        get_select_menu_panel_surface_class_name(surface),
        menuClassName,
      )}
      data-placement={menuPosition?.placement ?? "bottom"}
      data-state="open"
      data-surface={surface}
      data-ui-select-menu-open="true"
      id={menuId}
      role="listbox"
      style={menuStyle}
    >
      {options.map((option) => {
        const isActive = option.value === value;
        return (
          <button
            key={option.value}
            aria-selected={isActive}
            className={cn(
              "flex w-full justify-between gap-2 rounded-[10px] px-2.5 text-left transition-[background-color,color] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
              allowLabelWrap ? "items-start py-2" : "items-center",
              optionHeightClassName,
              get_select_menu_option_state_class_name(surface, isActive),
            )}
            data-active={isActive ? "true" : undefined}
            disabled={option.disabled}
            onClick={() => changeValue(option.value)}
            role="option"
            type="button"
          >
            <span
              className={cn(
                "min-w-0 flex-1",
                allowLabelWrap ? "whitespace-normal break-words leading-snug" : "truncate",
              )}
            >
              {option.label}
            </span>
            {isActive ? <Check className="mt-0.5 h-3.5 w-3.5 shrink-0 text-(--primary)" /> : null}
          </button>
        );
      })}
    </div>
  ) : null;

  return (
    <div
      ref={rootRef}
      className={cn("relative w-full", heightClassName, className)}
      data-ui-select-menu-open={isOpen ? "true" : undefined}
    >
      <button
        ref={buttonRef}
        aria-controls={isOpen ? menuId : undefined}
        aria-disabled={disabled}
        aria-expanded={isOpen}
        aria-haspopup="listbox"
        aria-label={ariaLabel}
        className={get_select_menu_button_class_name({
          rounded_class_name: roundedClassName,
          surface,
          text_class_name: textClassName,
          class_name: buttonClassName,
        })}
        disabled={disabled}
        id={id}
        onClick={() => {
          setIsOpen((open) => {
            if (!open) {
              updateMenuPosition();
            }
            return !open;
          });
        }}
        onKeyDown={handleKeyDown}
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
              allowLabelWrap ? "whitespace-normal break-words text-left leading-snug" : "truncate",
            )}
          >
            {activeOption?.label ?? placeholder}
          </span>
        </span>
        <ChevronDown
          className={cn(
            "h-4 w-4 shrink-0 text-(--icon-muted) transition-transform",
            isOpen && "rotate-180",
          )}
        />
      </button>

      {menu && portalContainer ? createPortal(menu, portalContainer) : null}
    </div>
  );
}
