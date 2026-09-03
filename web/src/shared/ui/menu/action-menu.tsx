// INPUT: 外部控制的打开态、锚点、菜单项与选择/关闭命令。
// OUTPUT: 定位后的可键盘遍历 action menu，并在选择/Escape 后归还焦点。
// POS: Action Menu 交互 pattern；不持有业务值或决定命令是否允许。
"use client";

import {
  type KeyboardEvent,
  type ReactNode,
  type RefObject,
  useCallback,
  useEffect,
} from "react";
import { createPortal } from "react-dom";

import { cn } from "@/shared/ui/class-name";

import {
  getMenuItemStateClassName,
  MENU_ITEM_BASE_CLASS_NAME,
  MENU_ITEM_GAP_PX,
  MENU_LIST_CLASS_NAME,
  MENU_SURFACE_VERTICAL_PADDING_PX,
} from "./menu-styles";
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
  disabled?: boolean;
  tone?: "default" | "primary" | "danger";
}

export type UiActionMenuDensity = "compact" | "default";

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
const ACTION_MENU_ITEM_HEIGHT = {
  compact: 32,
  default: 36,
} as const;
const ACTION_MENU_DESCRIBED_ITEM_HEIGHT = {
  compact: 40,
  default: 44,
} as const;
const ACTION_MENU_ITEM_CLASS_NAME = {
  compact: {
    described: "h-10 py-0.5",
    plain: "h-8",
    spacing: "gap-2 px-2",
  },
  default: {
    described: "h-11 py-1",
    plain: "h-9",
    spacing: "gap-3 px-2.5",
  },
} as const;
const ACTION_MENU_FOOTER_SEPARATOR_HEIGHT = 9;
const EMPTY_ACTION_MENU_ITEMS: UiActionMenuItem[] = [];
const ENABLED_ACTION_MENU_ITEM_SELECTOR = '[role="menuitem"]:not([aria-disabled="true"])';

function handleActionMenuKeyDown({
  anchorRef,
  event,
  onClose,
}: {
  anchorRef: RefObject<HTMLElement | null>;
  event: KeyboardEvent<HTMLDivElement>;
  onClose: () => void;
}) {
  if (event.key === "Escape") {
    event.preventDefault();
    event.stopPropagation();
    onClose();
    anchorRef.current?.focus();
    return;
  }
  if (!["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
    return;
  }
  const items = Array.from(
    event.currentTarget.querySelectorAll<HTMLElement>(
      ENABLED_ACTION_MENU_ITEM_SELECTOR,
    ),
  );
  if (items.length === 0) {
    return;
  }
  event.preventDefault();
  const currentIndex = items.indexOf(document.activeElement as HTMLElement);
  if (event.key === "Home") {
    items[0].focus();
    return;
  }
  if (event.key === "End") {
    items[items.length - 1].focus();
    return;
  }
  const direction = event.key === "ArrowDown" ? 1 : -1;
  const fallbackIndex = direction > 0 ? -1 : 0;
  const nextIndex = (
    (currentIndex >= 0 ? currentIndex : fallbackIndex)
    + direction
    + items.length
  ) % items.length;
  items[nextIndex].focus();
}

function estimateActionMenuHeight({
  density = "default",
  footerItems = EMPTY_ACTION_MENU_ITEMS,
  items,
}: {
  density?: UiActionMenuDensity;
  footerItems?: UiActionMenuItem[];
  items: UiActionMenuItem[];
}): number {
  const allItems = [...items, ...footerItems];
  const hasFooter = footerItems.length > 0;
  const contentBlockCount = allItems.length + (hasFooter ? 1 : 0);
  return allItems.reduce(
    (height, item) => height + (
      item.description
        ? ACTION_MENU_DESCRIBED_ITEM_HEIGHT[density]
        : ACTION_MENU_ITEM_HEIGHT[density]
    ),
    MENU_SURFACE_VERTICAL_PADDING_PX
      + (hasFooter ? ACTION_MENU_FOOTER_SEPARATOR_HEIGHT : 0),
  ) + MENU_ITEM_GAP_PX * Math.max(0, contentBlockCount - 1);
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
    Math.max(ACTION_MENU_ITEM_HEIGHT[density], contentHeight),
  );
  return resolveAnchoredOverlayPosition({
    align,
    anchor,
    estimatedHeight,
    maxHeight: ACTION_MENU_MAX_HEIGHT,
    minHeight: ACTION_MENU_ITEM_HEIGHT[density],
    minWidth,
    placement,
  });
}

function getItemBodyClassName(
  item: UiActionMenuItem,
  density: UiActionMenuDensity,
) {
  const densityClassName = ACTION_MENU_ITEM_CLASS_NAME[density];
  return cn(
    MENU_ITEM_BASE_CLASS_NAME,
    "flex cursor-pointer items-center justify-between",
    densityClassName.spacing,
    item.description
      ? densityClassName.described
      : densityClassName.plain,
    item.disabled && "cursor-not-allowed opacity-(--disabled-opacity)",
    getMenuItemStateClassName({
      active: item.active,
      tone: item.tone,
    }),
  );
}

function getItemLabelClassName(tone: UiActionMenuItem["tone"], active?: boolean) {
  if (tone === "primary") {
    return "text-(--brand-action)";
  }
  if (tone === "danger") {
    return "text-(--destructive)";
  }
  return active ? "text-(--text-strong)" : "text-(--text-default)";
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

  useEffect(() => {
    if (!isOpen || !portalContainer) {
      return;
    }
    menuRef.current
      ?.querySelector<HTMLElement>(ENABLED_ACTION_MENU_ITEM_SELECTOR)
      ?.focus();
  }, [isOpen, menuRef, portalContainer]);

  if (!isOpen) {
    return null;
  }
  if (!portalContainer) {
    return null;
  }
  const select = (value: string) => {
    onSelect(value);
    onClose();
    anchorRef.current?.focus();
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
      onKeyDown={(event) => handleActionMenuKeyDown({
        anchorRef,
        event,
        onClose,
      })}
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
          <div className="mx-1 my-1 border-t border-(--divider-subtle-color)" />
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
    <div
      aria-disabled={disabled || item.disabled || undefined}
      className={cn(
        getItemBodyClassName(item, density),
        disabled && "cursor-not-allowed opacity-(--disabled-opacity)",
      )}
      onClick={select}
      onKeyDown={(event) => {
        if (
          disabled
          || item.disabled
          || (event.key !== "Enter" && event.key !== " ")
        ) {
          return;
        }
        event.preventDefault();
        select();
      }}
      role="menuitem"
      tabIndex={disabled || item.disabled ? -1 : 0}
    >
      <span className="flex min-w-0 flex-1 items-center gap-2">
        {item.icon ? (
          <span className="flex h-4 w-4 shrink-0 items-center justify-center">
            {item.icon}
          </span>
        ) : null}
        <span className="min-w-0 flex-1">
          <span className={cn(
            "block truncate font-normal",
            density === "compact" ? "text-compact" : "text-sm",
            getItemLabelClassName(item.tone, item.active),
          )}>
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
    </div>
  );
}
