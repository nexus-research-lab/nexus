"use client";

import {
  type ReactNode,
  type RefObject,
  useCallback,
} from "react";
import { createPortal } from "react-dom";

import { cn } from "@/shared/ui/class-name";

import {
  getMenuItemStateClassName,
  MENU_ITEM_BASE_CLASS_NAME,
} from "./menu-styles";
import { useAnchoredOverlayLayer } from "../overlay/anchored-overlay-layer";
import {
  resolveAnchoredOverlayPosition,
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

type UiActionMenuPlacement = UiAnchoredOverlayPlacement;

interface UiActionMenuProps {
  anchorRef: RefObject<HTMLElement | null>;
  ariaLabel: string;
  className?: string;
  isOpen: boolean;
  items: UiActionMenuItem[];
  minWidth?: number;
  placement?: UiActionMenuPlacement;
  onClose: () => void;
  onSelect: (value: string) => void;
}

const ACTION_MENU_MAX_HEIGHT = 320;
const ACTION_MENU_ITEM_HEIGHT = 44;

function resolveActionMenuPosition({
  anchor,
  itemCount,
  minWidth,
  placement,
}: {
  anchor: HTMLElement;
  itemCount: number;
  minWidth: number;
  placement: UiActionMenuPlacement;
}) {
  const estimatedHeight = Math.min(
    ACTION_MENU_MAX_HEIGHT,
    Math.max(ACTION_MENU_ITEM_HEIGHT, itemCount * ACTION_MENU_ITEM_HEIGHT + 8),
  );
  return resolveAnchoredOverlayPosition({
    anchor,
    estimatedHeight,
    maxHeight: ACTION_MENU_MAX_HEIGHT,
    minHeight: ACTION_MENU_ITEM_HEIGHT,
    minWidth,
    placement,
  });
}

function getItemBodyClassName(item: UiActionMenuItem) {
  return cn(
    MENU_ITEM_BASE_CLASS_NAME,
    "flex cursor-pointer items-center justify-between gap-3 px-2.5",
    item.description ? "min-h-11 py-2" : "min-h-9 py-1.5",
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
  anchorRef: anchorRef,
  ariaLabel: ariaLabel,
  className: className,
  isOpen: isOpen,
  items,
  minWidth: minWidth = 220,
  placement = "auto",
  onClose: onClose,
  onSelect: onSelect,
}: UiActionMenuProps) {
  const estimatePosition = useCallback(
    (anchor: HTMLElement) => resolveActionMenuPosition({
      anchor,
      itemCount: items.length,
      minWidth,
      placement,
    }),
    [items.length, minWidth, placement],
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

  if (!isOpen) {
    return null;
  }
  if (!portalContainer) {
    return null;
  }

  return createPortal(
    <div
      ref={menuRef}
      aria-label={ariaLabel}
      className={cn(
        "fixed z-[130] overflow-y-auto p-1",
        OVERLAY_SURFACE_CLASS_NAME,
        ANCHORED_OVERLAY_MOTION_CLASS_NAME,
        className,
      )}
      data-placement={menuPosition?.placement ?? "bottom"}
      data-state="open"
      role="menu"
      style={menuStyle}
      {...OPEN_OVERLAY_DATA_ATTRIBUTES}
    >
      {items.map((item) => (
        <div
          key={item.value}
          aria-disabled={item.disabled || undefined}
          className={getItemBodyClassName(item)}
          onClick={() => {
            if (item.disabled) {
              return;
            }
            onSelect(item.value);
            onClose();
            anchorRef.current?.focus();
          }}
          onKeyDown={(event) => {
            if (item.disabled || (event.key !== "Enter" && event.key !== " ")) {
              return;
            }
            event.preventDefault();
            onSelect(item.value);
            onClose();
            anchorRef.current?.focus();
          }}
          role="menuitem"
          tabIndex={item.disabled ? -1 : 0}
        >
          <span className="flex min-w-0 flex-1 items-center gap-2">
            {item.icon ? (
              <span className="flex h-4 w-4 shrink-0 items-center justify-center">
                {item.icon}
              </span>
            ) : null}
            <span className="min-w-0 flex-1">
              <span className={cn("block truncate text-sm font-medium", getItemLabelClassName(item.tone, item.active))}>
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
      ))}
    </div>,
    portalContainer,
  );
}
