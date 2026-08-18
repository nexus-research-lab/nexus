/**
 * INPUT: 当前物理 round 身份与 live 状态。
 * OUTPUT: 整个 live round 内基于真实 DOM 高度只增不减的正文最小高度与测量节点。
 * POS: MessageItem 视图层的流式排版稳定器。
 */
import {
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type RefObject,
} from "react";

const STREAMING_MIN_HEIGHT = 60;

type MessageItemStreamingLayoutOptions = {
  active: boolean;
  layoutScopeKey: string;
};

type MessageItemStreamingLayout = {
  contentAreaRef: RefObject<HTMLDivElement | null>;
  contentAreaStyle: CSSProperties | undefined;
};

type MessageItemStreamingLayoutState = {
  active: boolean;
  layoutScopeKey: string;
  minHeight: number;
};

export function resolveMessageItemStreamingLayoutState(
  current: MessageItemStreamingLayoutState,
  layoutScopeKey: string,
  active: boolean,
): MessageItemStreamingLayoutState {
  if (current.layoutScopeKey !== layoutScopeKey) {
    return {
      active,
      layoutScopeKey,
      minHeight: active ? STREAMING_MIN_HEIGHT : 0,
    };
  }
  if (!active) {
    return current.active || current.minHeight !== 0
      ? { active: false, layoutScopeKey, minHeight: 0 }
      : current;
  }
  if (!current.active || current.minHeight < STREAMING_MIN_HEIGHT) {
    return {
      active: true,
      layoutScopeKey,
      minHeight: Math.max(current.minHeight, STREAMING_MIN_HEIGHT),
    };
  }
  return current;
}

export function useMessageItemStreamingLayout({
  active,
  layoutScopeKey,
}: MessageItemStreamingLayoutOptions): MessageItemStreamingLayout {
  const contentAreaRef = useRef<HTMLDivElement>(null);
  const [streamingLayoutState, setStreamingLayoutState] = useState<MessageItemStreamingLayoutState>({
    active,
    layoutScopeKey,
    minHeight: active ? STREAMING_MIN_HEIGHT : 0,
  });
  const renderedLayoutState = resolveMessageItemStreamingLayoutState(
    streamingLayoutState,
    layoutScopeKey,
    active,
  );

  useLayoutEffect(() => {
    setStreamingLayoutState((current) => (
      resolveMessageItemStreamingLayoutState(current, layoutScopeKey, active)
    ));
    const element = contentAreaRef.current;
    if (!active || !element || typeof ResizeObserver === "undefined") {
      return;
    }

    const retainMeasuredHeight = () => {
      const measuredHeight = Math.ceil(element.getBoundingClientRect().height);
      if (measuredHeight <= 0) {
        return;
      }
      setStreamingLayoutState((current) => {
        const normalized = resolveMessageItemStreamingLayoutState(
          current,
          layoutScopeKey,
          true,
        );
        if (normalized.minHeight >= measuredHeight) {
          return normalized;
        }
        return { ...normalized, minHeight: measuredHeight };
      });
    };
    const observer = new ResizeObserver(retainMeasuredHeight);
    observer.observe(element);
    retainMeasuredHeight();
    return () => observer.disconnect();
  }, [active, layoutScopeKey]);

  return {
    contentAreaRef,
    contentAreaStyle: renderedLayoutState.active
      ? { minHeight: renderedLayoutState.minHeight }
      : undefined,
  };
}
