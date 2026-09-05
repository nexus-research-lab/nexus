// INPUT: Select Menu controller 投影的触发器、面板、选项与事件处理器。
// OUTPUT: 共用 SelectMenuTrigger + portal listbox 的纯视图结构和选项状态。
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
  MENU_LIST_CLASS_NAME,
} from "@/shared/ui/menu/menu-styles";

import type { UiAnchoredOverlayPosition } from "../overlay/anchored-overlay-model";
import type {
  SelectMenuStyleProjection,
} from "./select-menu-styles";
import {
  type SelectMenuModel,
  type UiSelectMenuOption,
  type UiSelectMenuSurface,
} from "./select-menu-model";
import {
  getSelectMenuOptionStateClassName,
} from "./select-menu-styles";
import {
  SelectMenuOptionRow,
  SelectMenuPanel,
  SelectMenuTrigger,
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
  model: SelectMenuModel;
  surface: UiSelectMenuSurface;
  styles: SelectMenuStyleProjection;
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
  model,
  surface,
  styles,
  value,
}: SelectMenuViewProps) {
  return (
    <div
      className={cn("relative w-full", styles.heightClassName, className)}
    >
      <SelectMenuTrigger
        ariaLabel={ariaLabel}
        buttonRef={buttonRef}
        className={buttonClassName}
        disabled={disabled}
        id={id}
        isOpen={isOpen}
        menuId={menuId}
        onClick={onTriggerClick}
        onKeyDown={onTriggerKeyDown}
        styles={styles}
        surface={surface}
      >
        <SelectMenuTriggerContent isOpen={isOpen} label={label} leading={leading}>
          <span className="flex min-w-0 flex-1 items-center gap-2">
            <span
              className={cn(
                "min-w-0 flex-1 font-semibold text-(--text-strong)",
                styles.triggerLabelClassName,
              )}
              title={model.activeLabel}
            >
              {model.activeLabel}
            </span>
            <SelectMenuOptionBadge label={model.activeBadge} />
          </span>
        </SelectMenuTriggerContent>
      </SelectMenuTrigger>

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
        styles={styles}
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
  styles,
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
  | "styles"
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
        styles={styles}
        value={value}
      />
    </SelectMenuPanel>,
    portalContainer,
  );
}

function SelectMenuOptions({
  onSelect,
  options,
  styles,
  value,
}: Pick<
  SelectMenuViewProps,
  "onSelect" | "options" | "styles" | "value"
>) {
  return options.map((option) => (
    <SelectMenuOption
      key={option.value}
      isActive={option.value === value}
      onSelect={onSelect}
      option={option}
      styles={styles}
    />
  ));
}

function SelectMenuOption({
  isActive,
  onSelect,
  option,
  styles,
}: {
  isActive: boolean;
  onSelect: (value: string) => void;
  option: UiSelectMenuOption;
  styles: SelectMenuStyleProjection;
}) {
  return (
    <SelectMenuOptionRow
      active={isActive}
      className={cn(
        "flex justify-between gap-2 px-2.5 disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
        styles.optionButtonLayoutClassName,
        styles.optionHeightClassName,
        getSelectMenuOptionStateClassName(isActive),
      )}
      disabled={option.disabled}
      onClick={() => onSelect(option.value)}
    >
      <span className={cn("min-w-0 flex-1", styles.optionLabelClassName)}>
        {option.label}
      </span>
      <SelectMenuOptionBadge label={option.badge ?? null} />
      {isActive ? <Check className="mt-0.5 h-3.5 w-3.5 shrink-0 text-(--primary)" /> : null}
    </SelectMenuOptionRow>
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
