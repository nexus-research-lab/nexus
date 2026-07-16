"use client";

import type { Components } from "react-markdown";

import type { ResolveWorkspaceFilePath } from "../workspace/markdown-workspace-artifact-model";
import { createMarkdownComponents } from "./markdown-components";

interface CreateMarkdownSummaryComponentsOptions {
  monochrome?: boolean;
  strongAsText?: boolean;
}

export function createMarkdownSummaryComponents(
  resolveFilePath: ResolveWorkspaceFilePath,
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void,
  currentAgentId?: string | null,
  options: CreateMarkdownSummaryComponentsOptions = {},
): Components {
  const baseComponents = createMarkdownComponents(resolveFilePath, onOpenWorkspaceFile, currentAgentId);
  const headingClassName = options.monochrome
    ? `inline ${options.strongAsText ? "font-normal" : "font-medium"} text-inherit`
    : "inline font-medium text-foreground";

  return {
    ...baseComponents,
    // 摘要保留 Markdown 语义，但必须压成内联展示，避免列表项被正文块级样式撑高。
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
      return (
        <span className={options.monochrome
          ? "inline min-w-0 max-w-full italic text-inherit wrap-anywhere"
          : "inline min-w-0 max-w-full italic text-(--text-muted) wrap-anywhere"}
        >
          {children}
        </span>
      );
    },
    strong({ children }) {
      return options.strongAsText ? <span>{children}</span> : <strong>{children}</strong>;
    },
    a({ children }) {
      return <span className={options.monochrome ? "inline text-inherit" : "inline text-primary"}>{children}</span>;
    },
    code({ children }) {
      const value = String(children).replace(/\s+/g, " ").trim();
      return (
        <span className={options.monochrome
          ? "message-code-font inline text-[0.86em] text-inherit"
          : "message-code-font mx-0.5 inline rounded-[4px] bg-primary/10 px-1 text-[0.86em] text-primary"}
        >
          {value}
        </span>
      );
    },
    img({ alt }) {
      return alt ? <span className={options.monochrome ? "inline text-inherit" : "inline text-(--text-muted)"}>{alt}</span> : null;
    },
    ...(options.monochrome ? {
      kbd({ children }) {
        return <span className="inline text-inherit">{children}</span>;
      },
      mark({ children }) {
        return <span className="inline text-inherit">{children}</span>;
      },
    } satisfies Components : {}),
    h1({ children }) {
      return <span className={headingClassName}>{children}</span>;
    },
    h2({ children }) {
      return <span className={headingClassName}>{children}</span>;
    },
    h3({ children }) {
      return <span className={headingClassName}>{children}</span>;
    },
    h4({ children }) {
      return <span className={headingClassName}>{children}</span>;
    },
    h5({ children }) {
      return <span className={headingClassName}>{children}</span>;
    },
    h6({ children }) {
      return <span className={headingClassName}>{children}</span>;
    },
    hr() {
      return <span className={options.monochrome ? "inline text-inherit" : "inline text-(--text-soft)"}> · </span>;
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
