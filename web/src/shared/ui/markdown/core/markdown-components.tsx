"use client";

import type { ReactNode } from "react";
import { type Components } from "react-markdown";

import { getWorkspaceFilePreviewUrl } from "@/lib/api/agent/agent-api";
import {
  AgentMentionChip,
  type AgentMentionDirectory,
} from "@/features/conversation/shared/message/agent-mention-chip";

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
  agentMentionDirectory?: AgentMentionDirectory;
  onOpenAgentContact?: (agentId: string) => void;
}

interface MarkdownLinkProps {
  children: ReactNode;
  currentAgentId?: string | null;
  href?: string;
  onOpenWorkspaceFile?: (
    path: string,
    workspaceAgentId?: string | null,
  ) => void;
  resolveFilePath: ResolveWorkspaceFilePath;
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
  currentAgentId,
  href,
  onOpenWorkspaceFile,
  resolveFilePath,
  agentMentionDirectory,
  onOpenAgentContact,
}: MarkdownLinkProps & {
  agentMentionDirectory?: AgentMentionDirectory;
  onOpenAgentContact?: (agentId: string) => void;
}): ReactNode {
  const rawHref = String(href ?? "").trim();
  if (rawHref.startsWith("agent-mention://")) {
    const agentID = decodeURIComponent(rawHref.slice("agent-mention://".length));
    return (
      <AgentMentionChip
        agentId={agentID}
        directory={agentMentionDirectory}
        onOpenAgentContact={onOpenAgentContact}
      >
        {children}
      </AgentMentionChip>
    );
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
          workspaceAgentId={currentAgentId}
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
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void,
  currentAgentId?: string | null,
  options: CreateMarkdownComponentsOptions = {},
): Components {
  return {
    pre({ children }) {
      return <div className="my-2 w-full min-w-0 max-w-full overflow-hidden">{children}</div>;
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
            workspaceAgentId={currentAgentId}
          />
        );
      }

      return (
        <span className="message-cjk-code-font mx-0.5 inline-flex max-w-full overflow-hidden rounded-[5px] border border-primary/20 bg-primary/10 px-2 py-0.3 align-middle text-[0.9em] text-primary">
          <span className="max-w-full whitespace-pre-wrap break-words">
            {presentation.value}
          </span>
        </span>
      );
    },
    p({ children }) {
      return <div data-markdown-anchor className="mb-2 mt-2 min-w-0 max-w-full leading-relaxed text-pretty text-foreground/90 wrap-anywhere last:mb-0">{children}</div>;
    },
    ul({ children }) {
      return <ul className="markdown-list markdown-list-unordered">{children}</ul>;
    },
    ol({ children }) {
      return <ol className="markdown-list markdown-list-ordered">{children}</ol>;
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
        <blockquote data-markdown-anchor className="my-4 w-full min-w-0 max-w-full overflow-hidden border-l-[3px] border-primary/40 bg-primary/4 px-1 py-2 pl-4 text-pretty italic text-(--text-muted) wrap-anywhere">
          <div className="min-w-0 max-w-full">{children}</div>
        </blockquote>
      );
    },
    a({ href, children }) {
      return renderMarkdownLink({
        children,
        currentAgentId,
        href,
        onOpenWorkspaceFile,
        resolveFilePath,
        agentMentionDirectory: options.agentMentionDirectory,
        onOpenAgentContact: options.onOpenAgentContact,
      });
    },
    img({ alt, src }) {
      const rawSrc = String(src || "").trim();
      const resolvedPath = resolveWorkspaceImagePath(rawSrc, resolveFilePath);
      const imageSrc = resolvedPath && currentAgentId
        ? getWorkspaceFilePreviewUrl(currentAgentId, resolvedPath)
        : rawSrc;
      const image = (
        <img
          alt={alt || ""}
          className="my-3 block h-auto max-h-[420px] w-auto max-w-full rounded-[8px] border border-(--divider-subtle-color) object-contain sm:max-w-[560px]"
          loading="lazy"
          src={imageSrc}
        />
      );

      if (resolvedPath && onOpenWorkspaceFile) {
        return (
          <button
            className="block w-fit max-w-full text-left"
            onClick={() => onOpenWorkspaceFile(resolvedPath, currentAgentId)}
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
      return <h1 data-markdown-anchor className="mb-4 mt-6 max-w-full break-words text-2xl font-bold text-foreground first:mt-0">{children}</h1>;
    },
    h2({ children }) {
      return <h2 data-markdown-anchor className="mb-3 mt-5 max-w-full break-words text-xl font-bold text-foreground">{children}</h2>;
    },
    h3({ children }) {
      return <h3 data-markdown-anchor className="mb-2 mt-4 max-w-full break-words text-lg font-bold text-foreground">{children}</h3>;
    },
    kbd({ children }) {
      return <kbd className="message-cjk-code-font mx-0.5 inline-flex items-center rounded-[5px] border border-(--divider-subtle-color) bg-(--surface-panel-background) px-1.5 py-0.5 align-baseline text-[0.82em] font-medium text-(--text-strong) shadow-[inset_0_-1px_0_rgba(15,23,42,0.08)]">{children}</kbd>;
    },
    mark({ children }) {
      return <mark className="rounded-[4px] bg-amber-200/55 px-1 text-inherit">{children}</mark>;
    },
    sub({ children }) {
      return <sub className="text-[0.75em] leading-none">{children}</sub>;
    },
    sup({ children }) {
      return <sup className="text-[0.75em] leading-none">{children}</sup>;
    },
    table({ children }) {
      return <table className="my-4 block w-max max-w-full overflow-x-auto overflow-y-hidden rounded-[8px] border border-(--divider-subtle-color) border-collapse text-left text-sm">{children}</table>;
    },
    thead({ children }) {
      return <thead className="uppercase text-(--text-muted) font-semibold" style={{ background: "color-mix(in srgb, var(--surface-panel-background) 68%, var(--divider-subtle-color))" }}>{children}</thead>;
    },
    tbody({ children }) {
      return <tbody className="align-top">{children}</tbody>;
    },
    tr({ children }) {
      return <tr className="align-top">{children}</tr>;
    },
    th({ children }) {
      return <th data-markdown-anchor className="min-w-[120px] border-b px-3 py-2 text-start font-semibold whitespace-normal break-words sm:px-4 sm:py-3" style={{ borderColor: "var(--divider-subtle-color)" }}>{children}</th>;
    },
    td({ children }) {
      return <td data-markdown-anchor className="min-w-[120px] border-t border-b px-3 py-2 text-start align-top whitespace-normal break-words sm:px-4 sm:py-3" style={{ borderColor: "var(--divider-subtle-color)" }}>{children}</td>;
    },
  };
}
