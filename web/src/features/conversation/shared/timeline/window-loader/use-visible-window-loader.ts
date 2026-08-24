/**
 * INPUT: 当前索引时间线、滚动视口、消息驻留窗口 revision 与 round window loader。
 * OUTPUT: 可见旧轮次的串行加载、顶部下拉显式重试和零布局加载状态。
 * POS: 索引时间线按需恢复历史正文的 React 调度层。
 */
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type RefObject,
} from "react";

import { getConversationRoundNavigationTarget } from "../scroll/round-scroll";
import {
  resolveBoundaryUnloadedRoundId,
  resolveVisibleUnloadedRoundId,
} from "./visible-round-candidate";
import {
  LOAD_RECHECK_DELAY_MS,
  buildExcludedRoundIds,
  cancelWindowLoaderRuntime,
  createWindowLoaderRuntime,
  createWindowLoadRequest,
  isCurrentWindowLoadRequest,
  recordWindowLoadResult,
  refreshWindowLoaderContent,
  resetWindowLoaderScope,
  shouldRefreshWindowLoaderFromPull,
  updateWindowLoaderScroll,
  type WindowLoaderRuntime,
  type WindowLoadRequest,
  type WindowLoadResult,
} from "./window-loader-runtime";

interface UseVisibleRoundWindowLoaderOptions {
  enabled: boolean;
  loadRoundWindow?: (roundId: string) => Promise<boolean>;
  loadedRoundIds: readonly string[];
  revision: string | number;
  scopeKey: string | null;
  scrollRef: RefObject<HTMLDivElement | null>;
  windowRoundIds: readonly string[];
}

export function useVisibleRoundWindowLoader({
  enabled,
  loadRoundWindow,
  loadedRoundIds,
  revision,
  scopeKey,
  scrollRef,
  windowRoundIds,
}: UseVisibleRoundWindowLoaderOptions) {
  const frameRef = useRef<number | null>(null);
  const [loadingRequest, setLoadingRequest] = useState<WindowLoadRequest | null>(null);
  const requestSequenceRef = useRef(0);
  const runCheckRef = useRef<() => void>(() => {});
  const runtimeRef = useRef(createWindowLoaderRuntime());
  const timeoutRef = useRef<number | null>(null);
  const latestOptionsRef = useRef({
    enabled,
    loadedRoundIds,
    loadRoundWindow,
    scopeKey,
    windowRoundIds,
  });
  latestOptionsRef.current = {
    enabled,
    loadedRoundIds,
    loadRoundWindow,
    scopeKey,
    windowRoundIds,
  };

  const scheduleCheck = useCallback(() => {
    if (frameRef.current !== null) {
      return;
    }
    frameRef.current = window.requestAnimationFrame(() => {
      frameRef.current = null;
      runCheckRef.current();
    });
  }, []);

  const scheduleRetry = useCallback(
    (delay: number) => {
      if (timeoutRef.current !== null) {
        window.clearTimeout(timeoutRef.current);
      }
      timeoutRef.current = window.setTimeout(() => {
        timeoutRef.current = null;
        scheduleCheck();
      }, delay);
    },
    [scheduleCheck],
  );

  runCheckRef.current = () => {
    const latest = latestOptionsRef.current;
    const runtime = runtimeRef.current;
    if (!canStartWindowLoad(runtime, latest)) {
      return;
    }

    const scrollElement = scrollRef.current;
    if (!scrollElement || getConversationRoundNavigationTarget(scrollElement)) {
      return;
    }
    const excludedRoundIds = buildExcludedRoundIds(runtime, Date.now());
    const roundId = resolveVisibleUnloadedRoundId(
      scrollElement,
      excludedRoundIds,
      runtime.scrollDirection,
    ) ?? resolveBoundaryUnloadedRoundId({
      clientHeight: scrollElement.clientHeight,
      excludedRoundIds,
      loadedRoundIds: latest.loadedRoundIds,
      roundIds: latest.windowRoundIds,
      scrollHeight: scrollElement.scrollHeight,
      scrollTop: scrollElement.scrollTop,
    });
    if (!roundId) {
      return;
    }

    const request = createWindowLoadRequest(
      runtime,
      ++requestSequenceRef.current,
      roundId,
    );
    runtime.activeRequest = request;
    setLoadingRequest(request);
    void runWindowLoad({
      loader: latest.loadRoundWindow!,
      onSettled: () => {
        setLoadingRequest((current) => (
          current?.generation === request.generation && current.id === request.id
            ? null
            : current
        ));
      },
      request,
      runtime: runtimeRef,
      scheduleRetry,
    });
  };

  useEffect(() => {
    resetWindowLoaderScope(
      runtimeRef.current,
      scopeKey,
      scrollRef.current?.scrollTop ?? 0,
    );
    scheduleCheck();
  }, [scheduleCheck, scopeKey, scrollRef]);

  useEffect(() => {
    refreshWindowLoaderContent(runtimeRef.current);
    scheduleCheck();
  }, [revision, scheduleCheck]);

  useEffect(() => {
    scheduleCheck();
  }, [enabled, loadRoundWindow, scheduleCheck]);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) {
      return;
    }

    const runtime = runtimeRef.current;
    updateWindowLoaderScroll(runtime, scrollElement.scrollTop);
    let touchStartY: number | null = null;
    const refreshFromPull = (pullDistance: number) => {
      if (!shouldRefreshWindowLoaderFromPull(
        scrollElement.scrollTop,
        pullDistance,
      )) {
        return;
      }
      refreshWindowLoaderContent(runtime);
      scheduleCheck();
    };
    const handleScroll = () => {
      updateWindowLoaderScroll(runtime, scrollElement.scrollTop);
      scheduleCheck();
    };
    const handleWheel = (event: WheelEvent) => {
      refreshFromPull(-event.deltaY);
    };
    const handleTouchStart = (event: TouchEvent) => {
      touchStartY = scrollElement.scrollTop <= 12
        ? event.touches.item(0)?.clientY ?? null
        : null;
    };
    const handleTouchMove = (event: TouchEvent) => {
      const currentY = event.touches.item(0)?.clientY;
      if (touchStartY === null || currentY === undefined) {
        return;
      }
      refreshFromPull(currentY - touchStartY);
    };
    const handleTouchEnd = () => {
      touchStartY = null;
    };
    scheduleCheck();
    scrollElement.addEventListener("scroll", handleScroll, { passive: true });
    scrollElement.addEventListener("wheel", handleWheel, { passive: true });
    scrollElement.addEventListener("touchstart", handleTouchStart, { passive: true });
    scrollElement.addEventListener("touchmove", handleTouchMove, { passive: true });
    scrollElement.addEventListener("touchend", handleTouchEnd, { passive: true });
    scrollElement.addEventListener("touchcancel", handleTouchEnd, { passive: true });
    window.addEventListener("resize", handleScroll);
    return () => {
      scrollElement.removeEventListener("scroll", handleScroll);
      scrollElement.removeEventListener("wheel", handleWheel);
      scrollElement.removeEventListener("touchstart", handleTouchStart);
      scrollElement.removeEventListener("touchmove", handleTouchMove);
      scrollElement.removeEventListener("touchend", handleTouchEnd);
      scrollElement.removeEventListener("touchcancel", handleTouchEnd);
      window.removeEventListener("resize", handleScroll);
      cancelWindowLoaderRuntime(runtime);
      cancelScheduledCheck(frameRef, timeoutRef);
    };
  }, [scheduleCheck, scrollRef]);

  return {
    isLoading: Boolean(
      enabled
      && loadingRequest
      && loadingRequest.generation === runtimeRef.current.generation
      && runtimeRef.current.scopeKey === scopeKey,
    ),
  };
}

