"use client";

import { lazy, Suspense } from "react";

import { CodeShell } from "./code-shell";
import { StreamingCodeBlock } from "./streaming-code-block";

interface CodeBlockProps {
  language: string;
  is_streaming?: boolean;
  value: string;
}

const LazyCodeBlockContent = lazy(async () => {
  const module = await import("./code-block-content");
  return { default: module.CodeBlockContent };
});

function CodeBlockLoadingFallback({ language, value }: CodeBlockProps) {
  return (
    <CodeShell
      language={language}
      right_slot={(
        <span className="message-cjk-code-font text-[11px]" style={{ color: "var(--text-muted)" }}>
          Loading
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
    return <StreamingCodeBlock language={language} value={value} />;
  }

  return (
    <Suspense fallback={<CodeBlockLoadingFallback language={language} value={value} />}>
      <LazyCodeBlockContent language={language} value={value} />
    </Suspense>
  );
}
