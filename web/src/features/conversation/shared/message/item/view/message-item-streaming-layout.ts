import { prepare, layout } from "@chenglou/pretext";
import { useEffect, useRef, type CSSProperties, type RefObject } from "react";

import type { ContentBlock } from "@/types/conversation/message";

import { extractTextFromContentBlocks } from "../message-item-support";

const STREAMING_MIN_HEIGHT = 60;
const STREAMING_LAYOUT_DELAY_MS = 150;
const STREAMING_PROSE_FONT =
  "400 14px ui-sans-serif, system-ui, sans-serif";
const STREAMING_LINE_HEIGHT = 28;

type MessageItemStreamingLayoutOptions = {
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

export function useMessageItemStreamingLayout({
  assistantContentMode,
  directContent,
  finalAssistantText,
  showCursor,
}: MessageItemStreamingLayoutOptions): MessageItemStreamingLayout {
  const contentAreaRef = useRef<HTMLDivElement>(null);
  const streamingMinHeight = useRef(STREAMING_MIN_HEIGHT);
  const layoutThrottleRef = useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );

  useEffect(() => {
    const layoutText =
      assistantContentMode === "dm_live" ||
      assistantContentMode === "room_thread"
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
      if (!element) {
        return;
      }
      try {
        const width = element.offsetWidth || 640;
        const prepared = prepare(layoutText, STREAMING_PROSE_FONT);
        const result = layout(prepared, width, STREAMING_LINE_HEIGHT);
        streamingMinHeight.current = Math.max(
          streamingMinHeight.current,
          result.height,
        );
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
    directContent,
    finalAssistantText,
    showCursor,
  ]);

  useEffect(() => {
    if (!showCursor) {
      streamingMinHeight.current = STREAMING_MIN_HEIGHT;
      if (layoutThrottleRef.current !== null) {
        clearTimeout(layoutThrottleRef.current);
        layoutThrottleRef.current = null;
      }
    }
  }, [showCursor]);

  return {
    contentAreaRef,
    contentAreaStyle: showCursor
      ? { minHeight: streamingMinHeight.current }
      : undefined,
  };
}
