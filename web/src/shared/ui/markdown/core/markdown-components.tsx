/**
 * INPUT: Markdown AST 节点、Mermaid 展示选项；能力相关元素经 Context 读取受控 Workspace 能力与链接渲染槽。
 * OUTPUT: 按静态选项缓存、身份恒定的通用 Markdown 组件表。
 * POS: 共享 Markdown 渲染注册表；领域 URI 和业务状态由消费侧解释。
 */
"use client";

import { type Components } from "react-markdown";

import { MarkdownCode, MarkdownImage, MarkdownLink } from "./markdown-capability-elements";

interface CreateMarkdownComponentsOptions {
  compactMermaid?: boolean;
  showMermaidHeader?: boolean;
}

// react-markdown 把组件表里的函数当作元素类型：表身份一变，整段正文卸载重建，
// 重建期间的高度塌陷会让滚动容器夹紧 scrollTop。表只由静态选项决定并按选项缓存，
// 文件解析、预览和打开命令经 MarkdownCapabilitiesProvider 在渲染时读取。
const MARKDOWN_COMPONENTS_CACHE = new Map<string, Components>();

export function createMarkdownComponents(
  options: CreateMarkdownComponentsOptions = {},
): Components {
  const compactMermaid = options.compactMermaid ?? true;
  const cacheKey = `${compactMermaid}:${String(options.showMermaidHeader)}`;
  const cached = MARKDOWN_COMPONENTS_CACHE.get(cacheKey);
  if (cached) {
    return cached;
  }
  const components = buildMarkdownComponents(compactMermaid, options.showMermaidHeader);
  MARKDOWN_COMPONENTS_CACHE.set(cacheKey, components);
  return components;
}

function buildMarkdownComponents(
  compactMermaid: boolean,
  showMermaidHeader: boolean | undefined,
): Components {
  return {
    pre({ children }) {
      return <div className="w-full min-w-0 max-w-full overflow-hidden">{children}</div>;
    },
    code({ children, className, node }) {
      return (
        <MarkdownCode
          className={className}
          compactMermaid={compactMermaid}
          node={node}
          showMermaidHeader={showMermaidHeader}
        >
          {children}
        </MarkdownCode>
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
      return <MarkdownLink href={href}>{children}</MarkdownLink>;
    },
    img({ alt, src }) {
      return <MarkdownImage alt={alt} src={typeof src === "string" ? src : undefined} />;
    },
    h1({ children }) {
      // Streaming Markdown can change the first parsed block from a paragraph
      // to a heading. Keep its block geometry stable while the response grows.
      return <h1 data-markdown-anchor className="m-0 mt-3 -mb-1 max-w-full break-words text-[1.375rem] leading-[1.65rem] font-semibold text-foreground">{children}</h1>;
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
