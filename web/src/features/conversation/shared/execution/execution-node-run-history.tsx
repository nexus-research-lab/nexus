/**
 * INPUT: GraphNode 下的有界 NodeRun 历史、结构化 workspace Artifact 与正式交付引用。
 * OUTPUT: 可展开的运行结果/错误时间线和可打开文件引用。
 * POS: 节点悬浮检查器的深入事实视图；不从摘要推断状态或触发重试。
 */
"use client";

import { useState } from "react";
import { ChevronDown, FileText } from "lucide-react";

import { WorkspaceFileArtifactBlock } from "@/features/conversation/shared/message/blocks/artifact/workspace-file-artifacts";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import type {
  ExecutionGraphNodeRunView,
  ExecutionGraphNodeView,
  ExecutionWorkItemView,
} from "@/types/conversation/execution";

import { resolveExecutionWorkspaceReference } from "./execution-workgraph-interaction-model";

const RUN_STATUS_LABEL_KEY: Record<string, TranslationKey> = {
  cancelled: "execution.attempt_cancelled",
  failed: "execution.attempt_failed",
  interrupted: "execution.attempt_interrupted",
  pending: "execution.attempt_pending",
  running: "execution.attempt_running",
  succeeded: "execution.attempt_succeeded",
  timed_out: "execution.attempt_timed_out",
};

export function ExecutionNodeRunHistory({
  item,
  node,
  onOpenWorkspaceFile,
  workspaceAgentId,
}: {
  item: ExecutionWorkItemView | null;
  node: ExecutionGraphNodeView;
  onOpenWorkspaceFile?: (
    path: string,
    workspaceAgentId?: string | null,
  ) => void;
  workspaceAgentId?: string | null;
}) {
  const { t } = useI18n();
  const runs = node.runs ?? [];
  const references = collectExecutionOutputReferences(item);
  if (runs.length === 0 && references.length === 0) {
    return null;
  }
  return (
    <section data-execution-node-run-history>
      {runs.length > 0 ? (
        <>
          <div className="mb-1 flex items-center justify-between gap-2">
            <h4 className="text-2xs font-medium text-(--text-soft)">
              {t("execution.run_history")}
            </h4>
            <span className="text-2xs tabular-nums text-(--text-soft)">
              {t("execution.run_history_count", { count: runs.length })}
            </span>
          </div>
          <div className="space-y-1.5">
            {runs.map((run, index) => (
              <ExecutionNodeRunDetail
                defaultOpen={index === runs.length - 1}
                key={run.id}
                onOpenWorkspaceFile={onOpenWorkspaceFile}
                run={run}
                workspaceAgentId={workspaceAgentId}
              />
            ))}
          </div>
        </>
      ) : null}
      {references.length > 0 ? (
        <div className={cn(runs.length > 0 && "mt-3")}>
          <h4 className="mb-1 text-2xs font-medium text-(--text-soft)">
            {t("execution.reference_outputs")}
          </h4>
          <ul className="space-y-1">
            {references.map((reference) => {
              const workspacePath = resolveExecutionWorkspaceReference(reference);
              const actionable = Boolean(workspacePath && onOpenWorkspaceFile);
              return (
                <li key={reference}>
                  <button
                    className={cn(
                      "flex w-full min-w-0 items-center gap-2 rounded-[8px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] px-2 py-1.5 text-left text-2xs",
                      actionable
                        ? "text-(--text-default) transition hover:bg-(--surface-interactive-hover-background)"
                        : "cursor-default text-(--text-soft)",
                    )}
                    disabled={!actionable}
                    onClick={() => {
                      if (workspacePath) {
                        onOpenWorkspaceFile?.(workspacePath, workspaceAgentId);
                      }
                    }}
                    title={reference}
                    type="button"
                  >
                    <FileText className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
                    <span className="message-cjk-code-font truncate">{reference}</span>
                  </button>
                </li>
              );
            })}
          </ul>
        </div>
      ) : null}
    </section>
  );
}

