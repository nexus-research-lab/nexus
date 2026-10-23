// INPUT: 滚动容器元素引用与滚动/尺寸变化。
// OUTPUT: data-sidebar-scroll 与动态 data-sidebar-fade 属性，驱动遮罩淡入淡出。
// POS: 侧栏与列表通用的滚动渐隐 Hook；现代浏览器交由 CSS 时间轴，旧/WebKit 环境回退为状态标记。

import { useCallback, useLayoutEffect, useRef, useState } from "react";

export type ScrollFade = "both" | "bottom" | "none" | "top";

export function resolveScrollFade(element: {
  clientHeight: number;
  scrollHeight: number;
  scrollTop: number;
}): ScrollFade {
  const hasOverflow = element.scrollHeight - element.clientHeight > 1;
  if (!hasOverflow) {
    return "none";
  }
  if (element.scrollTop <= 1) {
    return "bottom";
  }
  const isAtBottom = element.scrollHeight - element.clientHeight - element.scrollTop <= 1;
  return isAtBottom ? "top" : "both";
}

export const NATIVE_SCROLL_FADE = typeof CSS !== "undefined"
  && typeof CSS.supports === "function"
  && CSS.supports("animation-timeline: scroll()");

export function useScrollFade<T extends HTMLElement = HTMLDivElement>() {
  const scrollRef = useRef<T | null>(null);
  const [fade, setFade] = useState<ScrollFade>("none");

  const updateFade = useCallback(() => {
    const element = scrollRef.current;
    if (!element || NATIVE_SCROLL_FADE) {
      return;
    }
    const nextFade = resolveScrollFade(element);
    setFade((current) => (current === nextFade ? current : nextFade));
  }, []);

  // 容器高度通常由 flex 固定，列表加载或增删条目不会改变容器尺寸；
  // 每次渲染后重算，并监听直接子节点增删，覆盖子组件自行渲染的情况。
  useLayoutEffect(() => {
    updateFade();
  });

  useLayoutEffect(() => {
    const element = scrollRef.current;
    if (!element || NATIVE_SCROLL_FADE) {
      return;
    }
    const resizeObserver = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(updateFade);
    resizeObserver?.observe(element);
    const mutationObserver = typeof MutationObserver === "undefined" ? null : new MutationObserver(updateFade);
    mutationObserver?.observe(element, { childList: true });
    return () => {
      resizeObserver?.disconnect();
      mutationObserver?.disconnect();
    };
  }, [updateFade]);

  return {
    scrollRef,
    fadeProps: {
      "data-sidebar-scroll": true,
      "data-sidebar-fade": NATIVE_SCROLL_FADE ? undefined : fade,
      onScroll: updateFade,
    },
  };
}
