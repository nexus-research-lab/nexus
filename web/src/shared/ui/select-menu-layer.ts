"use client";

import {
  type CSSProperties,
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import type {
  UiSelectMenuPosition,
} from "./select-menu-model";

interface SelectMenuLayerOptions {
  disabled: boolean;
  estimate_position: (button: HTMLButtonElement) => UiSelectMenuPosition;
}

export function useSelectMenuLayer({ disabled, estimate_position }: SelectMenuLayerOptions) {
  const [is_open, set_is_open] = useState(false);
  const [menu_position, set_menu_position] = useState<UiSelectMenuPosition | null>(null);
  const menu_id = useId();
  const root_ref = useRef<HTMLDivElement>(null);
  const button_ref = useRef<HTMLButtonElement>(null);
  const menu_ref = useRef<HTMLDivElement>(null);

  const update_menu_position = useCallback(() => {
    const button = button_ref.current;
    if (!button) {
      return;
    }
    set_menu_position(estimate_position(button));
  }, [estimate_position]);

  useEffect(() => {
    if (!is_open || disabled) {
      return;
    }

    const handle_pointer_down = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!root_ref.current?.contains(target) && !menu_ref.current?.contains(target)) {
        set_is_open(false);
      }
    };
    const handle_key_down = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        set_is_open(false);
        button_ref.current?.focus();
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
  }, [disabled, is_open, update_menu_position]);

  useLayoutEffect(() => {
    if (is_open) {
      update_menu_position();
    }
  }, [is_open, update_menu_position]);

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
    : root_ref.current?.closest("[data-modal-root='true']") ?? document.body;

  return {
    button_ref,
    is_open,
    menu_id,
    menu_position,
    menu_ref,
    menu_style,
    portal_container,
    root_ref,
    set_is_open,
    update_menu_position,
  };
}
