"use client";

import { cn } from "@/shared/ui/class-name";
import { MarkdownRenderer } from "../../../markdown-renderer";
import type { ContentRendererProps } from "./content-renderer-contract";
import {
  TIMELINE_LINE_CLASS_NAME,
  TimelineBlock,
} from "./content-renderer-timeline";
import { StructuredContentRenderer } from "./structured-content-renderer";

export function ContentRenderer({ content, ...props }: ContentRendererProps) {
  if (typeof content === "string") {
    return <MarkdownContent content={content} {...props} />;
  }
  return <StructuredContentRenderer content={content} {...props} />;
}

function MarkdownContent({
  className,
  content,
  isStreaming = false,
  onOpenWorkspaceFile,
  showTimelineDots = false,
  workspaceAgentId,
}: {
  className?: string;
  content: string;
  isStreaming?: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  showTimelineDots?: boolean;
  workspaceAgentId?: string | null;
}) {
  const markdown = (
    <MarkdownRenderer
      content={content}
      isStreaming={isStreaming}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
      workspaceAgentId={workspaceAgentId}
    />
  );
  if (!className) {
    return markdown;
  }

  return (
    <div className={cn(className, showTimelineDots && TIMELINE_LINE_CLASS_NAME)}>
      {showTimelineDots ? (
        <TimelineBlock active={isStreaming}>{markdown}</TimelineBlock>
      ) : markdown}
    </div>
  );
}
