/**
 * INPUT: 当前会话代次、可见轮次加载结果、滚动方向与用户顶部下拉意图。
 * OUTPUT: 可失效的完成/重试账本，以及决定下一次加载资格的纯状态转换。
 * POS: 索引时间线可见窗口加载器的状态机；React Hook 只负责事件和异步调度。
 */
import type { ScrollDirection } from "./visible-round-candidate";

export const LOAD_RECHECK_DELAY_MS = 80;

const RETRY_DELAYS_MS = [250, 1_000, 3_000] as const;
const PULL_REFRESH_TOP_TOLERANCE_PX = 12;
const PULL_REFRESH_DISTANCE_PX = 32;

export interface WindowLoadRequest {
  generation: number;
  id: number;
  roundId: string;
}

interface RoundAttempt {
  count: number;
  retryAfter: number;
}

export interface WindowLoaderRuntime {
  activeRequest: WindowLoadRequest | null;
  attempts: Map<string, RoundAttempt>;
  completedRoundIds: Set<string>;
  generation: number;
  lastScrollTop: number;
  scopeKey: string | null;
  scrollDirection: ScrollDirection;
}

export type WindowLoadResult =
  | { status: "loaded" }
  | { status: "missing" }
  | { error: unknown; status: "failed" };

export function createWindowLoaderRuntime(): WindowLoaderRuntime {
  return {
    activeRequest: null,
    attempts: new Map(),
    completedRoundIds: new Set(),
    generation: 0,
    lastScrollTop: 0,
    scopeKey: null,
    scrollDirection: "none",
  };
}

export function resetWindowLoaderScope(
  runtime: WindowLoaderRuntime,
  scopeKey: string | null,
  scrollTop: number,
): void {
  runtime.generation += 1;
  runtime.scopeKey = scopeKey;
  runtime.activeRequest = null;
  runtime.attempts.clear();
  runtime.completedRoundIds.clear();
  runtime.lastScrollTop = scrollTop;
  runtime.scrollDirection = "none";
}

export function cancelWindowLoaderRuntime(runtime: WindowLoaderRuntime): void {
  runtime.generation += 1;
  runtime.activeRequest = null;
}

export function createWindowLoadRequest(
  runtime: WindowLoaderRuntime,
  id: number,
  roundId: string,
): WindowLoadRequest {
  return { generation: runtime.generation, id, roundId };
}

export function isCurrentWindowLoadRequest(
  runtime: WindowLoaderRuntime,
  request: WindowLoadRequest,
): boolean {
  return (
    runtime.generation === request.generation &&
    runtime.activeRequest?.id === request.id
  );
}

export function buildExcludedRoundIds(
  runtime: WindowLoaderRuntime,
  now: number,
): Set<string> {
  const excluded = new Set(runtime.completedRoundIds);
  if (runtime.activeRequest) {
    excluded.add(runtime.activeRequest.roundId);
  }
  for (const [roundId, attempt] of runtime.attempts) {
    if (
      attempt.count >= RETRY_DELAYS_MS.length ||
      attempt.retryAfter > now
    ) {
      excluded.add(roundId);
    }
  }
  return excluded;
}

export function recordWindowLoadResult(
  runtime: WindowLoaderRuntime,
  request: WindowLoadRequest,
  result: WindowLoadResult,
  now: number,
): number | null {
  if (result.status === "loaded") {
    runtime.completedRoundIds.add(request.roundId);
    runtime.attempts.delete(request.roundId);
    return LOAD_RECHECK_DELAY_MS;
  }

  const count = (runtime.attempts.get(request.roundId)?.count ?? 0) + 1;
  const retryDelay = RETRY_DELAYS_MS[count - 1] ?? RETRY_DELAYS_MS.at(-1)!;
  runtime.attempts.set(request.roundId, {
    count,
    retryAfter: now + retryDelay,
  });
  return count < RETRY_DELAYS_MS.length ? retryDelay : null;
}

/**
 * 消息驻留窗口变化或用户再次顶部下拉时，旧的 completed/missing 结论都不再
 * 能代表当前 DOM：已成功加载的 round 可能已被浏览器窗口淘汰，失败也应获得
 * 一次新的显式用户重试机会。
 */
export function refreshWindowLoaderContent(
  runtime: WindowLoaderRuntime,
): void {
  runtime.attempts.clear();
  runtime.completedRoundIds.clear();
}

export function shouldRefreshWindowLoaderFromPull(
  scrollTop: number,
  pullDistance: number,
): boolean {
  return (
    Number.isFinite(scrollTop)
    && Number.isFinite(pullDistance)
    && scrollTop <= PULL_REFRESH_TOP_TOLERANCE_PX
    && pullDistance >= PULL_REFRESH_DISTANCE_PX
  );
}

export function updateWindowLoaderScroll(
  runtime: WindowLoaderRuntime,
  scrollTop: number,
): void {
  if (scrollTop !== runtime.lastScrollTop) {
    runtime.scrollDirection = scrollTop > runtime.lastScrollTop ? "down" : "up";
  }
  runtime.lastScrollTop = scrollTop;
}
