"use client";

import { memo } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";

import { CodeShell } from "./code-shell";

interface StreamingCodeBlockProps {
  language: string;
  value: string;
}

export const StreamingCodeBlock = memo(function StreamingCodeBlock({
  language,
  value,
}: StreamingCodeBlockProps) {
  const { t } = useI18n();

  return (
    <CodeShell
      language={language}
      rightSlot={(
        <span className="message-code-font text-xs" style={{ color: "var(--text-muted)" }}>
          {t("markdown.code.streaming")}
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
});
