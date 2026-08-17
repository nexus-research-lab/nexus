"use client";

import { Copy } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";

import {
  getRunDiagnosticRows,
  getRunOutputSections,
  type RunOutputSection,
} from "../scheduled-task-run-diagnostic-model";

interface ScheduledTaskRunDetailsProps {
  isCopied: boolean;
  onCopyDiagnostic: () => void | Promise<void>;
  run: ScheduledTaskRunItem;
}

export function ScheduledTaskRunDetails({
  isCopied,
  onCopyDiagnostic,
  run,
}: ScheduledTaskRunDetailsProps) {
  const diagnosticRows = getRunDiagnosticRows(run);
  const outputSections = getRunOutputSections(run);
  return (
    <>
      {outputSections.map((section, index) => (
        <RunOutput key={`${section.label ?? section.tone}:${index}`} section={section} />
      ))}
      <details className="mt-4 text-xs text-(--text-muted)">
        <summary className="cursor-pointer list-none font-medium text-(--text-default) hover:text-(--text-strong)">
          诊断详情
        </summary>
        <div className="mt-2 space-y-1.5 rounded-[10px] border border-(--divider-subtle-color) px-3 py-2.5">
          {diagnosticRows.map((row) => (
            <p className={cn(row.breakAll && "break-all")} key={row.label}>
              {row.label} {row.value}
            </p>
          ))}
          <button
            className="inline-flex items-center gap-1.5 pt-1 font-semibold text-(--text-default) hover:text-(--text-strong)"
            onClick={() => void onCopyDiagnostic()}
            type="button"
          >
            <Copy className="h-3.5 w-3.5" />
            {isCopied ? "已复制" : "复制诊断"}
          </button>
        </div>
      </details>
    </>
  );
}

function RunOutput({ section }: { section: RunOutputSection }) {
  if (section.tone === "default") {
    return (
      <div className="mt-3 min-w-0 text-sm text-(--text-default)">
        {section.label ? (
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
            {section.label}
          </p>
        ) : null}
        <UiMarkdownContent
          className={cn("text-sm", section.label && "mt-2")}
          content={section.content}
          mermaidShowHeader={false}
        />
      </div>
    );
  }
  return (
    <div className="mt-3 min-w-0 rounded-[8px] border border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] px-3 py-2.5 text-sm text-(--destructive)">
      {section.label ? (
        <p className="text-xs font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
          {section.label}
        </p>
      ) : null}
      <p className="whitespace-pre-wrap break-words leading-5">{section.content}</p>
    </div>
  );
}
