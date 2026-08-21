/**
 * INPUT: Composer 输入壳、当前人工介入 epoch 与 Session 草稿作用域。
 * OUTPUT: 同一权限/问答队列内单调不减的外壳高度，并在恢复输入框时原子释放高度负债。
 * POS: Composer 人工介入替换面的几何稳定层；内容继续在既有 max-height 内独立滚动。
 */
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  type RefObject,
} from "react";

const HEIGHT_TOLERANCE_PX = 0.5;

interface ComposerInteractionHeightState {
  minimumHeight: number;
  scopeKey: string;
  wasActive: boolean;
}

interface ComposerInteractionHeightInput {
  active: boolean;
  measuredHeight: number;
  scopeKey: string;
}

export interface ComposerInteractionHeightRevision {
  minimumHeight: number;
  releasing: boolean;
  state: ComposerInteractionHeightState;
}

export function resolveComposerInteractionHeightGuard(
  {
    active,
    measuredHeight,
    scopeKey,
  }: ComposerInteractionHeightInput,
  previous: ComposerInteractionHeightState,
): ComposerInteractionHeightRevision {
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

export function useComposerInteractionHeightGuard({
  active,
  elementRef,
  scopeKey,
}: {
  active: boolean;
  elementRef: RefObject<HTMLDivElement | null>;
  scopeKey: string;
}): void {
  const activeRef = useRef(active);
  const guardedElementRef = useRef<HTMLDivElement | null>(null);
  const stateRef = useRef<ComposerInteractionHeightState>({
    minimumHeight: 0,
    scopeKey,
    wasActive: false,
  });
  activeRef.current = active;

  const applyRevision = useCallback((element: HTMLDivElement) => {
    if (stateRef.current.scopeKey !== scopeKey) {
      const previousElement = guardedElementRef.current;
      if (previousElement) {
        clearComposerInteractionHeightStyle(previousElement);
      }
      clearComposerInteractionHeightStyle(element);
      stateRef.current = {
        minimumHeight: 0,
        scopeKey,
        wasActive: false,
      };
    }

    const previous = stateRef.current;
    let measuredHeight = element.getBoundingClientRect().height;
    if (!activeRef.current && previous.wasActive) {
      clearComposerInteractionHeightStyle(element);
      measuredHeight = element.getBoundingClientRect().height;
    }
    const next = resolveComposerInteractionHeightGuard({
      active: activeRef.current,
      measuredHeight,
      scopeKey,
    }, previous);
    stateRef.current = next.state;
    guardedElementRef.current = element;

    if (activeRef.current) {
      element.style.transition = "none";
      element.style.minHeight = `${next.minimumHeight}px`;
      element.dataset.composerInteractionHeightGuard = "active";
      return;
    }
    if (!next.releasing) {
      clearComposerInteractionHeightStyle(element);
      return;
    }

    // 一次性提交 intrinsic 高度，避免 transition 每帧触发外层 viewport 重算。
    clearComposerInteractionHeightStyle(element);
  }, [scopeKey]);

  useLayoutEffect(() => {
    const element = elementRef.current;
    if (element) {
      applyRevision(element);
    }
  }, [active, applyRevision, elementRef]);

  useEffect(() => {
    const element = elementRef.current;
    if (
      !element
      || !active
      || typeof ResizeObserver === "undefined"
    ) {
      return;
    }
    const observer = new ResizeObserver(() => applyRevision(element));
    observer.observe(element);
    return () => observer.disconnect();
  }, [active, applyRevision, elementRef]);

  useEffect(() => () => {
    const element = guardedElementRef.current;
    if (element) {
      clearComposerInteractionHeightStyle(element);
    }
  }, []);
}

function clearComposerInteractionHeightStyle(element: HTMLDivElement): void {
  element.style.removeProperty("min-height");
  element.style.removeProperty("transition");
  delete element.dataset.composerInteractionHeightGuard;
}
