/**
 * INPUT: Markdown AST 节点、Workspace 解析命令与 Agent mention 链接。
 * OUTPUT: 通用 Markdown 组件及安全解析后的 Agent/handoff chip。
 * POS: 共享 Markdown 节点到产品 UI 原语的渲染注册表。
 */
"use client";

import type { ReactNode } from "react";
import { type Components } from "react-markdown";

import { getWorkspaceFilePreviewUrl } from "@/lib/api/agent/agent-api";
import {
  AgentMentionChip,
  type AgentMentionDirectory,
} from "@/features/conversation/shared/message/agent-mention-chip";
import { isSlashCommandHref } from "@/features/conversation/shared/slash-command-presentation";
import { SlashCommandToken } from "@/features/conversation/shared/slash-command-token";

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
  if (isSlashCommandHref(rawHref)) {
    return <SlashCommandToken>{children}</SlashCommandToken>;
  }
  const agentMention = parseAgentMentionHref(rawHref);
  if (agentMention) {
    return (
      <AgentMentionChip
        agentId={agentMention.agentId}
        directory={agentMentionDirectory}
        handoffId={agentMention.handoffId}
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

function parseAgentMentionHref(
  href: string,
): { agentId: string; handoffId?: string } | null {
  const prefix = "agent-mention://";
  if (!href.startsWith(prefix)) {
    return null;
  }
  const target = href.slice(prefix.length);
  const queryIndex = target.indexOf("?");
  const encodedAgentId = queryIndex >= 0 ? target.slice(0, queryIndex) : target;
  const query = queryIndex >= 0 ? target.slice(queryIndex + 1) : "";
  try {
    const agentId = decodeURIComponent(encodedAgentId).trim();
    if (!agentId) {
      return null;
    }
    const handoffId = new URLSearchParams(query).get("handoff_id")?.trim();
    return {
      agentId,
      ...(handoffId ? { handoffId } : {}),
    };
  } catch {
    return null;
  }
}

export function createMarkdownComponents(
  resolveFilePath: ResolveWorkspaceFilePath,
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void,
  currentAgentId?: string | null,
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
            workspaceAgentId={currentAgentId}
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
          className="content-media block h-auto max-h-[420px] w-auto max-w-full object-contain sm:max-w-[560px]"
          loading="lazy"
          src={imageSrc}
        />
      );

      if (resolvedPath && onOpenWorkspaceFile) {
        return (
          <button
            className="content-media-action block w-fit max-w-full text-left"
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
