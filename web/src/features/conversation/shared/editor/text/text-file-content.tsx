import type { ComponentType } from "react";

import { LazyMermaidView } from "@/features/conversation/shared/message/markdown/mermaid/lazy-mermaid-view";
import { MarkdownRendererContent } from "@/features/conversation/shared/message/markdown/markdown-renderer-content";

import { HtmlFilePreview } from "../media/html-file-preview";
import type { WorkspaceFilePreviewKind } from "../workspace-file-preview-kind";

interface TextRendererProps {
  content: string;
  fileName: string;
  isStreaming: boolean;
}

interface TextFileContentProps extends TextRendererProps {
  fileType: WorkspaceFilePreviewKind;
  isLoading: boolean;
}

function MarkdownContent({ content }: TextRendererProps) {
  return (
    <MarkdownRendererContent
      className="min-h-full"
      content={content}
      mermaidShowHeader={false}
    />
  );
}

function MermaidContent({ content }: TextRendererProps) {
  return (
    <LazyMermaidView
      chart={content}
      className="min-h-full"
      constrainHeight={false}
      showHeader={false}
    />
  );
}

function HtmlContent({ content, fileName, isStreaming }: TextRendererProps) {
  return (
    <HtmlFilePreview
      content={content}
      isStreaming={isStreaming}
      title={fileName}
    />
  );
}

function PlainTextContent({ content }: TextRendererProps) {
  return (
    <pre className="message-cjk-code-font min-h-full whitespace-pre-wrap break-words text-sm leading-6 text-(--text-default)">
      {content}
    </pre>
  );
}

const TEXT_RENDERERS: Partial<
  Record<WorkspaceFilePreviewKind, ComponentType<TextRendererProps>>
> = {
  html: HtmlContent,
  markdown: MarkdownContent,
  mermaid: MermaidContent,
};

export function TextFileContent({
  content,
  fileName,
  fileType,
  isLoading,
  isStreaming,
}: TextFileContentProps) {
  if (isLoading) {
    return (
      <div className="font-mono text-sm leading-6 text-(--text-muted)">
        加载中...
      </div>
    );
  }
  const Renderer = TEXT_RENDERERS[fileType] ?? PlainTextContent;
  return (
    <Renderer
      content={content}
      fileName={fileName}
      isStreaming={isStreaming}
    />
  );
}
