"use client";

import { memo } from "react";

import { CodeShell } from "./code-shell";

interface StreamingCodeBlockProps {
  language: string;
  value: string;
}

export const StreamingCodeBlock = memo(function StreamingCodeBlock({
  language,
  value,
}: StreamingCodeBlockProps) {
  return (
    <CodeShell
      language={language}
      right_slot={(
        <span className="message-cjk-code-font text-[11px]" style={{ color: "var(--text-muted)" }}>
          输出中
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
});
