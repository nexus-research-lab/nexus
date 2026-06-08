"use client";

import { type RefObject, useEffect, useRef } from "react";

const FOCUSABLE_SELECTOR = [
  "a[href]",
  "button:not([disabled])",
  "textarea:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "[tabindex]:not([tabindex='-1'])",
].join(",");

const dialog_stack: symbol[] = [];
let scroll_lock_count = 0;
let body_overflow_before_lock = "";

interface DialogModalBehaviorOptions<T extends HTMLElement> {
  enabled?: boolean;
  initial_focus_ref?: RefObject<HTMLElement | null>;
  on_close?: () => void;
  root_ref: RefObject<T | null>;
}

function lock_body_scroll() {
  if (typeof document === "undefined") {
    return;
  }

  if (scroll_lock_count === 0) {
    body_overflow_before_lock = document.body.style.overflow;
    document.body.style.overflow = "hidden";
  }

  scroll_lock_count += 1;
}

function unlock_body_scroll() {
  if (typeof document === "undefined") {
    return;
  }

  scroll_lock_count = Math.max(0, scroll_lock_count - 1);
  if (scroll_lock_count === 0) {
    document.body.style.overflow = body_overflow_before_lock;
    body_overflow_before_lock = "";
  }
}

function is_visible_focus_target(element: HTMLElement): boolean {
  if (element.hasAttribute("disabled") || element.getAttribute("aria-hidden") === "true") {
    return false;
  }

  const style = window.getComputedStyle(element);
  if (style.visibility === "hidden" || style.display === "none") {
    return false;
  }

  return element.getClientRects().length > 0;
}

function get_focusable_elements(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
    is_visible_focus_target,
  );
}

function focus_element(element: HTMLElement | null | undefined) {
  element?.focus({ preventScroll: true });
}

function is_top_dialog(token: symbol): boolean {
  return dialog_stack[dialog_stack.length - 1] === token;
}

function has_open_overlay_control(): boolean {
  return Boolean(document.querySelector("[data-ui-select-menu-open='true']"));
}

function remove_dialog_token(token: symbol) {
  const index = dialog_stack.lastIndexOf(token);
  if (index >= 0) {
    dialog_stack.splice(index, 1);
  }
}

/** 中文注释：集中提供接近 Radix Dialog 的键盘与焦点行为，业务弹窗只关心内容。 */
export function useDialogModalBehavior<T extends HTMLElement>({
  enabled = true,
  initial_focus_ref,
  on_close,
  root_ref,
}: DialogModalBehaviorOptions<T>) {
  const on_close_ref = useRef(on_close);

  useEffect(() => {
    on_close_ref.current = on_close;
  }, [on_close]);

  useEffect(() => {
    if (!enabled || typeof document === "undefined") {
      return;
    }

    const token = Symbol("ui-dialog");
    const previous_focus = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    dialog_stack.push(token);
    lock_body_scroll();

    const focus_timer = window.setTimeout(() => {
      const root = root_ref.current;
      if (!root || !is_top_dialog(token)) {
        return;
      }

      const auto_focus_target =
        initial_focus_ref?.current ??
        root.querySelector<HTMLElement>("[data-autofocus='true'], [autofocus]") ??
        get_focusable_elements(root)[0] ??
        root;
      focus_element(auto_focus_target);
    }, 0);

    const handle_key_down = (event: KeyboardEvent) => {
      if (!is_top_dialog(token) || event.defaultPrevented) {
        return;
      }

      const root = root_ref.current;
      if (!root) {
        return;
      }

      if (event.key === "Escape") {
        if (has_open_overlay_control()) {
          return;
        }
        event.preventDefault();
        on_close_ref.current?.();
        return;
      }

      if (event.key !== "Tab") {
        return;
      }

      const focusable = get_focusable_elements(root);
      if (focusable.length === 0) {
        event.preventDefault();
        focus_element(root);
        return;
      }

      const active_element = document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
      const active_index = active_element ? focusable.indexOf(active_element) : -1;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const is_focus_outside = !active_element || !root.contains(active_element);

      if (event.shiftKey && (is_focus_outside || active_index <= 0)) {
        event.preventDefault();
        focus_element(last);
        return;
      }

      if (!event.shiftKey && (is_focus_outside || active_index === focusable.length - 1)) {
        event.preventDefault();
        focus_element(first);
      }
    };

    document.addEventListener("keydown", handle_key_down);

    return () => {
      window.clearTimeout(focus_timer);
      document.removeEventListener("keydown", handle_key_down);
      remove_dialog_token(token);
      unlock_body_scroll();

      if (previous_focus?.isConnected) {
        focus_element(previous_focus);
      }
    };
  }, [enabled, initial_focus_ref, root_ref]);
}
