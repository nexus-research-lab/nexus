// INPUT: exact Agent、文件类型及已加载正文。
// OUTPUT: 绑定文件归属的 Markdown 预览或对应文本渲染器。
// POS: Workspace 文本预览消费侧；文件资源经窄能力注入共享 Markdown。
import {
  lazy,
  Suspense,
  type ComponentType,
} from "react";

import { useWorkspaceMarkdown } from "@/hooks/agent/use-workspace-markdown";

import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { LazyMermaidView } from "@/shared/ui/markdown/mermaid/lazy-mermaid-view";

import { HtmlFilePreview } from "../media/html-file-preview";
import {
  getWorkspaceFileCodeLanguage,
  type WorkspaceFilePreviewKind,
} from "../workspace-file-preview-kind";

interface TextRendererProps {
  agentId: string;
  content: string;
  fileName: string;
  isStreaming: boolean;
}

interface TextFileContentProps extends TextRendererProps {
  fileType: WorkspaceFilePreviewKind;
  isLoading: boolean;
}

function MarkdownContent({ agentId, content }: TextRendererProps) {
  const { resolveFilePath, getFilePreviewUrl } = useWorkspaceMarkdown(agentId);
  return (
    <UiMarkdownContent
      className="nexus-workspace-file-markdown min-h-full"
      content={content}
      getFilePreviewUrl={getFilePreviewUrl}
      resolveFilePath={resolveFilePath}
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
    <pre className="message-code-font min-h-full whitespace-pre-wrap break-words text-sm leading-[1.6] text-(--text-strong)">
      {content}
    </pre>
  );
}

const LazySyntaxHighlightedCode = lazy(async () => {
  const module = await import(
    "@/shared/ui/markdown/code/code-block-content"
  );
  return { default: module.SyntaxHighlightedCode };
});

function SourceCodeContent(props: TextRendererProps) {
  const { content, fileName } = props;
  const language = getWorkspaceFileCodeLanguage(fileName);
  if (!language) {
    return <PlainTextContent {...props} />;
  }
  return (
    <Suspense
      fallback={(
        <PlainTextContent {...props} />
      )}
    >
      <LazySyntaxHighlightedCode
        language={language}
        value={content}
        variant="workspace"
      />
    </Suspense>
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
  agentId,
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
  const Renderer = fileType === "text"
    ? SourceCodeContent
    : (TEXT_RENDERERS[fileType] ?? PlainTextContent);
  return (
    <Renderer
      agentId={agentId}
      content={content}
      fileName={fileName}
      isStreaming={isStreaming}
    />
  );
}
