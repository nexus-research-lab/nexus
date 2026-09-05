// INPUT: Select Menu disabled 状态、定位函数和触发器键盘事件。
// OUTPUT: 内部开关、锚点引用、Portal/定位状态与统一键盘协议。
// POS: Select Menu 生命周期 adapter；不解释选项、值或业务权限。
"use client";

import {
  type KeyboardEvent,
  useCallback,
  useEffect,
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

function handleSelectionKey({
  event,
  moveSelection,
  openMenu,
}: {
  event: KeyboardEvent<HTMLButtonElement>;
  moveSelection?: MoveSelection;
  openMenu: () => void;
}) {
  const direction = SELECTION_DIRECTION_BY_KEY[event.key];
  if (!direction || !moveSelection?.(direction)) {
    return;
  }
  event.preventDefault();
  openMenu();
}

/** Select 家族共用内部开关与键盘协议，选项变化语义仍由消费者决定。 */
export function useSelectMenuOverlay({
  disabled,
  estimatePosition,
}: UseSelectMenuOverlayOptions) {
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const closeMenu = useCallback(() => setIsOpen(false), []);
  const isMenuOpen = isOpen && !disabled;
  useEffect(() => {
    if (disabled) {
      closeMenu();
    }
  }, [closeMenu, disabled]);
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
    isOpen: isMenuOpen,
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
    if (TOGGLE_KEYS.has(event.key)) {
      event.preventDefault();
      toggleMenu();
      return;
    }
    handleSelectionKey({ event, moveSelection, openMenu });
  }, [disabled, openMenu, toggleMenu]);

  return {
    buttonRef,
    closeMenu,
    handleTriggerKeyDown,
    isOpen: isMenuOpen,
    menuId,
    menuPosition,
    menuRef,
    menuStyle,
    portalContainer,
    toggleMenu,
    updateMenuPosition,
  };
}
