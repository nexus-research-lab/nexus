"use client";

import { type ReactNode } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkBreaks from "remark-breaks";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

import { CodeBlock } from "./block/code-block";
import { cn } from "@/lib/utils";
import { useAgentStore } from "@/store/agent";
import { useWorkspaceFilesStore } from "@/store/workspace-files";
import { type WorkspaceFileEntry } from "@/types/agent";

import "katex/dist/katex.min.css";

interface MarkdownRendererProps {
  content: string;
  class_name?: string;
  is_streaming?: boolean;
  on_open_workspace_file?: (path: string) => void;
}

type MarkdownNodeLike = {
  position?: {
    start?: { line?: number };
    end?: { line?: number };
  };
};

type ResolveWorkspaceFilePath = (value: string) => string | null;

const WORKSPACE_FILE_PATTERN = /([A-Za-z0-9_./-]+\.[A-Za-z0-9]{1,10})/g;
const MARKDOWN_PLUGINS = [remarkGfm, remarkMath, remarkBreaks];
const REHYPE_PLUGINS = [rehypeKatex];

const FILE_TRIGGER_CLASS =
  "mx-0.5 inline-flex max-w-full cursor-pointer overflow-hidden rounded-xs border border-primary/20 bg-primary/10 px-2 py-0.5 align-middle text-left text-sm font-mono text-primary transition-colors hover:border-primary/30 hover:bg-primary/12";
const FILE_TRIGGER_LABEL_CLASS = "max-w-full break-all whitespace-pre-wrap [overflow-wrap:anywhere]";
const INLINE_CODE_CLASS =
  "mx-0.5 inline-flex max-w-full overflow-hidden rounded-xs border border-primary/20 bg-primary/10 px-2 py-0.5 align-middle text-sm font-mono text-primary";

