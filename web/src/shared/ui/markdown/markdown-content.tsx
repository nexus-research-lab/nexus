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
import { MarkdownCapabilitiesProvider } from "./core/markdown-capabilities-context";
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
  const components = createMarkdownComponentSet({
    mermaidShowHeader,
    summaryMonochrome,
    summaryStrongAsText,
    variant,
  });
  const capabilities = useMemo(
    () => ({ getFilePreviewUrl, onOpenWorkspaceFile, resolveFilePath }),
    [getFilePreviewUrl, onOpenWorkspaceFile, resolveFilePath],
  );
  // 全文规范化是多轮正则扫描；只在展示正文或解析能力变化时重算。
  const normalizedContent = useMemo(
    () => normalizeMarkdownContent(
      displayedContent,
      resolveFilePath,
      onOpenWorkspaceFile,
      { is_streaming: shouldRenderStreaming },
    ),
    [displayedContent, onOpenWorkspaceFile, resolveFilePath, shouldRenderStreaming],
  );
  const plugins = useMemo(() => ({
    rehypePlugins: variant === "summary" ? [] : REHYPE_PLUGINS,
    remarkPlugins: variant === "summary"
      ? [...MARKDOWN_PLUGINS, [remarkMathSummary, { label: summaryMathLabel }]] as PluggableList
      : MARKDOWN_PLUGINS,
  }), [summaryMathLabel, variant]);
  const sharedProps = {
    ...plugins,
    components,
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
      <MarkdownCapabilitiesProvider value={capabilities}>
        <MarkdownText
          {...sharedProps}
          isStreaming={shouldRenderStreaming}
        />
      </MarkdownCapabilitiesProvider>
    </div>
  );
}

interface CreateMarkdownComponentSetOptions {
  mermaidShowHeader: boolean;
  summaryMonochrome: boolean;
  summaryStrongAsText: boolean;
  variant: "body" | "summary";
}

function resolveNoWorkspaceFile(): null {
  return null;
}

function createMarkdownComponentSet({
  mermaidShowHeader,
  summaryMonochrome,
  summaryStrongAsText,
  variant,
}: CreateMarkdownComponentSetOptions): Components {
  if (variant === "summary") {
    return createMarkdownSummaryComponents(
      { monochrome: summaryMonochrome, strongAsText: summaryStrongAsText },
    );
  }
  // 静态与流式区块共用一份组件表；流式差异由 MarkdownText 的区块 Context 提供。
  return createMarkdownComponents(
    { compactMermaid: false, showMermaidHeader: mermaidShowHeader },
  );
}
