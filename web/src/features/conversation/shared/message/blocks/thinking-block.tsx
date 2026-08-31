/**
 * INPUT: Thought 正文、流式状态与展开默认值。
 * OUTPUT: 收起时显示单行正文预览，展开时仅保留状态标题并呈现完整明细。
 * POS: Assistant 执行过程中的 Thought 二级明细入口。
 */
"use client";

import { Brain, ChevronRight } from "lucide-react";
import type { RefObject } from "react";

import { useScrollAnchoredState } from "@/features/conversation/shared/timeline/scroll/use-scroll-anchored-state";
import { cn } from "@/shared/ui/class-name";

import { MarkdownRenderer } from "../markdown-renderer";
import {
  MessageDetailFrame,
  MessageDetailScroll,
} from "../ui/message-rail";

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
      <button
        aria-expanded={isExpanded}
        className="grid min-h-7 w-full min-w-0 grid-cols-[20px_minmax(0,1fr)_auto] items-center gap-1.5 radius-control-sm px-1.5 py-0.5 text-left text-sm font-normal leading-5 text-(--text-soft) transition-colors hover:bg-(--surface-interactive-hover-background)"
        data-activity-row="thinking"
        data-message-detail-sticky-header={isExpanded || undefined}
        onClick={expansion.toggle}
        type="button"
      >
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
        <span className="flex min-w-0 items-baseline gap-1.5">
          <span
            className={cn(
              "shrink-0",
              isStreaming && "text-(--primary)",
            )}
          >
            {presentation.label}
          </span>
          {!isExpanded ? (
            <span
              className="min-w-0 flex-1 truncate text-(--text-soft)"
              data-thinking-block-preview
            >
              {preview}
            </span>
          ) : null}
        </span>
        <ChevronRight
          className={cn(
            "h-3.5 w-3.5 shrink-0 text-(--icon-muted) transition-transform duration-(--motion-duration-fast)",
            isExpanded && "rotate-90",
          )}
        />
      </button>
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