function normalizeWorkspaceReference(value: string): string {
  return value.replace(/^[("'`【]+|[)"'`】,，。；：:!?]+$/g, "");
}

function looksLikeWorkspaceFileReference(value: string): boolean {
  if (!value.includes(".") || /^https?:\/\//i.test(value) || value.startsWith("/")) {
    return false;
  }

  return /[A-Za-z0-9]/.test(value);
}

function resolveWorkspaceFileReference(value: string, files: WorkspaceFileEntry[]): string | null {
  const normalized = normalizeWorkspaceReference(value);
  if (!looksLikeWorkspaceFileReference(normalized)) {
    return null;
  }

  const candidateFiles = files.filter((entry) => !entry.is_dir);
  const exactMatch = candidateFiles.find((entry) => entry.path === normalized);
  if (exactMatch) {
    return exactMatch.path;
  }

  const basenameMatches = candidateFiles.filter((entry) => entry.name === normalized);
  return basenameMatches.length === 1 ? basenameMatches[0].path : null;
}

function isBlockCode(node: MarkdownNodeLike | null | undefined, className: string | undefined, value: string): boolean {
  if (className && /language-\w+/.test(className)) {
    return true;
  }

  if (value.includes("\n")) {
    return true;
  }

  const startLine = node?.position?.start?.line;
  const endLine = node?.position?.end?.line;
  return typeof startLine === "number" && typeof endLine === "number" && startLine !== endLine;
}

function WorkspaceFileButton({
  label,
  path,
  on_open_workspace_file,
}: {
  label: ReactNode;
  path: string;
  on_open_workspace_file: (path: string) => void;
}) {
  return (
    <button
      className={FILE_TRIGGER_CLASS}
      onClick={() => on_open_workspace_file(path)}
      title={`Open ${path}`}
      type="button"
    >
      <span className={FILE_TRIGGER_LABEL_CLASS}>{label}</span>
    </button>
  );
}

function createMarkdownComponents(
  resolveFilePath: ResolveWorkspaceFilePath,
  on_open_workspace_file?: (path: string) => void,
): Components {
  return {
    pre({ children }) {
      return <div className="my-4 w-full min-w-0 max-w-full overflow-hidden">{children}</div>;
    },
    code({ children, className, node }) {
      const value = String(children).replace(/\n$/, "");
      if (isBlockCode(node as MarkdownNodeLike | undefined, className, value)) {
        const language = /language-(\w+)/.exec(className || "")?.[1] || "text";
        return <CodeBlock language={language} value={value} />;
      }

      const resolvedPath = resolveFilePath(value);
      if (resolvedPath && on_open_workspace_file) {
        return (
          <WorkspaceFileButton
            label={value}
            path={resolvedPath}
            on_open_workspace_file={on_open_workspace_file}
          />
        );
      }

      return (
        <span className={INLINE_CODE_CLASS}>
          <span className={FILE_TRIGGER_LABEL_CLASS}>{value}</span>
        </span>
      );
    },
    p({ children }) {
      return (
        <div className="mb-2 mt-2 min-w-0 max-w-full leading-relaxed text-foreground/90 [overflow-wrap:anywhere] last:mb-0">
          {children}
        </div>
      );
    },
    ul({ children }) {
      return (
        <ul className="mb-4 max-w-full list-disc space-y-2 pl-5 text-foreground/90 marker:text-muted-foreground sm:pl-6">
          {children}
        </ul>
      );
    },
    ol({ children }) {
      return (
        <ol className="mb-4 max-w-full list-decimal space-y-2 pl-5 text-foreground/90 marker:text-muted-foreground sm:pl-6">
          {children}
        </ol>
      );
    },
    li({ children }) {
      return (
        <li className="max-w-full pl-1 leading-relaxed [overflow-wrap:anywhere] [&>p]:m-0 [&>p]:leading-relaxed">
          {children}
        </li>
      );
    },
    blockquote({ children }) {
      return (
        <blockquote className="my-4 w-full min-w-0 max-w-full overflow-hidden rounded-r border-l-4 border-primary/30 bg-primary/5 py-2 pl-4 pr-1 italic text-muted-foreground [overflow-wrap:anywhere]">
          <div className="min-w-0 max-w-full">{children}</div>
        </blockquote>
      );
    },
    a({ href, children }) {
      if (!href) {
        return <span className="text-primary">{children}</span>;
      }

      return (
        <a
          className="text-primary transition-all decoration-primary/30 underline-offset-4 hover:underline"
          href={href}
          rel="noopener noreferrer"
          target="_blank"
        >
          {children}
        </a>
      );
    },
    img({ alt, src }) {
      return (
        <img
          alt={alt || ""}
          className="my-4 h-auto max-w-full rounded-2xl border border-white/40 object-cover"
          loading="lazy"
          src={src || ""}
        />
      );
    },
    h1({ children }) {
      return <h1 className="mb-4 mt-6 max-w-full break-words text-2xl font-bold text-foreground first:mt-0">{children}</h1>;
    },
    h2({ children }) {
      return <h2 className="mb-3 mt-5 max-w-full break-words text-xl font-bold text-foreground">{children}</h2>;
    },
    h3({ children }) {
      return <h3 className="mb-2 mt-4 max-w-full break-words text-lg font-bold text-foreground">{children}</h3>;
    },
    table({ children }) {
      return <table className="my-4 w-full max-w-full table-fixed border-collapse text-left text-sm sm:table-auto">{children}</table>;
    },
    thead({ children }) {
      return <thead className="bg-muted/50 uppercase text-muted-foreground">{children}</thead>;
    },
    tbody({ children }) {
      return <tbody className="align-top">{children}</tbody>;
    },
    tr({ children }) {
      return <tr className="align-top">{children}</tr>;
    },
    th({ children }) {
      return <th className="px-2 py-2 font-medium whitespace-normal [overflow-wrap:anywhere] sm:px-4 sm:py-3">{children}</th>;
    },
    td({ children }) {
      return <td className="border-t border-border/60 px-2 py-2 align-top whitespace-normal [overflow-wrap:anywhere] sm:px-4 sm:py-3">{children}</td>;
    },
  };
}

export function MarkdownRendererContent({
  content,
  class_name,
  is_streaming = false,
  on_open_workspace_file,
}: MarkdownRendererProps) {
  const agent_id = useAgentStore((state) => state.current_agent_id);
  const files = useWorkspaceFilesStore((state) => state.files_by_agent[agent_id ?? ""] ?? []);

  const resolveFilePath: ResolveWorkspaceFilePath = (value) => resolveWorkspaceFileReference(value, files);
  const markdownComponents = createMarkdownComponents(resolveFilePath, on_open_workspace_file);

  const normalizedContent = content.replace(WORKSPACE_FILE_PATTERN, (match) => {
    const resolvedPath = resolveFilePath(match);
    return resolvedPath && on_open_workspace_file ? `\`${match}\`` : match;
  });

  return (
    <div
      className={cn(
        "min-w-0 max-w-full text-[14px] leading-7 text-foreground/90",
        is_streaming && "animate-in fade-in-0",
        class_name,
      )}
    >
      <ReactMarkdown
        components={markdownComponents}
        rehypePlugins={REHYPE_PLUGINS}
        remarkPlugins={MARKDOWN_PLUGINS}
      >
        {normalizedContent}
      </ReactMarkdown>
    </div>
  );
}
