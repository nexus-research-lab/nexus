/**
 * INPUT: 滚动/Feed 容器、驻留 round 顺序、窗口断点和四个历史窗口命令。
 * OUTPUT: 按阅读方向扩展连续窗口的 React 调度，以及对外的加载状态。
 * POS: 会话历史的唯一自动加载入口；同一时刻只有一个边界请求，不直接写 scrollTop。
 */
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type RefObject,
} from "react";

import {
  CONVERSATION_ROUND_SELECTOR,
  getConversationRoundNavigationTarget,
} from "../scroll/round-scroll";
import {
  resolveWindowEdgeLoad,
  resolveWindowEvictionEdge,
  WINDOW_EDGE_COOLDOWN_MS,
  type WindowEdgeLoad,
} from "./continuous-window-model";

interface UseContinuousWindowLoaderOptions {
  enabled: boolean;
  feedRef: RefObject<HTMLDivElement | null>;
  /** 断点前最后一个驻留 round；null 表示窗口连到最新。 */
  gapAfterRoundId: string | null;
  /** 驻留窗口之前是否还有历史；到达会话起点后不再向前请求。 */
  hasOlderHistory: boolean;
  /** 程序化回到底部（含首屏定位）仍在进行；此时的 scrollTop 不代表阅读位置。 */
  isBottomScrollActive: () => boolean;
  isFollowingLatest: () => boolean;
  isSessionLoading: boolean;
  loadNewerMessages: () => Promise<boolean>;
  loadOlderMessages: () => Promise<boolean>;
  returnToLatestMessages: () => Promise<boolean>;
  /** 驻留内容每次提交后变化的代次，用于在新布局上重新评估边界。 */
  revision: string;
  scopeKey: string | null;
  scrollRef: RefObject<HTMLDivElement | null>;
  setEvictionEdge: (edge: "head" | "tail") => void;
  /** 窗口变更请求在途时，让滚动所有者在窗口起点也保留阅读锚点。 */
  setWindowChangePending: (pending: boolean) => void;
  windowRoundIds: string[];
}

interface WindowLoaderRuntime {
  cooldownUntil: Record<"newer" | "older", number>;
  direction: "up" | "down";
  frame: number | null;
  generation: number;
  inFlight: boolean;
  lastScrollTop: number;
}

function createRuntime(generation: number): WindowLoaderRuntime {
  return {
    cooldownUntil: { newer: 0, older: 0 },
    direction: "up",
    frame: null,
    generation,
    inFlight: false,
    lastScrollTop: 0,
  };
}

