"use client";

import type { ReactNode } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

const SKILL_MARKDOWN_CLASS_NAME =
  "text-sm leading-[1.9] text-(--text-default) [&>*:first-child]:mt-0 [&>*:last-child]:mb-0";

interface SkillMarkdownProps {
  markdown: string;
}

export function SkillMarkdown({ markdown }: SkillMarkdownProps) {
  return (
    <div className={SKILL_MARKDOWN_CLASS_NAME}>
      <ReactMarkdown
        components={{
          a: ({ children, href }) => (
            <a
              className="text-sky-500/92 underline decoration-sky-400/50 underline-offset-4"
              href={href}
              rel="noreferrer"
              target="_blank"
            >
              {children}
            </a>
          ),
          h1: ({ children }) => (
            <h1 className="mt-8 text-[28px] font-black tracking-[-0.04em] text-(--text-strong)">
              {children}
            </h1>
          ),
          h2: ({ children }) => (
            <h2 className="mt-7 text-[22px] font-bold tracking-[-0.03em] text-(--text-strong)">
              {children}
            </h2>
          ),
          h3: ({ children }) => (
            <h3 className="mt-6 text-[18px] font-bold text-(--text-strong)">
              {children}
            </h3>
          ),
          p: ({ children }) => (
            <p className="mt-4">
              {children}
            </p>
          ),
          ul: ({ children }) => (
            <ul className="mt-4 list-disc pl-6">
              {children}
            </ul>
          ),
          ol: ({ children }) => (
            <ol className="mt-4 list-decimal pl-6">
              {children}
            </ol>
          ),
          li: ({ children }) => (
            <li className="mt-[0.45rem] first:mt-0">
              {children}
            </li>
          ),
          pre: ({ children }) => (
            <pre className="mt-4 overflow-x-auto rounded-[18px] bg-(--surface-inset-background) border border-(--divider-subtle-color) p-4 text-xs text-(--text-default) shadow-[0_8px_20px_rgba(0,0,0,0.12)]">
              {children}
            </pre>
          ),
          code: ({ children, className }) => {
            if (className) {
              return (
                <code className={className}>
                  {children}
                </code>
              );
            }

            return (
              <code className="rounded-lg bg-(--surface-inset-background) border border-(--divider-subtle-color) px-[0.42rem] py-[0.16rem] text-[12px] text-(--text-default)">
                {children as ReactNode}
              </code>
            );
          },
        }}
        remarkPlugins={[remarkGfm]}
      >
        {markdown}
      </ReactMarkdown>
    </div>
  );
}
