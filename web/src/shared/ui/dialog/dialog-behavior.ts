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

interface DialogModalBehaviorOptions<T extends HTMLElement> {
  enabled?: boolean;
  initialFocusRef?: RefObject<HTMLElement | null>;
  onClose?: () => void;
  rootRef: RefObject<T | null>;
}

function hasOpenOverlayControl(): boolean {
  return Boolean(document.querySelector("[data-ui-select-menu-open='true']"));
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

    const token = registerDialogModal();
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
        hasOpenOverlay: hasOpenOverlayControl(),
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
