/**
 * INPUT: GraphNode 下的有界 NodeRun 历史、结构化 workspace Artifact 与正式交付引用。
 * OUTPUT: 本地化运行历史、共享行内错误与来源节点工作区的文件引用；缺失事实不以内部身份冒充可读详情。
 * POS: 节点悬浮检查器的深入事实视图；不从摘要推断状态或触发重试。
 */
"use client";

import { FileText } from "lucide-react";

import { WorkspaceFileArtifactBlock } from "@/features/conversation/shared/message/blocks/artifact/workspace-file-artifacts";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type {
  ExecutionGraphNodeRunView,
  ExecutionGraphNodeView,
  ExecutionWorkItemView,
} from "@/types/conversation/execution";

import { resolveExecutionWorkspaceReference } from "./execution-workgraph-interaction-model";
import { formatExecutionRunTime, getExecutionRunStatusLabel } from "./execution-run-presentation";

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
              role: "metadata",
              tone: "default",
              weight: "medium",
            })}>
              {t("execution.run_history")}
            </h4>
            <span className={cn(
              "tabular-nums",
              getUiTypographyClassName({ role: "metadata", tone: "muted" }),
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
              role: "metadata",
              tone: "default",
              weight: "medium",
            }),
          )}>
            {t("execution.reference_outputs")}
          </h4>
          <ul className="space-y-1">
            {references.map((reference) => {
              const workspacePath = resolveExecutionWorkspaceReference(reference);
              const actionable = Boolean(workspacePath && workspaceAgentId?.trim() && onOpenWorkspaceFile);
              return (
                <li key={reference}>
                  <UiButton
                    className="w-full min-w-0 justify-start"
                    disabled={!actionable}
                    onClick={() => {
                      if (actionable && workspacePath) {
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
  const { locale, t } = useI18n();
  const status = run.status?.trim() ?? "";
  const statusLabel = getExecutionRunStatusLabel(status, t);
  const timeLabel = formatExecutionRunTime(run, { locale, t });
  const errorSummary = run.error_summary?.trim();
  const errorCode = run.error_code?.trim();
  return (
    <UiDisclosure
      data-execution-node-run={run.id}
      defaultOpen={defaultOpen}
      density="compact"
      label={statusLabel}
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
      summaryRole="supporting"
      surfaceTone="subtle"
      variant="panel"
    >
      <div className={cn(
        "space-y-2 break-words",
        getUiTypographyClassName({ role: "supporting", tone: "default" }),
      )}>
        {errorSummary || errorCode ? (
          <UiInlineNotice
            aria-live="off"
            message={(
              <>
                {errorSummary ? <p className="whitespace-pre-wrap">{errorSummary}</p> : null}
                {errorCode ? (
                  <p className={cn(
                    errorSummary && "mt-1",
                    getUiTypographyClassName({ role: "code", tone: "muted" }),
                  )}>
                    {errorCode}
                  </p>
                ) : null}
              </>
            )}
            role="note"
            tone="warning"
          />
        ) : null}
        {run.result_summary?.trim() ? <p className="whitespace-pre-wrap">{run.result_summary.trim()}</p> : null}
        {run.summary_truncated ? (
          <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>
            {t("execution.summary_truncated")}
          </p>
        ) : null}
        {(run.artifacts?.length ?? 0) > 0 ? (
          <div className="space-y-1.5 pt-0.5">
            <p className={getUiTypographyClassName({ role: "metadata", tone: "default", weight: "medium" })}>
              {t("execution.artifacts")}
            </p>
            {run.artifacts?.map((artifact) => (
              <WorkspaceFileArtifactBlock
                artifact={{
                  ...artifact,
                  scope: "agentWorkspace",
                }}
                compact
                key={artifact.id || `${artifact.source_tool_use_id}:${artifact.path}`}
                onOpenWorkspaceFile={onOpenWorkspaceFile}
                workspaceAgentId={workspaceAgentId}
              />
            ))}
          </div>
        ) : null}
        {!errorSummary && !errorCode
          && !run.result_summary?.trim()
          && (run.artifacts?.length ?? 0) === 0 ? (
            <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>
              {t("execution.run_details_unavailable")}
            </p>
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
