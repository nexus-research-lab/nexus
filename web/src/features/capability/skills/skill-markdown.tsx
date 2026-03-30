"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

interface SkillMarkdownProps {
  markdown: string;
}

export function SkillMarkdown({ markdown }: SkillMarkdownProps) {
  return (
    <div className="skill-markdown text-[14px] leading-7 text-slate-700/85">
      <ReactMarkdown
        components={{
          h1: ({ children }) => (
            <h1 className="mt-8 text-[28px] font-black tracking-[-0.04em] text-slate-950/92 first:mt-0">
              {children}
            </h1>
          ),
          h2: ({ children }) => (
            <h2 className="mt-7 text-[22px] font-bold tracking-[-0.03em] text-slate-950/90">
              {children}
            </h2>
          ),
          h3: ({ children }) => (
            <h3 className="mt-6 text-[18px] font-bold text-slate-900/88">
              {children}
            </h3>
          ),
          p: ({ children }) => (
            <p className="mt-4 text-[14px] leading-7 text-slate-700/82">{children}</p>
          ),
          ul: ({ children }) => (
            <ul className="mt-4 list-disc space-y-2 pl-6 text-[14px] leading-7 text-slate-700/82">
              {children}
            </ul>
          ),
          ol: ({ children }) => (
            <ol className="mt-4 list-decimal space-y-2 pl-6 text-[14px] leading-7 text-slate-700/82">
              {children}
            </ol>
          ),
          li: ({ children }) => <li>{children}</li>,
          code: ({ children }) => (
            <code className="rounded bg-slate-100 px-1.5 py-0.5 text-[12px] text-slate-800">
              {children}
            </code>
          ),
          pre: ({ children }) => (
            <pre className="mt-4 overflow-x-auto rounded-[18px] bg-slate-950 px-4 py-4 text-[12px] text-slate-100">
              {children}
            </pre>
          ),
          a: ({ children, href }) => (
            <a
              className="text-sky-600 underline decoration-sky-300 underline-offset-4"
              href={href}
              rel="noreferrer"
              target="_blank"
            >
              {children}
            </a>
          ),
        }}
        remarkPlugins={[remarkGfm]}
      >
        {markdown}
      </ReactMarkdown>
    </div>
  );
}
