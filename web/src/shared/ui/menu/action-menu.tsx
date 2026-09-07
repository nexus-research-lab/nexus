// INPUT: 外部控制的打开态、锚点、菜单项与选择/关闭命令。
// OUTPUT: 统一动作/勾选项与唯一激活入口；可见后聚焦、重定位保留焦点，选择/Escape 归还触发器，Tab 退出。
// POS: Action Menu 交互 pattern；不持有业务值或决定命令是否允许。
"use client";

import {
  type ReactNode,
  type RefObject,
  useCallback,
  useEffect,
} from "react";
import { createPortal } from "react-dom";
import { Check } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import { focusFirstMenuItem, handleMenuKeyDown } from "./menu-keyboard";

import {
  getMenuItemLayout,
  getMenuContentHeight,
  MENU_LIST_CLASS_NAME,
  MENU_SEPARATOR_CLASS_NAME,
} from "./menu-styles";
import {
  UiMenuActionRow,
  type UiMenuActionRowDensity,
} from "./menu-action-row";
import { useAnchoredOverlayLayer } from "../overlay/anchored-overlay-layer";
import {
  resolveAnchoredOverlayPosition,
  type UiAnchoredOverlayAlignment,
  type UiAnchoredOverlayPlacement,
} from "../overlay/anchored-overlay-model";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "../overlay/overlay-contract";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "../overlay/overlay-styles";

export interface UiActionMenuItem {
  value: string;
  label: ReactNode;
  description?: ReactNode;
  icon?: ReactNode;
  trailing?: ReactNode;
  active?: boolean;
  /** Controlled checked state; labels, icons and trailing content must be non-interactive. */
  checked?: boolean;
  disabled?: boolean;
  tone?: "default" | "primary" | "danger";
}

export type UiActionMenuDensity = UiMenuActionRowDensity;

export interface UiActionMenuContentProps {
  density?: UiActionMenuDensity;
  disabled?: boolean;
  footerItems?: UiActionMenuItem[];
  items: UiActionMenuItem[];
  onSelect: (value: string) => void;
}

type UiActionMenuPlacement = UiAnchoredOverlayPlacement;

interface UiActionMenuProps {
  align?: UiAnchoredOverlayAlignment;
  anchorRef: RefObject<HTMLElement | null>;
  ariaLabel: string;
  density?: UiActionMenuDensity;
  footerItems?: UiActionMenuItem[];
  isOpen: boolean;
  items: UiActionMenuItem[];
  minWidth?: number;
  placement?: UiActionMenuPlacement;
  onClose: () => void;
  onSelect: (value: string) => void;
}

const ACTION_MENU_MAX_HEIGHT = 320;
const EMPTY_ACTION_MENU_ITEMS: UiActionMenuItem[] = [];

function estimateActionMenuHeight({
  density = "default",
  footerItems = EMPTY_ACTION_MENU_ITEMS,
  items,
}: {
  density?: UiActionMenuDensity;
  footerItems?: UiActionMenuItem[];
  items: UiActionMenuItem[];
}): number {
  return getMenuContentHeight(
    [...items, ...footerItems].map((item) => getMenuItemLayout({ density, hasDescription: Boolean(item.description) }).height),
    footerItems.length > 0 ? 1 : 0,
  );
}

function resolveActionMenuPosition({
  align,
  anchor,
  density,
  items,
  footerItems,
  minWidth,
  placement,
}: {
  align: UiAnchoredOverlayAlignment;
  anchor: HTMLElement;
  density: UiActionMenuDensity;
  items: UiActionMenuItem[];
  footerItems: UiActionMenuItem[];
  minWidth: number;
  placement: UiActionMenuPlacement;
}) {
  const contentHeight = estimateActionMenuHeight({
    density,
    footerItems,
    items,
  });
  const estimatedHeight = Math.min(
    ACTION_MENU_MAX_HEIGHT,
    Math.max(getMenuItemLayout({ density }).height, contentHeight),
  );
  return resolveAnchoredOverlayPosition({
    align,
    anchor,
    estimatedHeight,
    maxHeight: ACTION_MENU_MAX_HEIGHT,
    minHeight: getMenuItemLayout({ density }).height,
    minWidth,
    placement,
  });
}

