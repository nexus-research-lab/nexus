"use client";

import { lazy, Suspense } from "react";

import { CodeShell } from "./code-shell";
import { StreamingCodeBlock } from "./streaming-code-block";

interface CodeBlockProps {
  language: string;
  isStreaming?: boolean;
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
      rightSlot={(
        <span className="message-code-font text-xs" style={{ color: "var(--text-muted)" }}>
          Loading
        </span>
      )}
      contentClassName="overflow-x-auto"
    >
      <pre
        className="message-code-font min-w-full whitespace-pre p-3.5 text-sm leading-relaxed"
        style={{ color: "var(--text-strong)" }}
      >
        {value}
      </pre>
    </CodeShell>
  );
}

export function CodeBlock({ language, value, isStreaming: isStreaming }: CodeBlockProps) {
  if (isStreaming) {
    return <StreamingCodeBlock language={language} value={value} />;
  }

  return (
    <Suspense fallback={<CodeBlockLoadingFallback language={language} value={value} />}>
      <LazyCodeBlockContent language={language} value={value} />
    </Suspense>
  );
}
