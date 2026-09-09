// INPUT: 代码语言、原文与流式状态。
// OUTPUT: 流式正文或延迟语法高亮，流式与加载占位共用纯文本视图，状态按当前语言显示。
// POS: 代码渲染入口；外壳与操作交给公共 CodeShell。
"use client";

import { lazy, memo, Suspense } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";

import { CodeShell } from "./code-shell";

interface CodeBlockProps {
  language: string;
  isStreaming?: boolean;
  value: string;
}

const LazyCodeBlockContent = lazy(async () => {
  const module = await import("./code-block-content");
  return { default: module.CodeBlockContent };
});

const PlainCodeBlock = memo(function PlainCodeBlock({ language, value, isStreaming }: CodeBlockProps) {
  const { t } = useI18n();
  return (
    <CodeShell
      language={language}
      rightSlot={(
        <span className="message-code-font text-xs text-(--text-muted)">
          {t(isStreaming ? "markdown.code.streaming" : "common.loading")}
        </span>
      )}
      contentClassName="overflow-x-auto"
    >
      <pre
        className="message-code-font min-w-full whitespace-pre p-3.5 text-sm leading-relaxed text-(--text-strong)"
      >
        {value}
      </pre>
    </CodeShell>
  );
});

export function CodeBlock({ language, value, isStreaming }: CodeBlockProps) {
  if (isStreaming) {
    return <PlainCodeBlock language={language} value={value} isStreaming />;
  }

  return (
    <Suspense fallback={<PlainCodeBlock language={language} value={value} />}>
      <LazyCodeBlockContent language={language} value={value} />
    </Suspense>
  );
}
