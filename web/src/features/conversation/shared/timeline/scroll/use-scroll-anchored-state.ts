import {
  useCallback,
  useLayoutEffect,
  useRef,
  type Dispatch,
  type SetStateAction,
} from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";

import { notifyConversationExplicitShrink } from "./conversation-layout-events";

interface ScrollAnchorSnapshot {
  container: HTMLElement;
  distanceFromBottom: number;
}

interface PendingCollapseSnapshot {
  anchor: HTMLElement;
  height: number;
}

interface UseScrollAnchoredStateReturn {
  isOpen: boolean;
  toggle: () => void;
  setOpen: Dispatch<SetStateAction<boolean>>;
  anchorRef: React.RefObject<HTMLElement | null>;
}

/**
 * 局部内容展开或收起时保持当前视觉锚点，避免底部附近的内容发生跳动。
 * resetKey 变化时按新阶段恢复 initialValue，不把旧阶段的选择带入新阶段。
 * 程序驱动的状态变化不自动锚定，由调用方通过 setOpen 明确控制。
 */
export function useScrollAnchoredState(
  initialValue: boolean,
  resetKey?: unknown,
): UseScrollAnchoredStateReturn {
  const [isOpen, setOpen] = useResettableState(initialValue, resetKey);
  const anchorRef = useRef<HTMLElement | null>(null);
  const snapshotRef = useRef<ScrollAnchorSnapshot | null>(null);
  const pendingCollapseRef = useRef<PendingCollapseSnapshot | null>(null);

  const toggle = useCallback(() => {
    const anchor = anchorRef.current;
    const container = findScrollContainer(anchor);
    if (
      container
      && !anchor?.closest('[data-conversation-virtual-feed="true"]')
    ) {
      snapshotRef.current = {
        container,
        distanceFromBottom: container.scrollHeight - container.scrollTop,
      };
    } else {
      // Virtualizer 已按 item 测高维护视口。这里再写一次 scrollTop 会把同一
      // 展开/收起 delta 计算两遍，表现为先位移再弹回。
      snapshotRef.current = null;
    }
    pendingCollapseRef.current = isOpen && anchor
      ? {
          anchor,
          height: anchor.getBoundingClientRect().height,
        }
      : null;
    setOpen(!isOpen);
  }, [isOpen, setOpen]);

  useLayoutEffect(() => {
    const pendingCollapse = pendingCollapseRef.current;
    if (pendingCollapse) {
      pendingCollapseRef.current = null;
      const anchor = anchorRef.current ?? pendingCollapse.anchor;
      notifyConversationExplicitShrink(
        anchor,
        pendingCollapse.height - anchor.getBoundingClientRect().height,
      );
    }

    const snapshot = snapshotRef.current;
    if (!snapshot) {
      return;
    }
    snapshotRef.current = null;

    const nextScrollTop =
      snapshot.container.scrollHeight - snapshot.distanceFromBottom;
    if (Math.abs(snapshot.container.scrollTop - nextScrollTop) > 1) {
      snapshot.container.scrollTop = nextScrollTop;
    }
  }, [isOpen]);

  return { anchorRef, isOpen, setOpen, toggle };
}

function findScrollContainer(element: HTMLElement | null): HTMLElement | null {
  let candidate = element?.parentElement ?? null;
  while (candidate) {
    const { overflowY } = getComputedStyle(candidate);
    if (overflowY === "auto" || overflowY === "scroll") {
      return candidate;
    }
    candidate = candidate.parentElement;
  }
  return null;
}
