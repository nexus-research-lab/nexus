/**
 * INPUT: Thought 正文、流式状态与展开默认值。
 * OUTPUT: 收起时显示单行正文预览，展开时仅保留状态标题并呈现完整明细。
 * POS: Assistant 执行过程中的 Thought 二级明细入口。
 */
"use client";

import { Brain } from "lucide-react";
import type { RefObject } from "react";

import { useScrollAnchoredState } from "@/features/conversation/shared/timeline/scroll/use-scroll-anchored-state";
import { cn } from "@/shared/ui/class-name";

import { MarkdownRenderer } from "../markdown-renderer";
import {
  MessageDetailFrame,
  MessageDetailScroll,
} from "../ui/message-rail";
import { MessageDetailToggle } from "../ui/message-detail-toggle";

interface ThinkingBlockProps {
  defaultExpanded?: boolean;
  thinking: string;
  initialRevealFromEmpty?: boolean;
  isStreaming?: boolean;
  workspaceAgentId?: string | null;
}

interface ThinkingPresentation {
  className: string;
  label: string;
}

const THINKING_PRESENTATIONS: Readonly<Record<
  "idle" | "streaming",
  ThinkingPresentation
>> = {
  idle: {
    className: "text-(--icon-muted)",
    label: "Thought",
  },
  streaming: {
    className: "animate-pulse text-(--primary)",
    label: "Thinking……",
  },
};

export function ThinkingBlock({
  defaultExpanded,
  thinking,
  initialRevealFromEmpty = false,
  isStreaming = false,
  workspaceAgentId,
}: ThinkingBlockProps) {
  // 流式边界是展开状态的重置域；同一阶段内仍允许用户手动切换。
  const expansion = useScrollAnchoredState(
    defaultExpanded ?? isStreaming,
    isStreaming,
  );
  const isExpanded = expansion.isOpen;
  const presentation = resolveThinkingPresentation(isStreaming);
  const preview = thinking.replace(/\s+/g, " ").trim();
  if (!thinking) {
    return null;
  }

  return (
    <div
      className="min-w-0"
      ref={expansion.anchorRef as RefObject<HTMLDivElement>}
    >
      <MessageDetailToggle
        contentClassName="flex items-baseline gap-1.5"
        data-activity-row="thinking"
        data-message-detail-sticky-header={isExpanded || undefined}
        expanded={isExpanded}
        leading={(
          <span
            className={cn(
              "flex h-5 w-5 shrink-0 items-center justify-center",
              presentation.className,
            )}
            data-thinking-block-icon="thinking"
            data-timeline-anchor
            data-timeline-anchor-mode="box"
          >
            <Brain aria-hidden className="h-3.5 w-3.5" strokeWidth={1.8} />
          </span>
        )}
        onClick={expansion.toggle}
        tone={isStreaming ? "active" : "default"}
      >
        <span className="shrink-0">{presentation.label}</span>
        {!isExpanded ? (
          <span
            className="min-w-0 flex-1 truncate text-(--text-soft)"
            data-thinking-block-preview
          >
            {preview}
          </span>
        ) : null}
      </MessageDetailToggle>
      {isExpanded ? (
        <MessageDetailFrame>
          <MessageDetailScroll followContent={isStreaming}>
            <MarkdownRenderer
              key={isStreaming ? "streaming" : "complete"}
              className="nexus-message-detail-markdown min-w-0 max-w-full overflow-hidden break-all text-(--text-muted)"
              content={thinking}
              initialRevealFromEmpty={initialRevealFromEmpty}
              isStreaming={isStreaming}
              workspaceAgentId={workspaceAgentId}
            />
          </MessageDetailScroll>
        </MessageDetailFrame>
      ) : null}
    </div>
  );
}

function resolveThinkingPresentation(
  isStreaming: boolean,
): ThinkingPresentation {
  return THINKING_PRESENTATIONS[isStreaming ? "streaming" : "idle"];
}
