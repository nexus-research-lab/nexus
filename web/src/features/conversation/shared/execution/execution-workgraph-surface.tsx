/**
 * INPUT: Room/DM 共用 Execution resource、Agent 目录与精确 Agent round Task run。
 * OUTPUT: 以标题为主的 WorkGraph 主视图，仅在非 active 生命周期或投影异常时补充提示。
 * POS: 底部节点轨迹之外的完整图入口；只消费同一权威 ExecutionView，不解析 metadata 或另起状态机。
 */
"use client";

import { CircleAlert, LoaderCircle, RotateCw, Workflow } from "lucide-react";

import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import type { ExecutionStatus } from "@/types/conversation/execution";

import {
  hasExecutionGraph,
  hasManagedExecutionGraph,
  resolveExecutionWorkGraphHeaderModel,
  type ExecutionAgentDirectory,
} from "./execution-process-model";
import { ExecutionWorkGraphCanvas } from "./execution-workgraph-canvas";
import type { ExecutionResource } from "./use-execution-resource";

const EXECUTION_HEADER_STATUS_TONE: Record<ExecutionStatus, string> = {
  active: "border-[color:color-mix(in_srgb,var(--success)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_9%,transparent)] text-(--success)",
  waiting: "border-[color:color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_9%,transparent)] text-(--warning)",
  paused: "border-[color:color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_9%,transparent)] text-(--warning)",
  completed: "border-[color:color-mix(in_srgb,var(--success)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_9%,transparent)] text-(--success)",
  failed: "border-destructive/20 bg-destructive/10 text-destructive",
  cancelled: "border-(--surface-control-border) bg-(--surface-muted-background) text-(--text-soft)",
  superseded: "border-(--surface-control-border) bg-(--surface-muted-background) text-(--text-soft)",
};

export function ExecutionWorkGraphSurface({
  directory,
  onOpenWorkspaceFile,
  resource,
  taskRuns,
}: {
  directory: ExecutionAgentDirectory;
  onOpenWorkspaceFile?: (
    path: string,
    workspaceAgentId?: string | null,
  ) => void;
  resource: ExecutionResource;
  taskRuns: readonly ConversationTaskRun[];
}) {
  const { t } = useI18n();
  const execution = hasManagedExecutionGraph(resource.execution)
    ? resource.execution
    : null;
  const header = execution
    ? resolveExecutionWorkGraphHeaderModel(execution)
    : null;
  const hasNodes = hasExecutionGraph(execution);
  const runtimeProjectionPartial = Boolean(
    execution?.graph?.runtime_nodes_truncated
    || execution?.graph?.runtime_edges_truncated,
  );
  const lastSuccessfulAt = resource.lastSuccessfulAt
    ? new Date(resource.lastSuccessfulAt).toISOString()
    : null;

  return (
    <section
      aria-label={t("execution.label")}
      className="flex h-full min-h-0 min-w-0 flex-1 flex-col"
      data-execution-workgraph-surface
      data-execution-workgraph-stale={resource.isStale ? "true" : undefined}
      data-execution-workgraph-partial={runtimeProjectionPartial ? "true" : undefined}
      data-execution-last-successful-at={lastSuccessfulAt ?? undefined}
    >
      <header
        className="flex min-h-11 shrink-0 items-center gap-2 border-b dialog-divider px-3 py-2"
        data-execution-header-status={header?.status}
      >
        <Workflow className="h-4 w-4 shrink-0 text-(--icon-muted)" />
        <div className="min-w-0 flex-1 truncate text-compact font-semibold text-(--text-strong)">
          {header?.summary || t("execution.label")}
        </div>
        {header && header.status !== "active" ? (
          <span
            className={cn(
              "inline-flex shrink-0 items-center rounded-[6px] border px-1.5 text-[11px] font-semibold leading-4",
              EXECUTION_HEADER_STATUS_TONE[header.status],
            )}
            data-execution-header-notice-status={header.status}
          >
            {t(header.statusLabelKey)}
          </span>
        ) : null}
        {runtimeProjectionPartial ? (
          <span
            aria-label={t("execution.surface_partial", {
              nodes: execution?.graph?.runtime_node_total ?? 0,
              edges: execution?.graph?.runtime_edge_total ?? 0,
            })}
            className="flex shrink-0 items-center gap-1 rounded-full bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] px-1.5 py-0.5 text-[10px] font-medium text-(--warning)"
            title={t("execution.surface_partial", {
              nodes: execution?.graph?.runtime_node_total ?? 0,
              edges: execution?.graph?.runtime_edge_total ?? 0,
            })}
          >
            <CircleAlert aria-hidden="true" className="h-3 w-3" />
            <span>{t("execution.surface_partial_short")}</span>
          </span>
        ) : null}
        {resource.isStale ? (
          <span
            aria-label={t("execution.surface_stale")}
            className="flex shrink-0 items-center gap-1 rounded-full bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] px-1.5 py-0.5 text-[10px] font-medium text-(--warning)"
            title={lastSuccessfulAt
              ? t("execution.surface_stale_at", { time: lastSuccessfulAt })
              : t("execution.surface_stale")}
          >
            <CircleAlert aria-hidden="true" className="h-3 w-3" />
            <span>{t("execution.surface_stale_short")}</span>
          </span>
        ) : null}
        {resource.error ? (
          <UiIconButton
            aria-label={t("execution.refresh")}
            onClick={resource.refresh}
            size="sm"
            title={t("execution.refresh")}
            variant="ghost"
          >
            <RotateCw className="h-3.5 w-3.5" />
          </UiIconButton>
        ) : null}
      </header>

      {execution && hasNodes ? (
        <ExecutionWorkGraphCanvas
          currentId={header?.currentNodeId ?? null}
          directory={directory}
          execution={execution}
          key={execution.id}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          taskRuns={taskRuns}
        />
      ) : (
        <div className="grid min-h-0 flex-1 place-items-center px-6 py-8 text-center">
          <div className="flex max-w-64 flex-col items-center gap-2 text-(--text-soft)">
            {resource.isLoading ? (
              <LoaderCircle className="h-5 w-5 animate-spin text-(--icon-muted)" />
            ) : resource.error ? (
              <CircleAlert className="h-5 w-5 text-(--warning)" />
            ) : (
              <Workflow className="h-5 w-5 text-(--icon-muted)" />
            )}
            <p className="text-compact leading-5">
              {resource.isLoading
                ? t("execution.surface_loading")
                : resource.error
                ? t("execution.surface_error")
                : t("execution.surface_empty")}
            </p>
          </div>
        </div>
      )}
    </section>
  );
}
