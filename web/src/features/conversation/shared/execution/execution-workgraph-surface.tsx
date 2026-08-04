/**
 * INPUT: Room/DM 共用 Execution resource、Agent 目录与精确 Agent round Task run。
 * OUTPUT: 右侧或移动端辅助面中的紧凑 WorkGraph 主视图。
 * POS: 底部节点轨迹之外的完整图入口；只消费同一权威资源，不另起轮询或状态机。
 */
"use client";

import { CircleAlert, LoaderCircle, RotateCw, Workflow } from "lucide-react";

import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";

import {
  EXECUTION_STATUS_LABEL_KEY,
  hasManagedExecutionGraph,
  resolveExecutionNodeSummary,
  type ExecutionAgentDirectory,
} from "./execution-process-model";
import { ExecutionWorkGraphCanvas } from "./execution-workgraph-canvas";
import type { ExecutionResource } from "./use-execution-resource";

export function ExecutionWorkGraphSurface({
  directory,
  resource,
  taskRuns,
}: {
  directory: ExecutionAgentDirectory;
  resource: ExecutionResource;
  taskRuns: readonly ConversationTaskRun[];
}) {
  const { t } = useI18n();
  const execution = resource.execution;
  const summary = execution ? resolveExecutionNodeSummary(execution) : null;
  const hasNodes = Boolean(
    hasManagedExecutionGraph(execution)
    && execution
    && ((execution.graph?.nodes?.length ?? 0) > 0
      || (execution.work_items?.length ?? 0) > 0),
  );

  return (
    <section
      aria-label={t("execution.label")}
      className="flex h-full min-h-0 min-w-0 flex-1 flex-col"
      data-execution-workgraph-surface
    >
      <header className="flex h-11 shrink-0 items-center gap-2 border-b dialog-divider px-3">
        <Workflow className="h-4 w-4 shrink-0 text-(--icon-muted)" />
        <span className="min-w-0 flex-1 truncate text-compact font-semibold text-(--text-strong)">
          {summary?.summary || t("execution.label")}
        </span>
        {execution ? (
          <span className="shrink-0 text-compact tabular-nums text-(--text-soft)">
            {summary && summary.totalCount > 0
              ? t("execution.node_progress", {
                  current: summary.currentStep,
                  total: summary.totalCount,
                })
              : t(EXECUTION_STATUS_LABEL_KEY[execution.status])}
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
          currentId={summary?.currentNode?.id ?? null}
          directory={directory}
          execution={execution}
          key={execution.id}
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
