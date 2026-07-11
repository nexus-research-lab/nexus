"use client";

import {
  type ReactNode,
  useCallback,
  useMemo,
} from "react";
import { createPortal } from "react-dom";
import { Check, Loader2, Search, X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import {
  estimateSelectMenuHeight,
  getSelectMenuButtonClassName,
  getSelectMenuOptionStateClassName,
  getSelectMenuSizeConfig,
  resolveSelectMenuPosition,
  SELECT_MENU_SEARCH_ROW_HEIGHT,
  type UiSelectMenuPlacement,
  type UiSelectMenuSize,
  type UiSelectMenuSurface,
} from "./select-menu-model";
import {
  SelectMenuPanel,
  SelectMenuTriggerContent,
} from "./select-menu-primitives";
import { useSelectMenuOverlay } from "./use-select-menu-overlay";

interface UiMultiSelectMenuOption {
  value: string;
  label: string;
  disabled?: boolean;
  description?: ReactNode;
}

interface UiMultiSelectMenuProps {
  ariaLabel: string;
  buttonClassName?: string;
  className?: string;
  disabled?: boolean;
  emptyText?: ReactNode;
  errorText?: ReactNode;
  id?: string;
  isLoading?: boolean;
  label?: ReactNode;
  leading?: ReactNode;
  loadingText?: ReactNode;
  menuClassName?: string;
  onChange: (value: string[]) => void;
  onQueryChange?: (value: string) => void;
  options: UiMultiSelectMenuOption[];
  placement?: UiSelectMenuPlacement;
  placeholder?: ReactNode;
  query?: string;
  searchPlaceholder?: string;
  size?: UiSelectMenuSize;
  surface?: UiSelectMenuSurface;
  value: string[];
}

export function UiMultiSelectMenu({
  ariaLabel: ariaLabel,
  buttonClassName: buttonClassName,
  className: className,
  disabled = false,
  emptyText: emptyText = "暂无选项",
  errorText: errorText,
  id,
  isLoading: isLoading = false,
  label,
  leading,
  loadingText: loadingText = "加载中...",
  menuClassName: menuClassName,
  onChange: onChange,
  onQueryChange: onQueryChange,
  options,
  placement = "auto",
  placeholder = "请选择",
  query = "",
  searchPlaceholder: searchPlaceholder = "搜索",
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
    estimatedOptionHeight,
    heightClassName,
    optionHeightClassName,
    roundedClassName,
    textClassName,
  } = getSelectMenuSizeConfig(size);
  const hasSearch = Boolean(onQueryChange);

  const estimatePosition = useCallback((button: HTMLButtonElement) => {
    return resolveSelectMenuPosition({
      button,
      estimatedHeight: estimateSelectMenuHeight(
        Math.max(options.length, 1),
        hasOptionDescription ? 52 : estimatedOptionHeight,
        hasSearch ? SELECT_MENU_SEARCH_ROW_HEIGHT + 8 : 8,
      ),
      estimatedOptionHeight: hasOptionDescription ? 52 : estimatedOptionHeight,
      placement,
    });
  }, [estimatedOptionHeight, hasOptionDescription, hasSearch, options.length, placement]);

  const {
    buttonRef,
    handleTriggerKeyDown,
    isOpen,
    menuId,
    menuPosition,
    menuRef,
    menuStyle,
    portalContainer,
    toggleMenu,
    updateMenuPosition,
  } = useSelectMenuOverlay({
    disabled,
    estimatePosition,
  });

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
  const menu = isOpen ? (
    <SelectMenuPanel
      ariaLabel={ariaLabel}
      id={menuId}
      layoutClassName="flex flex-col overflow-hidden"
      menuClassName={menuClassName}
      panelRef={menuRef}
      placement={menuPosition?.placement}
      style={menuStyle}
      surface={surface}
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
                  getSelectMenuOptionStateClassName(surface, isActive),
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
    </SelectMenuPanel>
  ) : null;

  return (
    <div
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
        className={getSelectMenuButtonClassName({
          roundedClassName,
          surface,
          textClassName,
          className: cn(value.length > 0 && "min-h-10 py-1.5", buttonClassName),
        })}
        disabled={disabled}
        id={id}
        onClick={toggleMenu}
        onKeyDown={handleTriggerKeyDown}
        type="button"
      >
        <SelectMenuTriggerContent
          isOpen={isOpen}
          label={label}
          leading={leading}
        >
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
        </SelectMenuTriggerContent>
      </button>

      {menu && portalContainer ? createPortal(menu, portalContainer) : null}
    </div>
  );
}
