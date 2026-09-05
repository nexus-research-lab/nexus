// INPUT: 锚点、开关状态、定位投影以及可选的焦点归还策略。
// OUTPUT: Portal 容器、稳定浮层身份、定位样式与按模态范围仲裁的关闭/重定位生命周期。
// POS: 锚定浮层浏览器适配层；不决定 Menu、Tooltip 或 Popover 的内容与键盘语义。
"use client";

import {
  type CSSProperties,
  type RefObject,
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import {
  areAnchoredOverlayPositionsEqual,
  type UiAnchoredOverlayPosition,
} from "./anchored-overlay-model";
import {
  isAnchoredOverlayOutsidePress,
  isTopAnchoredOverlay,
  registerAnchoredOverlay,
} from "./overlay-dismissal-runtime";

interface AnchoredOverlayLayerOptions<T extends HTMLElement> {
  anchorRef: RefObject<T | null>;
  disabled: boolean;
  estimatePosition: (anchor: T) => UiAnchoredOverlayPosition;
  isOpen: boolean;
  onClose: () => void;
  restoreFocus?: () => void;
}

interface AnchoredOverlayRegistration {
  anchor: HTMLElement;
  overlay: HTMLElement;
  unregister: () => void;
}

function buildOverlayStyle(
  position: UiAnchoredOverlayPosition | null,
): CSSProperties {
  if (!position) {
    return { visibility: "hidden" };
  }
  return {
    // 消费者通常用 top-0 提供未测量前的稳定原点；定位完成后必须显式
    // 清空另一条轴，否则 top 与 bottom 同时生效会把向上浮层钉到视口顶部。
    bottom: position.bottom ?? "auto",
    left: position.left,
    maxHeight: position.maxHeight,
    top: position.top ?? "auto",
    visibility: "visible",
    width: position.width,
  };
}

function resolvePortalContainer(anchor: HTMLElement | null): Element | null {
  if (typeof document === "undefined") {
    return null;
  }
  return anchor?.closest("[data-modal-root='true']") ?? document.body;
}

/** 统一锚定浮层的浏览器生命周期，消费者只负责交互语义和内容。 */
export function useAnchoredOverlayLayer<T extends HTMLElement>({
  anchorRef,
  disabled,
  estimatePosition,
  isOpen,
  onClose,
  restoreFocus,
}: AnchoredOverlayLayerOptions<T>) {
  const overlayId = useId();
  const overlayRef = useRef<HTMLDivElement>(null);
  const registrationRef = useRef<AnchoredOverlayRegistration | null>(null);
  const [position, setPosition] = useState<UiAnchoredOverlayPosition | null>(null);
  const portalContainer = resolvePortalContainer(anchorRef.current);

  useLayoutEffect(() => {
    // RefObject 的挂载不属于 React 依赖；每次提交核对真实节点，节点未变时保留原打开顺序。
    const anchor = isOpen && !disabled ? anchorRef.current : null;
    const overlay = anchor ? overlayRef.current : null;
    const registration = registrationRef.current;
    if (registration?.anchor === anchor && registration?.overlay === overlay) {
      return;
    }
    registration?.unregister();
    registrationRef.current = anchor && overlay ? {
      anchor,
      overlay,
      unregister: registerAnchoredOverlay(anchor, overlay),
    } : null;
  });

  useLayoutEffect(() => () => {
    registrationRef.current?.unregister();
    registrationRef.current = null;
  }, []);

  const updatePosition = useCallback(() => {
    const anchor = anchorRef.current;
    if (anchor) {
      const nextPosition = estimatePosition(anchor);
      setPosition((currentPosition) => (
        areAnchoredOverlayPositionsEqual(currentPosition, nextPosition)
          ? currentPosition
          : nextPosition
      ));
    }
  }, [anchorRef, estimatePosition]);

  useEffect(() => {
    if (!isOpen || disabled) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!isAnchoredOverlayOutsidePress(overlayRef.current, target)) {
        return;
      }
      onClose();
    };
    const handleKeyDown = (event: globalThis.KeyboardEvent) => {
      if (
        event.key !== "Escape"
        || event.defaultPrevented
        || !isTopAnchoredOverlay(overlayRef.current)
      ) {
        return;
      }
      event.preventDefault();
      onClose();
      if (restoreFocus) {
        restoreFocus();
      } else {
        anchorRef.current?.focus();
      }
    };

    document.addEventListener("pointerdown", handlePointerDown, true);
    document.addEventListener("keydown", handleKeyDown);
    window.addEventListener("resize", updatePosition);
    window.addEventListener("scroll", updatePosition, true);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown, true);
      document.removeEventListener("keydown", handleKeyDown);
      window.removeEventListener("resize", updatePosition);
      window.removeEventListener("scroll", updatePosition, true);
    };
  }, [anchorRef, disabled, isOpen, onClose, restoreFocus, updatePosition]);

  useLayoutEffect(() => {
    if (isOpen && !disabled) {
      updatePosition();
    }
  }, [disabled, isOpen, updatePosition]);

  const overlayStyle = buildOverlayStyle(position);

  return {
    overlayId,
    overlayPosition: position,
    overlayRef,
    overlayStyle,
    portalContainer,
    updateOverlayPosition: updatePosition,
  };
}
