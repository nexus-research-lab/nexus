// INPUT: 代码语言、原文与流式状态。
// OUTPUT: 流式正文或延迟语法高亮，加载说明按当前语言显示。
// POS: 代码渲染入口；外壳与操作交给公共 CodeShell。
"use client";

import { lazy, Suspense } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";

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
  const { t } = useI18n();
  return (
    <CodeShell
      language={language}
      rightSlot={(
        <span className="message-code-font text-xs" style={{ color: "var(--text-muted)" }}>
          {t("common.loading")}
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

export function CodeBlock({ language, value, isStreaming }: CodeBlockProps) {
  if (isStreaming) {
    return <StreamingCodeBlock language={language} value={value} />;
  }

  return (
    <Suspense fallback={<CodeBlockLoadingFallback language={language} value={value} />}>
      <LazyCodeBlockContent language={language} value={value} />
    </Suspense>
  );
}
