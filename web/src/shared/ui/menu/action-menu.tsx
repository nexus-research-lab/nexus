"use client";

import {
  type ReactNode,
  type RefObject,
  useCallback,
} from "react";
import { createPortal } from "react-dom";

import { cn } from "@/shared/ui/class-name";

import { useAnchoredOverlayLayer } from "../overlay/anchored-overlay-layer";
import {
  resolveAnchoredOverlayPosition,
  type UiAnchoredOverlayPlacement,
} from "../overlay/anchored-overlay-model";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "../overlay/overlay-contract";

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

function getItemStateClassName(item: UiActionMenuItem) {
  if (item.tone === "danger") {
    return "text-(--destructive) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)]";
  }
  if (item.active && item.tone === "primary") {
    return "bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)] font-semibold text-(--primary) hover:bg-[color:color-mix(in_srgb,var(--primary)_14%,transparent)]";
  }
  if (item.active) {
    return "bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)] font-semibold text-(--text-strong) hover:bg-[color:color-mix(in_srgb,var(--primary)_14%,transparent)]";
  }
  if (item.tone === "primary") {
    return "text-(--primary) hover:bg-[color:color-mix(in_srgb,var(--primary)_9%,transparent)]";
  }
  return "text-(--text-default) hover:bg-(--surface-interactive-hover-background)";
}

function getItemBodyClassName(item: UiActionMenuItem) {
  return cn(
    "flex w-full cursor-pointer items-center justify-between gap-3 rounded-[10px] px-2.5 text-left transition-[background-color,color] duration-(--motion-duration-fast) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_18%,transparent)]",
    item.description ? "min-h-11 py-2" : "min-h-9 py-1.5",
    item.disabled && "cursor-not-allowed opacity-(--disabled-opacity)",
    getItemStateClassName(item),
  );
}

function getItemLabelClassName(tone: UiActionMenuItem["tone"], active?: boolean) {
  if (tone === "primary") {
    return "text-(--primary)";
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
        "fixed z-[130] overflow-y-auto rounded-[14px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_96%,white)] p-1 shadow-[0_14px_32px_rgba(15,23,42,0.12)] backdrop-blur animate-in fade-in-0 zoom-in-95 duration-(--motion-duration-fast) data-[placement=bottom]:slide-in-from-top-1 data-[placement=top]:slide-in-from-bottom-1",
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
              <span className={cn("block truncate text-[13px] font-medium", getItemLabelClassName(item.tone, item.active))}>
                {item.label}
              </span>
              {item.description ? (
                <span className="block truncate text-[10px] font-normal text-(--text-soft)">
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
