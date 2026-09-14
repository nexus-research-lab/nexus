// INPUT: 锚点、开关、定位投影、源区域命中语义、Escape 阶段与焦点归还策略。
// OUTPUT: Portal、定位、按模态范围仲裁的关闭及失效锚点清理；输入法候选键不触发退出。
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
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";

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
  // Context menus use the invoking region for scope, but clicking it dismisses the menu.
  anchorPress?: "inside" | "outside";
  // Text-editor suggestions retain focus in a field whose parent may stop key bubbling.
  captureEscape?: boolean;
  disabled: boolean;
  estimatePosition: (anchor: T) => UiAnchoredOverlayPosition;
  isOpen: boolean;
  onClose: () => void;
  // 只读 hover 提示不移动焦点，关闭时也不能抢走原控件的焦点。
  restoreFocus?: false | (() => void);
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
  anchorPress = "inside",
  captureEscape = false,
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
      if (!anchor.isConnected) {
        onClose();
        return;
      }
      const nextPosition = estimatePosition(anchor);
      setPosition((currentPosition) => (
        areAnchoredOverlayPositionsEqual(currentPosition, nextPosition)
          ? currentPosition
          : nextPosition
      ));
    }
  }, [anchorRef, estimatePosition, onClose]);

  useEffect(() => {
    if (!isOpen || disabled) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      if (anchorRef.current && !anchorRef.current.isConnected) {
        onClose();
        return;
      }
      const target = event.target as Node;
      if (!isAnchoredOverlayOutsidePress(overlayRef.current, target, anchorPress)) {
        return;
      }
      onClose();
    };
    const handleKeyDown = (event: globalThis.KeyboardEvent) => {
      // 记录的调用元素可能已被目录刷新移除；只清理失效浮层，不消费当前模态的按键。
      if (event.key === "Escape" && !isImeKeyboardEvent(event)
        && anchorRef.current && !anchorRef.current.isConnected) {
        onClose();
        return;
      }
      if (
        event.key !== "Escape"
        || isImeKeyboardEvent(event)
        || event.defaultPrevented
        || !isTopAnchoredOverlay(overlayRef.current)
      ) {
        return;
      }
      event.preventDefault();
      if (captureEscape) event.stopPropagation();
      onClose();
      if (restoreFocus) {
        restoreFocus();
      } else if (restoreFocus !== false) {
        anchorRef.current?.focus();
      }
    };

    document.addEventListener("pointerdown", handlePointerDown, true);
    document.addEventListener("keydown", handleKeyDown, captureEscape);
    window.addEventListener("resize", updatePosition);
    window.addEventListener("scroll", updatePosition, true);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown, true);
      document.removeEventListener("keydown", handleKeyDown, captureEscape);
      window.removeEventListener("resize", updatePosition);
      window.removeEventListener("scroll", updatePosition, true);
    };
  }, [anchorPress, anchorRef, captureEscape, disabled, isOpen, onClose, restoreFocus, updatePosition]);

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
