/**
 * =====================================================
 * @File   : markdown-renderer-shared.tsx
 * @Date   : 2026-04-05 15:26
 * @Author : leemysw
 * 2026-04-05 15:26   Create
 * =====================================================
 */

/* eslint-disable react-refresh/only-export-components */

"use client";

import { useCallback, useMemo, type ReactNode } from "react";
import { type Components } from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkBreaks from "remark-breaks";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

import { useAgentStore } from "@/store/agent";
import { useWorkspaceFilesStore } from "@/store/workspace-files";
import { type WorkspaceFileEntry } from "@/types/agent/agent";

import { CodeBlock } from "../blocks/code-block";

type MarkdownNodeLike = {
  position?: {
    start?: { line?: number };
    end?: { line?: number };
  };
};

type ResolveWorkspaceFilePath = (value: string) => string | null;

const WORKSPACE_FILE_PATTERN = /([A-Za-z0-9_./-]+\.[A-Za-z0-9]{1,10})/g;
const WORKSPACE_ABSOLUTE_FILE_PATTERN = /(?<path>\/[^\s`"'，。；！？]+\/\.nexus\/workspace\/(?<agent>[^/\s`"'，。；！？]+)\/(?<relative>[^\s`"'，。；！？]+\.[A-Za-z0-9]{1,10}))/;
const SAVED_FILE_LINE_PATTERN = /^(?<prefix>.*?(?:已保存到|保存到|写入到|生成到|created at|saved to|written to)\s*)[`"']?(?<path>\/[^\s`"'，。；！？]+\/\.nexus\/workspace\/[^/\s`"'，。；！？]+\/[^\s`"'，。；！？]+\.[A-Za-z0-9]{1,10}|[A-Za-z0-9_.-][A-Za-z0-9_./-]*\.[A-Za-z0-9]{1,10})[`"']?(?<suffix>.*)$/i;

export const MARKDOWN_PLUGINS = [remarkGfm, remarkMath, remarkBreaks];
export const REHYPE_PLUGINS = [rehypeKatex];
export const MARKDOWN_BODY_CLASS_NAME = "message-cjk-font w-full min-w-0 max-w-full overflow-x-hidden text-[15px] leading-7 text-(--text-strong) [&_strong]:font-semibold [&_strong]:text-(--text-strong) [&_em]:italic [&_hr]:my-4 [&_hr]:border-(--divider-subtle-color)";
export const MARKDOWN_SUMMARY_CLASS_NAME = "message-cjk-font w-full min-w-0 max-w-full overflow-hidden text-[15px] leading-7 text-(--text-strong) [&_strong]:font-semibold [&_strong]:text-(--text-strong) [&_em]:italic";

export interface MarkdownTextSegment {
  type: "text";
  text: string;
}

export interface MarkdownFileArtifactSegment {
  type: "file_artifact";
  label: string;
  path: string;
  display_path: string;
}

export type MarkdownContentSegment = MarkdownTextSegment | MarkdownFileArtifactSegment;

function normalize_workspace_reference(value: string): string {
  return value.replace(/^[("'`【]+|[)"'`】,，。；：:!?]+$/g, "");
}

function looks_like_workspace_file_reference(value: string): boolean {
  if (!value.includes(".") || /^https?:\/\//i.test(value) || value.startsWith("/")) {
    return false;
  }

  return /[A-Za-z0-9]/.test(value);
}

function resolve_workspace_file_reference(value: string, files: WorkspaceFileEntry[]): string | null {
  const normalized = normalize_workspace_reference(value);
  if (!looks_like_workspace_file_reference(normalized)) {
    return null;
  }

  const candidate_files = files.filter((entry) => !entry.is_dir);
  const exact_match = candidate_files.find((entry) => entry.path === normalized);
  if (exact_match) {
    return exact_match.path;
  }

  const basename_matches = candidate_files.filter((entry) => entry.name === normalized);
  return basename_matches.length === 1 ? basename_matches[0].path : null;
}

