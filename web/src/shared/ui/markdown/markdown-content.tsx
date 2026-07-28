"use client";

import { useMemo } from "react";
import type { Components } from "react-markdown";

import { cn } from "@/shared/ui/class-name";

import "katex/dist/katex.min.css";
import { createMarkdownComponents } from "./core/markdown-components";
import { createMarkdownSummaryComponents } from "./core/markdown-summary-components";
import {
  MARKDOWN_BODY_CLASS_NAME,
  MARKDOWN_SUMMARY_CLASS_NAME,
  MARKDOWN_PLUGINS,
  normalizeMarkdownContent,
  REHYPE_PLUGINS,
  transformMarkdownUrl,
} from "./core/markdown-renderer-shared";
import {
  type ResolveWorkspaceFilePath,
} from "./workspace/markdown-workspace-artifact-model";
import {
  useMarkdownCurrentAgentID,
  useMarkdownFileResolver,
} from "./workspace/use-markdown-workspace-files";
import { MarkdownText } from "./streaming/markdown-streaming";
import { useSmoothStreamingMarkdownState } from "./streaming/use-smooth-streaming-markdown-content";

interface UiMarkdownContentProps {
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

export function UiMarkdownContent({
  content,
  className,
  isStreaming = false,
  mermaidShowHeader = true,
  onOpenWorkspaceFile,
  summaryMonochrome = false,
  summaryStrongAsText = false,
  workspaceAgentId,
  variant = "body",
}: UiMarkdownContentProps) {
  const resolveFilePath = useMarkdownFileResolver(workspaceAgentId);
  const currentAgentId = useMarkdownCurrentAgentID(workspaceAgentId);
  const shouldStream = isStreaming;
  const smoothStreaming = useSmoothStreamingMarkdownState(
    content,
    shouldStream,
  );
  const displayedContent = smoothStreaming.content;
  const shouldRenderStreaming = smoothStreaming.isStreaming;
  const components = useMemo(
    () => createMarkdownComponentSet({
      currentAgentId,
      mermaidShowHeader,
      onOpenWorkspaceFile,
      resolveFilePath,
      summaryMonochrome,
      summaryStrongAsText,
      variant,
    }),
    [
      currentAgentId,
      mermaidShowHeader,
      onOpenWorkspaceFile,
      resolveFilePath,
      summaryMonochrome,
      summaryStrongAsText,
      variant,
    ],
  );
  const normalizedContent = normalizeMarkdownContent(
    displayedContent,
    resolveFilePath,
    onOpenWorkspaceFile,
    { is_streaming: shouldRenderStreaming },
  );
  const sharedProps = {
    components: components.stable,
    content: normalizedContent,
    rehypePlugins: REHYPE_PLUGINS,
    remarkPlugins: MARKDOWN_PLUGINS,
    urlTransform: transformMarkdownUrl,
  };

  return (
    <div
      className={cn(
        variant === "summary" ? MARKDOWN_SUMMARY_CLASS_NAME : MARKDOWN_BODY_CLASS_NAME,
        shouldRenderStreaming && "animate-in fade-in-0",
        className,
      )}
    >
      <MarkdownText
        {...sharedProps}
        isStreaming={shouldRenderStreaming}
        streamingComponents={components.streaming}
      />
    </div>
  );
}

interface CreateMarkdownComponentSetOptions {
  currentAgentId: string | null;
  mermaidShowHeader: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  resolveFilePath: ResolveWorkspaceFilePath;
  summaryMonochrome: boolean;
  summaryStrongAsText: boolean;
  variant: "body" | "summary";
}

interface MarkdownComponentSet {
  stable: Components;
  streaming: Components;
}

function createMarkdownComponentSet({
  currentAgentId,
  mermaidShowHeader,
  onOpenWorkspaceFile,
  resolveFilePath,
  summaryMonochrome,
  summaryStrongAsText,
  variant,
}: CreateMarkdownComponentSetOptions): MarkdownComponentSet {
  if (variant === "summary") {
    const summary = createMarkdownSummaryComponents(
      resolveFilePath,
      onOpenWorkspaceFile,
      currentAgentId,
      { monochrome: summaryMonochrome, strongAsText: summaryStrongAsText },
    );
    return { stable: summary, streaming: summary };
  }
  return {
    stable: createMarkdownComponents(
      resolveFilePath,
      onOpenWorkspaceFile,
      currentAgentId,
      { compactMermaid: false, showMermaidHeader: mermaidShowHeader },
    ),
    streaming: createMarkdownComponents(
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
  };
}
