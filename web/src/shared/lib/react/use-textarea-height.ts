/**
 * INPUT: textarea 当前正文、浏览器真实排版结果与最小/最大高度约束。
 * OUTPUT: 正文、原生输入或宽度变化时同步更新的有界高度，超出上限后只滚动内部正文。
 * POS: 共享无 React 状态 textarea 测量入口；文字、设计尺寸和编辑权限由消费者持有。
 *
 * 支持 `field-sizing: content` 的浏览器由排版引擎直接按内容定高（含字体、换行与 IME 组合），
 * 不再每次输入先压到 0 再读 scrollHeight；不支持时才走 JS 测量回退。
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

/** 浏览器能否原生按内容定高；jsdom 等无 CSS.supports 的环境走测量回退。 */
export function supportsNativeFieldSizing(): boolean {
  return typeof CSS !== "undefined"
    && typeof CSS.supports === "function"
    && CSS.supports("field-sizing", "content");
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
    if (supportsNativeFieldSizing()) {
      // 清掉回退路径或外部重置留下的内联高度，否则会压过 field-sizing 的内容高度。
      el.style.setProperty("field-sizing", "content");
      el.style.height = "";
      el.style.minHeight = `${currentMinHeight}px`;
      el.style.maxHeight = `${currentMaxHeight}px`;
      el.style.overflowY = "auto";
      return;
    }
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
