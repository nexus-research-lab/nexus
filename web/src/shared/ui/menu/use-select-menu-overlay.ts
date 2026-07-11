"use client";

import {
  type KeyboardEvent,
  useCallback,
  useRef,
  useState,
} from "react";

import { useAnchoredOverlayLayer } from "../overlay/anchored-overlay-layer";
import type { UiAnchoredOverlayPosition } from "../overlay/anchored-overlay-model";

type MoveSelection = (direction: 1 | -1) => boolean;

interface UseSelectMenuOverlayOptions {
  disabled: boolean;
  estimatePosition: (button: HTMLButtonElement) => UiAnchoredOverlayPosition;
}

const SELECTION_DIRECTION_BY_KEY: Record<string, 1 | -1> = {
  ArrowDown: 1,
  ArrowUp: -1,
};

const TOGGLE_KEYS = new Set(["Enter", " "]);

/** Select 家族共用内部开关与键盘协议，选项变化语义仍由消费者决定。 */
export function useSelectMenuOverlay({
  disabled,
  estimatePosition,
}: UseSelectMenuOverlayOptions) {
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const closeMenu = useCallback(() => setIsOpen(false), []);
  const {
    overlayId: menuId,
    overlayPosition: menuPosition,
    overlayRef: menuRef,
    overlayStyle: menuStyle,
    portalContainer,
    updateOverlayPosition: updateMenuPosition,
  } = useAnchoredOverlayLayer({
    anchorRef: buttonRef,
    disabled,
    estimatePosition,
    isOpen,
    onClose: closeMenu,
  });

  const openMenu = useCallback(() => {
    if (disabled) {
      return;
    }
    updateMenuPosition();
    setIsOpen(true);
  }, [disabled, updateMenuPosition]);

  const toggleMenu = useCallback(() => {
    if (disabled) {
      return;
    }
    setIsOpen((open) => {
      if (!open) {
        updateMenuPosition();
      }
      return !open;
    });
  }, [disabled, updateMenuPosition]);

  const handleTriggerKeyDown = useCallback((
    event: KeyboardEvent<HTMLButtonElement>,
    moveSelection?: MoveSelection,
  ) => {
    if (disabled) {
      return;
    }
    if (event.key === "Escape") {
      closeMenu();
      return;
    }
    if (TOGGLE_KEYS.has(event.key)) {
      event.preventDefault();
      toggleMenu();
      return;
    }
    const direction = SELECTION_DIRECTION_BY_KEY[event.key];
    if (direction && moveSelection?.(direction)) {
      event.preventDefault();
      openMenu();
    }
  }, [closeMenu, disabled, openMenu, toggleMenu]);

  return {
    buttonRef,
    closeMenu,
    handleTriggerKeyDown,
    isOpen,
    menuId,
    menuPosition,
    menuRef,
    menuStyle,
    portalContainer,
    toggleMenu,
    updateMenuPosition,
  };
}
