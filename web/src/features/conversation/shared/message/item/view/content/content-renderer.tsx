// INPUT: String or structured message content and exact rendering context.
// OUTPUT: Markdown or structured blocks.
// POS: Shared content entry; className does not decide behavior.
"use client";

import { MarkdownRenderer } from "../../../markdown-renderer";
import type { ContentRendererProps } from "./content-renderer-contract";
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
	workspaceAgentId,
	onOpenAgentContact,
  renderLeadingSlashCommand = false,
}: {
	agentMentions?: ContentRendererProps["agentMentions"];
	agentMentionDirectory?: ContentRendererProps["agentMentionDirectory"];
  className?: string;
  content: string;
  isStreaming?: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
	workspaceAgentId?: string | null;
	onOpenAgentContact?: ContentRendererProps["onOpenAgentContact"];
  renderLeadingSlashCommand?: boolean;
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
      renderLeadingSlashCommand={renderLeadingSlashCommand}
    />
  );
  return className ? <div className={className}>{markdown}</div> : markdown;
}
