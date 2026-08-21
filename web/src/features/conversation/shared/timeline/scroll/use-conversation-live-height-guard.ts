/**
 * INPUT: 当前会话、Feed 实测高度、内容对齐方向与是否仍处于 live layout epoch。
 * OUTPUT: live 自动变化期间单调不减、用户主动收起时精确回退、按目标锚点对齐的 Feed 高度。
 * POS: FOLLOW/READING 共用的 Feed 级几何保护；普通聊天底部锚定，显式编辑流可从顶部向下增长。
 */
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  type RefObject,
} from "react";

import {
  CONVERSATION_EXPLICIT_SHRINK_EVENT,
  getConversationExplicitShrinkDetail,
} from "./conversation-layout-events";

const LIVE_HEIGHT_SETTLE_MS = 160;
const HEIGHT_TOLERANCE_PX = 0.5;

export interface ConversationLiveHeightGuardState {
  minimumHeight: number;
  scopeKey: string;
  wasActive: boolean;
}

interface ConversationLiveHeightGuardInput {
  active: boolean;
  measuredHeight: number;
  scopeKey: string;
}

export interface ConversationLiveHeightGuardRevision {
  minimumHeight: number;
  releasing: boolean;
  state: ConversationLiveHeightGuardState;
}

export function resolveConversationLiveHeightAfterExplicitShrink(
  previous: ConversationLiveHeightGuardState,
  heightDelta: number,
): ConversationLiveHeightGuardState {
  const normalizedDelta = Number.isFinite(heightDelta)
    ? Math.max(0, heightDelta)
    : 0;
  return {
    ...previous,
    minimumHeight: Math.max(0, previous.minimumHeight - normalizedDelta),
  };
}

export function resolveConversationLiveHeightGuard({
  active,
  measuredHeight,
  scopeKey,
}: ConversationLiveHeightGuardInput, previous: ConversationLiveHeightGuardState): ConversationLiveHeightGuardRevision {
  const normalizedHeight = Math.max(0, measuredHeight);
  if (previous.scopeKey !== scopeKey) {
    const minimumHeight = active ? normalizedHeight : 0;
    return {
      minimumHeight,
      releasing: false,
      state: { minimumHeight, scopeKey, wasActive: active },
    };
  }
  if (active) {
    const minimumHeight = previous.wasActive
      ? Math.max(previous.minimumHeight, normalizedHeight)
      : normalizedHeight;
    return {
      minimumHeight,
      releasing: false,
      state: { minimumHeight, scopeKey, wasActive: true },
    };
  }
  return {
    minimumHeight: 0,
    releasing: previous.wasActive
      && previous.minimumHeight > normalizedHeight + HEIGHT_TOLERANCE_PX,
    state: { minimumHeight: 0, scopeKey, wasActive: false },
  };
}

interface UseConversationLiveHeightGuardOptions {
  active: boolean;
  contentAlignment?: ConversationLiveContentAlignment;
  feedRef: RefObject<HTMLDivElement | null>;
  revision: string;
  scopeKey: string | null;
}

export type ConversationLiveContentAlignment = "start" | "end";

export function resolveConversationLiveContentJustifyContent(
  alignment: ConversationLiveContentAlignment,
): "flex-start" | "flex-end" {
  return alignment === "start" ? "flex-start" : "flex-end";
}

