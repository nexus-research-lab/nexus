// INPUT: 模态根、初始焦点、关闭动作与启用状态。
// OUTPUT: 模态栈注册、滚动锁、焦点循环及仅由当前模态子浮层让出的 Escape。
// POS: Dialog 的 React 生命周期适配；焦点计算、键盘规则和栈状态分别归专用模块。
"use client";

import { type RefObject, useEffect, useRef } from "react";

import {
  focusDialogElement,
  getDialogFocusState,
  getDialogFocusableElements,
} from "@/shared/ui/dialog/dialog-focus";
import {
  type DialogKeyboardAction,
  resolveDialogKeyboardAction,
} from "@/shared/ui/dialog/dialog-keyboard";
import {
  isTopDialogModal,
  registerDialogModal,
  unregisterDialogModal,
} from "@/shared/ui/dialog/dialog-modal-runtime";
import { OPEN_OVERLAY_SELECTOR } from "@/shared/ui/overlay/overlay-contract";

interface DialogModalBehaviorOptions<T extends HTMLElement> {
  enabled?: boolean;
  initialFocusRef?: RefObject<HTMLElement | null>;
  onClose?: () => void;
  rootRef: RefObject<T | null>;
}

function hasOpenOverlayControl(root: HTMLElement): boolean {
  return Boolean(root.querySelector(OPEN_OVERLAY_SELECTOR));
}

interface DialogKeyboardActionContext {
  event: KeyboardEvent;
  first: HTMLElement | null;
  last: HTMLElement | null;
  onClose?: () => void;
  root: HTMLElement;
}

type DialogKeyboardActionHandler = (context: DialogKeyboardActionContext) => void;

const DIALOG_KEYBOARD_ACTION_HANDLERS: Record<
  DialogKeyboardAction,
  DialogKeyboardActionHandler
> = {
  close: ({ event, onClose }) => {
    event.preventDefault();
    onClose?.();
  },
  "focus-first": ({ event, first }) => {
    event.preventDefault();
    focusDialogElement(first);
  },
  "focus-last": ({ event, last }) => {
    event.preventDefault();
    focusDialogElement(last);
  },
  "focus-root": ({ event, root }) => {
    event.preventDefault();
    focusDialogElement(root);
  },
  ignore: () => undefined,
};

/** 统一装配模态栈、滚动锁定、初始焦点、焦点循环与焦点恢复。 */
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

    const token = registerDialogModal(rootRef.current);
    const previousFocus = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;

    const focusTimer = window.setTimeout(() => {
      const root = rootRef.current;
      if (!root || !isTopDialogModal(token)) {
        return;
      }

      const autoFocusTarget =
        initialFocusRef?.current ??
        root.querySelector<HTMLElement>("[data-autofocus='true'], [autofocus]") ??
        getDialogFocusableElements(root)[0] ??
        root;
      focusDialogElement(autoFocusTarget);
    }, 0);

    const handleKeyDown = (event: KeyboardEvent) => {
      const root = rootRef.current;
      if (!root || !isTopDialogModal(token) || event.defaultPrevented) {
        return;
      }

      const focusable = getDialogFocusableElements(root);
      const focusState = getDialogFocusState(root, focusable);
      const action = resolveDialogKeyboardAction({
        ...focusState,
        focusableCount: focusable.length,
        hasOpenOverlay: hasOpenOverlayControl(root),
        key: event.key,
        shiftKey: event.shiftKey,
      });
      DIALOG_KEYBOARD_ACTION_HANDLERS[action]({
        event,
        first: focusable[0] ?? null,
        last: focusable.at(-1) ?? null,
        onClose: onCloseRef.current,
        root,
      });
    };

    document.addEventListener("keydown", handleKeyDown);

    return () => {
      window.clearTimeout(focusTimer);
      document.removeEventListener("keydown", handleKeyDown);
      unregisterDialogModal(token);

      if (previousFocus?.isConnected) {
        focusDialogElement(previousFocus);
      }
    };
  }, [enabled, initialFocusRef, rootRef]);
}
