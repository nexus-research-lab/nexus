/**
 * INPUT: Markdown 内容、本地化摘要公式标记、受控文件解析/预览/打开能力。
 * OUTPUT: 静态或流式正文，以及不排版公式的紧凑摘要。
 * POS: 无业务状态的共享入口；消费者绑定资源身份，不读取当前 Agent 或 Store。
 */
"use client";

import { useMemo } from "react";
import type { Components } from "react-markdown";
import { defaultUrlTransform } from "react-markdown";
import type { PluggableList } from "unified";

import { cn } from "@/shared/ui/class-name";

import "katex/dist/katex.min.css";
import { createMarkdownComponents } from "./core/markdown-components";
import { createMarkdownSummaryComponents } from "./core/markdown-summary-components";
import { remarkMathSummary } from "./core/markdown-math";
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
  summaryMathLabel?: string;
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
  summaryMathLabel = "[Formula]",
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
  const plugins = useMemo(() => ({
    rehypePlugins: variant === "summary" ? [] : REHYPE_PLUGINS,
    remarkPlugins: variant === "summary"
      ? [...MARKDOWN_PLUGINS, [remarkMathSummary, { label: summaryMathLabel }]] as PluggableList
      : MARKDOWN_PLUGINS,
  }), [summaryMathLabel, variant]);
  const sharedProps = {
    ...plugins,
    components: components.stable,
    content: normalizedContent,
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
