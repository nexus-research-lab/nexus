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
	agentMentions = [],
	agentMentionDirectory,
	className,
  content,
  isStreaming = false,
  onOpenWorkspaceFile,
  showTimelineDots = false,
	workspaceAgentId,
	onOpenAgentContact,
}: {
	agentMentions?: ContentRendererProps["agentMentions"];
	agentMentionDirectory?: ContentRendererProps["agentMentionDirectory"];
  className?: string;
  content: string;
  isStreaming?: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  showTimelineDots?: boolean;
	workspaceAgentId?: string | null;
	onOpenAgentContact?: ContentRendererProps["onOpenAgentContact"];
}) {
  const markdown = (
    <MarkdownRenderer
      content={content}
      isStreaming={isStreaming}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
      workspaceAgentId={workspaceAgentId}
      agentMentions={agentMentions}
      agentMentionDirectory={agentMentionDirectory}
      onOpenAgentContact={onOpenAgentContact}
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
