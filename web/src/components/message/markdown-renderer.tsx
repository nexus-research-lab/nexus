"use client";

import { Fragment, ReactNode } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import remarkBreaks from 'remark-breaks';
import rehypeKatex from 'rehype-katex';
import { CodeBlock } from './block/code-block';
import { useAgentStore } from '@/store/agent';
import { useWorkspaceFilesStore } from '@/store/workspace-files';
import { cn } from '@/lib/utils';
import 'katex/dist/katex.min.css';

interface MarkdownRendererProps {
  content: string;
  className?: string;
  isStreaming?: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
}

const WORKSPACE_FILE_PATTERN = /([A-Za-z0-9_./-]+\.[A-Za-z0-9]{1,10})/g;

function looksLikeWorkspaceFilePath(value: string): boolean {
  if (!value.includes('.') || /^https?:\/\//i.test(value) || value.startsWith('/')) {
    return false;
  }
  return /[A-Za-z0-9]/.test(value);
}

function normalizeWorkspaceReference(value: string): string {
  return value.replace(/^[("'`【]+|[)"'`】,，。；：:!?]+$/g, '');
}

function resolveWorkspaceFileReference(
  value: string,
  currentAgentId: string | null,
  filesByAgent: Record<string, Array<{ path: string; name: string; is_dir: boolean }>>,
): string | null {
  const normalized = normalizeWorkspaceReference(value);
  if (!currentAgentId || !looksLikeWorkspaceFilePath(normalized)) {
    return null;
  }

  const files = (filesByAgent[currentAgentId] || []).filter((entry) => !entry.is_dir);
  const exactMatch = files.find((entry) => entry.path === normalized);
  if (exactMatch) {
    return exactMatch.path;
  }

  const basenameMatches = files.filter((entry) => entry.name === normalized);
  if (basenameMatches.length === 1) {
    return basenameMatches[0].path;
  }

  return null;
}

function renderInteractiveText(
  children: ReactNode,
  resolveFilePath: (path: string) => string | null,
  onOpenWorkspaceFile?: (path: string) => void,
): ReactNode {
  if (Array.isArray(children)) {
    return children.map((child, index) => (
      <Fragment key={index}>
        {renderInteractiveText(child, resolveFilePath, onOpenWorkspaceFile)}
      </Fragment>
    ));
  }

  if (typeof children !== 'string') {
    return children;
  }

  return children.split(WORKSPACE_FILE_PATTERN).map((part, index) => {
    if (!onOpenWorkspaceFile || !looksLikeWorkspaceFilePath(part)) {
      return <Fragment key={index}>{part}</Fragment>;
    }

    const normalizedPath = normalizeWorkspaceReference(part);
    const resolvedPath = resolveFilePath(normalizedPath);
    if (!looksLikeWorkspaceFilePath(normalizedPath) || !resolvedPath) {
      return <Fragment key={index}>{part}</Fragment>;
    }

    const startIndex = part.indexOf(normalizedPath);
    const prefix = startIndex > 0 ? part.slice(0, startIndex) : '';
    const suffix = part.slice(startIndex + normalizedPath.length);

    return (
      <Fragment key={index}>
        {prefix}
        <button
          className="inline-block max-w-full cursor-pointer rounded-md border border-primary/20 bg-primary/10 px-2 py-0.5 text-sm font-mono text-primary break-all whitespace-pre-wrap align-middle transition-colors hover:bg-primary/12 hover:border-primary/30"
          onClick={() => onOpenWorkspaceFile(resolvedPath)}
          title={`Open ${resolvedPath}`}
          type="button"
        >
          {normalizedPath}
        </button>
        {suffix}
      </Fragment>
    );
  });
}

export function MarkdownRenderer({ content, className, isStreaming = false, onOpenWorkspaceFile }: MarkdownRendererProps) {
  const currentAgentId = useAgentStore((state) => state.current_agent_id);
  const filesByAgent = useWorkspaceFilesStore((state) => state.filesByAgent);
  const resolveFilePath = (path: string) => resolveWorkspaceFileReference(path, currentAgentId, filesByAgent);

  return (
    <div className={cn("prose prose-sm max-w-none min-w-0 overflow-hidden break-words", className)}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath, remarkBreaks]}
        rehypePlugins={[rehypeKatex]}
        components={{
          code({ className, children, ...props }: any) {
            const match = /language-(\w+)/.exec(className || '');
            const value = String(children).replace(/\n$/, '');
            const isCodeBlock = match && value.includes('\n');
            const resolvedPath = resolveFilePath(value);

            return isCodeBlock ? (
              <CodeBlock language={match[1]} value={value} />
            ) : resolvedPath && onOpenWorkspaceFile ? (
              <button
                className="mx-0.5 inline-block max-w-full cursor-pointer rounded-md border border-primary/20 bg-primary/10 px-2 py-0.5 text-sm font-mono text-primary break-all whitespace-pre-wrap align-middle transition-colors hover:bg-primary/12 hover:border-primary/30"
                onClick={() => onOpenWorkspaceFile(resolvedPath)}
                title={`Open ${resolvedPath}`}
                type="button"
                {...props}
              >
                {children}
              </button>
            ) : (
              <span
                className="mx-0.5 inline-block max-w-full rounded-md border border-primary/20 bg-primary/10 px-2 py-0.5 text-sm font-mono text-primary break-all whitespace-pre-wrap align-middle"
                {...props}
              >
                {children}
              </span>
            );
          },
          p({ children }) {
            return (
              <p className="mb-2 mt-2 last:mb-0 break-words leading-relaxed text-foreground/90 [overflow-wrap:anywhere]">
                {renderInteractiveText(children, resolveFilePath, onOpenWorkspaceFile)}
              </p>
            );
          },
          ul({ children }) {
            return <ul className="mb-4 list-disc space-y-2 pl-6 text-foreground/90 marker:text-muted-foreground">{children}</ul>;
          },
          ol({ children }) {
            return <ol className="mb-4 list-decimal space-y-2 pl-6 text-foreground/90 marker:text-muted-foreground">{children}</ol>;
          },
          li({ children }) {
            return (
              <li className="pl-1 break-words leading-relaxed [overflow-wrap:anywhere] [&>p]:m-0 [&>p]:inline [&>p]:leading-relaxed">
                {renderInteractiveText(children, resolveFilePath, onOpenWorkspaceFile)}
              </li>
            );
          },
          blockquote({ children }) {
            return (
              <blockquote
                className="border-l-4 border-primary/30 pl-4 my-4 text-muted-foreground italic bg-primary/5 py-2 rounded-r">
                {renderInteractiveText(children, resolveFilePath, onOpenWorkspaceFile)}
              </blockquote>
            );
          },
          a({ href, children }) {
            return (
              <a
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                className="text-primary hover:underline decoration-primary/30 underline-offset-4 transition-all"
              >
                {children}
              </a>
            );
          },
          h1({ children }) {
            return <h1 className="text-2xl font-bold mb-4 mt-6 first:mt-0 text-foreground">{children}</h1>;
          },
          h2({ children }) {
            return <h2 className="text-xl font-bold mb-3 mt-5 text-foreground">{children}</h2>;
          },
          h3({ children }) {
            return <h3 className="text-lg font-bold mb-2 mt-4 text-foreground">{children}</h3>;
          },
          table({ children }) {
            return (
              <div className="neo-card-flat radius-shell-md my-4 overflow-x-auto">
                <table className="w-full text-sm text-left">{children}</table>
              </div>
            );
          },
          thead({ children }) {
            return <thead className="bg-muted/50 text-muted-foreground uppercase">{children}</thead>;
          },
          th({ children }) {
            return <th className="px-4 py-3 font-medium">{children}</th>;
          },
          td({ children }) {
            return <td className="border-t border-border px-4 py-3">{children}</td>;
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
