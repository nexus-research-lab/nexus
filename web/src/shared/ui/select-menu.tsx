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
  estimateSelectMenuHeight,
  getSelectMenuButtonClassName,
  getSelectMenuOptionStateClassName,
  getSelectMenuPanelSurfaceClassName,
  getSelectMenuSizeConfig,
  resolveSelectMenuPosition,
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
    isOpen,
    menuId,
    menuPosition,
    menuRef,
    menuStyle,
    portalContainer,
    rootRef,
    setIsOpen,
    updateMenuPosition,
  } = useSelectMenuLayer({ disabled, estimatePosition });

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
        getSelectMenuPanelSurfaceClassName(surface),
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
        className={getSelectMenuButtonClassName({
          roundedClassName,
          surface,
          textClassName,
          className: buttonClassName,
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