export function UiActionMenu({
  align = "start",
  anchorRef: anchorRef,
  ariaLabel: ariaLabel,
  density = "default",
  footerItems = EMPTY_ACTION_MENU_ITEMS,
  isOpen: isOpen,
  items,
  minWidth: minWidth = 220,
  placement = "auto",
  onClose: onClose,
  onSelect: onSelect,
}: UiActionMenuProps) {
  const estimatePosition = useCallback(
    (anchor: HTMLElement) => resolveActionMenuPosition({
      align,
      anchor,
      density,
      footerItems,
      items,
      minWidth,
      placement,
    }),
    [align, density, footerItems, items, minWidth, placement],
  );
  const {
    overlayPosition: menuPosition,
    overlayRef: menuRef,
    overlayStyle: menuStyle,
    portalContainer,
  } = useAnchoredOverlayLayer({
    anchorRef,
    disabled: false,
    estimatePosition,
    isOpen,
    onClose,
  });
  const isMenuPositioned = menuPosition !== null;

  useEffect(() => {
    if (!isOpen || !portalContainer || !isMenuPositioned) {
      return;
    }
    focusFirstMenuItem(menuRef.current);
  }, [isMenuPositioned, isOpen, menuRef, portalContainer]);

  if (!isOpen) {
    return null;
  }
  if (!portalContainer) {
    return null;
  }
  const closeAndRestoreFocus = () => {
    onClose();
    anchorRef.current?.focus();
  };
  const select = (value: string) => {
    onSelect(value);
    closeAndRestoreFocus();
  };

  return createPortal(
    <div
      ref={menuRef}
      aria-label={ariaLabel}
      className={cn(
        "fixed ui-layer-action-menu overflow-y-auto p-1",
        OVERLAY_SURFACE_CLASS_NAME,
        ANCHORED_OVERLAY_MOTION_CLASS_NAME,
      )}
      data-placement={menuPosition?.placement ?? "bottom"}
      data-state="open"
      onKeyDown={(event) => handleMenuKeyDown(event, closeAndRestoreFocus)}
      role="menu"
      style={menuStyle}
      tabIndex={-1}
      {...OPEN_OVERLAY_DATA_ATTRIBUTES}
    >
      <UiActionMenuContent
        density={density}
        footerItems={footerItems}
        items={items}
        onSelect={select}
      />
    </div>,
    portalContainer,
  );
}

export function UiActionMenuContent({
  density = "default",
  disabled = false,
  footerItems = EMPTY_ACTION_MENU_ITEMS,
  items,
  onSelect,
}: UiActionMenuContentProps) {
  return (
    <div className={MENU_LIST_CLASS_NAME} role="none">
      {items.map((item) => (
        <ActionMenuItem
          density={density}
          disabled={disabled}
          item={item}
          key={item.value}
          onSelect={onSelect}
        />
      ))}
      {footerItems.length > 0 ? (
        <>
          <div className={MENU_SEPARATOR_CLASS_NAME} role="separator" />
          {footerItems.map((item) => (
            <ActionMenuItem
              density={density}
              disabled={disabled}
              item={item}
              key={item.value}
              onSelect={onSelect}
            />
          ))}
        </>
      ) : null}
    </div>
  );
}

function ActionMenuItem({
  density,
  disabled,
  item,
  onSelect,
}: {
  density: UiActionMenuDensity;
  disabled: boolean;
  item: UiActionMenuItem;
  onSelect: (value: string) => void;
}) {
  const select = () => {
    if (disabled || item.disabled) {
      return;
    }
    onSelect(item.value);
  };
  return (
    <UiMenuActionRow
      active={item.active}
      checked={item.checked}
      density={density}
      disabled={disabled || item.disabled}
      hasDescription={Boolean(item.description)}
      onClick={select}
      tone={item.tone}
    >
      <span className="flex min-w-0 flex-1 items-center gap-2">
        {item.icon ? (
          <span className="flex h-4 w-4 shrink-0 items-center justify-center">
            {item.icon}
          </span>
        ) : null}
        <span className="min-w-0 flex-1">
          <span className="block truncate font-normal">
            {item.label}
          </span>
          {item.description ? (
            <span className="block truncate text-2xs font-normal text-(--text-soft)">
              {item.description}
            </span>
          ) : null}
        </span>
      </span>
      {item.trailing ? (
        <span className="flex shrink-0 items-center">
          {item.trailing}
        </span>
      ) : null}
      {item.checked !== undefined ? (
        <span aria-hidden="true" className="flex h-3.5 w-3.5 shrink-0 items-center justify-center">
          {item.checked ? <Check className="h-3.5 w-3.5" /> : null}
        </span>
      ) : null}
    </UiMenuActionRow>
  );
}
