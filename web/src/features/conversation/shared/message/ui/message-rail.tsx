/**
 * =====================================================
 * @File   : message-rail.tsx
 * @Date   : 2026-04-05 15:08
 * @Author : leemysw
 * 2026-04-05 15:08   Create
 * =====================================================
 */

"use client";

import {
  createContext,
  useCallback,
  useContext,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";

import {
  getScrollBottomTop,
  isAtScrollBottom,
  resolveScrollFade,
  shouldPauseFollowOnScroll,
  type ScrollFade,
} from "@/features/conversation/shared/timeline/scroll/follow-scroll-model";

export function MessageDetailFrame({ children }: { children: ReactNode }) {
  return (
    <div
      className="ml-4 mt-1 min-w-0 border-l border-(--divider-subtle-color) py-0.5 pl-4"
      data-message-detail-frame
    >
      {children}
    </div>
  );
}

const MessageDetailScrollContext = createContext(false);

// 支持滚动时间轴时渐隐完全交给 CSS，滚动和流式增长不再为虚化方向触发 React 渲染。
const NATIVE_SCROLL_FADE = typeof CSS !== "undefined"
  && typeof CSS.supports === "function"
  && CSS.supports("animation-timeline: scroll()");

export function MessageDetailScroll({
  children,
  followContent = false,
}: {
  children: ReactNode;
  followContent?: boolean;
}) {
  const nested = useContext(MessageDetailScrollContext);
  if (nested) {
    return children;
  }
  return (
    <MessageDetailScrollRoot followContent={followContent}>
      {children}
    </MessageDetailScrollRoot>
  );
}

function MessageDetailScrollRoot({
  children,
  followContent,
}: {
  children: ReactNode;
  followContent: boolean;
}) {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const contentRef = useRef<HTMLDivElement | null>(null);
  const followingRef = useRef(followContent);
  const lastScrollTopRef = useRef(0);
  const [fade, setFade] = useState<ScrollFade>("none");
  const updateFade = useCallback(() => {
    const element = scrollRef.current;
    if (!element || NATIVE_SCROLL_FADE) {
      return;
    }
    const nextFade = resolveScrollFade(element);
    setFade((currentFade) => currentFade === nextFade
      ? currentFade
      : nextFade);
  }, []);
  const updateLayout = useCallback(() => {
    const element = scrollRef.current;
    if (!element) {
      return;
    }
    if (followContent && followingRef.current) {
      element.scrollTop = getScrollBottomTop(element);
      lastScrollTopRef.current = element.scrollTop;
    }
    if (followContent && isAtScrollBottom(element)) {
      followingRef.current = true;
    }
    updateFade();
  }, [followContent, updateFade]);
  const handleScroll = useCallback(() => {
    const element = scrollRef.current;
    if (element) {
      // 只有向上滚动才退出跟随。程序化贴底的 scroll 事件在下一帧才派发，期间新内容可能
      // 已提交（代码块转正文一次增长几十 px），此时按“是否贴底”判断会把自身贴底误当成用户离开。
      const previousScrollTop = lastScrollTopRef.current;
      lastScrollTopRef.current = element.scrollTop;
      followingRef.current = followContent && (
        isAtScrollBottom(element)
        || (followingRef.current
          && !shouldPauseFollowOnScroll(element, previousScrollTop, true))
      );
    }
    updateFade();
  }, [followContent, updateFade]);

  useLayoutEffect(() => {
    followingRef.current = followContent;
  }, [followContent]);

  useLayoutEffect(() => {
    updateLayout();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(updateLayout);
    if (scrollRef.current) {
      observer.observe(scrollRef.current);
    }
    if (contentRef.current) {
      observer.observe(contentRef.current);
    }
    return () => observer.disconnect();
  // 内容尺寸由 contentRef 的 ResizeObserver 收口；不要把每次流式 React
  // 子树更新都当成观察器重挂载，否则高频 Markdown 提交会反复解绑/重绑，
  // 让明细滚动在生成期间出现额外的时序抖动。
  }, [followContent, updateLayout]);

  return (
    <div
      className="min-w-0 max-h-[17.5rem] overflow-auto overscroll-contain custom-scrollbar"
      data-message-detail-fade={fade}
      data-message-detail-follow={followContent || undefined}
      data-message-detail-scroll
      onScroll={handleScroll}
      ref={scrollRef}
    >
      <MessageDetailScrollContext.Provider value>
        <div ref={contentRef}>{children}</div>
      </MessageDetailScrollContext.Provider>
    </div>
  );
}
