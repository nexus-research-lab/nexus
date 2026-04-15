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
function find_scroll_container(el: HTMLElement | null): HTMLElement | null {
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
  is_open: boolean;
  /** Toggle with scroll anchoring — use for user-initiated expand/collapse. */
  toggle: () => void;
  /** Direct setter without scroll anchoring — use for programmatic changes (e.g. auto-expand on loading). */
  set_open: Dispatch<SetStateAction<boolean>>;
  /** Ref to attach to a DOM element inside the scrollable area. */
  anchor_ref: React.RefObject<HTMLElement | null>;
}

export function useScrollAnchoredState(
  initial_value: boolean,
): UseScrollAnchoredStateReturn {
  const [is_open, set_open] = useState(initial_value);
  const anchor_ref = useRef<HTMLElement | null>(null);

  // Snapshot: distance from bottom before toggle
  const snapshot_ref = useRef<{
    distance_from_bottom: number;
    container: HTMLElement;
  } | null>(null);

  const toggle = useCallback(() => {
    const container = find_scroll_container(anchor_ref.current);
    if (container) {
      snapshot_ref.current = {
        distance_from_bottom:
          container.scrollHeight - container.scrollTop,
        container,
      };
    }
    set_open((prev) => !prev);
  }, []);

  useLayoutEffect(() => {
    const snapshot = snapshot_ref.current;
    if (!snapshot) return;
    snapshot_ref.current = null;

    const { container, distance_from_bottom } = snapshot;
    const new_scroll_top = container.scrollHeight - distance_from_bottom;
    if (Math.abs(container.scrollTop - new_scroll_top) > 1) {
      container.scrollTop = new_scroll_top;
    }
  }, [is_open]);

  return { is_open, toggle, set_open, anchor_ref };
}
