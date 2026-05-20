"use client";

import { lazy, Suspense } from "react";

import { CodeShell } from "./code-shell";

interface CodeBlockProps {
  language: string;
  is_streaming?: boolean;
  value: string;
}

const LazyCodeBlockContent = lazy(async () => {
  const module = await import("./code-block-content");
  return { default: module.CodeBlockContent };
});

function CodeBlockFallback({ language, value, is_streaming }: CodeBlockProps) {
  return (
    <CodeShell
      language={language}
      right_slot={(
        <span className="message-cjk-code-font text-[11px]" style={{ color: "var(--text-muted)" }}>
          {is_streaming ? "输出中" : "Loading"}
        </span>
      )}
      content_class_name="overflow-x-auto"
    >
      <pre
        className="message-cjk-code-font min-w-full whitespace-pre px-4 py-3.5 text-[13px] leading-[1.6]"
        style={{ color: "var(--text-strong)" }}
      >
        {value}
      </pre>
    </CodeShell>
  );
}

export function CodeBlock({ language, value, is_streaming }: CodeBlockProps) {
  if (is_streaming) {
    return <CodeBlockFallback language={language} value={value} is_streaming />;
  }

  return (
    <Suspense fallback={<CodeBlockFallback language={language} value={value} />}>
      <LazyCodeBlockContent language={language} value={value} />
    </Suspense>
  );
}
