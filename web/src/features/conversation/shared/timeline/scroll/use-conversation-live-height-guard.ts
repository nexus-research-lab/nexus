/**
 * INPUT: 当前会话、Feed 实测高度与是否仍处于 live layout epoch。
 * OUTPUT: live 期间单调不减的 Feed 最小高度，并在 epoch 收口后一次性平滑释放高度负债。
 * POS: FOLLOW/READING 共用的 Feed 级几何保护；不冻结消息子树，也不阻止正文和工具块继续自然增长。
 */
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  type RefObject,
} from "react";

const LIVE_HEIGHT_RELEASE_MS = 220;
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
  feedRef: RefObject<HTMLDivElement | null>;
  revision: string;
  scopeKey: string | null;
}

export function useConversationLiveHeightGuard({
  active,
  feedRef,
  revision,
  scopeKey,
}: UseConversationLiveHeightGuardOptions): void {
  const normalizedScopeKey = scopeKey ?? "";
  const activeRef = useRef(active);
  const stateRef = useRef<ConversationLiveHeightGuardState>({
    minimumHeight: 0,
    scopeKey: normalizedScopeKey,
    wasActive: false,
  });
  const releaseTimerRef = useRef<number | null>(null);
  const guardedFeedRef = useRef<HTMLDivElement | null>(null);
  activeRef.current = active;

  const cancelRelease = useCallback(() => {
    if (releaseTimerRef.current !== null) {
      window.clearTimeout(releaseTimerRef.current);
      releaseTimerRef.current = null;
    }
  }, []);

  const applyRevision = useCallback((feed: HTMLDivElement) => {
    if (stateRef.current.scopeKey !== normalizedScopeKey) {
      cancelRelease();
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
    if (!activeRef.current && previous.wasActive) {
      // min-height 本身会污染 border-box 测量；结算 epoch 时只在本次
      // layout effect 内短暂读取 intrinsic height，随后恢复 held height。
      clearConversationLiveHeightGuardStyle(feed);
      measuredHeight = feed.getBoundingClientRect().height;
    }
    const next = resolveConversationLiveHeightGuard({
      active: activeRef.current,
      measuredHeight,
      scopeKey: normalizedScopeKey,
    }, stateRef.current);
    stateRef.current = next.state;
    guardedFeedRef.current = feed;

    if (activeRef.current) {
      cancelRelease();
      feed.style.transition = "none";
      feed.style.minHeight = `${next.minimumHeight}px`;
      feed.dataset.conversationLiveHeightGuard = "active";
      return;
    }

    if (!next.releasing) {
      clearConversationLiveHeightGuardStyle(feed);
      return;
    }

    cancelRelease();
    feed.style.transition = "none";
    feed.style.minHeight = `${previous.minimumHeight}px`;
    feed.dataset.conversationLiveHeightGuard = "releasing";
    // 先提交 held height，再平滑收口到 intrinsic height。
    void feed.offsetHeight;
    feed.style.transition = [
      "min-height",
      `${LIVE_HEIGHT_RELEASE_MS}ms`,
      "cubic-bezier(0.2, 0.8, 0.2, 1)",
    ].join(" ");
    feed.style.minHeight = `${measuredHeight}px`;
    releaseTimerRef.current = window.setTimeout(() => {
      releaseTimerRef.current = null;
      if (guardedFeedRef.current === feed && !activeRef.current) {
        clearConversationLiveHeightGuardStyle(feed);
      }
    }, LIVE_HEIGHT_RELEASE_MS + 50);
  }, [cancelRelease, normalizedScopeKey]);

  useLayoutEffect(() => {
    const feed = feedRef.current;
    if (!feed) {
      return;
    }
    applyRevision(feed);
  }, [active, applyRevision, feedRef, revision]);

  useEffect(() => {
    const feed = feedRef.current;
    if (!feed || typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(() => {
      if (!activeRef.current) {
        return;
      }
      applyRevision(feed);
    });
    observer.observe(feed);
    return () => observer.disconnect();
  }, [applyRevision, feedRef, revision]);

  useEffect(() => () => {
    cancelRelease();
    const feed = guardedFeedRef.current;
    if (feed) {
      clearConversationLiveHeightGuardStyle(feed);
    }
  }, [cancelRelease]);
}

function clearConversationLiveHeightGuardStyle(feed: HTMLDivElement): void {
  feed.style.removeProperty("min-height");
  feed.style.removeProperty("transition");
  delete feed.dataset.conversationLiveHeightGuard;
}
