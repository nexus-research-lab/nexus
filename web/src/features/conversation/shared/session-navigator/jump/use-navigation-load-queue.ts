import {
  useCallback,
  useEffect,
  useRef,
  type RefObject,
} from "react";

import type { PendingRoundJump } from "./round-jump-model";

interface NavigationLoadRequest extends PendingRoundJump {
  generation: number;
  id: number;
}

interface NavigationLoadRuntime {
  activeRequest: NavigationLoadRequest | null;
  generation: number;
  queuedTarget: PendingRoundJump | null;
  scopeKey: string | null;
}

type NavigationLoadResult =
  | { status: "loaded" }
  | { status: "missing" }
  | { error: unknown; status: "failed" };

interface UseNavigationLoadQueueOptions {
  cancelNavigation: (target: PendingRoundJump) => void;
  loadRoundWindow?: (roundId: string) => Promise<boolean>;
  scopeKey: string | null;
}

export function useNavigationLoadQueue({
  cancelNavigation,
  loadRoundWindow,
  scopeKey,
}: UseNavigationLoadQueueOptions) {
  const loadSequenceRef = useRef(0);
  const runtimeRef = useRef<NavigationLoadRuntime>({
    activeRequest: null,
    generation: 0,
    queuedTarget: null,
    scopeKey: null,
  });
  const drainLoadQueueRef = useRef<() => void>(() => {});
  const latestCancelRef = useRef(cancelNavigation);
  const latestLoaderRef = useRef(loadRoundWindow);
  latestCancelRef.current = cancelNavigation;
  latestLoaderRef.current = loadRoundWindow;

  drainLoadQueueRef.current = () => {
    const runtime = runtimeRef.current;
    const target = runtime.queuedTarget;
    const loader = latestLoaderRef.current;
    if (!target || !loader || runtime.activeRequest) {
      return;
    }

    runtime.queuedTarget = null;
    const request: NavigationLoadRequest = {
      ...target,
      generation: runtime.generation,
      id: ++loadSequenceRef.current,
    };
    runtime.activeRequest = request;
    void runNavigationLoad({
      cancelNavigation: latestCancelRef,
      drainLoadQueue: drainLoadQueueRef,
      loader,
      request,
      runtime: runtimeRef,
    });
  };

  useEffect(() => {
    const runtime = runtimeRef.current;
    runtime.generation += 1;
    runtime.scopeKey = scopeKey;
    runtime.activeRequest = null;
    runtime.queuedTarget = null;
    return () => {
      runtime.generation += 1;
      runtime.activeRequest = null;
      runtime.queuedTarget = null;
    };
  }, [scopeKey]);

  return useCallback(
    (target: PendingRoundJump): boolean => {
      const runtime = runtimeRef.current;
      if (!loadRoundWindow || runtime.scopeKey !== target.scopeKey) {
        return false;
      }
      runtime.queuedTarget = target;
      drainLoadQueueRef.current();
      return true;
    },
    [loadRoundWindow],
  );
}

async function runNavigationLoad({
  cancelNavigation,
  drainLoadQueue,
  loader,
  request,
  runtime,
}: {
  cancelNavigation: RefObject<(target: PendingRoundJump) => void>;
  drainLoadQueue: RefObject<() => void>;
  loader: (roundId: string) => Promise<boolean>;
  request: NavigationLoadRequest;
  runtime: RefObject<NavigationLoadRuntime>;
}): Promise<void> {
  try {
    const result = await requestRoundWindow(loader, request.scrollRoundId);
    if (!isCurrentRequest(runtime.current, request)) {
      return;
    }
    if (result.status === "failed") {
      console.warn("加载会话导航轮次失败", {
        error: result.error,
        roundId: request.scrollRoundId,
      });
    }
    if (result.status !== "loaded") {
      cancelNavigation.current(request);
    }
  } finally {
    const currentRuntime = runtime.current;
    if (!isCurrentRequest(currentRuntime, request)) {
      return;
    }
    currentRuntime.activeRequest = null;
    drainLoadQueue.current();
  }
}

async function requestRoundWindow(
  loader: (roundId: string) => Promise<boolean>,
  roundId: string,
): Promise<NavigationLoadResult> {
  try {
    return await loader(roundId)
      ? { status: "loaded" }
      : { status: "missing" };
  } catch (error) {
    return { error, status: "failed" };
  }
}

function isCurrentRequest(
  runtime: NavigationLoadRuntime,
  request: NavigationLoadRequest,
): boolean {
  return (
    runtime.generation === request.generation &&
    runtime.scopeKey === request.scopeKey &&
    runtime.activeRequest?.id === request.id
  );
}