export function useContinuousWindowLoader(
  options: UseContinuousWindowLoaderOptions,
): { isLoading: boolean } {
  const { enabled, revision, scopeKey, scrollRef, setWindowChangePending } = options;
  const [isLoading, setIsLoading] = useState(false);
  const runtimeRef = useRef<WindowLoaderRuntime>(createRuntime(0));
  const latestRef = useRef(options);
  latestRef.current = options;

  const check = useCallback(() => {
    const runtime = runtimeRef.current;
    const latest = latestRef.current;
    const container = latest.scrollRef.current;
    runtime.frame = null;
    if (!latest.enabled || !container) {
      return;
    }
    if (runtime.inFlight || latest.isSessionLoading) {
      return;
    }
    if (
      getConversationRoundNavigationTarget(container)
      || latest.isBottomScrollActive()
    ) {
      // 导航定位和回到底部事务结束时不一定再有 scroll 事件，逐帧等待它们交还所有权；
      // 否则跳转页末尾的断点要等到下一次滚动才会被确认或接上最新。
      runtime.frame = requestAnimationFrame(check);
      return;
    }
    latest.setEvictionEdge(resolveWindowEvictionEdge({
      clientHeight: container.clientHeight,
      followingLatest: latest.isFollowingLatest(),
      scrollHeight: container.scrollHeight,
      scrollTop: container.scrollTop,
    }));

    const now = performance.now();
    const visible = latest.gapAfterRoundId === null
      ? NO_VISIBLE_ROUNDS
      : resolveVisibleRoundIndexes(
        container,
        latest.feedRef.current,
        latest.windowRoundIds,
      );
    const edge = resolveWindowEdgeLoad({
      clientHeight: container.clientHeight,
      direction: runtime.direction,
      firstVisibleIndex: visible.first,
      gapIndex: latest.gapAfterRoundId === null
        ? null
        : latest.windowRoundIds.indexOf(latest.gapAfterRoundId),
      lastVisibleIndex: visible.last,
      newerUnavailable: now < runtime.cooldownUntil.newer,
      olderUnavailable: (
        !latest.hasOlderHistory || now < runtime.cooldownUntil.older
      ),
      scrollHeight: container.scrollHeight,
      scrollTop: container.scrollTop,
    });
    if (!edge) {
      return;
    }

    const { generation } = runtime;
    runtime.inFlight = true;
    setIsLoading(true);
    latest.setWindowChangePending(true);
    void runEdgeLoad(edge, latest).then((progressed) => {
      if (runtimeRef.current.generation !== generation) {
        return;
      }
      runtime.inFlight = false;
      setIsLoading(false);
      if (!progressed) {
        // 没有提交就没有布局变化：立即释放锚点并让该端冷却，等待下一次滚动。
        latestRef.current.setWindowChangePending(false);
        runtime.cooldownUntil[edge === "older" ? "older" : "newer"] = (
          performance.now() + WINDOW_EDGE_COOLDOWN_MS
        );
      }
    });
  }, []);

  const scheduleCheck = useCallback(() => {
    const runtime = runtimeRef.current;
    if (runtime.frame === null) {
      runtime.frame = requestAnimationFrame(check);
    }
  }, [check]);

  // 会话切换时旧请求的完成不得影响新会话的互斥、冷却或锚点状态。
  useLayoutEffect(() => {
    const previous = runtimeRef.current;
    if (previous.frame !== null) {
      cancelAnimationFrame(previous.frame);
    }
    runtimeRef.current = createRuntime(previous.generation + 1);
    setIsLoading(false);
    setWindowChangePending(false);
    return () => {
      const current = runtimeRef.current;
      if (current.frame !== null) {
        cancelAnimationFrame(current.frame);
        current.frame = null;
      }
    };
  }, [scopeKey, setWindowChangePending]);

  // 窗口提交已由滚动所有者在本次 layout 阶段恢复锚点，之后才释放并评估下一页。
  useLayoutEffect(() => {
    if (!runtimeRef.current.inFlight) {
      setWindowChangePending(false);
    }
    if (enabled) {
      scheduleCheck();
    }
  }, [enabled, isLoading, revision, scheduleCheck, setWindowChangePending]);

  useEffect(() => {
    const container = scrollRef.current;
    if (!enabled || !container) {
      return;
    }
    const handleScroll = () => {
      const runtime = runtimeRef.current;
      const { scrollTop } = container;
      if (scrollTop !== runtime.lastScrollTop) {
        runtime.direction = scrollTop < runtime.lastScrollTop ? "up" : "down";
        runtime.lastScrollTop = scrollTop;
      }
      scheduleCheck();
    };
    container.addEventListener("scroll", handleScroll, { passive: true });
    return () => container.removeEventListener("scroll", handleScroll);
  }, [enabled, scheduleCheck, scopeKey, scrollRef]);

  return { isLoading };
}

function runEdgeLoad(
  edge: WindowEdgeLoad,
  options: UseContinuousWindowLoaderOptions,
): Promise<boolean> {
  const load = edge === "older"
    ? options.loadOlderMessages
    : edge === "newer"
      ? options.loadNewerMessages
      : options.returnToLatestMessages;
  // 命令自身已记录并上报失败；这里只需要把异常收敛为“没有进展”。
  return load().catch(() => false);
}

const NO_VISIBLE_ROUNDS = { first: -1, last: -1 } as const;

function resolveVisibleRoundIndexes(
  container: HTMLDivElement,
  feed: HTMLDivElement | null,
  windowRoundIds: string[],
): { first: number; last: number } {
  if (!feed) {
    return NO_VISIBLE_ROUNDS;
  }
  const viewport = container.getBoundingClientRect();
  let first = -1;
  let last = -1;
  for (const round of feed.querySelectorAll<HTMLElement>(
    CONVERSATION_ROUND_SELECTOR,
  )) {
    const bounds = round.getBoundingClientRect();
    if (bounds.bottom <= viewport.top) {
      continue;
    }
    if (bounds.top >= viewport.bottom) {
      break;
    }
    const index = windowRoundIds.indexOf(
      round.dataset.conversationRootRoundId
      ?? round.dataset.conversationRoundId
      ?? "",
    );
    if (index < 0) {
      continue;
    }
    first = first < 0 ? index : first;
    last = index;
  }
  return { first, last };
}
