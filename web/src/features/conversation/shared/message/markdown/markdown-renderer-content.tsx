"use client";

import { useMemo } from "react";
import { cn } from "@/lib/utils";

import "katex/dist/katex.min.css";
import {
  createMarkdownComponents,
  createMarkdownSummaryComponents,
} from "./markdown-components";
import {
  MARKDOWN_BODY_CLASS_NAME,
  MARKDOWN_SUMMARY_CLASS_NAME,
  MARKDOWN_PLUGINS,
  normalizeMarkdownContent,
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
  className?: string;
  isStreaming?: boolean;
  mermaidShowHeader?: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  summaryMonochrome?: boolean;
  summaryStrongAsText?: boolean;
  workspaceAgentId?: string | null;
  variant?: "body" | "summary";
}

export function MarkdownRendererContent({
  content,
  className: className,
  isStreaming: isStreaming = false,
  mermaidShowHeader: mermaidShowHeader = true,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  summaryMonochrome: summaryMonochrome = false,
  summaryStrongAsText: summaryStrongAsText = false,
  workspaceAgentId: workspaceAgentId,
  variant = "body",
}: MarkdownRendererProps) {
  const resolveFilePath = useMarkdownFileResolver(workspaceAgentId);
  const currentAgentId = useMarkdownCurrentAgentID(workspaceAgentId);
  const shouldStream = Boolean(isStreaming);
  const displayedContent = useSmoothStreamingMarkdownContent(content, shouldStream);
  const markdownComponents = useMemo(
    () => variant === "summary"
      ? createMarkdownSummaryComponents(
        resolveFilePath,
        onOpenWorkspaceFile,
        currentAgentId,
        { monochrome: summaryMonochrome, strongAsText: summaryStrongAsText },
      )
      : createMarkdownComponents(
        resolveFilePath,
        onOpenWorkspaceFile,
        currentAgentId,
        { compactMermaid: false, showMermaidHeader: mermaidShowHeader },
      ),
    [currentAgentId, mermaidShowHeader, onOpenWorkspaceFile, resolveFilePath, summaryMonochrome, summaryStrongAsText, variant],
  );
  const streamingMarkdownComponents = useMemo(
    () => variant === "summary"
      ? createMarkdownSummaryComponents(
        resolveFilePath,
        onOpenWorkspaceFile,
        currentAgentId,
        { monochrome: summaryMonochrome, strongAsText: summaryStrongAsText },
      )
      : createMarkdownComponents(
        resolveFilePath,
        onOpenWorkspaceFile,
        currentAgentId,
        {
          compactMermaid: false,
          showMermaidHeader: mermaidShowHeader,
          streamCodeBlocks: true,
          streamMermaid: true,
        },
      ),
    [currentAgentId, mermaidShowHeader, onOpenWorkspaceFile, resolveFilePath, summaryMonochrome, summaryStrongAsText, variant],
  );
  const normalizedContent = normalizeMarkdownContent(
    displayedContent,
    resolveFilePath,
    onOpenWorkspaceFile,
    { is_streaming: shouldStream },
  );
  const sharedProps = {
    components: markdownComponents,
    content: normalizedContent,
    rehypePlugins: REHYPE_PLUGINS,
    remarkPlugins: MARKDOWN_PLUGINS,
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
          streamingComponents={streamingMarkdownComponents}
        />
      ) : (
        <StableMarkdownText {...sharedProps} />
      )}
    </div>
  );
}
