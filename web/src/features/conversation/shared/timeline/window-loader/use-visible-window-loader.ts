import {
  useCallback,
  useEffect,
  useRef,
  type RefObject,
} from "react";

import { getConversationRoundNavigationTarget } from "../scroll/round-scroll";
import { resolveVisibleUnloadedRoundId } from "./visible-round-candidate";
import {
  LOAD_RECHECK_DELAY_MS,
  buildExcludedRoundIds,
  cancelWindowLoaderRuntime,
  clearWindowLoadAttempts,
  createWindowLoaderRuntime,
  createWindowLoadRequest,
  isCurrentWindowLoadRequest,
  recordWindowLoadResult,
  resetWindowLoaderScope,
  updateWindowLoaderScroll,
  type WindowLoaderRuntime,
  type WindowLoadRequest,
  type WindowLoadResult,
} from "./window-loader-runtime";

interface UseVisibleRoundWindowLoaderOptions {
  enabled: boolean;
  loadRoundWindow?: (roundId: string) => Promise<boolean>;
  revision: string | number;
  scopeKey: string | null;
  scrollRef: RefObject<HTMLDivElement | null>;
}

export function useVisibleRoundWindowLoader({
  enabled,
  loadRoundWindow,
  revision,
  scopeKey,
  scrollRef,
}: UseVisibleRoundWindowLoaderOptions) {
  const frameRef = useRef<number | null>(null);
  const requestSequenceRef = useRef(0);
  const runCheckRef = useRef<() => void>(() => {});
  const runtimeRef = useRef(createWindowLoaderRuntime());
  const timeoutRef = useRef<number | null>(null);
  const latestOptionsRef = useRef({ enabled, loadRoundWindow, scopeKey });
  latestOptionsRef.current = { enabled, loadRoundWindow, scopeKey };

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
    const roundId = resolveVisibleUnloadedRoundId(
      scrollElement,
      buildExcludedRoundIds(runtime, Date.now()),
      runtime.scrollDirection,
    );
    if (!roundId) {
      return;
    }

    const request = createWindowLoadRequest(
      runtime,
      ++requestSequenceRef.current,
      roundId,
    );
    runtime.activeRequest = request;
    void runWindowLoad({
      loader: latest.loadRoundWindow!,
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
    clearWindowLoadAttempts(runtimeRef.current);
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
    const handleScroll = () => {
      updateWindowLoaderScroll(runtime, scrollElement.scrollTop);
      scheduleCheck();
    };
    scheduleCheck();
    scrollElement.addEventListener("scroll", handleScroll, { passive: true });
    window.addEventListener("resize", handleScroll);
    return () => {
      scrollElement.removeEventListener("scroll", handleScroll);
      window.removeEventListener("resize", handleScroll);
      cancelWindowLoaderRuntime(runtime);
      cancelScheduledCheck(frameRef, timeoutRef);
    };
  }, [scheduleCheck, scrollRef]);
}

function canStartWindowLoad(
  runtime: WindowLoaderRuntime,
  options: {
    enabled: boolean;
    loadRoundWindow?: (roundId: string) => Promise<boolean>;
    scopeKey: string | null;
  },
): options is {
  enabled: true;
  loadRoundWindow: (roundId: string) => Promise<boolean>;
  scopeKey: string | null;
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
  request,
  runtime,
  scheduleRetry,
}: {
  loader: (roundId: string) => Promise<boolean>;
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
