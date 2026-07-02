"use client";

import { useMemo } from "react";
import { cn } from "@/lib/utils";

import "katex/dist/katex.min.css";
import {
  create_markdown_components,
  create_markdown_summary_components,
} from "./markdown-components";
import {
  MARKDOWN_BODY_CLASS_NAME,
  MARKDOWN_SUMMARY_CLASS_NAME,
  MARKDOWN_PLUGINS,
  normalize_markdown_content,
  REHYPE_PLUGINS,
} from "./markdown-renderer-shared";
import {
  useMarkdownCurrentAgentID,
  useMarkdownFileResolver,
} from "./markdown-workspace-artifacts";
import {
  StableMarkdownText,
  StreamingMarkdownText,
} from "./markdown-streaming";
import { useSmoothStreamingMarkdownContent } from "./use-smooth-streaming-markdown-content";

interface MarkdownRendererProps {
  content: string;
  class_name?: string;
  is_streaming?: boolean;
  mermaid_show_header?: boolean;
  on_open_workspace_file?: (path: string) => void;
  workspace_agent_id?: string | null;
  variant?: "body" | "summary";
}

export function MarkdownRendererContent({
  content,
  class_name: className,
  is_streaming: isStreaming = false,
  mermaid_show_header: mermaidShowHeader = true,
  on_open_workspace_file: onOpenWorkspaceFile,
  workspace_agent_id: workspaceAgentId,
  variant = "body",
}: MarkdownRendererProps) {
  const resolveFilePath = useMarkdownFileResolver(workspaceAgentId);
  const currentAgentId = useMarkdownCurrentAgentID(workspaceAgentId);
  const shouldStream = Boolean(isStreaming);
  const displayedContent = useSmoothStreamingMarkdownContent(content, shouldStream);
  const markdownComponents = useMemo(
    () => variant === "summary"
      ? create_markdown_summary_components(resolveFilePath, onOpenWorkspaceFile, currentAgentId)
      : create_markdown_components(
        resolveFilePath,
        onOpenWorkspaceFile,
        currentAgentId,
        { compact_mermaid: false, show_mermaid_header: mermaidShowHeader },
      ),
    [currentAgentId, mermaidShowHeader, onOpenWorkspaceFile, resolveFilePath, variant],
  );
  const streamingMarkdownComponents = useMemo(
    () => variant === "summary"
      ? create_markdown_summary_components(resolveFilePath, onOpenWorkspaceFile, currentAgentId)
      : create_markdown_components(
        resolveFilePath,
        onOpenWorkspaceFile,
        currentAgentId,
        {
          compact_mermaid: false,
          show_mermaid_header: mermaidShowHeader,
          stream_code_blocks: true,
          stream_mermaid: true,
        },
      ),
    [currentAgentId, mermaidShowHeader, onOpenWorkspaceFile, resolveFilePath, variant],
  );
  const normalizedContent = normalize_markdown_content(
    displayedContent,
    resolveFilePath,
    onOpenWorkspaceFile,
    { is_streaming: shouldStream },
  );
  const sharedProps = {
    components: markdownComponents,
    content: normalizedContent,
    rehype_plugins: REHYPE_PLUGINS,
    remark_plugins: MARKDOWN_PLUGINS,
  };

  return (
    <div
      className={cn(
        variant === "summary" ? MARKDOWN_SUMMARY_CLASS_NAME : MARKDOWN_BODY_CLASS_NAME,
        isStreaming && "animate-in fade-in-0",
        className,
      )}
    >
      {shouldStream ? (
        <StreamingMarkdownText
          {...sharedProps}
          streaming_components={streamingMarkdownComponents}
        />
      ) : (
        <StableMarkdownText {...sharedProps} />
      )}
    </div>
  );
}
