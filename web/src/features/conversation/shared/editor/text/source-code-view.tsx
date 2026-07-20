/**
 * INPUT: Workspace source text plus an optional tool-owned focus range or changed snippets.
 * OUTPUT: A scrollable source view with stable line numbers and truthful operation highlights.
 * POS: Shared text preview primitive used by the workspace editor and Operation Stage Editor app.
 */
import { useEffect, useRef } from "react";

import { cn } from "@/shared/ui/class-name";

import type { WorkspaceSourceFocus } from "../workspace-file-preview-types";
import { resolveSourceFocusRanges } from "./source-code-focus";

const FOCUS_ROW_CLASS: Record<WorkspaceSourceFocus["tone"], string> = {
  edit: "border-l-[#d39a45] bg-[#fff8ea] text-[#27303c]",
  error: "border-l-[#d8666f] bg-[#fff0f1] text-[#51242a]",
  read: "border-l-[#6585e8] bg-[#eef3ff] text-[#25324e]",
  write: "border-l-[#35a778] bg-[#edf9f4] text-[#203d33]",
};

const FOCUS_GUTTER_CLASS: Record<WorkspaceSourceFocus["tone"], string> = {
  edit: "text-[#a56d1e]",
  error: "text-[#bc4d58]",
  read: "text-[#526fd0]",
  write: "text-[#25835d]",
};

export function SourceCodeView({
  content,
  focus,
  isStreaming,
}: {
  content: string;
  focus?: WorkspaceSourceFocus | null;
  isStreaming: boolean;
}) {
  const firstFocusedLineRef = useRef<HTMLDivElement>(null);
  const lines = content.split("\n");
  const focus_ranges = resolveSourceFocusRanges(content, focus);
  const first_focused_line = focus_ranges.at(0)?.start ?? null;

  useEffect(() => {
    firstFocusedLineRef.current?.scrollIntoView({
      block: "center",
      inline: "nearest",
    });
  }, [first_focused_line]);

  return (
    <div
      aria-label="文件源码"
      className="soft-scrollbar h-full overflow-auto bg-[#fbfcfe] py-2 message-code-font"
      data-focus-tone={focus?.tone}
    >
      <div className="min-w-max text-[11px] leading-[1.65] text-[#34404f]">
        {lines.map((line, index) => {
          const line_number = index + 1;
          const is_focused = focus_ranges.some((range) => (
            line_number >= range.start && line_number <= range.end
          ));
          return (
            <div
              className={cn(
                "grid min-h-[18px] grid-cols-[46px_minmax(0,1fr)] border-l-2 border-l-transparent pr-5",
                is_focused && focus ? FOCUS_ROW_CLASS[focus.tone] : "",
              )}
              data-source-line={line_number}
              key={line_number}
              ref={line_number === first_focused_line ? firstFocusedLineRef : undefined}
            >
              <span className={cn(
                "select-none pr-3 text-right text-[#9aa5b3]",
                is_focused && focus ? FOCUS_GUTTER_CLASS[focus.tone] : "",
              )}>
                {line_number}
              </span>
              <span className="whitespace-pre">
                {line || "\u00a0"}
                {isStreaming && index === lines.length - 1 ? (
                  <span
                    aria-label="正在写入"
                    className="ml-0.5 inline-block h-[1.05em] w-[5px] animate-pulse bg-[#35a778] align-[-2px]"
                  />
                ) : null}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