function canStartWindowLoad(
  runtime: WindowLoaderRuntime,
  options: {
    enabled: boolean;
    loadedRoundIds: readonly string[];
    loadRoundWindow?: (roundId: string) => Promise<boolean>;
    scopeKey: string | null;
    windowRoundIds: readonly string[];
  },
): options is {
  enabled: true;
  loadedRoundIds: readonly string[];
  loadRoundWindow: (roundId: string) => Promise<boolean>;
  scopeKey: string | null;
  windowRoundIds: readonly string[];
} {
  return Boolean(
    options.enabled &&
    options.loadRoundWindow &&
    !runtime.activeRequest &&
    runtime.scopeKey === options.scopeKey,
  );
}

async function runWindowLoad({
  loader,
  onSettled,
  request,
  runtime,
  scheduleRetry,
}: {
  loader: (roundId: string) => Promise<boolean>;
  onSettled: () => void;
  request: WindowLoadRequest;
  runtime: RefObject<WindowLoaderRuntime>;
  scheduleRetry: (delay: number) => void;
}): Promise<void> {
  let nextCheckDelay: number | null = LOAD_RECHECK_DELAY_MS;
  try {
    const result = await requestWindowLoad(loader, request.roundId);
    if (!isCurrentWindowLoadRequest(runtime.current, request)) {
      return;
    }
    nextCheckDelay = recordWindowLoadResult(
      runtime.current,
      request,
      result,
      Date.now(),
    );
    if (result.status === "failed") {
      console.warn("加载可见对话轮次失败", {
        error: result.error,
        roundId: request.roundId,
      });
    }
  } finally {
    const currentRuntime = runtime.current;
    if (!isCurrentWindowLoadRequest(currentRuntime, request)) {
      return;
    }
    currentRuntime.activeRequest = null;
    onSettled();
    if (nextCheckDelay !== null) {
      scheduleRetry(nextCheckDelay);
    }
  }
}

async function requestWindowLoad(
  loader: (roundId: string) => Promise<boolean>,
  roundId: string,
): Promise<WindowLoadResult> {
  try {
    return await loader(roundId)
      ? { status: "loaded" }
      : { status: "missing" };
  } catch (error) {
    return { error, status: "failed" };
  }
}

function cancelScheduledCheck(
  frameRef: RefObject<number | null>,
  timeoutRef: RefObject<number | null>,
): void {
  if (frameRef.current !== null) {
    window.cancelAnimationFrame(frameRef.current);
    frameRef.current = null;
  }
  if (timeoutRef.current !== null) {
    window.clearTimeout(timeoutRef.current);
    timeoutRef.current = null;
  }
}
