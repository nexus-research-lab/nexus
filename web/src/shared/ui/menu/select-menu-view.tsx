import type {
  CSSProperties,
  KeyboardEventHandler,
  ReactNode,
  RefObject,
} from "react";
import { createPortal } from "react-dom";
import { Check } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

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
  menuClassName?: string;
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
  menuClassName,
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
          <span
            className={cn(
              "min-w-0 font-semibold text-(--text-strong)",
              presentation.triggerLabelClassName,
            )}
          >
            {presentation.activeLabel}
          </span>
        </SelectMenuTriggerContent>
      </button>

      <SelectMenuPortal
        ariaLabel={ariaLabel}
        isOpen={isOpen}
        menuClassName={menuClassName}
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
  menuClassName,
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
  | "menuClassName"
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
      layoutClassName="overflow-y-auto p-1"
      menuClassName={menuClassName}
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
        "flex w-full justify-between gap-2 rounded-[10px] px-2.5 text-left transition-[background-color,color] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
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
      {isActive ? <Check className="mt-0.5 h-3.5 w-3.5 shrink-0 text-(--primary)" /> : null}
    </button>
  );
}
