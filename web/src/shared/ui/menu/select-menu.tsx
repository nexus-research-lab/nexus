"use client";

import {
  type ReactNode,
  useCallback,
  useMemo,
} from "react";
import { createPortal } from "react-dom";
import { Check } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import {
  estimateSelectMenuHeight,
  getSelectMenuButtonClassName,
  getSelectMenuOptionStateClassName,
  getSelectMenuSizeConfig,
  resolveSelectMenuPosition,
  type UiSelectMenuPlacement,
  type UiSelectMenuSize,
  type UiSelectMenuSurface,
} from "./select-menu-model";
import {
  SelectMenuPanel,
  SelectMenuTriggerContent,
} from "./select-menu-primitives";
import { useSelectMenuOverlay } from "./use-select-menu-overlay";

export interface UiSelectMenuOption {
  value: string;
  label: string;
  disabled?: boolean;
}

interface UiSelectMenuProps {
  ariaLabel: string;
  allowLabelWrap?: boolean;
  buttonClassName?: string;
  className?: string;
  disabled?: boolean;
  id?: string;
  label?: ReactNode;
  leading?: ReactNode;
  menuClassName?: string;
  menuMinWidth?: number;
  onChange: (value: string) => void;
  options: UiSelectMenuOption[];
  placement?: UiSelectMenuPlacement;
  placeholder?: string;
  size?: UiSelectMenuSize;
  surface?: UiSelectMenuSurface;
  value: string;
}

/** 共享自定义下拉菜单，避免业务侧重复实现原生 select 无法控制的弹层定位。 */
export function UiSelectMenu({
  ariaLabel: ariaLabel,
  allowLabelWrap: allowLabelWrap = false,
  buttonClassName: buttonClassName,
  className: className,
  disabled = false,
  id,
  label,
  leading,
  menuClassName: menuClassName,
  menuMinWidth: menuMinWidth,
  onChange: onChange,
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
    estimatedOptionHeight,
    heightClassName,
    optionHeightClassName,
    roundedClassName,
    textClassName,
  } = getSelectMenuSizeConfig(size);

  const estimatePosition = useCallback((button: HTMLButtonElement) => {
    const resolvedOptionHeight = allowLabelWrap
      ? Math.max(estimatedOptionHeight, 46)
      : estimatedOptionHeight;
    return resolveSelectMenuPosition({
      button,
      estimatedHeight: estimateSelectMenuHeight(options.length, resolvedOptionHeight),
      estimatedOptionHeight: resolvedOptionHeight,
      menuMinWidth,
      placement,
    });
  }, [allowLabelWrap, estimatedOptionHeight, menuMinWidth, options.length, placement]);

  const {
    buttonRef,
    closeMenu,
    handleTriggerKeyDown,
    isOpen,
    menuId,
    menuPosition,
    menuRef,
    menuStyle,
    portalContainer,
    toggleMenu,
  } = useSelectMenuOverlay({
    disabled,
    estimatePosition,
  });

  const changeValue = (nextValue: string) => {
    if (disabled) {
      return;
    }
    onChange(nextValue);
    closeMenu();
    buttonRef.current?.focus();
  };

  const moveSelection = (direction: 1 | -1): boolean => {
    if (disabled || enabledOptions.length === 0) {
      return false;
    }
    const currentIndex = Math.max(
      0,
      enabledOptions.findIndex((option) => option.value === value),
    );
    const nextIndex = (currentIndex + direction + enabledOptions.length) % enabledOptions.length;
    onChange(enabledOptions[nextIndex].value);
    return true;
  };

  const menu = isOpen ? (
    <SelectMenuPanel
      ariaLabel={ariaLabel}
      id={menuId}
      layoutClassName="overflow-y-auto p-1"
      menuClassName={menuClassName}
      panelRef={menuRef}
      placement={menuPosition?.placement}
      style={menuStyle}
      surface={surface}
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
              getSelectMenuOptionStateClassName(surface, isActive),
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
    </SelectMenuPanel>
  ) : null;

  return (
    <div
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
        className={getSelectMenuButtonClassName({
          roundedClassName,
          surface,
          textClassName,
          className: buttonClassName,
        })}
        disabled={disabled}
        id={id}
        onClick={toggleMenu}
        onKeyDown={(event) => handleTriggerKeyDown(event, moveSelection)}
        type="button"
      >
        <SelectMenuTriggerContent
          isOpen={isOpen}
          label={label}
          leading={leading}
        >
          <span
            className={cn(
              "min-w-0 font-semibold text-(--text-strong)",
              allowLabelWrap ? "whitespace-normal break-words text-left leading-snug" : "truncate",
            )}
          >
            {activeOption?.label ?? placeholder}
          </span>
        </SelectMenuTriggerContent>
      </button>

      {menu && portalContainer ? createPortal(menu, portalContainer) : null}
    </div>
  );
}