function ExecutionNodeRunDetail({
  defaultOpen,
  onOpenWorkspaceFile,
  run,
  workspaceAgentId,
}: {
  defaultOpen: boolean;
  onOpenWorkspaceFile?: (
    path: string,
    workspaceAgentId?: string | null,
  ) => void;
  run: ExecutionGraphNodeRunView;
  workspaceAgentId?: string | null;
}) {
  const { t } = useI18n();
  const [open, setOpen] = useState(defaultOpen);
  const status = run.status?.trim() ?? "";
  const statusLabel = RUN_STATUS_LABEL_KEY[status]
    ? t(RUN_STATUS_LABEL_KEY[status])
    : status;
  const timeLabel = formatExecutionRunTime(run);
  return (
    <details
      className="group rounded-[9px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-control-background)_64%,transparent)]"
      data-execution-node-run={run.id}
      onToggle={(event) => setOpen(event.currentTarget.open)}
      open={open}
    >
      <summary className="flex cursor-pointer list-none items-center gap-2 px-2 py-1.5 text-2xs [&::-webkit-details-marker]:hidden">
        <span
          aria-hidden="true"
          className={cn(
            "h-1.5 w-1.5 shrink-0 rounded-full bg-current",
            runStatusTone(status),
          )}
        />
        <span className="min-w-0 flex-1 truncate font-medium text-(--text-default)">
          {statusLabel || run.id}
        </span>
        {timeLabel ? (
          <span className="shrink-0 tabular-nums text-(--text-soft)">{timeLabel}</span>
        ) : null}
        <ChevronDown className="h-3 w-3 shrink-0 text-(--icon-muted) transition-transform group-open:rotate-180" />
      </summary>
      <div className="space-y-2 border-t dialog-divider px-2 py-2 text-2xs leading-4 text-(--text-default)">
        {run.error_summary?.trim() ? (
          <div className="rounded-[7px] bg-[color:color-mix(in_srgb,var(--warning)_8%,transparent)] px-2 py-1.5">
            <p>{run.error_summary.trim()}</p>
            {run.error_code?.trim() ? (
              <p className="mt-1 font-mono text-[9px] text-(--text-soft)">
                {run.error_code.trim()}
              </p>
            ) : null}
          </div>
        ) : null}
        {run.result_summary?.trim() ? <p>{run.result_summary.trim()}</p> : null}
        {run.summary_truncated ? (
          <p className="text-[9px] text-(--text-soft)">{t("execution.summary_truncated")}</p>
        ) : null}
        {(run.artifacts?.length ?? 0) > 0 ? (
          <div className="space-y-1.5 pt-0.5">
            <p className="text-[9px] font-medium text-(--text-soft)">
              {t("execution.artifacts")}
            </p>
            {run.artifacts?.map((artifact) => (
              <WorkspaceFileArtifactBlock
                artifact={{
                  ...artifact,
                  scope: "agentWorkspace",
                  workspace_agent_id: artifact.workspace_agent_id ?? workspaceAgentId,
                }}
                compact
                key={artifact.id || `${artifact.source_tool_use_id}:${artifact.path}`}
                onOpenWorkspaceFile={onOpenWorkspaceFile}
              />
            ))}
          </div>
        ) : null}
        {!run.error_summary?.trim()
          && !run.result_summary?.trim()
          && (run.artifacts?.length ?? 0) === 0 ? (
            <p className="text-(--text-soft)">{run.id}</p>
          ) : null}
      </div>
    </details>
  );
}

function collectExecutionOutputReferences(
  item: ExecutionWorkItemView | null,
): string[] {
  const values = [
    ...(item?.submission?.result_refs ?? []),
    ...(item?.submission?.evidence ?? []),
    ...(item?.acceptance?.criteria_results ?? []).flatMap(
      (criterion) => criterion.evidence ?? [],
    ),
  ];
  return [...new Set(values.map((value) => value.trim()).filter(Boolean))];
}

function formatExecutionRunTime(run: ExecutionGraphNodeRunView): string {
  if ((run.duration_ms ?? 0) > 0) {
    const milliseconds = run.duration_ms ?? 0;
    if (milliseconds < 1_000) {
      return `${Math.round(milliseconds)}ms`;
    }
    const seconds = milliseconds / 1_000;
    return seconds < 60
      ? `${seconds < 10 ? seconds.toFixed(1) : Math.round(seconds)}s`
      : `${Math.floor(seconds / 60)}m ${Math.round(seconds % 60)}s`;
  }
  const timestamp = run.finished_at ?? run.started_at;
  if (!timestamp) {
    return "";
  }
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function runStatusTone(status: string): string {
  if (status === "succeeded") {
    return "text-(--success)";
  }
  if (["failed", "interrupted", "timed_out"].includes(status)) {
    return "text-(--warning)";
  }
  if (status === "running") {
    return "text-(--primary)";
  }
  return "text-(--icon-muted)";
}
