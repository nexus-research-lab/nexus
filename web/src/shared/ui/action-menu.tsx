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
  anchor_ref: RefObject<HTMLElement | null>;
  aria_label: string;
  class_name?: string;
  is_open: boolean;
  items: UiActionMenuItem[];
  min_width?: number;
  placement?: UiActionMenuPlacement;
  on_close: () => void;
  on_select: (value: string) => void;
}

interface UiActionMenuPosition {
  bottom?: number;
  left: number;
  max_height: number;
  placement: "bottom" | "top";
  top?: number;
  width: number;
}

const ACTION_MENU_GAP = 6;
const ACTION_MENU_VIEWPORT_MARGIN = 12;
const ACTION_MENU_MAX_HEIGHT = 320;
const ACTION_MENU_ITEM_HEIGHT = 44;

function resolve_action_menu_position({
  anchor,
  item_count,
  min_width,
  placement,
}: {
  anchor: HTMLElement;
  item_count: number;
  min_width: number;
  placement: UiActionMenuPlacement;
}): UiActionMenuPosition {
  const rect = anchor.getBoundingClientRect();
  const viewport_width = window.innerWidth;
  const viewport_height = window.innerHeight;
  const estimated_height = Math.min(
    ACTION_MENU_MAX_HEIGHT,
    Math.max(ACTION_MENU_ITEM_HEIGHT, item_count * ACTION_MENU_ITEM_HEIGHT + 8),
  );
  const available_above = Math.max(0, rect.top - ACTION_MENU_VIEWPORT_MARGIN);
  const available_below = Math.max(0, viewport_height - rect.bottom - ACTION_MENU_VIEWPORT_MARGIN);
  const should_place_top =
    placement === "top" ||
    (placement === "auto" && available_below < estimated_height && available_above > available_below);
  const available_space = should_place_top ? available_above : available_below;
  const width = Math.min(
    Math.max(rect.width, min_width),
    viewport_width - ACTION_MENU_VIEWPORT_MARGIN * 2,
  );
  const left = Math.min(
    Math.max(ACTION_MENU_VIEWPORT_MARGIN, rect.left),
    Math.max(ACTION_MENU_VIEWPORT_MARGIN, viewport_width - width - ACTION_MENU_VIEWPORT_MARGIN),
  );
  const max_height = Math.min(
    ACTION_MENU_MAX_HEIGHT,
    estimated_height,
    Math.max(ACTION_MENU_ITEM_HEIGHT, available_space - ACTION_MENU_GAP),
  );

  return {
    left,
    max_height,
    placement: should_place_top ? "top" : "bottom",
    width,
    ...(should_place_top
      ? { bottom: Math.max(ACTION_MENU_VIEWPORT_MARGIN, viewport_height - rect.top + ACTION_MENU_GAP) }
      : { top: Math.min(rect.bottom + ACTION_MENU_GAP, viewport_height - ACTION_MENU_VIEWPORT_MARGIN - max_height) }),
  };
}

function get_item_state_class_name(item: UiActionMenuItem) {
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

function get_item_body_class_name(item: UiActionMenuItem) {
  return cn(
    "flex w-full cursor-pointer items-center justify-between gap-3 rounded-[10px] px-2.5 text-left transition-[background-color,color] duration-(--motion-duration-fast) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_18%,transparent)]",
    item.description ? "min-h-11 py-2" : "min-h-9 py-1.5",
    item.disabled && "cursor-not-allowed opacity-(--disabled-opacity)",
    get_item_state_class_name(item),
  );
}

function get_item_label_class_name(tone: UiActionMenuItem["tone"], active?: boolean) {
  if (tone === "primary") {
    return "text-(--primary)";
  }
  if (tone === "danger") {
    return "text-(--destructive)";
  }
  return active ? "text-(--text-strong)" : "text-(--text-default)";
}

export function UiActionMenu({
  anchor_ref,
  aria_label,
  class_name,
  is_open,
  items,
  min_width = 220,
  placement = "auto",
  on_close,
  on_select,
}: UiActionMenuProps) {
  const menu_ref = useRef<HTMLDivElement>(null);
  const [menu_position, set_menu_position] = useState<UiActionMenuPosition | null>(null);

  const update_menu_position = useCallback(() => {
    const anchor = anchor_ref.current;
    if (!anchor) {
      return;
    }
    set_menu_position(resolve_action_menu_position({
      anchor,
      item_count: items.length,
      min_width,
      placement,
    }));
  }, [anchor_ref, items.length, min_width, placement]);

  useEffect(() => {
    if (!is_open) {
      set_menu_position(null);
      return;
    }

    const handle_pointer_down = (event: PointerEvent) => {
      const target = event.target as Node;
      if (anchor_ref.current?.contains(target) || menu_ref.current?.contains(target)) {
        return;
      }
      on_close();
    };
    const handle_key_down = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        on_close();
        anchor_ref.current?.focus();
      }
    };

    document.addEventListener("pointerdown", handle_pointer_down, true);
    document.addEventListener("keydown", handle_key_down);
    window.addEventListener("resize", update_menu_position);
    window.addEventListener("scroll", update_menu_position, true);
    return () => {
      document.removeEventListener("pointerdown", handle_pointer_down, true);
      document.removeEventListener("keydown", handle_key_down);
      window.removeEventListener("resize", update_menu_position);
      window.removeEventListener("scroll", update_menu_position, true);
    };
  }, [anchor_ref, is_open, on_close, update_menu_position]);

  useLayoutEffect(() => {
    if (is_open) {
      update_menu_position();
    }
  }, [is_open, update_menu_position]);

  if (!is_open) {
    return null;
  }

  const menu_style: CSSProperties = {
    bottom: menu_position?.bottom,
    left: menu_position?.left,
    maxHeight: menu_position?.max_height,
    top: menu_position?.top,
    visibility: menu_position ? "visible" : "hidden",
    width: menu_position?.width,
  };
  const portal_container = typeof document === "undefined"
    ? null
    : anchor_ref.current?.closest("[data-modal-root='true']") ?? document.body;
  if (!portal_container) {
    return null;
  }

  return createPortal(
    <div
      ref={menu_ref}
      aria-label={aria_label}
      className={cn(
        "fixed z-[130] overflow-y-auto rounded-[14px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_96%,white)] p-1 shadow-[0_14px_32px_rgba(15,23,42,0.12)] backdrop-blur animate-in fade-in-0 zoom-in-95 duration-(--motion-duration-fast) data-[placement=bottom]:slide-in-from-top-1 data-[placement=top]:slide-in-from-bottom-1",
        class_name,
      )}
      data-placement={menu_position?.placement ?? "bottom"}
      data-state="open"
      role="menu"
      style={menu_style}
    >
      {items.map((item) => (
        <div
          key={item.value}
          aria-disabled={item.disabled || undefined}
          className={get_item_body_class_name(item)}
          onClick={() => {
            if (item.disabled) {
              return;
            }
            on_select(item.value);
            on_close();
            anchor_ref.current?.focus();
          }}
          onKeyDown={(event) => {
            if (item.disabled || (event.key !== "Enter" && event.key !== " ")) {
              return;
            }
            event.preventDefault();
            on_select(item.value);
            on_close();
            anchor_ref.current?.focus();
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
              <span className={cn("block truncate text-[13px] font-medium", get_item_label_class_name(item.tone, item.active))}>
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
    portal_container,
  );
}
