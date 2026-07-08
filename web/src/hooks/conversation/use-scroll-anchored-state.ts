/**
 * useScrollAnchoredState
 *
 * A boolean state hook that preserves the scroll container's
 * distance-from-bottom across state changes. Prevents visible
 * jitter when expanding/collapsing content near the bottom of
 * a scrollable feed.
 *
 * How it works:
 * 1. On toggle: snapshot (scrollHeight - scrollTop) before React commits.
 * 2. useLayoutEffect (before paint): restore scrollTop so that
 *    distance-from-bottom stays the same.
 */

import { type Dispatch, type SetStateAction, useCallback, useLayoutEffect, useRef, useState } from "react";

/**
 * Find the nearest scrollable ancestor of `el`.
 * Returns null if none found (unlikely in a chat UI).
 */
function findScrollContainer(el: HTMLElement | null): HTMLElement | null {
  let node = el?.parentElement ?? null;
  while (node) {
    const { overflowY } = getComputedStyle(node);
    if (overflowY === "auto" || overflowY === "scroll") {
      return node;
    }
    node = node.parentElement;
  }
  return null;
}

interface UseScrollAnchoredStateReturn {
  isOpen: boolean;
  /** Toggle with scroll anchoring — use for user-initiated expand/collapse. */
  toggle: () => void;
  /** Direct setter without scroll anchoring — use for programmatic changes (e.g. auto-expand on loading). */
  setOpen: Dispatch<SetStateAction<boolean>>;
  /** Ref to attach to a DOM element inside the scrollable area. */
  anchorRef: React.RefObject<HTMLElement | null>;
}

export function useScrollAnchoredState(
  initialValue: boolean,
): UseScrollAnchoredStateReturn {
  const [isOpen, setOpen] = useState(initialValue);
  const anchorRef = useRef<HTMLElement | null>(null);

  // Snapshot: distance from bottom before toggle
  const snapshotRef = useRef<{
    distance_from_bottom: number;
    container: HTMLElement;
  } | null>(null);

  const toggle = useCallback(() => {
    const container = findScrollContainer(anchorRef.current);
    if (container) {
      snapshotRef.current = {
        distance_from_bottom:
          container.scrollHeight - container.scrollTop,
        container,
      };
    }
    setOpen((prev) => !prev);
  }, []);

  useLayoutEffect(() => {
    const snapshot = snapshotRef.current;
    if (!snapshot) return;
    snapshotRef.current = null;

    const { container, distance_from_bottom: distanceFromBottom } = snapshot;
    const newScrollTop = container.scrollHeight - distanceFromBottom;
    if (Math.abs(container.scrollTop - newScrollTop) > 1) {
      container.scrollTop = newScrollTop;
    }
  }, [isOpen]);

  return { isOpen: isOpen, toggle, setOpen: setOpen, anchorRef: anchorRef };
}
