/**
 * INPUT: Assistant 正文 surface、当前 Assistant turn、可见文本与流式游标状态。
 * OUTPUT: 同一 turn 内只增不减、跨 turn 立即复位的正文最小高度与测量节点。
 * POS: MessageItem 视图层的流式排版稳定器。
 */
import { prepare, layout } from "@chenglou/pretext";
import { useEffect, useRef, type CSSProperties, type RefObject } from "react";

import type { ContentBlock } from "@/types/conversation/message/content";

import { extractTextFromContentBlocks } from "../../message-content-model";
import { resolveAssistantResponseSurface } from "../message-item-projection";

const STREAMING_MIN_HEIGHT = 60;
const STREAMING_LAYOUT_DELAY_MS = 150;
const STREAMING_PROSE_FONT =
  '400 16px "KingHwaOldSong", "Source Han Serif SC", "Songti SC", serif';
const STREAMING_LINE_HEIGHT = 28;

type MessageItemStreamingLayoutOptions = {
  assistantTurnKey: string | null;
  assistantContentMode:
    | "dm_live"
    | "dm_archived"
    | "room_thread"
    | "room_result";
  directContent: ContentBlock[];
  finalAssistantText: string;
  showCursor: boolean;
};

type MessageItemStreamingLayout = {
  contentAreaRef: RefObject<HTMLDivElement | null>;
  contentAreaStyle: CSSProperties | undefined;
};

type MessageItemStreamingLayoutState = {
  active: boolean;
  assistantTurnKey: string | null;
  minHeight: number;
};

export function resolveMessageItemStreamingLayoutState(
  current: MessageItemStreamingLayoutState,
  assistantTurnKey: string | null,
  showCursor: boolean,
): MessageItemStreamingLayoutState {
  if (
    !showCursor
    || !current.active
    || current.assistantTurnKey !== assistantTurnKey
  ) {
    return {
      active: showCursor,
      assistantTurnKey,
      minHeight: STREAMING_MIN_HEIGHT,
    };
  }
  return current;
}

export function useMessageItemStreamingLayout({
  assistantTurnKey,
  assistantContentMode,
  directContent,
  finalAssistantText,
  showCursor,
}: MessageItemStreamingLayoutOptions): MessageItemStreamingLayout {
  const contentAreaRef = useRef<HTMLDivElement>(null);
  const streamingLayoutState = useRef<MessageItemStreamingLayoutState>({
    active: showCursor,
    assistantTurnKey,
    minHeight: STREAMING_MIN_HEIGHT,
  });
  const renderedLayoutState = resolveMessageItemStreamingLayoutState(
    streamingLayoutState.current,
    assistantTurnKey,
    showCursor,
  );
  const layoutThrottleRef = useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );

  useEffect(() => {
    streamingLayoutState.current = resolveMessageItemStreamingLayoutState(
      streamingLayoutState.current,
      assistantTurnKey,
      showCursor,
    );
    const layoutText = resolveAssistantResponseSurface(assistantContentMode)
      === "direct"
      ? extractTextFromContentBlocks(directContent)
      : finalAssistantText;

    if (!showCursor || !layoutText) {
      return;
    }
    if (layoutThrottleRef.current !== null) {
      return;
    }

    layoutThrottleRef.current = setTimeout(() => {
      layoutThrottleRef.current = null;
      const element = contentAreaRef.current;
      const currentLayoutState = streamingLayoutState.current;
      if (
        !element
        || !currentLayoutState.active
        || currentLayoutState.assistantTurnKey !== assistantTurnKey
      ) {
        return;
      }
      try {
        const width = element.offsetWidth || 640;
        const prepared = prepare(layoutText, STREAMING_PROSE_FONT);
        const result = layout(prepared, width, STREAMING_LINE_HEIGHT);
        streamingLayoutState.current = {
          ...currentLayoutState,
          minHeight: Math.max(currentLayoutState.minHeight, result.height),
        };
      } catch {
        // 这里只保留上一次可用高度，避免流式阶段因为排版测量失败产生闪动。
      }
    }, STREAMING_LAYOUT_DELAY_MS);

    return () => {
      if (layoutThrottleRef.current !== null) {
        clearTimeout(layoutThrottleRef.current);
        layoutThrottleRef.current = null;
      }
    };
  }, [
    assistantContentMode,
    assistantTurnKey,
    directContent,
    finalAssistantText,
    showCursor,
  ]);

  return {
    contentAreaRef,
    contentAreaStyle: showCursor
      ? { minHeight: renderedLayoutState.minHeight }
      : undefined,
  };
}
