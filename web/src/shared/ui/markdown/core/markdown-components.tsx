/**
 * INPUT: Markdown AST 节点、受控 Workspace 解析命令与可选链接渲染槽。
 * OUTPUT: 通用 Markdown 元素、文件交互与消费侧注入的链接视图。
 * POS: 共享 Markdown 渲染注册表；领域 URI 和业务状态由消费侧解释。
 */
"use client";

import type { ReactNode } from "react";
import { type Components } from "react-markdown";

import { CodeBlock } from "../code/code-block";
import { LazyMermaidView } from "../mermaid/lazy-mermaid-view";
import { WorkspaceFileButton } from "../workspace/markdown-workspace-file-button";
import {
  resolveWorkspaceArtifactPath,
  resolveWorkspaceImagePath,
  type ResolveWorkspaceFilePath,
} from "../workspace/markdown-workspace-artifact-model";
import {
  buildMarkdownLinkPresentation,
} from "./markdown-link-model";
import {
  buildMarkdownCodePresentation,
  type MarkdownCodeNode,
} from "./markdown-code-model";

interface CreateMarkdownComponentsOptions {
  compactMermaid?: boolean;
  showMermaidHeader?: boolean;
  streamCodeBlocks?: boolean;
  streamMermaid?: boolean;
  // 返回 null/undefined 时继续通用安全链接渲染；领域协议放行由消费侧 URL transform 决定。
  renderLink?: (href: string, children: ReactNode) => ReactNode | null;
  getFilePreviewUrl?: (path: string) => string;
}

interface MarkdownLinkProps {
  children: ReactNode;
  href?: string;
  onOpenWorkspaceFile?: (path: string) => void;
  resolveFilePath: ResolveWorkspaceFilePath;
  renderLink?: CreateMarkdownComponentsOptions["renderLink"];
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
          <a
            className="inline max-w-full text-primary transition-all decoration-primary/30 underline-offset-4 break-words hover:underline"
            href={presentation.href}
            rel="noopener noreferrer"
            target={presentation.openInNewTab ? "_blank" : undefined}
            title={presentation.href}
          >
            {presentation.compactLabel ?? children}
          </a>
          {presentation.trailingText}
        </>
      );
  }
  return assertNever(presentation);
}

export function createMarkdownComponents(
  resolveFilePath: ResolveWorkspaceFilePath,
  onOpenWorkspaceFile?: (path: string) => void,
  options: CreateMarkdownComponentsOptions = {},
): Components {
  return {
    pre({ children }) {
      return <div className="w-full min-w-0 max-w-full overflow-hidden">{children}</div>;
    },
    code({ children, className, node }) {
      const presentation = buildMarkdownCodePresentation(
        node as MarkdownCodeNode | undefined,
        className,
        children,
      );
      if (presentation.kind === "mermaid") {
        return (
          <LazyMermaidView
            chart={presentation.value}
            compact={options.compactMermaid ?? true}
            isStreaming={options.streamMermaid}
            showHeader={options.showMermaidHeader}
          />
        );
      }
      if (presentation.kind === "block") {
        return (
          <CodeBlock
            isStreaming={options.streamCodeBlocks}
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
        <span className="content-inline-code message-code-font align-baseline text-[0.9rem] leading-[1.25]">
          <span className="max-w-full whitespace-pre-wrap break-words">
            {presentation.value}
          </span>
        </span>
      );
    },
    p({ children }) {
      return <div data-markdown-anchor className="m-0 min-w-0 max-w-full leading-[1.65rem] wrap-anywhere">{children}</div>;
    },
    ul({ children }) {
      return <ul className="markdown-list markdown-list-unordered">{children}</ul>;
    },
    ol({ children, start }) {
      return (
        <ol
          className="markdown-list markdown-list-ordered"
          start={start}
        >
          {children}
        </ol>
      );
    },
    li({ children }) {
      return (
        <li data-markdown-anchor className="markdown-list-item">
          <span className="markdown-list-item-body">{children}</span>
        </li>
      );
    },
    blockquote({ children }) {
      return (
        <blockquote data-markdown-anchor className="content-quote m-0 wrap-anywhere">
          <div className="min-w-0 max-w-full">{children}</div>
        </blockquote>
      );
    },
    a({ href, children }) {
      return renderMarkdownLink({
        children,
        href,
        onOpenWorkspaceFile,
        resolveFilePath,
        renderLink: options.renderLink,
      });
    },
    img({ alt, src }) {
      const rawSrc = String(src || "").trim();
      const resolvedPath = resolveWorkspaceImagePath(rawSrc, resolveFilePath);
      const imageSrc = resolvedPath && options.getFilePreviewUrl
        ? options.getFilePreviewUrl(resolvedPath)
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
          <button
            className="content-media-action content-media-frame block text-left"
            onClick={() => onOpenWorkspaceFile(resolvedPath)}
            title={resolvedPath}
            type="button"
          >
            {image}
          </button>
        );
      }

      return image;
    },
    h1({ children }) {
      return <h1 data-markdown-anchor className="m-0 mt-3 -mb-1 max-w-full break-words text-[1.375rem] leading-[1.65rem] font-semibold text-foreground first:mt-0">{children}</h1>;
    },
    h2({ children }) {
      return <h2 data-markdown-anchor className="m-0 mt-3 -mb-1 max-w-full break-words text-[1.125rem] leading-[1.65rem] font-semibold text-foreground">{children}</h2>;
    },
    h3({ children }) {
      return <h3 data-markdown-anchor className="m-0 mt-2 -mb-1 max-w-full break-words text-base leading-[1.65rem] font-semibold text-foreground">{children}</h3>;
    },
    h4({ children }) {
      return <h4 data-markdown-anchor className="m-0 mt-2 -mb-1 max-w-full break-words text-base leading-[1.65rem] font-semibold text-foreground">{children}</h4>;
    },
    h5({ children }) {
      return <h5 data-markdown-anchor className="m-0 mt-2 -mb-1 max-w-full break-words text-base leading-[1.65rem] font-semibold text-foreground">{children}</h5>;
    },
    h6({ children }) {
      return <h6 data-markdown-anchor className="m-0 mt-2 -mb-1 max-w-full break-words text-base leading-[1.65rem] font-semibold text-foreground">{children}</h6>;
    },
    hr() {
      return <hr className="content-divider" />;
    },
    kbd({ children }) {
      return <kbd className="content-kbd message-code-font mx-0.5 align-baseline text-[0.82em] font-medium">{children}</kbd>;
    },
    mark({ children }) {
      return <mark className="content-highlight text-inherit">{children}</mark>;
    },
    sub({ children }) {
      return <sub className="text-[0.75em] leading-none">{children}</sub>;
    },
    sup({ children }) {
      return <sup className="text-[0.75em] leading-none">{children}</sup>;
    },
    table({ children }) {
      return (
        <div className="content-table-scroll">
          <table className="content-table text-sm">{children}</table>
        </div>
      );
    },
    thead({ children }) {
      return <thead className="content-table-head">{children}</thead>;
    },
    tbody({ children }) {
      return <tbody className="align-top">{children}</tbody>;
    },
    tr({ children }) {
      return <tr className="align-top">{children}</tr>;
    },
    th({ children }) {
      return <th data-markdown-anchor className="content-table-heading whitespace-normal">{children}</th>;
    },
    td({ children }) {
      return <td data-markdown-anchor className="content-table-cell whitespace-normal">{children}</td>;
    },
  };
}
