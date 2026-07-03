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

const dialogStack: symbol[] = [];
let scrollLockCount = 0;
let bodyOverflowBeforeLock = "";

interface DialogModalBehaviorOptions<T extends HTMLElement> {
  enabled?: boolean;
  initialFocusRef?: RefObject<HTMLElement | null>;
  onClose?: () => void;
  rootRef: RefObject<T | null>;
}

function lockBodyScroll() {
  if (typeof document === "undefined") {
    return;
  }

  if (scrollLockCount === 0) {
    bodyOverflowBeforeLock = document.body.style.overflow;
    document.body.style.overflow = "hidden";
  }

  scrollLockCount += 1;
}

function unlockBodyScroll() {
  if (typeof document === "undefined") {
    return;
  }

  scrollLockCount = Math.max(0, scrollLockCount - 1);
  if (scrollLockCount === 0) {
    document.body.style.overflow = bodyOverflowBeforeLock;
    bodyOverflowBeforeLock = "";
  }
}

function isVisibleFocusTarget(element: HTMLElement): boolean {
  if (element.hasAttribute("disabled") || element.getAttribute("aria-hidden") === "true") {
    return false;
  }

  const style = window.getComputedStyle(element);
  if (style.visibility === "hidden" || style.display === "none") {
    return false;
  }

  return element.getClientRects().length > 0;
}

function getFocusableElements(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
    isVisibleFocusTarget,
  );
}

function focusElement(element: HTMLElement | null | undefined) {
  element?.focus({ preventScroll: true });
}

function isTopDialog(token: symbol): boolean {
  return dialogStack[dialogStack.length - 1] === token;
}

function hasOpenOverlayControl(): boolean {
  return Boolean(document.querySelector("[data-ui-select-menu-open='true']"));
}

function removeDialogToken(token: symbol) {
  const index = dialogStack.lastIndexOf(token);
  if (index >= 0) {
    dialogStack.splice(index, 1);
  }
}

/** 中文注释：集中提供接近 Radix Dialog 的键盘与焦点行为，业务弹窗只关心内容。 */
export function useDialogModalBehavior<T extends HTMLElement>({
  enabled = true,
  initialFocusRef,
  onClose,
  rootRef,
}: DialogModalBehaviorOptions<T>) {
  const onCloseRef = useRef(onClose);

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    if (!enabled || typeof document === "undefined") {
      return;
    }

    const token = Symbol("ui-dialog");
    const previousFocus = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    dialogStack.push(token);
    lockBodyScroll();

    const focusTimer = window.setTimeout(() => {
      const root = rootRef.current;
      if (!root || !isTopDialog(token)) {
        return;
      }

      const autoFocusTarget =
        initialFocusRef?.current ??
        root.querySelector<HTMLElement>("[data-autofocus='true'], [autofocus]") ??
        getFocusableElements(root)[0] ??
        root;
      focusElement(autoFocusTarget);
    }, 0);

    const handleKeyDown = (event: KeyboardEvent) => {
      if (!isTopDialog(token) || event.defaultPrevented) {
        return;
      }

      const root = rootRef.current;
      if (!root) {
        return;
      }

      if (event.key === "Escape") {
        if (hasOpenOverlayControl()) {
          return;
        }
        event.preventDefault();
        onCloseRef.current?.();
        return;
      }

      if (event.key !== "Tab") {
        return;
      }

      const focusable = getFocusableElements(root);
      if (focusable.length === 0) {
        event.preventDefault();
        focusElement(root);
        return;
      }

      const activeElement = document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
      const activeIndex = activeElement ? focusable.indexOf(activeElement) : -1;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const isFocusOutside = !activeElement || !root.contains(activeElement);

      if (event.shiftKey && (isFocusOutside || activeIndex <= 0)) {
        event.preventDefault();
        focusElement(last);
        return;
      }

      if (!event.shiftKey && (isFocusOutside || activeIndex === focusable.length - 1)) {
        event.preventDefault();
        focusElement(first);
      }
    };

    document.addEventListener("keydown", handleKeyDown);

    return () => {
      window.clearTimeout(focusTimer);
      document.removeEventListener("keydown", handleKeyDown);
      removeDialogToken(token);
      unlockBodyScroll();

      if (previousFocus?.isConnected) {
        focusElement(previousFocus);
      }
    };
  }, [enabled, initialFocusRef, rootRef]);
}
