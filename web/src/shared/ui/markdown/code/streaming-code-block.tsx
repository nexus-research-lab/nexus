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
      rightSlot={(
        <span className="message-code-font text-[11px]" style={{ color: "var(--text-muted)" }}>
          输出中
        </span>
      )}
      contentClassName="overflow-x-auto"
    >
      <pre
        className="message-code-font min-w-full whitespace-pre px-3 py-2.5 text-[12px] leading-[1.5]"
        style={{ color: "var(--text-strong)" }}
      >
        {value}
      </pre>
    </CodeShell>
  );
});
