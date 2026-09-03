// INPUT: Select Menu controller 投影的触发器、面板、选项与事件处理器。
// OUTPUT: button + portal listbox 的纯视图结构和选中/disabled ARIA 状态。
// POS: Select Menu 视图；不持有开关、选值、定位或业务状态。

import type {
  CSSProperties,
  KeyboardEventHandler,
  ReactNode,
  RefObject,
} from "react";
import { createPortal } from "react-dom";
import { Check } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import {
  MENU_ITEM_BASE_CLASS_NAME,
  MENU_LIST_CLASS_NAME,
} from "@/shared/ui/menu/menu-styles";

import type { UiAnchoredOverlayPosition } from "../overlay/anchored-overlay-model";
import {
  getSelectMenuButtonClassName,
  getSelectMenuOptionStateClassName,
  type SelectMenuPresentation,
  type UiSelectMenuOption,
  type UiSelectMenuSurface,
} from "./select-menu-model";
import {
  SelectMenuPanel,
  SelectMenuTriggerContent,
} from "./select-menu-primitives";

interface SelectMenuViewProps {
  ariaLabel: string;
  buttonClassName?: string;
  buttonRef: RefObject<HTMLButtonElement | null>;
  className?: string;
  disabled: boolean;
  id?: string;
  isOpen: boolean;
  label?: ReactNode;
  leading?: ReactNode;
  menuId: string;
  menuPlacement?: UiAnchoredOverlayPosition["placement"];
  menuRef: RefObject<HTMLDivElement | null>;
  menuStyle: CSSProperties;
  onSelect: (value: string) => void;
  onTriggerClick: () => void;
  onTriggerKeyDown: KeyboardEventHandler<HTMLButtonElement>;
  options: UiSelectMenuOption[];
  portalContainer: Element | null;
  presentation: SelectMenuPresentation;
  surface: UiSelectMenuSurface;
  value: string;
}

export function SelectMenuView({
  ariaLabel,
  buttonClassName,
  buttonRef,
  className,
  disabled,
  id,
  isOpen,
  label,
  leading,
  menuId,
  menuPlacement,
  menuRef,
  menuStyle,
  onSelect,
  onTriggerClick,
  onTriggerKeyDown,
  options,
  portalContainer,
  presentation,
  surface,
  value,
}: SelectMenuViewProps) {
  return (
    <div
      className={cn("relative w-full", presentation.heightClassName, className)}
    >
      <button
        ref={buttonRef}
        aria-controls={isOpen ? menuId : undefined}
        aria-disabled={disabled}
        aria-expanded={isOpen}
        aria-haspopup="listbox"
        aria-label={ariaLabel}
        className={getSelectMenuButtonClassName({
          roundedClassName: presentation.roundedClassName,
          surface,
          textClassName: presentation.textClassName,
          className: buttonClassName,
        })}
        disabled={disabled}
        id={id}
        onClick={onTriggerClick}
        onKeyDown={onTriggerKeyDown}
        type="button"
      >
        <SelectMenuTriggerContent isOpen={isOpen} label={label} leading={leading}>
          <span className="flex min-w-0 flex-1 items-center gap-2">
            <span
              className={cn(
                "min-w-0 flex-1 font-semibold text-(--text-strong)",
                presentation.triggerLabelClassName,
              )}
              title={presentation.activeLabel}
            >
              {presentation.activeLabel}
            </span>
            <SelectMenuOptionBadge label={presentation.activeBadge} />
          </span>
        </SelectMenuTriggerContent>
      </button>

      <SelectMenuPortal
        ariaLabel={ariaLabel}
        isOpen={isOpen}
        menuId={menuId}
        menuPlacement={menuPlacement}
        menuRef={menuRef}
        menuStyle={menuStyle}
        onSelect={onSelect}
        options={options}
        portalContainer={portalContainer}
        presentation={presentation}
        surface={surface}
        value={value}
      />
    </div>
  );
}

function SelectMenuPortal({
  ariaLabel,
  isOpen,
  menuId,
  menuPlacement,
  menuRef,
  menuStyle,
  onSelect,
  options,
  portalContainer,
  presentation,
  surface,
  value,
}: Pick<
  SelectMenuViewProps,
  | "ariaLabel"
  | "isOpen"
  | "menuId"
  | "menuPlacement"
  | "menuRef"
  | "menuStyle"
  | "onSelect"
  | "options"
  | "portalContainer"
  | "presentation"
  | "surface"
  | "value"
>) {
  if (!isOpen || !portalContainer) {
    return null;
  }

  return createPortal(
    <SelectMenuPanel
      ariaLabel={ariaLabel}
      id={menuId}
      layoutClassName={cn(MENU_LIST_CLASS_NAME, "overflow-y-auto p-1")}
      panelRef={menuRef}
      placement={menuPlacement}
      style={menuStyle}
      surface={surface}
    >
      <SelectMenuOptions
        onSelect={onSelect}
        options={options}
        presentation={presentation}
        surface={surface}
        value={value}
      />
    </SelectMenuPanel>,
    portalContainer,
  );
}

function SelectMenuOptions({
  onSelect,
  options,
  presentation,
  surface,
  value,
}: Pick<
  SelectMenuViewProps,
  "onSelect" | "options" | "presentation" | "surface" | "value"
>) {
  return options.map((option) => (
    <SelectMenuOption
      key={option.value}
      isActive={option.value === value}
      onSelect={onSelect}
      option={option}
      presentation={presentation}
      surface={surface}
    />
  ));
}

function SelectMenuOption({
  isActive,
  onSelect,
  option,
  presentation,
  surface,
}: {
  isActive: boolean;
  onSelect: (value: string) => void;
  option: UiSelectMenuOption;
  presentation: SelectMenuPresentation;
  surface: UiSelectMenuSurface;
}) {
  return (
    <button
      aria-selected={isActive}
      className={cn(
        MENU_ITEM_BASE_CLASS_NAME,
        "flex justify-between gap-2 px-2.5 disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
        presentation.optionButtonLayoutClassName,
        presentation.optionHeightClassName,
        getSelectMenuOptionStateClassName(surface, isActive),
      )}
      data-active={isActive ? "true" : undefined}
      disabled={option.disabled}
      onClick={() => onSelect(option.value)}
      role="option"
      type="button"
    >
      <span className={cn("min-w-0 flex-1", presentation.optionLabelClassName)}>
        {option.label}
      </span>
      <SelectMenuOptionBadge label={option.badge ?? null} />
      {isActive ? <Check className="mt-0.5 h-3.5 w-3.5 shrink-0 text-(--primary)" /> : null}
    </button>
  );
}

function SelectMenuOptionBadge({ label }: { label: string | null }) {
  if (!label) {
    return null;
  }
  return (
    <span className="inline-flex shrink-0 items-center rounded-[6px] border border-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] px-1.5 py-0.5 text-[9px] font-medium leading-none text-(--primary)">
      {label}
    </span>
  );
}
