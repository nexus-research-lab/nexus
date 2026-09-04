/**
 * INPUT: GraphNode 下的有界 NodeRun 历史、结构化 workspace Artifact 与正式交付引用。
 * OUTPUT: 可展开的运行结果/错误时间线和可打开文件引用。
 * POS: 节点悬浮检查器的深入事实视图；不从摘要推断状态或触发重试。
 */
"use client";

import { FileText } from "lucide-react";

import { WorkspaceFileArtifactBlock } from "@/features/conversation/shared/message/blocks/artifact/workspace-file-artifacts";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
            <h4 className={getUiTypographyClassName({
              role: "caption",
              tone: "soft",
              weight: "medium",
            })}>
              {t("execution.run_history")}
            </h4>
            <span className={cn(
              "tabular-nums",
              getUiTypographyClassName({ role: "caption", tone: "soft" }),
            )}>
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
          <h4 className={cn(
            "mb-1",
            getUiTypographyClassName({
              role: "caption",
              tone: "soft",
              weight: "medium",
            }),
          )}>
            {t("execution.reference_outputs")}
          </h4>
          <ul className="space-y-1">
            {references.map((reference) => {
              const workspacePath = resolveExecutionWorkspaceReference(reference);
              const actionable = Boolean(workspacePath && onOpenWorkspaceFile);
              return (
                <li key={reference}>
                  <UiButton
                    className="w-full min-w-0 justify-start"
                    disabled={!actionable}
                    onClick={() => {
                      if (workspacePath) {
                        onOpenWorkspaceFile?.(workspacePath, workspaceAgentId);
                      }
                    }}
                    size="xs"
                    title={reference}
                    variant="surface"
                  >
                    <FileText className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
                    <span className={cn(
                      "message-cjk-code-font truncate",
                      getUiTypographyClassName({ role: "code", tone: "default" }),
                    )}>{reference}</span>
                  </UiButton>
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
  const status = run.status?.trim() ?? "";
  const statusLabel = RUN_STATUS_LABEL_KEY[status]
    ? t(RUN_STATUS_LABEL_KEY[status])
    : status;
  const timeLabel = formatExecutionRunTime(run);
  return (
    <UiDisclosure
      data-execution-node-run={run.id}
      defaultOpen={defaultOpen}
      density="compact"
      label={statusLabel || run.id}
      leading={(
        <span
          aria-hidden="true"
          className={cn(
            "h-1.5 w-1.5 shrink-0 rounded-full bg-current",
            runStatusTone(status),
          )}
        />
      )}
      meta={timeLabel ? <span className="tabular-nums">{timeLabel}</span> : null}
      summaryRole="caption"
      surfaceTone="subtle"
      variant="panel"
    >
      <div className={cn(
        "space-y-2",
        getUiTypographyClassName({ role: "caption", tone: "default" }),
      )}>
        {run.error_summary?.trim() ? (
          <div className="radius-control-xs bg-[color:color-mix(in_srgb,var(--warning)_8%,transparent)] px-2 py-1.5">
            <p>{run.error_summary.trim()}</p>
            {run.error_code?.trim() ? (
              <p className={cn(
                "mt-1",
                getUiTypographyClassName({ role: "code", tone: "soft" }),
              )}>
                {run.error_code.trim()}
              </p>
            ) : null}
          </div>
        ) : null}
        {run.result_summary?.trim() ? <p>{run.result_summary.trim()}</p> : null}
        {run.summary_truncated ? (
          <p className={getUiTypographyClassName({ role: "overline", tone: "soft" })}>
            {t("execution.summary_truncated")}
          </p>
        ) : null}
        {(run.artifacts?.length ?? 0) > 0 ? (
          <div className="space-y-1.5 pt-0.5">
            <p className={getUiTypographyClassName({ role: "overline", tone: "soft" })}>
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
    </UiDisclosure>
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
