"use client";

import { useState } from "react";
import { Brain, ChevronRight } from "lucide-react";
import { MarkdownRenderer } from "../markdown/markdown-renderer";
import { MessageRail, MessageRailBody, MessageRailLabel } from "../ui/message-rail";

interface ThinkingBlockProps {
  thinking: string;
  is_streaming?: boolean;
  workspace_agent_id?: string | null;
}

export function ThinkingBlock({ thinking, is_streaming, workspace_agent_id }: ThinkingBlockProps) {
  const [is_expanded, set_is_expanded] = useState(Boolean(is_streaming));
  const [was_streaming, set_was_streaming] = useState(Boolean(is_streaming));

  // 流式思考需要即时可见，历史思考默认保持收起。
  if (is_streaming && !was_streaming) {
    set_was_streaming(true);
    set_is_expanded(true);
  } else if (!is_streaming && was_streaming) {
    set_was_streaming(false);
  }

  if (!thinking) return null;

  return (
    <MessageRail>
      <button
        className="flex w-full items-center gap-2 text-left"
        onClick={() => set_is_expanded((previous) => !previous)}
        type="button"
      >
        <MessageRailLabel active={Boolean(is_streaming)} class_name="flex-1">
          <span data-timeline-anchor data-timeline-anchor-mode="box" className="flex h-4 w-4 shrink-0 items-center justify-center">
            <Brain className={is_streaming ? "h-3 w-3 animate-pulse text-(--primary)" : "h-3 w-3 text-(--icon-muted)"} />
          </span>
          <span>{is_streaming ? "Thinking……" : "Thought"}</span>
        </MessageRailLabel>
        <span className="shrink-0 text-(--icon-muted)">
          <ChevronRight
            className={is_expanded
              ? "h-3.5 w-3.5 rotate-90 transition-transform duration-(--motion-duration-fast)"
              : "h-3.5 w-3.5 transition-transform duration-(--motion-duration-fast)"}
          />
        </span>
      </button>
      {is_expanded ? (
        <MessageRailBody class_name="pt-1">
          <MarkdownRenderer
            content={thinking}
            is_streaming={is_streaming}
            class_name="min-w-0 max-w-full overflow-hidden break-all"
            workspace_agent_id={workspace_agent_id}
          />
        </MessageRailBody>
      ) : null}
    </MessageRail>
  );
}
