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

interface UiMultiSelectMenuOption {
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
  aria_label: ariaLabel,
  button_class_name: buttonClassName,
  class_name: className,
  disabled = false,
  empty_text: emptyText = "暂无选项",
  error_text: errorText,
  id,
  is_loading: isLoading = false,
  label,
  leading,
  loading_text: loadingText = "加载中...",
  menu_class_name: menuClassName,
  on_change: onChange,
  on_query_change: onQueryChange,
  options,
  placement = "auto",
  placeholder = "请选择",
  query = "",
  search_placeholder: searchPlaceholder = "搜索",
  size = "md",
  surface = "surface",
  value,
}: UiMultiSelectMenuProps) {
  const selectedValueSet = useMemo(() => new Set(value), [value]);
  const selectedOptions = useMemo(
    () => value.map((item) => options.find((option) => option.value === item) ?? { value: item, label: item }),
    [options, value],
  );
  const hasOptionDescription = options.some((option) => Boolean(option.description));
  const {
    estimated_option_height: estimatedOptionHeight,
    height_class_name: heightClassName,
    option_height_class_name: optionHeightClassName,
    rounded_class_name: roundedClassName,
    text_class_name: textClassName,
  } = get_select_menu_size_config(size);
  const hasSearch = Boolean(onQueryChange);

  const estimatePosition = useCallback((button: HTMLButtonElement) => {
    return resolve_select_menu_position({
      button,
      estimated_height: estimate_select_menu_height(
        Math.max(options.length, 1),
        hasOptionDescription ? 52 : estimatedOptionHeight,
        hasSearch ? SELECT_MENU_SEARCH_ROW_HEIGHT + 8 : 8,
      ),
      estimated_option_height: hasOptionDescription ? 52 : estimatedOptionHeight,
      placement,
    });
  }, [estimatedOptionHeight, hasOptionDescription, hasSearch, options.length, placement]);

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

  const toggleOpen = () => {
    if (disabled) {
      return;
    }
    setIsOpen((open) => {
      if (!open) {
        updateMenuPosition();
      }
      return !open;
    });
  };

  const toggleValue = (nextValue: string) => {
    if (disabled) {
      return;
    }
    const nextValues = selectedValueSet.has(nextValue)
      ? value.filter((item) => item !== nextValue)
      : [...value, nextValue];
    onChange(nextValues);
    updateMenuPosition();
  };

  const removeValue = (nextValue: string) => {
    onChange(value.filter((item) => item !== nextValue));
    updateMenuPosition();
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
      toggleOpen();
    }
  };

  const menu = isOpen ? (
    <div
      ref={menuRef}
      aria-label={ariaLabel}
      className={cn(
        "fixed z-[120] flex flex-col overflow-hidden rounded-[14px] border animate-in fade-in-0 zoom-in-95 duration-(--motion-duration-fast) data-[placement=bottom]:slide-in-from-top-1 data-[placement=top]:slide-in-from-bottom-1",
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
      {hasSearch ? (
        <label className="flex h-11 items-center gap-2 border-b border-(--divider-subtle-color) px-3">
          <Search className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
          <input
            className="min-w-0 flex-1 bg-transparent text-[13px] font-medium text-(--text-strong) outline-none placeholder:text-(--text-soft)"
            onChange={(event) => onQueryChange?.(event.target.value)}
            placeholder={searchPlaceholder}
            type="search"
            value={query}
          />
        </label>
      ) : null}

      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto p-1">
        {isLoading ? (
          <div className="flex min-h-10 items-center gap-2 px-2.5 text-[13px] text-(--text-muted)">
            <Loader2 className="h-4 w-4 animate-spin" />
            {loadingText}
          </div>
        ) : errorText ? (
          <div className="m-1 rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_18%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--destructive)_7%,transparent)] px-2.5 py-2 text-[13px] leading-5 text-(--destructive)">
            {errorText}
          </div>
        ) : options.length === 0 ? (
          <div className="flex min-h-10 items-center px-2.5 text-[13px] text-(--text-muted)">
            {emptyText}
          </div>
        ) : (
          options.map((option) => {
            const isActive = selectedValueSet.has(option.value);
            return (
              <button
                key={option.value}
                aria-selected={isActive}
                className={cn(
                  "flex w-full items-center gap-2 rounded-[10px] px-2.5 text-left transition-[background-color,color] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
                  option.description ? "py-2 text-[13px]" : optionHeightClassName,
                  get_select_menu_option_state_class_name(surface, isActive),
                )}
                data-active={isActive ? "true" : undefined}
                disabled={option.disabled}
                onClick={() => toggleValue(option.value)}
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
                  {isActive ? <Check className="h-3.5 w-3.5" /> : null}
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
      ref={rootRef}
      className={cn("relative w-full", value.length > 0 ? "min-h-10" : heightClassName, className)}
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
          class_name: cn(value.length > 0 && "min-h-10 py-1.5", buttonClassName),
        })}
        disabled={disabled}
        id={id}
        onClick={toggleOpen}
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
          <span className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
            {selectedOptions.length > 0 ? (
              selectedOptions.map((option) => {
                const accessibleLabel = typeof option.label === "string" || typeof option.label === "number"
                  ? String(option.label)
                  : option.value;
                return (
                  <span
                    key={option.value}
                    className="inline-flex max-w-[11rem] items-center gap-1 rounded-[6px] border border-(--divider-subtle-color) bg-transparent py-0.5 pl-2 pr-1 text-[11px] font-medium text-(--text-strong)"
                  >
                    <span className="min-w-0 truncate">{option.label}</span>
                    <span
                      aria-label={`移除 ${accessibleLabel}`}
                      className="flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-(--icon-muted) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
                      onClick={(event) => {
                        event.stopPropagation();
                        removeValue(option.value);
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
            isOpen && "rotate-180",
          )}
        />
      </button>

      {menu && portalContainer ? createPortal(menu, portalContainer) : null}
    </div>
  );
}
