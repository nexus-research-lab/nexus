import {
  useCallback,
  useLayoutEffect,
  useRef,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";

interface ScrollAnchorSnapshot {
  container: HTMLElement;
  distanceFromBottom: number;
}

interface UseScrollAnchoredStateReturn {
  isOpen: boolean;
  toggle: () => void;
  setOpen: Dispatch<SetStateAction<boolean>>;
  anchorRef: React.RefObject<HTMLElement | null>;
}

/**
 * 局部内容展开或收起时保持当前视觉锚点，避免底部附近的内容发生跳动。
 * 程序驱动的状态变化不自动锚定，由调用方通过 setOpen 明确控制。
 */
export function useScrollAnchoredState(
  initialValue: boolean,
): UseScrollAnchoredStateReturn {
  const [isOpen, setOpen] = useState(initialValue);
  const anchorRef = useRef<HTMLElement | null>(null);
  const snapshotRef = useRef<ScrollAnchorSnapshot | null>(null);

  const toggle = useCallback(() => {
    const container = findScrollContainer(anchorRef.current);
    if (container) {
      snapshotRef.current = {
        container,
        distanceFromBottom: container.scrollHeight - container.scrollTop,
      };
    }
    setOpen((current) => !current);
  }, []);

  useLayoutEffect(() => {
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