function display_workspace_artifact_path(path: string): string {
  const normalized = normalize_workspace_reference(path).replace(/\\/g, "/");
  const match = WORKSPACE_ABSOLUTE_FILE_PATTERN.exec(normalized);
  if (!match?.groups?.agent || !match.groups.relative) {
    return normalized.replace(/^\.\//, "");
  }
  return `${match.groups.agent}/${match.groups.relative}`;
}

function clickable_workspace_artifact_path(path: string): string {
  const normalized = normalize_workspace_reference(path).replace(/\\/g, "/");
  const match = WORKSPACE_ABSOLUTE_FILE_PATTERN.exec(normalized);
  if (!match?.groups?.relative) {
    return normalized.replace(/^\.\//, "");
  }
  return match.groups.relative;
}

function resolve_workspace_artifact_path(
  path: string,
  resolve_file_path: ResolveWorkspaceFilePath,
): string | null {
  const normalized = normalize_workspace_reference(path).replace(/\\/g, "/");
  if (WORKSPACE_ABSOLUTE_FILE_PATTERN.test(normalized)) {
    return clickable_workspace_artifact_path(normalized);
  }
  return resolve_file_path(normalized);
}

function normalize_artifact_label(prefix: string): string {
  const label = prefix.trim().replace(/[：:，,]$/, "").trim();
  return label || "已保存到";
}

export function split_markdown_file_artifacts(
  content: string,
  resolve_file_path: ResolveWorkspaceFilePath,
): MarkdownContentSegment[] {
  const segments: MarkdownContentSegment[] = [];
  const pending_text: string[] = [];

  const flush_text = () => {
    if (pending_text.length === 0) {
      return;
    }
    segments.push({ type: "text", text: pending_text.join("\n") });
    pending_text.length = 0;
  };

  for (const line of content.split("\n")) {
    const match = SAVED_FILE_LINE_PATTERN.exec(line.trim());
    const absolute_match = WORKSPACE_ABSOLUTE_FILE_PATTERN.exec(line.trim());
    const path = match?.groups?.path ?? absolute_match?.groups?.path;
    if (!path) {
      pending_text.push(line);
      continue;
    }

    const resolved_path = resolve_workspace_artifact_path(path, resolve_file_path);
    if (!resolved_path) {
      pending_text.push(line);
      continue;
    }

    flush_text();
    segments.push({
      type: "file_artifact",
      label: match?.groups?.prefix ? normalize_artifact_label(match.groups.prefix) : "文件",
      path: resolved_path,
      display_path: display_workspace_artifact_path(path),
    });

    const suffix = match?.groups?.suffix?.trim() ?? "";
    if (suffix && /[\p{L}\p{N}]/u.test(suffix)) {
      pending_text.push(suffix);
    }
  }

  flush_text();
  return segments.length > 0 ? segments : [{ type: "text", text: content }];
}

function is_block_code(node: MarkdownNodeLike | null | undefined, class_name: string | undefined, value: string): boolean {
  if (class_name && /language-\w+/.test(class_name)) {
    return true;
  }

  if (value.includes("\n")) {
    return true;
  }

  const start_line = node?.position?.start?.line;
  const end_line = node?.position?.end?.line;
  return typeof start_line === "number" && typeof end_line === "number" && start_line !== end_line;
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
      className="message-cjk-code-font inline-flex max-w-full items-center overflow-hidden rounded-[5px] border border-primary/20 bg-primary/10 px-2 py-0.4 text-left align-middle text-[13px] text-primary transition-colors hover:border-primary/30 hover:bg-primary/15"
      onClick={() => on_open_workspace_file(path)}
      title={`Open ${path}`}
      type="button"
    >
      <span className="max-w-full whitespace-pre-wrap break-words">{label}</span>
    </button>
  );
}

export function useMarkdownFileResolver(): ResolveWorkspaceFilePath {
  const current_agent_id = useAgentStore((state) => state.current_agent_id);
  const files_by_agent = useWorkspaceFilesStore((state) => state.files_by_agent);
  const agent_files = useMemo(
    () => files_by_agent[current_agent_id ?? ""] ?? [],
    [current_agent_id, files_by_agent],
  );

  return useCallback(
    (value: string) => resolve_workspace_file_reference(value, agent_files),
    [agent_files],
  );
}

export function normalize_markdown_content(
  content: string,
  resolve_file_path: ResolveWorkspaceFilePath,
  on_open_workspace_file?: (path: string) => void,
): string {
  return content.replace(WORKSPACE_FILE_PATTERN, (match) => {
    const resolved_path = resolve_file_path(match);
    return resolved_path && on_open_workspace_file ? `\`${match}\`` : match;
  });
}

export function create_markdown_components(
  resolve_file_path: ResolveWorkspaceFilePath,
  on_open_workspace_file?: (path: string) => void,
): Components {
  return {
    pre({ children }) {
      return <div className="my-2 w-full min-w-0 max-w-full overflow-hidden">{children}</div>;
    },
    code({ children, className, node }) {
      const value = String(children).replace(/\n$/, "");
      if (is_block_code(node as MarkdownNodeLike | undefined, className, value)) {
        const language = /language-(\w+)/.exec(className || "")?.[1] || "text";
        return <CodeBlock language={language} value={value} />;
      }

      const resolved_path = resolve_file_path(value);
      if (resolved_path && on_open_workspace_file) {
        return (
          <WorkspaceFileButton
            label={value}
            path={resolved_path}
            on_open_workspace_file={on_open_workspace_file}
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
      return <div data-markdown-anchor className="mb-2 mt-2 min-w-0 max-w-full leading-relaxed text-foreground/90 wrap-anywhere last:mb-0">{children}</div>;
    },
    ul({ children }) {
      return <ul className="mb-4 max-w-full list-inside list-disc space-y-2 text-foreground/90 marker:text-muted-foreground">{children}</ul>;
    },
    ol({ children }) {
      return <ol className="mb-4 max-w-full list-inside list-decimal space-y-2 text-foreground/90 marker:font-medium marker:tabular-nums marker:text-muted-foreground">{children}</ol>;
    },
    li({ children }) {
      return <li data-markdown-anchor className="max-w-full overflow-visible leading-relaxed wrap-anywhere [&>p]:m-0 [&>p]:leading-relaxed">{children}</li>;
    },
    blockquote({ children }) {
      return (
        <blockquote data-markdown-anchor className="my-4 w-full min-w-0 max-w-full overflow-hidden border-l-[3px] border-primary/40 bg-primary/4 px-1 py-2 pl-4 italic text-(--text-muted) wrap-anywhere">
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
      return <img alt={alt || ""} className="my-4 h-auto max-w-full rounded-[18px] border border-white/40 object-cover" loading="lazy" src={src || ""} />;
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
    table({ children }) {
      return <table className="my-4 w-full max-w-full table-fixed border-collapse overflow-hidden rounded-[14px] border border-(--divider-subtle-color) text-left text-sm sm:table-auto">{children}</table>;
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
      return <th data-markdown-anchor className="border-b px-3 py-2 font-semibold sm:px-4 sm:py-3" style={{ borderColor: "var(--divider-subtle-color)" }}>{children}</th>;
    },
    td({ children }) {
      return <td data-markdown-anchor className="border-t border-b px-3 py-2 align-top whitespace-normal break-words sm:px-4 sm:py-3" style={{ borderColor: "var(--divider-subtle-color)" }}>{children}</td>;
    },
  };
}

export function create_markdown_summary_components(
  resolve_file_path: ResolveWorkspaceFilePath,
  on_open_workspace_file?: (path: string) => void,
): Components {
  const base_components = create_markdown_components(resolve_file_path, on_open_workspace_file);

  return {
    ...base_components,
    // 主时间线摘要需要保留 Markdown 的基础语义，但必须压成单行内联展示，
    // 不能再沿用正文里的块级布局，否则会把占位卡撑高并造成跳动。
    p({ children }) {
      return <span className="inline min-w-0 max-w-full wrap-anywhere">{children}</span>;
    },
    ul({ children }) {
      return <span className="inline min-w-0 max-w-full wrap-anywhere">{children}</span>;
    },
    ol({ children }) {
      return <span className="inline min-w-0 max-w-full wrap-anywhere">{children}</span>;
    },
    li({ children }) {
      return <span className="inline min-w-0 max-w-full wrap-anywhere [&_p]:inline [&_p]:m-0">• {children} </span>;
    },
    blockquote({ children }) {
      return <span className="inline min-w-0 max-w-full italic text-(--text-muted) wrap-anywhere">{children}</span>;
    },
    h1({ children }) {
      return <span className="inline font-semibold text-foreground">{children}</span>;
    },
    h2({ children }) {
      return <span className="inline font-semibold text-foreground">{children}</span>;
    },
    h3({ children }) {
      return <span className="inline font-semibold text-foreground">{children}</span>;
    },
    table({ children }) {
      return <span className="inline min-w-0 max-w-full wrap-anywhere">{children}</span>;
    },
    thead({ children }) {
      return <span className="inline">{children}</span>;
    },
    tbody({ children }) {
      return <span className="inline">{children}</span>;
    },
    tr({ children }) {
      return <span className="inline">{children}</span>;
    },
    th({ children }) {
      return <span className="inline font-medium">{children}</span>;
    },
    td({ children }) {
      return <span className="inline">{children}</span>;
    },
    pre({ children }) {
      return <span className="inline min-w-0 max-w-full overflow-hidden">{children}</span>;
    },
    br() {
      return <span>{" "}</span>;
    },
  };
}
