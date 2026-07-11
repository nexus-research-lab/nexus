"use client";

import { type Components } from "react-markdown";

import { getWorkspaceFilePreviewUrl } from "@/lib/api/agent-manage-api";

import { CodeBlock } from "../../blocks/code/code-block";
import { LazyMermaidView } from "../mermaid/lazy-mermaid-view";
import { WorkspaceFileButton } from "../workspace/markdown-workspace-file-button";
import {
  resolveWorkspaceArtifactPath,
  type ResolveWorkspaceFilePath,
} from "../workspace/markdown-workspace-artifacts";
import {
  compactExternalUrlLabel,
  getPlainTextFromChildren,
  normalizeExternalMarkdownHref,
  splitTrailingUrlPunctuation,
} from "./markdown-link-model";

type MarkdownNodeLike = {
  position?: {
    start?: { line?: number };
    end?: { line?: number };
  };
};

interface CreateMarkdownComponentsOptions {
  compactMermaid?: boolean;
  showMermaidHeader?: boolean;
  streamCodeBlocks?: boolean;
  streamMermaid?: boolean;
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
      const value = String(children).replace(/\n$/, "");
      if (isBlockCode(node as MarkdownNodeLike | undefined, className, value)) {
        const language = /language-(\w+)/.exec(className || "")?.[1] || "text";
        if (language.toLowerCase() === "mermaid" || language.toLowerCase() === "mmd") {
          return (
            <LazyMermaidView
              chart={value}
              compact={options.compactMermaid ?? true}
              isStreaming={options.streamMermaid}
              showHeader={options.showMermaidHeader}
            />
          );
        }
        return <CodeBlock language={language} value={value} isStreaming={options.streamCodeBlocks} />;
      }

      const resolvedPath = resolveWorkspaceArtifactPath(value, resolveFilePath);
      if (resolvedPath && onOpenWorkspaceFile) {
        return (
          <WorkspaceFileButton
            label={value}
            path={resolvedPath}
            onOpenWorkspaceFile={onOpenWorkspaceFile}
            workspaceAgentId={currentAgentId}
          />
        );
      }

      return (
        <span className="message-cjk-code-font mx-0.5 inline-flex max-w-full overflow-hidden rounded-[5px] border border-primary/20 bg-primary/10 px-2 py-0.3 align-middle text-[0.9em] text-primary">
          <span className="max-w-full whitespace-pre-wrap break-words">{value}</span>
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
      const rawHref = String(href ?? "").trim();
      if (!rawHref) {
        return <span className="text-primary">{children}</span>;
      }

      const resolvedPath = resolveWorkspaceArtifactPath(rawHref, resolveFilePath);
      if (resolvedPath && onOpenWorkspaceFile) {
        return (
          <WorkspaceFileButton
            label={children}
            path={resolvedPath}
            onOpenWorkspaceFile={onOpenWorkspaceFile}
            workspaceAgentId={currentAgentId}
          />
        );
      }

      if (rawHref.startsWith("#")) {
        return (
          <a
            className="inline max-w-full text-primary transition-all decoration-primary/30 underline-offset-4 break-words hover:underline"
            href={rawHref}
          >
            {children}
          </a>
        );
      }

      const { href: hrefWithoutTrailing, trailingText: trailingText } = splitTrailingUrlPunctuation(rawHref);
      const externalHref = normalizeExternalMarkdownHref(hrefWithoutTrailing);
      if (!externalHref) {
        return <span className="text-primary">{children}</span>;
      }

      const plainText = getPlainTextFromChildren(children);
      const plainTextHref = plainText
        ? normalizeExternalMarkdownHref(splitTrailingUrlPunctuation(plainText).href)
        : null;
      const linkChildren = plainText && (
        plainText === rawHref ||
        plainText === hrefWithoutTrailing ||
        plainText === externalHref ||
        plainTextHref === externalHref
      )
        ? compactExternalUrlLabel(externalHref)
        : children;

      return (
        <>
          <a
            className="inline max-w-full text-primary transition-all decoration-primary/30 underline-offset-4 break-words hover:underline"
            href={externalHref}
            rel="noopener noreferrer"
            target={externalHref.startsWith("mailto:") ? undefined : "_blank"}
            title={externalHref}
          >
            {linkChildren}
          </a>
          {trailingText}
        </>
      );
    },
    img({ alt, src }) {
      const rawSrc = String(src || "").trim();
      const resolvedPath = resolveWorkspaceArtifactPath(rawSrc, resolveFilePath);
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