export function useConversationLiveHeightGuard({
  active,
  contentAlignment = "end",
  feedRef,
  revision,
  scopeKey,
}: UseConversationLiveHeightGuardOptions): void {
  const normalizedScopeKey = scopeKey ?? "";
  const activeRef = useRef(active);
  const requestedActiveRef = useRef(active);
  const stateRef = useRef<ConversationLiveHeightGuardState>({
    minimumHeight: 0,
    scopeKey: normalizedScopeKey,
    wasActive: false,
  });
  const settleTimerRef = useRef<number | null>(null);
  const guardedFeedRef = useRef<HTMLDivElement | null>(null);
  requestedActiveRef.current = active;

  const cancelSettle = useCallback(() => {
    if (settleTimerRef.current !== null) {
      window.clearTimeout(settleTimerRef.current);
      settleTimerRef.current = null;
    }
  }, []);

  const applyRevision = useCallback((
    feed: HTMLDivElement,
    effectiveActive: boolean,
  ) => {
    if (stateRef.current.scopeKey !== normalizedScopeKey) {
      cancelSettle();
      const previousFeed = guardedFeedRef.current;
      if (previousFeed) {
        clearConversationLiveHeightGuardStyle(previousFeed);
      }
      clearConversationLiveHeightGuardStyle(feed);
      stateRef.current = {
        minimumHeight: 0,
        scopeKey: normalizedScopeKey,
        wasActive: false,
      };
    }
    const previous = stateRef.current;
    let measuredHeight = feed.getBoundingClientRect().height;
    if (!effectiveActive && previous.wasActive) {
      // min-height 本身会污染 border-box 测量；结算 epoch 时只在本次
      // layout effect 内短暂读取 intrinsic height，随后恢复 held height。
      clearConversationLiveHeightGuardStyle(feed);
      measuredHeight = feed.getBoundingClientRect().height;
    }
    const next = resolveConversationLiveHeightGuard({
      active: effectiveActive,
      measuredHeight,
      scopeKey: normalizedScopeKey,
    }, stateRef.current);
    stateRef.current = next.state;
    guardedFeedRef.current = feed;

    if (effectiveActive) {
      feed.style.transition = "none";
      feed.style.minHeight = `${next.minimumHeight}px`;
      feed.style.justifyContent = resolveConversationLiveContentJustifyContent(contentAlignment);
      feed.dataset.conversationLiveHeightGuard = "active";
      return;
    }

    if (!next.releasing) {
      clearConversationLiveHeightGuardStyle(feed);
      return;
    }

    // 安静窗口结束后只提交一次 intrinsic 几何。min-height transition 会在每一帧
    // 触发 ResizeObserver，并与 follow/virtualizer 的 scrollTop 写入形成反馈环。
    clearConversationLiveHeightGuardStyle(feed);
  }, [cancelSettle, contentAlignment, normalizedScopeKey]);

  useLayoutEffect(() => {
    const feed = feedRef.current;
    if (!feed) {
      return;
    }
    if (active) {
      cancelSettle();
      activeRef.current = true;
      applyRevision(feed, true);
      return;
    }
    if (!activeRef.current) {
      applyRevision(feed, false);
      return;
    }

    // terminal message、footer 和 iframe 的最终尺寸通常晚于 runtime 状态一到数帧。
    // 结算窗口继续持有顶部高度负债，避免先释放、随后又被最终工具尺寸拉动。
    applyRevision(feed, true);
    feed.dataset.conversationLiveHeightGuard = "settling";
    cancelSettle();
    settleTimerRef.current = window.setTimeout(() => {
      settleTimerRef.current = null;
      if (
        requestedActiveRef.current
        || guardedFeedRef.current !== feed
      ) {
        return;
      }
      activeRef.current = false;
      applyRevision(feed, false);
    }, LIVE_HEIGHT_SETTLE_MS);
  }, [active, applyRevision, cancelSettle, feedRef, revision]);

  useEffect(() => {
    const feed = feedRef.current;
    if (!feed || typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(() => {
      if (!activeRef.current) {
        return;
      }
      applyRevision(feed, true);
    });
    observer.observe(feed);
    return () => observer.disconnect();
  }, [applyRevision, feedRef, revision]);

  useEffect(() => {
    const handleExplicitShrink = (event: Event) => {
      const feed = feedRef.current;
      const target = event.target;
      const detail = getConversationExplicitShrinkDetail(event);
      if (
        !feed
        || !activeRef.current
        || !(target instanceof Node)
        || !feed.contains(target)
        || !detail
      ) {
        return;
      }
      const next = resolveConversationLiveHeightAfterExplicitShrink(
        stateRef.current,
        detail.heightDelta,
      );
      stateRef.current = next;
      feed.style.transition = "none";
      feed.style.minHeight = `${next.minimumHeight}px`;
      feed.style.justifyContent = resolveConversationLiveContentJustifyContent(contentAlignment);
      feed.dataset.conversationLiveHeightGuard = "active";
    };
    document.addEventListener(
      CONVERSATION_EXPLICIT_SHRINK_EVENT,
      handleExplicitShrink,
    );
    return () => document.removeEventListener(
      CONVERSATION_EXPLICIT_SHRINK_EVENT,
      handleExplicitShrink,
    );
  }, [contentAlignment, feedRef]);

  useEffect(() => () => {
    cancelSettle();
    const feed = guardedFeedRef.current;
    if (feed) {
      clearConversationLiveHeightGuardStyle(feed);
    }
  }, [cancelSettle]);
}

function clearConversationLiveHeightGuardStyle(feed: HTMLDivElement): void {
  feed.style.removeProperty("min-height");
  feed.style.removeProperty("justify-content");
  feed.style.removeProperty("transition");
  delete feed.dataset.conversationLiveHeightGuard;
}
