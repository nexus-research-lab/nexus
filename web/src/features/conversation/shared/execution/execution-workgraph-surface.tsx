/**
 * INPUT: Room/DM 共用 Execution resource、Agent 目录与精确 Agent round Task run。
 * OUTPUT: 以标题旁唯一的下拉入口展示当前项并切换已有历史的 WorkGraph 主视图。
 * POS: 底部节点轨迹之外的完整图入口；只消费同一权威 ExecutionView，不解析 metadata 或另起状态机。
 */
"use client";

import { useEffect, useRef, useState } from "react";
import {
  Check,
  ChevronDown,
  CircleAlert,
  Clock3,
  GitBranchPlus,
  LoaderCircle,
  RotateCw,
  Workflow,
} from "lucide-react";

import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { previewWorkGraphWorkflowApi } from "@/lib/api/conversation/execution-api";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import type { Agent } from "@/types/agent/agent";
import type { ExecutionStatus } from "@/types/conversation/execution";
import type { WorkGraphWorkflowPreview } from "@/types/conversation/workgraph-workflow";

import {
  hasExecutionGraph,
  hasManagedExecutionGraph,
  resolveExecutionWorkGraphHeaderModel,
  type ExecutionAgentDirectory,
} from "./execution-process-model";
import { ExecutionWorkGraphCanvas } from "./execution-workgraph-canvas";
import type { ExecutionResource } from "./use-execution-resource";
import { useWorkGraphHistoryResource } from "./use-workgraph-history-resource";
import { WorkGraphDistillationDialog } from "./workgraph-distillation-dialog";

