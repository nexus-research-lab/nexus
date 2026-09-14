// INPUT: Exact file contents as they grow and outer layout classes.
// OUTPUT: Read-only source text, localized logical line count and a decorative writing cursor.
// POS: Streaming file presentation; preserves bottom-follow behavior and owns no file commands.
"use client";

import { useEffect, useMemo, useRef } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UI_SOURCE_TEXT_CLASS_NAME } from "@/shared/ui/form/source-text-styles";

interface TypewriterFileViewProps {
  /** The full content being written (grows over time) */
  content: string;
  className?: string;
}

/**
 * Replaces the plain textarea when an agent is actively writing a file.
 *
 * Counts file lines rather than display wraps, so resizing never changes the count.
 * Source text shares the editor metrics and remains selectable without an overlay.
 */
export function TypewriterFileView({
  content,
  className,
}: TypewriterFileViewProps) {
  const { t } = useI18n();
  const lineCount = useMemo(() => content.split(/\r\n|\r|\n/).length, [content]);
  const preRef = useRef<HTMLPreElement>(null);

  // Scroll to bottom as content grows
  useEffect(() => {
    const el = preRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [content]);

  return (
    <div className={cn("flex h-full min-h-0 flex-col overflow-hidden", className)}>
      <div className="flex shrink-0 justify-end pb-2">
        <UiBadge className="tabular-nums" tone="running">
          {t(lineCount === 1 ? "common.source_line_count_one" : "common.source_line_count_other", { count: lineCount })}
        </UiBadge>
      </div>

      <pre
        ref={preRef}
        className={cn("soft-scrollbar min-h-0 min-w-0 flex-1 overflow-auto overscroll-contain whitespace-pre-wrap break-words text-(--text-default)", UI_SOURCE_TEXT_CLASS_NAME)}
      >
        {content}
        <span className="ui-source-write-cursor" aria-hidden="true" />
      </pre>
    </div>
  );
}
