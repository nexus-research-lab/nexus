/**
 * INPUT: Markdown 代码/链接/图片节点，以及 Context 中已绑定作用域的 Workspace 能力与链接渲染槽。
 * OUTPUT: 行内文件按钮、代码块/Mermaid、受控链接和可预览图片视图。
 * POS: 组件表中唯一读取能力 Context 的元素；能力变化只重渲染这些元素，正文其余节点保持挂载。
 */
"use client";

import type { ReactNode } from "react";
import type { ExtraProps } from "react-markdown";

import { UiTooltip } from "@/shared/ui/overlay/tooltip";

import { WorkspaceFileButton } from "../workspace/markdown-workspace-file-button";
import {
  resolveWorkspaceArtifactPath,
  resolveWorkspaceImagePath,
  type ResolveWorkspaceFilePath,
} from "../workspace/markdown-workspace-artifact-model";
import { buildMarkdownLinkPresentation } from "./markdown-link-model";
import {
  buildMarkdownCodePresentation,
  type MarkdownCodeNode,
} from "./markdown-code-model";
import {
  type MarkdownCapabilities,
  useMarkdownCapabilities,
} from "./markdown-capabilities-context";
import { StreamingAwareCodeBlock, StreamingAwareMermaid } from "./markdown-streaming-code";

interface MarkdownLinkProps {
  children: ReactNode;
  href?: string;
  onOpenWorkspaceFile?: (path: string) => void;
  resolveFilePath: ResolveWorkspaceFilePath;
  renderLink?: MarkdownCapabilities["renderLink"];
}

type OpenWorkspaceFile = NonNullable<
  MarkdownLinkProps["onOpenWorkspaceFile"]
>;

function requireWorkspaceFileCommand(
  command: MarkdownLinkProps["onOpenWorkspaceFile"],
): OpenWorkspaceFile {
  if (!command) {
    throw new Error("工作区链接缺少文件打开命令");
  }
  return command;
}

function assertNever(value: never): never {
  throw new Error(`未处理的 Markdown 链接状态: ${String(value)}`);
}

function renderMarkdownLink({
  children,
  href,
  onOpenWorkspaceFile,
  resolveFilePath,
  renderLink,
}: MarkdownLinkProps): ReactNode {
  const rawHref = String(href ?? "").trim();
  const customLink = renderLink?.(rawHref, children);
  if (customLink != null) {
    return customLink;
  }
  const workspacePath = onOpenWorkspaceFile
    ? resolveWorkspaceArtifactPath(rawHref, resolveFilePath)
    : null;
  const presentation = buildMarkdownLinkPresentation(
    href,
    children,
    workspacePath,
  );

  switch (presentation.kind) {
    case "text":
      return <span className="text-primary">{children}</span>;
    case "workspace":
      return (
        <WorkspaceFileButton
          label={children}
          onOpenWorkspaceFile={requireWorkspaceFileCommand(onOpenWorkspaceFile)}
          path={presentation.path}
        />
      );
    case "anchor":
      return (
        <a
          className="inline max-w-full text-primary transition-all decoration-primary/30 underline-offset-4 break-words hover:underline"
          href={presentation.href}
        >
          {children}
        </a>
      );
    case "external":
      return (
        <>
          <UiTooltip label={presentation.href}><a
            className="inline max-w-full text-primary transition-all decoration-primary/30 underline-offset-4 break-words hover:underline"
            href={presentation.href}
            rel="noopener noreferrer"
            target={presentation.openInNewTab ? "_blank" : undefined}

          >
            {presentation.compactLabel ?? children}
          </a></UiTooltip>
          {presentation.trailingText}
        </>
      );
  }
  return assertNever(presentation);
}

export function MarkdownCode({
  children,
  className,
  compactMermaid,
  node,
  showMermaidHeader,
}: {
  children?: ReactNode;
  className?: string;
  compactMermaid: boolean;
  node?: ExtraProps["node"];
  showMermaidHeader?: boolean;
}) {
  const { onOpenWorkspaceFile, resolveFilePath } = useMarkdownCapabilities();
  const presentation = buildMarkdownCodePresentation(
    node as MarkdownCodeNode | undefined,
    className,
    children,
  );
  if (presentation.kind === "mermaid") {
    return (
      <StreamingAwareMermaid
        chart={presentation.value}
        compact={compactMermaid}
        showHeader={showMermaidHeader}
      />
    );
  }
  if (presentation.kind === "block") {
    return (
      <StreamingAwareCodeBlock
        language={presentation.language}
        value={presentation.value}
      />
    );
  }

  const resolvedPath = resolveWorkspaceArtifactPath(
    presentation.value,
    resolveFilePath,
  );
  if (resolvedPath && onOpenWorkspaceFile) {
    return (
      <WorkspaceFileButton
        label={presentation.value}
        path={resolvedPath}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
      />
    );
  }

  return (
    <code className="content-inline-code message-code-font">{presentation.value}</code>
  );
}

export function MarkdownLink({ children, href }: { children?: ReactNode; href?: string }) {
  const { onOpenWorkspaceFile, renderLink, resolveFilePath } = useMarkdownCapabilities();
  return renderMarkdownLink({
    children,
    href,
    onOpenWorkspaceFile,
    renderLink,
    resolveFilePath,
  });
}

export function MarkdownImage({ alt, src }: { alt?: string; src?: string }) {
  const { getFilePreviewUrl, onOpenWorkspaceFile, resolveFilePath } = useMarkdownCapabilities();
  const rawSrc = String(src || "").trim();
  const resolvedPath = resolveWorkspaceImagePath(rawSrc, resolveFilePath);
  const imageSrc = resolvedPath && getFilePreviewUrl
    ? getFilePreviewUrl(resolvedPath)
    : rawSrc;
  const interactive = Boolean(resolvedPath && onOpenWorkspaceFile);
  const image = (
    <img
      alt={alt || ""}
      className={interactive
        ? "content-media block h-full w-full object-contain"
        : "content-media content-media-frame block object-contain"}
      loading="lazy"
      src={imageSrc}
    />
  );

  if (resolvedPath && onOpenWorkspaceFile) {
    return (
      <UiTooltip label={resolvedPath}><button
        className="content-media-action content-media-frame block text-left"
        onClick={() => onOpenWorkspaceFile(resolvedPath)}

        type="button"
      >
        {image}
      </button></UiTooltip>
    );
  }

  return image;
}