type WorkGraphSurfaceMode = "current" | "history";

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
  agents,
  directory,
  onOpenWorkspaceFile,
  resource,
  taskRuns,
}: {
  agents: readonly Agent[];
  directory: ExecutionAgentDirectory;
  onOpenWorkspaceFile?: (
    path: string,
    workspaceAgentId?: string | null,
  ) => void;
  resource: ExecutionResource;
  taskRuns: readonly ConversationTaskRun[];
}) {
  const { locale, t } = useI18n();
  const [mode, setMode] = useState<WorkGraphSurfaceMode>("current");
  const [historyMenuOpen, setHistoryMenuOpen] = useState(false);
  const [selectedHistoryId, setSelectedHistoryId] = useState<string | null>(null);
  const [sketchPreview, setSketchPreview] = useState<WorkGraphWorkflowPreview | null>(null);
  const [sketchLoading, setSketchLoading] = useState(false);
  const [sketchError, setSketchError] = useState<string | null>(null);
  const sketchSessionKey = resource.sessionKey ?? "";
  const historyTriggerRef = useRef<HTMLButtonElement>(null);
  const historyResource = useWorkGraphHistoryResource(
    resource.sessionKey,
    mode === "history" || historyMenuOpen,
  );
  useEffect(() => {
    if (!selectedHistoryId && historyResource.history.length > 0) {
      setSelectedHistoryId(historyResource.history[0].id);
    }
  }, [historyResource.history, selectedHistoryId]);
  const currentExecution = hasManagedExecutionGraph(resource.execution)
    ? resource.execution
    : null;
  const historyExecution = historyResource.history.find(
    (item) => item.id === selectedHistoryId,
  ) ?? historyResource.history[0] ?? null;
  const execution = mode === "history" ? historyExecution : currentExecution;
  useEffect(() => {
    setSketchPreview(null);
    setSketchError(null);
  }, [execution?.id, resource.sessionKey]);
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
  const currentHeader = currentExecution
    ? resolveExecutionWorkGraphHeaderModel(currentExecution)
    : null;
  const historicalExecutions = historyResource.history.filter(
    (item) => item.id !== currentExecution?.id,
  );
  const historyMenuItems: UiActionMenuItem[] = [];
  if (currentExecution) {
    historyMenuItems.push({
      active: mode === "current",
      description: t("execution.surface_current"),
      icon: <Workflow className="h-3.5 w-3.5" />,
      label: currentHeader?.summary || t("execution.label"),
      trailing: mode === "current" ? <Check className="h-3.5 w-3.5 text-(--success)" /> : null,
      value: "current",
    });
  }
  historicalExecutions.forEach((item) => {
    const active = mode === "history" && item.id === execution?.id;
    historyMenuItems.push({
      active,
      description: `${new Date(item.updated_at).toLocaleDateString()} · ${item.work_items?.length ?? 0} ${t("execution.workflow_nodes_short")}`,
      icon: <Clock3 className="h-3.5 w-3.5" />,
      label: item.objective,
      trailing: active ? <Check className="h-3.5 w-3.5 text-(--success)" /> : null,
      value: `history:${item.id}`,
    });
  });
  if (historyResource.isLoading) {
    historyMenuItems.push({
      disabled: true,
      icon: <LoaderCircle className="h-3.5 w-3.5 animate-spin" />,
      label: t("execution.surface_loading"),
      value: "loading",
    });
  }

  return (
    <section
      aria-label={t("execution.label")}
      className="flex h-full min-h-0 min-w-0 flex-1 flex-col"
      data-execution-workgraph-surface
      data-execution-workgraph-stale={mode === "current" && resource.isStale ? "true" : undefined}
      data-execution-workgraph-partial={runtimeProjectionPartial ? "true" : undefined}
      data-execution-last-successful-at={lastSuccessfulAt ?? undefined}
    >
      <header
        className="flex min-h-11 shrink-0 items-center gap-2 border-b dialog-divider px-3 py-2"
        data-execution-header-status={header?.status}
      >
        <div
          className="flex min-w-0 flex-1 items-center gap-0.5"
          data-execution-header-context
        >
          <div className="min-w-0 truncate text-compact font-semibold text-(--text-strong)">
            {header?.summary || t("execution.label")}
          </div>
          <UiIconButton
            ref={historyTriggerRef}
            aria-expanded={historyMenuOpen}
            aria-haspopup="menu"
            aria-label={t("execution.surface_history")}
            onClick={() => setHistoryMenuOpen((open) => !open)}
            size="sm"
            title={t("execution.surface_history")}
            variant="ghost"
          >
            <ChevronDown className={cn(
              "h-3.5 w-3.5 transition-transform",
              historyMenuOpen && "rotate-180",
            )} />
          </UiIconButton>
          <UiActionMenu
            anchorRef={historyTriggerRef}
            ariaLabel={t("execution.surface_history")}
            density="compact"
            isOpen={historyMenuOpen}
            items={historyMenuItems}
            minWidth={280}
            onClose={() => setHistoryMenuOpen(false)}
            onSelect={(value) => {
              if (value === "current") {
                setMode("current");
              } else if (value.startsWith("history:")) {
                setSelectedHistoryId(value.slice("history:".length));
                setMode("history");
              }
            }}
            placement="bottom"
          />
        </div>
        <div
          className="ml-auto flex shrink-0 items-center gap-2"
          data-execution-header-actions
        >
          {header && header.status !== "active" ? (
            <span
              className={cn(
                "inline-flex h-6 shrink-0 items-center rounded-full border px-2 text-[11px] font-semibold leading-none",
                EXECUTION_HEADER_STATUS_TONE[header.status],
              )}
              data-execution-header-notice-status={header.status}
            >
              {t(header.statusLabelKey)}
            </span>
          ) : null}
          {header?.status === "completed" && execution && sketchSessionKey ? (
            <button
              className="inline-flex h-7 shrink-0 items-center gap-1.5 rounded-[7px] border border-[color:color-mix(in_srgb,var(--primary)_24%,var(--surface-control-border))] bg-[color:color-mix(in_srgb,var(--primary)_6%,var(--surface-control-background))] px-2.5 text-[11px] font-semibold text-(--primary) transition-colors hover:bg-[color:color-mix(in_srgb,var(--primary)_11%,var(--surface-control-background))] disabled:cursor-wait disabled:opacity-60"
              data-workgraph-save-sketch
              disabled={sketchLoading}
              onClick={() => {
                setSketchLoading(true);
                setSketchError(null);
                void previewWorkGraphWorkflowApi(sketchSessionKey, execution.id, locale)
                  .then(setSketchPreview)
                  .catch((reason: unknown) => {
                    setSketchError(getErrorMessage(reason, t("execution.workflow_preview_failed")));
                  })
                  .finally(() => setSketchLoading(false));
              }}
              type="button"
            >
              {sketchLoading
                ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
                : <GitBranchPlus className="h-3.5 w-3.5" />}
              {t(sketchLoading
                ? "execution.workflow_extracting_sketch"
                : "execution.workflow_save_as_sketch")}
            </button>
          ) : null}
          {sketchError ? (
            <span className="max-w-56 truncate text-[10px] text-(--destructive)" title={sketchError}>{sketchError}</span>
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
          {mode === "current" && resource.isStale ? (
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
          {mode === "current" && resource.error ? (
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
        </div>
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
            {(mode === "history" ? historyResource.isLoading : resource.isLoading) ? (
              <LoaderCircle className="h-5 w-5 animate-spin text-(--icon-muted)" />
            ) : (mode === "history" ? historyResource.error : resource.error) ? (
              <CircleAlert className="h-5 w-5 text-(--warning)" />
            ) : (
              <Workflow className="h-5 w-5 text-(--icon-muted)" />
            )}
            <p className="text-compact leading-5">
              {(mode === "history" ? historyResource.isLoading : resource.isLoading)
                ? t("execution.surface_loading")
                : (mode === "history" ? historyResource.error : resource.error)
                ? t("execution.surface_error")
                : mode === "history"
                ? t("execution.surface_history_empty")
                : t("execution.surface_empty")}
            </p>
          </div>
        </div>
      )}
      {sketchPreview ? (
        <WorkGraphDistillationDialog
          agents={agents}
          onClose={() => setSketchPreview(null)}
          preview={sketchPreview}
          sessionKey={sketchSessionKey}
        />
      ) : null}
    </section>
  );
}
