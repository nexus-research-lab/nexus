"use client";

import { Brain, ChevronRight } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { cn } from "@/shared/ui/class-name";

import { MarkdownRenderer } from "../markdown-renderer";
import {
  MessageRail,
  MessageRailBody,
  MessageRailLabel,
} from "../ui/message-rail";

interface ThinkingBlockProps {
  thinking: string;
  isStreaming?: boolean;
  workspaceAgentId?: string | null;
}

interface ThinkingPresentation {
  iconClassName: string;
  label: string;
}

const THINKING_PRESENTATIONS: Readonly<Record<
  "idle" | "streaming",
  ThinkingPresentation
>> = {
  idle: {
    iconClassName: "h-3 w-3 text-(--icon-muted)",
    label: "Thought",
  },
  streaming: {
    iconClassName: "h-3 w-3 animate-pulse text-(--primary)",
    label: "Thinking……",
  },
};

export function ThinkingBlock({
  thinking,
  isStreaming = false,
  workspaceAgentId,
}: ThinkingBlockProps) {
  // 流式边界是展开状态的重置域；同一阶段内仍允许用户手动切换。
  const [isExpanded, setIsExpanded] = useResettableState(
    isStreaming,
    isStreaming,
  );
  const presentation = resolveThinkingPresentation(isStreaming);
  if (!thinking) {
    return null;
  }

  return (
    <MessageRail>
      <button
        className="flex w-full items-center gap-2 text-left"
        onClick={() => setIsExpanded((previous) => !previous)}
        type="button"
      >
        <MessageRailLabel active={isStreaming} className="flex-1">
          <span
            className="flex h-4 w-4 shrink-0 items-center justify-center"
            data-timeline-anchor
            data-timeline-anchor-mode="box"
          >
            <Brain className={presentation.iconClassName} />
          </span>
          <span>{presentation.label}</span>
        </MessageRailLabel>
        <ChevronRight
          className={cn(
            "h-3.5 w-3.5 shrink-0 text-(--icon-muted) transition-transform duration-(--motion-duration-fast)",
            isExpanded && "rotate-90",
          )}
        />
      </button>
      {isExpanded ? (
        <MessageRailBody className="pt-1">
          <MarkdownRenderer
            className="min-w-0 max-w-full overflow-hidden break-all"
            content={thinking}
            isStreaming={isStreaming}
            workspaceAgentId={workspaceAgentId}
          />
        </MessageRailBody>
      ) : null}
    </MessageRail>
  );
}

function resolveThinkingPresentation(
  isStreaming: boolean,
): ThinkingPresentation {
  return THINKING_PRESENTATIONS[isStreaming ? "streaming" : "idle"];
}
