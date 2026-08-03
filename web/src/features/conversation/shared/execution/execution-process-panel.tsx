/**
 * INPUT: 当前/最近 ExecutionView、Agent 目录与 terminal dismiss 动作。
 * OUTPUT: Composer 上方的紧凑节点胶囊，以及服从活动 Dock 本地宽度的可展开只读节点图。
 * POS: DM 与 Room 共用的权威 Execution 进度 UI；存在时替代 legacy Todo 进程。
 */
"use client";

import { X } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "@/shared/ui/overlay/overlay-styles";
import type { ExecutionView } from "@/types/conversation/execution";

import {
  EXECUTION_STATUS_LABEL_KEY,
  isTerminalExecutionStatus,
  resolveExecutionAgent,
  resolveExecutionNodeSummary,
  resolveExecutionNodeWindow,
  type ExecutionAgentDirectory,
} from "./execution-process-model";
import { ExecutionNodeAvatar } from "./execution-node-avatar";
import { ExecutionWorkGraphCanvas } from "./execution-workgraph-canvas";

const EXECUTION_TRIGGER_CLASS_NAME =
  "border border-(--surface-control-border) bg-(--surface-control-background) shadow-(--surface-control-shadow)";

export function ExecutionProcessPanel({
  className,
  directory,
  execution,
  onDismiss,
}: {
  className?: string;
  directory: ExecutionAgentDirectory;
  execution: ExecutionView;
  onDismiss: () => void;
}) {
  const { t } = useI18n();
  const [isExpanded, setIsExpanded] = useState(false);
  const panelId = useId();
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const nodeSummary = resolveExecutionNodeSummary(execution);
  const terminal = isTerminalExecutionStatus(execution.status);
  const nodeProgressLabel = nodeSummary.totalCount > 0
    ? t("execution.node_progress", {
        current: nodeSummary.currentStep,
        total: nodeSummary.totalCount,
      })
    : t(EXECUTION_STATUS_LABEL_KEY[execution.status]);

  useEffect(() => {
    if (!isExpanded) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        triggerRef.current?.focus();
        setIsExpanded(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isExpanded]);

  return (
    <aside
      aria-label={t("execution.label")}
      aria-live="polite"
      className={cn(
        "pointer-events-none relative flex w-full min-w-0 max-w-[580px] justify-center",
        className,
      )}
      data-execution-process-panel
      data-execution-status={execution.status}
    >
      <button
        ref={triggerRef}
        aria-controls={panelId}
        aria-expanded={isExpanded}
        aria-label={`${
          isExpanded
            ? t("execution.collapse_panel")
            : t("execution.expand_panel")
        } · ${nodeSummary.summary} · ${nodeProgressLabel}`}
        className={cn(
          "pointer-events-auto flex min-h-10 min-w-10 max-w-full items-center justify-center overflow-hidden rounded-[14px] px-2.5 py-1.5 text-xs text-(--text-default) transition-[background,border-color,color,box-shadow] hover:border-(--surface-control-hover-border) hover:bg-(--surface-control-hover-background) hover:text-(--text-strong)",
          EXECUTION_TRIGGER_CLASS_NAME,
        )}
        data-execution-node-summary={nodeSummary.summary}
        data-execution-process-trigger
        data-execution-trigger-content={
          nodeSummary.totalCount > 0 ? "node-rail" : "status"
        }
        onClick={() => setIsExpanded((current) => !current)}
        title={`${nodeSummary.summary} · ${nodeProgressLabel}`}
        type="button"
      >
        {nodeSummary.totalCount > 0 ? (
          <ExecutionNodeRail
            currentId={nodeSummary.current?.id ?? null}
            directory={directory}
            execution={execution}
          />
        ) : (
          <span className="truncate text-compact text-(--text-soft)">
            {nodeProgressLabel}
          </span>
        )}
      </button>

      {isExpanded ? (
        <div className="pointer-events-none absolute inset-x-0 bottom-[calc(100%+0.5rem)] flex justify-center">
          <section
            aria-label={t("execution.label")}
            className={cn(
              "pointer-events-auto flex max-h-[min(440px,54dvh)] w-full origin-bottom flex-col overflow-hidden",
              OVERLAY_SURFACE_CLASS_NAME,
              ANCHORED_OVERLAY_MOTION_CLASS_NAME,
            )}
            data-execution-workgraph
            data-placement="top"
            id={panelId}
          >
            <ExecutionPanelHeader
              execution={execution}
              nodeProgressLabel={nodeProgressLabel}
              onDismiss={terminal
                ? () => {
                    setIsExpanded(false);
                    onDismiss();
                  }
                : null}
              summary={nodeSummary.summary}
            />
            {execution.plan && (execution.work_items?.length ?? 0) > 0 ? (
              <ExecutionWorkGraphCanvas
                currentId={nodeSummary.current?.id ?? null}
                directory={directory}
                execution={execution}
                key={execution.id}
              />
            ) : (
              <div className="grid min-h-24 place-items-center px-6 py-6 text-center">
                <p className="text-xs text-(--text-soft)">
                  {t("execution.no_plan")}
                </p>
              </div>
            )}
          </section>
        </div>
      ) : null}
    </aside>
  );
}

function ExecutionPanelHeader({
  execution,
  nodeProgressLabel,
  onDismiss,
  summary,
}: {
  execution: ExecutionView;
  nodeProgressLabel: string;
  onDismiss: (() => void) | null;
  summary: string;
}) {
  const { t } = useI18n();
  return (
    <header className="flex h-10 shrink-0 items-center gap-2 px-3">
      <span
        className="min-w-0 flex-1 truncate text-compact font-semibold text-(--text-strong)"
        data-execution-panel-heading
        title={execution.plan
          ? `${summary} · ${t("execution.plan_revision", {
              revision: execution.plan.revision,
            })}`
          : summary}
      >
        {summary}
      </span>
      <span className="shrink-0 text-compact tabular-nums text-(--text-soft)">
        {nodeProgressLabel}
      </span>
      {onDismiss ? (
        <button
          aria-label={t("execution.dismiss")}
          className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-[6px] text-(--icon-muted) transition-[background,color] hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
          onClick={onDismiss}
          title={t("execution.dismiss")}
          type="button"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      ) : null}
    </header>
  );
}

function ExecutionNodeRail({
  currentId,
  directory,
  execution,
}: {
  currentId: string | null;
  directory: ExecutionAgentDirectory;
  execution: ExecutionView;
}) {
  const { t } = useI18n();
  const nodeWindow = resolveExecutionNodeWindow(execution, currentId);
  if (nodeWindow.items.length === 0) {
    return null;
  }
  return (
    <span
      aria-hidden="true"
      className="flex min-w-0 items-center"
      data-execution-node-rail
    >
      {nodeWindow.hiddenBefore > 0 ? (
        <span className="mr-1 text-[9px] leading-none text-(--text-soft)">…</span>
      ) : null}
      {nodeWindow.items.map((item, index) => {
        const owner = resolveExecutionAgent(directory, item.owner_agent_id);
        return (
          <span className="inline-flex items-center" key={item.id}>
            {index > 0 ? (
              <span
                className="h-px w-2.5 bg-(--divider-subtle-color)"
                data-execution-node-connection
              />
            ) : null}
            <ExecutionNodeAvatar
              agent={owner}
              current={item.id === currentId}
              status={item.status}
              title={`${item.subject} · ${
                owner?.name ?? t("execution.owner_unassigned")
              }`}
            />
          </span>
        );
      })}
      {nodeWindow.hiddenAfter > 0 ? (
        <span className="ml-1 text-[9px] leading-none text-(--text-soft)">…</span>
      ) : null}
    </span>
  );
}
