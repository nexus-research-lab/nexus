import { prepare, layout } from "@chenglou/pretext";
import { useEffect, useRef, type CSSProperties, type RefObject } from "react";

import type { ContentBlock } from "@/types/conversation/message";

import { extract_text_from_content_blocks } from "./message-item-support";

const STREAMING_MIN_HEIGHT = 60;
const STREAMING_LAYOUT_DELAY_MS = 150;
const STREAMING_PROSE_FONT =
  "400 14px ui-sans-serif, system-ui, sans-serif";
const STREAMING_LINE_HEIGHT = 28;

type MessageItemStreamingLayoutOptions = {
  assistant_content_mode:
    | "dm_live"
    | "dm_archived"
    | "room_thread"
    | "room_result";
  direct_content: ContentBlock[];
  final_assistant_text: string;
  show_cursor: boolean;
};

type MessageItemStreamingLayout = {
  content_area_ref: RefObject<HTMLDivElement | null>;
  content_area_style: CSSProperties | undefined;
};

export function useMessageItemStreamingLayout({
  assistant_content_mode,
  direct_content,
  final_assistant_text,
  show_cursor,
}: MessageItemStreamingLayoutOptions): MessageItemStreamingLayout {
  const content_area_ref = useRef<HTMLDivElement>(null);
  const streaming_min_height = useRef(STREAMING_MIN_HEIGHT);
  const layout_throttle_ref = useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );

  useEffect(() => {
    const layout_text =
      assistant_content_mode === "dm_live" ||
      assistant_content_mode === "room_thread"
        ? extract_text_from_content_blocks(direct_content)
        : final_assistant_text;

    if (!show_cursor || !layout_text) {
      return;
    }
    if (layout_throttle_ref.current !== null) {
      return;
    }

    layout_throttle_ref.current = setTimeout(() => {
      layout_throttle_ref.current = null;
      const element = content_area_ref.current;
      if (!element) {
        return;
      }
      try {
        const width = element.offsetWidth || 640;
        const prepared = prepare(layout_text, STREAMING_PROSE_FONT);
        const result = layout(prepared, width, STREAMING_LINE_HEIGHT);
        streaming_min_height.current = Math.max(
          streaming_min_height.current,
          result.height,
        );
      } catch {
        // 这里只保留上一次可用高度，避免流式阶段因为排版测量失败产生闪动。
      }
    }, STREAMING_LAYOUT_DELAY_MS);

    return () => {
      if (layout_throttle_ref.current !== null) {
        clearTimeout(layout_throttle_ref.current);
        layout_throttle_ref.current = null;
      }
    };
  }, [
    assistant_content_mode,
    direct_content,
    final_assistant_text,
    show_cursor,
  ]);

  useEffect(() => {
    if (!show_cursor) {
      streaming_min_height.current = STREAMING_MIN_HEIGHT;
      if (layout_throttle_ref.current !== null) {
        clearTimeout(layout_throttle_ref.current);
        layout_throttle_ref.current = null;
      }
    }
  }, [show_cursor]);

  return {
    content_area_ref,
    content_area_style: show_cursor
      ? { minHeight: streaming_min_height.current }
      : undefined,
  };
}
