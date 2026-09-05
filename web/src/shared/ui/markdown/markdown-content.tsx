/**
 * INPUT: Markdown 内容、受控文件索引解析器及可选文件预览和打开命令。
 * OUTPUT: 静态或流式的通用 Markdown 正文/摘要。
 * POS: 无业务状态的共享入口；消费者绑定资源身份，不读取当前 Agent 或 Store。
 */
"use client";

import { useMemo } from "react";
import type { Components } from "react-markdown";
import { defaultUrlTransform } from "react-markdown";

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
} from "./core/markdown-renderer-shared";
import {
  type ResolveWorkspaceFilePath,
} from "./workspace/markdown-workspace-artifact-model";
import { MarkdownText } from "./streaming/markdown-streaming";
import { useSmoothStreamingMarkdownState } from "./streaming/use-smooth-streaming-markdown-content";

interface UiMarkdownContentProps {
  content: string;
  className?: string;
  isStreaming?: boolean;
  mermaidShowHeader?: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  resolveFilePath?: ResolveWorkspaceFilePath;
  summaryMonochrome?: boolean;
  summaryStrongAsText?: boolean;
  getFilePreviewUrl?: (path: string) => string;
  variant?: "body" | "summary";
}

export function UiMarkdownContent({
  content,
  className,
  isStreaming = false,
  mermaidShowHeader = true,
  onOpenWorkspaceFile,
  resolveFilePath = resolveNoWorkspaceFile,
  summaryMonochrome = false,
  summaryStrongAsText = false,
  getFilePreviewUrl,
  variant = "body",
}: UiMarkdownContentProps) {
  const shouldStream = isStreaming;
  const smoothStreaming = useSmoothStreamingMarkdownState(
    content,
    shouldStream,
  );
  const displayedContent = smoothStreaming.content;
  const shouldRenderStreaming = smoothStreaming.isStreaming;
  const components = useMemo(
    () => createMarkdownComponentSet({
      getFilePreviewUrl,
      mermaidShowHeader,
      onOpenWorkspaceFile,
      resolveFilePath,
      summaryMonochrome,
      summaryStrongAsText,
      variant,
    }),
    [
      getFilePreviewUrl,
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
    urlTransform: defaultUrlTransform,
  };

  return (
    <div
      className={cn(
        variant === "summary" ? MARKDOWN_SUMMARY_CLASS_NAME : MARKDOWN_BODY_CLASS_NAME,
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
  getFilePreviewUrl?: (path: string) => string;
  mermaidShowHeader: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  resolveFilePath: ResolveWorkspaceFilePath;
  summaryMonochrome: boolean;
  summaryStrongAsText: boolean;
  variant: "body" | "summary";
}

function resolveNoWorkspaceFile(): null {
  return null;
}

interface MarkdownComponentSet {
  stable: Components;
  streaming: Components;
}

function createMarkdownComponentSet({
  getFilePreviewUrl,
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
      { monochrome: summaryMonochrome, strongAsText: summaryStrongAsText },
    );
    return { stable: summary, streaming: summary };
  }
  return {
    stable: createMarkdownComponents(
      resolveFilePath,
      onOpenWorkspaceFile,
      { getFilePreviewUrl, compactMermaid: false, showMermaidHeader: mermaidShowHeader },
    ),
    streaming: createMarkdownComponents(
      resolveFilePath,
      onOpenWorkspaceFile,
      {
        getFilePreviewUrl,
        compactMermaid: false,
        showMermaidHeader: mermaidShowHeader,
        streamCodeBlocks: true,
        streamMermaid: true,
      },
    ),
  };
}
