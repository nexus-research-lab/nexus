"use client";

import { type ReactNode } from "react";
import { type Components } from "react-markdown";

import { get_workspace_file_preview_url } from "@/lib/api/agent-manage-api";

import { CodeBlock } from "../blocks/code-block";
import { LazyMermaidView } from "./lazy-mermaid-view";
import { WorkspaceFileButton } from "./markdown-workspace-file-button";
import {
  resolve_workspace_artifact_path,
  type ResolveWorkspaceFilePath,
} from "./markdown-workspace-artifacts";

type MarkdownNodeLike = {
  position?: {
    start?: { line?: number };
    end?: { line?: number };
  };
};

interface CreateMarkdownComponentsOptions {
  compact_mermaid?: boolean;
  show_mermaid_header?: boolean;
  stream_code_blocks?: boolean;
  stream_mermaid?: boolean;
}

const URL_TRAILING_PUNCTUATION_PATTERN = /[.,;:!?，。；：！？、]+$/u;
const ALLOWED_MARKDOWN_LINK_PROTOCOLS = new Set(["http:", "https:", "mailto:"]);

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

function get_plain_text_from_children(children: ReactNode): string | null {
  if (typeof children === "string" || typeof children === "number") {
    return String(children);
  }

  if (!Array.isArray(children)) {
    return null;
  }

  const parts: string[] = [];
  for (const child of children) {
    if (typeof child === "string" || typeof child === "number") {
      parts.push(String(child));
      continue;
    }
    if (child === null || child === undefined || typeof child === "boolean") {
      continue;
    }
    return null;
  }

  return parts.join("");
}

function count_char(value: string, char: string): number {
  return Array.from(value).filter((item) => item === char).length;
}

function split_trailing_url_punctuation(value: string): { href: string; trailing_text: string } {
  let href = value.trim();
  let trailing_text = "";

  const append_trailing = (text: string) => {
    trailing_text = `${text}${trailing_text}`;
  };

  while (href) {
    const punctuation_match = URL_TRAILING_PUNCTUATION_PATTERN.exec(href);
    if (punctuation_match?.[0]) {
      href = href.slice(0, -punctuation_match[0].length);
      append_trailing(punctuation_match[0]);
      continue;
    }

    const last_char = href.at(-1);
    if (
      last_char === ")" &&
      count_char(href, ")") > count_char(href, "(")
    ) {
      href = href.slice(0, -1);
      append_trailing(")");
      continue;
    }

    if (
      last_char === "]" &&
      count_char(href, "]") > count_char(href, "[")
    ) {
      href = href.slice(0, -1);
      append_trailing("]");
      continue;
    }

    break;
  }

  return { href, trailing_text };
}

function normalize_external_markdown_href(href: string): string | null {
  const trimmed = href.trim();
  if (!trimmed) {
    return null;
  }

  const normalized = /^www\./i.test(trimmed) ? `https://${trimmed}` : trimmed;
  try {
    const url = new URL(normalized);
    return ALLOWED_MARKDOWN_LINK_PROTOCOLS.has(url.protocol) ? url.toString() : null;
  } catch {
    return null;
  }
}

function compact_external_url_label(href: string): string {
  if (href.startsWith("mailto:")) {
    return href.slice("mailto:".length);
  }

  try {
    const url = new URL(href);
    const host = url.hostname.replace(/^www\./i, "");
    const suffix = `${url.pathname === "/" ? "" : url.pathname}${url.search ? "?..." : ""}${url.hash ? "#..." : ""}`;
    const label = `${host}${suffix}`;
    return label.length > 64 ? `${label.slice(0, 42)}...${label.slice(-16)}` : label;
  } catch {
    return href;
  }
}

export function create_markdown_components(
  resolve_file_path: ResolveWorkspaceFilePath,
  on_open_workspace_file?: (path: string) => void,
  current_agent_id?: string | null,
  options: CreateMarkdownComponentsOptions = {},
): Components {
  return {
    pre({ children }) {
      return <div className="my-2 w-full min-w-0 max-w-full overflow-hidden">{children}</div>;
    },
    code({ children, className, node }) {
      const value = String(children).replace(/\n$/, "");
      if (is_block_code(node as MarkdownNodeLike | undefined, className, value)) {
        const language = /language-(\w+)/.exec(className || "")?.[1] || "text";
        if (language.toLowerCase() === "mermaid" || language.toLowerCase() === "mmd") {
          return (
            <LazyMermaidView
              chart={value}
              compact={options.compact_mermaid ?? true}
              is_streaming={options.stream_mermaid}
              show_header={options.show_mermaid_header}
            />
          );
        }
        return <CodeBlock language={language} value={value} is_streaming={options.stream_code_blocks} />;
      }

      const resolved_path = resolve_workspace_artifact_path(value, resolve_file_path);
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
      const raw_href = String(href ?? "").trim();
      if (!raw_href) {
        return <span className="text-primary">{children}</span>;
      }

      const resolved_path = resolve_workspace_artifact_path(raw_href, resolve_file_path);
      if (resolved_path && on_open_workspace_file) {
        return (
          <WorkspaceFileButton
            label={children}
            path={resolved_path}
            on_open_workspace_file={on_open_workspace_file}
          />
        );
      }

      if (raw_href.startsWith("#")) {
        return (
          <a
            className="inline max-w-full text-primary transition-all decoration-primary/30 underline-offset-4 break-words hover:underline"
            href={raw_href}
          >
            {children}
          </a>
        );
      }

      const { href: href_without_trailing, trailing_text } = split_trailing_url_punctuation(raw_href);
      const external_href = normalize_external_markdown_href(href_without_trailing);
      if (!external_href) {
        return <span className="text-primary">{children}</span>;
      }

      const plain_text = get_plain_text_from_children(children);
      const plain_text_href = plain_text
        ? normalize_external_markdown_href(split_trailing_url_punctuation(plain_text).href)
        : null;
      const link_children = plain_text && (
        plain_text === raw_href ||
        plain_text === href_without_trailing ||
        plain_text === external_href ||
        plain_text_href === external_href
      )
        ? compact_external_url_label(external_href)
        : children;

      return (
        <>
          <a
            className="inline max-w-full text-primary transition-all decoration-primary/30 underline-offset-4 break-words hover:underline"
            href={external_href}
            rel="noopener noreferrer"
            target={external_href.startsWith("mailto:") ? undefined : "_blank"}
            title={external_href}
          >
            {link_children}
          </a>
          {trailing_text}
        </>
      );
    },
    img({ alt, src }) {
      const raw_src = String(src || "").trim();
      const resolved_path = resolve_workspace_artifact_path(raw_src, resolve_file_path);
      const image_src = resolved_path && current_agent_id
        ? get_workspace_file_preview_url(current_agent_id, resolved_path)
        : raw_src;
      const image = (
        <img
          alt={alt || ""}
          className="my-3 block h-auto max-h-[420px] w-auto max-w-full rounded-[8px] border border-(--divider-subtle-color) object-contain sm:max-w-[560px]"
          loading="lazy"
          src={image_src}
        />
      );

      if (resolved_path && on_open_workspace_file) {
        return (
          <button
            className="block w-fit max-w-full text-left"
            onClick={() => on_open_workspace_file(resolved_path)}
            title={resolved_path}
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

export function create_markdown_summary_components(
  resolve_file_path: ResolveWorkspaceFilePath,
  on_open_workspace_file?: (path: string) => void,
  current_agent_id?: string | null,
): Components {
  const base_components = create_markdown_components(resolve_file_path, on_open_workspace_file, current_agent_id);

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
