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
  isAtScrollBottom,
  resolveScrollFade,
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
  const [fade, setFade] = useState<ScrollFade>("none");
  const updateFade = useCallback(() => {
    const element = scrollRef.current;
    if (!element) {
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
      element.scrollTop = element.scrollHeight;
    }
    if (followContent && isAtScrollBottom(element)) {
      followingRef.current = true;
    }
    updateFade();
  }, [followContent, updateFade]);
  const handleScroll = useCallback(() => {
    const element = scrollRef.current;
    if (element) {
      followingRef.current = followContent
        && isAtScrollBottom(element);
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
  }, [children, updateLayout]);

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
