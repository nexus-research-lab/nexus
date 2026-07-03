"use client";

import {
  type CSSProperties,
  type ReactNode,
  type RefObject,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

import { cn } from "@/lib/utils";

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

type UiActionMenuPlacement = "auto" | "bottom" | "top";

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

interface UiActionMenuPosition {
  bottom?: number;
  left: number;
  maxHeight: number;
  placement: "bottom" | "top";
  top?: number;
  width: number;
}

const ACTION_MENU_GAP = 6;
const ACTION_MENU_VIEWPORT_MARGIN = 12;
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
}): UiActionMenuPosition {
  const rect = anchor.getBoundingClientRect();
  const viewportWidth = window.innerWidth;
  const viewportHeight = window.innerHeight;
  const estimatedHeight = Math.min(
    ACTION_MENU_MAX_HEIGHT,
    Math.max(ACTION_MENU_ITEM_HEIGHT, itemCount * ACTION_MENU_ITEM_HEIGHT + 8),
  );
  const availableAbove = Math.max(0, rect.top - ACTION_MENU_VIEWPORT_MARGIN);
  const availableBelow = Math.max(0, viewportHeight - rect.bottom - ACTION_MENU_VIEWPORT_MARGIN);
  const shouldPlaceTop =
    placement === "top" ||
    (placement === "auto" && availableBelow < estimatedHeight && availableAbove > availableBelow);
  const availableSpace = shouldPlaceTop ? availableAbove : availableBelow;
  const width = Math.min(
    Math.max(rect.width, minWidth),
    viewportWidth - ACTION_MENU_VIEWPORT_MARGIN * 2,
  );
  const left = Math.min(
    Math.max(ACTION_MENU_VIEWPORT_MARGIN, rect.left),
    Math.max(ACTION_MENU_VIEWPORT_MARGIN, viewportWidth - width - ACTION_MENU_VIEWPORT_MARGIN),
  );
  const maxHeight = Math.min(
    ACTION_MENU_MAX_HEIGHT,
    estimatedHeight,
    Math.max(ACTION_MENU_ITEM_HEIGHT, availableSpace - ACTION_MENU_GAP),
  );

  return {
    left,
    maxHeight,
    placement: shouldPlaceTop ? "top" : "bottom",
    width,
    ...(shouldPlaceTop
      ? { bottom: Math.max(ACTION_MENU_VIEWPORT_MARGIN, viewportHeight - rect.top + ACTION_MENU_GAP) }
      : { top: Math.min(rect.bottom + ACTION_MENU_GAP, viewportHeight - ACTION_MENU_VIEWPORT_MARGIN - maxHeight) }),
  };
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
  const menuRef = useRef<HTMLDivElement>(null);
  const [menuPosition, setMenuPosition] = useState<UiActionMenuPosition | null>(null);

  const updateMenuPosition = useCallback(() => {
    const anchor = anchorRef.current;
    if (!anchor) {
      return;
    }
    setMenuPosition(resolveActionMenuPosition({
      anchor,
      itemCount: items.length,
      minWidth,
      placement,
    }));
  }, [anchorRef, items.length, minWidth, placement]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      if (anchorRef.current?.contains(target) || menuRef.current?.contains(target)) {
        return;
      }
      onClose();
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
        anchorRef.current?.focus();
      }
    };

    document.addEventListener("pointerdown", handlePointerDown, true);
    document.addEventListener("keydown", handleKeyDown);
    window.addEventListener("resize", updateMenuPosition);
    window.addEventListener("scroll", updateMenuPosition, true);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown, true);
      document.removeEventListener("keydown", handleKeyDown);
      window.removeEventListener("resize", updateMenuPosition);
      window.removeEventListener("scroll", updateMenuPosition, true);
    };
  }, [anchorRef, isOpen, onClose, updateMenuPosition]);

  useLayoutEffect(() => {
    if (isOpen) {
      updateMenuPosition();
    }
  }, [isOpen, updateMenuPosition]);

  if (!isOpen) {
    return null;
  }

  const menuStyle: CSSProperties = {
    bottom: menuPosition?.bottom,
    left: menuPosition?.left,
    maxHeight: menuPosition?.maxHeight,
    top: menuPosition?.top,
    visibility: menuPosition ? "visible" : "hidden",
    width: menuPosition?.width,
  };
  const portalContainer = typeof document === "undefined"
    ? null
    : anchorRef.current?.closest("[data-modal-root='true']") ?? document.body;
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
