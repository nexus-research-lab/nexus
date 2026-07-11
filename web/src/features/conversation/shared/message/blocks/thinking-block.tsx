"use client";

import { useState } from "react";
import { Brain, ChevronRight } from "lucide-react";
import { MarkdownRenderer } from "../markdown-renderer";
import { MessageRail, MessageRailBody, MessageRailLabel } from "../ui/message-rail";

interface ThinkingBlockProps {
  thinking: string;
  isStreaming?: boolean;
  workspaceAgentId?: string | null;
}

export function ThinkingBlock({ thinking, isStreaming: isStreaming, workspaceAgentId: workspaceAgentId }: ThinkingBlockProps) {
  const [isExpanded, setIsExpanded] = useState(Boolean(isStreaming));
  const [wasStreaming, setWasStreaming] = useState(Boolean(isStreaming));

  // 流式思考需要即时可见；输出结束后自动收起，历史思考默认保持收起。
  if (isStreaming && !wasStreaming) {
    setWasStreaming(true);
    setIsExpanded(true);
  } else if (!isStreaming && wasStreaming) {
    setWasStreaming(false);
    setIsExpanded(false);
  }

  if (!thinking) return null;

  return (
    <MessageRail>
      <button
        className="flex w-full items-center gap-2 text-left"
        onClick={() => setIsExpanded((previous) => !previous)}
        type="button"
      >
        <MessageRailLabel active={Boolean(isStreaming)} className="flex-1">
          <span data-timeline-anchor data-timeline-anchor-mode="box" className="flex h-4 w-4 shrink-0 items-center justify-center">
            <Brain className={isStreaming ? "h-3 w-3 animate-pulse text-(--primary)" : "h-3 w-3 text-(--icon-muted)"} />
          </span>
          <span>{isStreaming ? "Thinking……" : "Thought"}</span>
        </MessageRailLabel>
        <span className="shrink-0 text-(--icon-muted)">
          <ChevronRight
            className={isExpanded
              ? "h-3.5 w-3.5 rotate-90 transition-transform duration-(--motion-duration-fast)"
              : "h-3.5 w-3.5 transition-transform duration-(--motion-duration-fast)"}
          />
        </span>
      </button>
      {isExpanded ? (
        <MessageRailBody className="pt-1">
          <MarkdownRenderer
            content={thinking}
            isStreaming={isStreaming}
            className="min-w-0 max-w-full overflow-hidden break-all"
            workspaceAgentId={workspaceAgentId}
          />
        </MessageRailBody>
      ) : null}
    </MessageRail>
  );
}
