/**
 * INPUT: textarea 当前正文、浏览器真实排版结果与最小/最大高度约束。
 * OUTPUT: 正文、原生输入或宽度变化时同步更新的有界高度，超出上限后只滚动内部正文。
 * POS: 共享无 React 状态 textarea 测量入口；文字、设计尺寸和编辑权限由消费者持有。
 */
import {
  RefObject,
  useCallback,
  useLayoutEffect,
  useRef,
} from "react";

interface UseTextareaHeightOptions {
  /** Minimum height in px (default 24) */
  minHeight?: number;
  /** Maximum height in px, element scrolls beyond this (default 128) */
  maxHeight?: number;
}

export function useTextareaHeight(
  ref: RefObject<HTMLTextAreaElement | null>,
  value: string,
  {
    minHeight = 24,
    maxHeight = 128,
  }: UseTextareaHeightOptions = {},
): void {
  const optionsRef = useRef({ maxHeight, minHeight });
  optionsRef.current = { maxHeight, minHeight };

  const applyHeight = useCallback(() => {
    const el = ref.current;
    if (!el) {
      return;
    }
    const { maxHeight: currentMaxHeight, minHeight: currentMinHeight } =
      optionsRef.current;
    const style = window.getComputedStyle(el);
    const borderY = (parseFloat(style.borderTopWidth) || 0)
      + (parseFloat(style.borderBottomWidth) || 0);
    const paddingY = (parseFloat(style.paddingTop) || 0)
      + (parseFloat(style.paddingBottom) || 0);
    const wasAtEnd = document.activeElement === el
      && el.selectionStart === el.value.length
      && el.selectionEnd === el.value.length;

    // Reset first so scrollHeight can shrink after text or available width does.
    // Native scrollHeight is the only source that includes the active font,
    // textarea padding, browser line breaking, and in-progress IME composition.
    el.style.height = "0px";
    const contentHeight = style.boxSizing === "border-box"
      ? el.scrollHeight + borderY
      : Math.max(0, el.scrollHeight - paddingY);
    const clamped = Math.min(
      Math.max(contentHeight, currentMinHeight),
      currentMaxHeight,
    );
    el.style.height = `${clamped}px`;
    el.style.overflowY = contentHeight > currentMaxHeight ? "auto" : "hidden";
    if (contentHeight > currentMaxHeight && wasAtEnd) {
      el.scrollTop = el.scrollHeight;
    }
  }, [ref]);

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) {
      return;
    }
    applyHeight();
    el.addEventListener("input", applyHeight);
    const observer = new ResizeObserver(applyHeight);
    observer.observe(el);
    return () => {
      el.removeEventListener("input", applyHeight);
      observer.disconnect();
    };
  }, [applyHeight, ref]);

  useLayoutEffect(() => {
    applyHeight();
  }, [applyHeight, maxHeight, minHeight, value]);
}
