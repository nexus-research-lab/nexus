"use client";

import { lazy, Suspense } from "react";

interface CodeBlockProps {
  language: string;
  value: string;
}

const LazyCodeBlockContent = lazy(async () => {
  const module = await import("./code-block-content");
  return { default: module.CodeBlockContent };
});

function CodeBlockFallback({ language, value }: CodeBlockProps) {
  return (
    <div className="relative my-4 overflow-hidden rounded-[22px] border border-white/10 bg-[#1e1e1e] shadow-[0_22px_36px_rgba(17,24,39,0.28)]">
      <div className="flex items-center justify-between border-b border-white/5 bg-[#252526] px-4 py-2">
        <span className="text-xs font-mono text-muted-foreground">{language || "text"}</span>
        <span className="text-xs text-muted-foreground">Loading</span>
      </div>
      <pre className="overflow-x-auto p-6 text-sm leading-6 whitespace-pre-wrap break-words text-slate-100">
        {value}
      </pre>
    </div>
  );
}

export function CodeBlock({ language, value }: CodeBlockProps) {
  return (
    <Suspense fallback={<CodeBlockFallback language={language} value={value} />}>
      <LazyCodeBlockContent language={language} value={value} />
    </Suspense>
  );
}
