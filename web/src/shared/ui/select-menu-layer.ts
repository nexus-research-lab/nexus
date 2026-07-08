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
  estimatePosition: (button: HTMLButtonElement) => UiSelectMenuPosition;
}

export function useSelectMenuLayer({ disabled, estimatePosition }: SelectMenuLayerOptions) {
  const [isOpen, setIsOpen] = useState(false);
  const [menuPosition, setMenuPosition] = useState<UiSelectMenuPosition | null>(null);
  const menuId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  const updateMenuPosition = useCallback(() => {
    const button = buttonRef.current;
    if (!button) {
      return;
    }
    setMenuPosition(estimatePosition(button));
  }, [estimatePosition]);

  useEffect(() => {
    if (!isOpen || disabled) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!rootRef.current?.contains(target) && !menuRef.current?.contains(target)) {
        setIsOpen(false);
      }
    };
    const handleKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsOpen(false);
        buttonRef.current?.focus();
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
  }, [disabled, isOpen, updateMenuPosition]);

  useLayoutEffect(() => {
    if (isOpen) {
      updateMenuPosition();
    }
  }, [isOpen, updateMenuPosition]);

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
    : rootRef.current?.closest("[data-modal-root='true']") ?? document.body;

  return {
    buttonRef,
    isOpen,
    menuId,
    menuPosition,
    menuRef,
    menuStyle,
    portalContainer,
    rootRef,
    setIsOpen,
    updateMenuPosition,
  };
}
