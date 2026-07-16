"use client";

import { Copy } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
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
      <details className="mt-3 text-xs text-(--text-muted)">
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
      {outputSections.map((section, index) => (
        <RunOutput key={`${section.label ?? section.tone}:${index}`} section={section} />
      ))}
    </>
  );
}

function RunOutput({ section }: { section: RunOutputSection }) {
  return (
    <div className={cn(
      "mt-3 rounded-[8px] border px-3 py-2.5 text-[13px]",
      section.tone === "danger"
        ? "border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] text-(--destructive)"
        : "border-(--divider-subtle-color) text-(--text-default)",
    )}>
      {section.label ? (
        <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
          {section.label}
        </p>
      ) : null}
      {section.label ? (
        <pre className="mt-2 max-h-64 whitespace-pre-wrap break-words leading-5">
          {section.content}
        </pre>
      ) : (
        <p className="leading-5">{section.content}</p>
      )}
    </div>
  );
}
