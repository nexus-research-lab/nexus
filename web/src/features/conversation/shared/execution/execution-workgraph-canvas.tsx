/**
 * INPUT: 权威 Execution Graph、Agent 目录、当前 Graph 节点与精确 Agent round Task run。
 * OUTPUT: 仅显示 Agent/Subagent/Tool/Gate 图标、方向边和按点击展开节点详情的分层运行图。
 * POS: DM/Room 共用的 Execution Graph 主视图；不从文本或布局反推运行身份。
 */
"use client";

import { useEffect, useId, useMemo, useRef, useState } from "react";

import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import type {
  ExecutionAttemptView,
  ExecutionGraphNodeView,
  ExecutionView,
  ExecutionWorkItemStatus,
  ExecutionWorkItemView,
} from "@/types/conversation/execution";

import { ExecutionNodeAvatar } from "./execution-node-avatar";
import { ExecutionNodeTaskList } from "./execution-node-task-list";
import { resolveExecutionNodeTaskRun } from "./execution-node-task-model";
import {
  compactExecutionNodeObjective,
  resolveExecutionAgent,
  resolveExecutionGraphNodeStatus,
  type ExecutionAgentDirectory,
  WORK_ITEM_STATUS_LABEL_KEY,
} from "./execution-process-model";
import { buildExecutionGraphLayout } from "./execution-workgraph-layout";

const ATTEMPT_STATUS_LABEL_KEY: Record<
  ExecutionAttemptView["status"],
  TranslationKey
> = {
  cancelled: "execution.attempt_cancelled",
  failed: "execution.attempt_failed",
  interrupted: "execution.attempt_interrupted",
  pending: "execution.attempt_pending",
  running: "execution.attempt_running",
  succeeded: "execution.attempt_succeeded",
  timed_out: "execution.attempt_timed_out",
};

export function ExecutionWorkGraphCanvas({
  currentId,
  directory,
  execution,
  taskRuns,
}: {
  currentId: string | null;
  directory: ExecutionAgentDirectory;
  execution: ExecutionView;
  taskRuns: readonly ConversationTaskRun[];
}) {
  const { t } = useI18n();
  const markerId = `execution-arrow-${useId().replace(/:/g, "")}`;
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const [availableWidth, setAvailableWidth] = useState<number | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const layout = useMemo(
    () => buildExecutionGraphLayout(execution, availableWidth ?? undefined),
    [availableWidth, execution],
  );
  const selectedLayoutNode = layout.nodes.find(
    (candidate) => candidate.node.id === selectedId,
  ) ?? null;
  const selectedItem = selectedLayoutNode?.item ?? null;
  const selectedAttempt = selectedLayoutNode?.node.attempt_id
    ? selectedItem?.attempts?.find(
      (attempt) => attempt.id === selectedLayoutNode.node.attempt_id,
    ) ?? null
    : null;
  const selectedTaskRun = selectedItem && selectedLayoutNode?.node.kind === "agent"
    ? resolveExecutionNodeTaskRun(selectedItem, taskRuns)
    : null;

  useEffect(() => {
    if (selectedId && !layout.nodes.some((node) => node.node.id === selectedId)) {
      setSelectedId(null);
    }
  }, [layout.nodes, selectedId]);

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return;
    }
    const updateWidth = () => {
      const style = window.getComputedStyle(viewport);
      const horizontalPadding = (Number.parseFloat(style.paddingLeft) || 0)
        + (Number.parseFloat(style.paddingRight) || 0);
      const width = Math.max(0, viewport.clientWidth - horizontalPadding);
      const nextWidth = Math.floor(width);
      setAvailableWidth((current) => current === nextWidth ? current : nextWidth);
    };
    updateWidth();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(updateWidth);
    observer.observe(viewport);
    return () => observer.disconnect();
  }, []);

  return (
    <div className="flex min-h-0 flex-1 flex-col" data-execution-node-map>
      <div
        ref={viewportRef}
        className="soft-scrollbar min-h-0 flex-1 overflow-auto px-2 py-2"
      >
        <div
          aria-label={t("execution.label")}
          className="relative mx-auto overflow-visible rounded-[12px] bg-[color:color-mix(in_srgb,var(--surface-panel-background)_62%,transparent)]"
          data-execution-workgraph-canvas
          data-execution-node-detail-mode="popover"
          role="group"
          style={{ height: layout.height, width: layout.width }}
        >
          {selectedLayoutNode ? (
            <button
              aria-label={t("execution.close_node_details")}
              className="absolute inset-0 z-0 cursor-default focus-visible:outline-none"
              onClick={() => setSelectedId(null)}
              type="button"
            />
          ) : null}
          <svg
            aria-hidden="true"
            className="pointer-events-none absolute inset-0 h-full w-full overflow-visible"
            data-execution-edge-layer
            viewBox={`0 0 ${layout.width} ${layout.height}`}
          >
            <defs>
              <marker
                id={markerId}
                markerHeight="5"
                markerUnits="strokeWidth"
                markerWidth="5"
                orient="auto"
                refX="4"
                refY="2.5"
                viewBox="0 0 5 5"
              >
                <path d="M 0 0 L 5 2.5 L 0 5 z" fill="var(--icon-muted)" />
              </marker>
            </defs>
            {layout.edges.map((edge) => {
              const emphasized = edge.sourceId === selectedId
                || edge.targetId === selectedId
                || edge.targetId === currentId;
              return (
                <path
                  d={edge.path}
                  data-execution-edge-kind={edge.kind}
                  data-execution-edge-source={edge.sourceId}
                  data-execution-edge-target={edge.targetId}
                  fill="none"
                  key={edge.id}
                  markerEnd={`url(#${markerId})`}
                  opacity={emphasized ? 0.9 : 0.46}
                  stroke={emphasized
                    ? "var(--primary)"
                    : "var(--divider-subtle-color)"}
                  strokeDasharray={edge.kind === "spawn" || edge.kind === "invoke"
                    ? "3 3"
                    : edge.kind === "loop_back"
                    ? "5 3"
                    : undefined}
                  strokeLinecap="round"
                  strokeWidth={emphasized ? 1.6 : 1.25}
                />
              );
            })}
          </svg>

          {layout.nodes.map(({ item, node, size, x, y }) => {
            const owner = node.kind === "tool"
              ? null
              : resolveExecutionAgent(
                directory,
                node.agent_id ?? item?.owner_agent_id,
              );
            const status = resolveExecutionGraphNodeStatus(node, item);
            const selected = node.id === selectedId;
            const current = node.id === currentId;
            const title = graphNodeTitle(node, item, owner?.name, t);
            return (
              <button
                aria-label={`${t("execution.details")}: ${title}`}
                aria-pressed={selected}
                className="absolute z-10 grid place-items-center rounded-[16px] transition-[left,top,transform] duration-300 hover:scale-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--primary)"
                data-execution-attempt-id={node.attempt_id || undefined}
                data-execution-current-node={current ? "true" : undefined}
                data-execution-graph-node-id={node.id}
                data-execution-node-selected={selected ? "true" : undefined}
                data-execution-work-item-id={node.work_item_id || undefined}
                key={node.id}
                onClick={() => setSelectedId((value) => (
                  value === node.id ? null : node.id
                ))}
                style={{
                  height: size + 8,
                  left: x - (size + 8) / 2,
                  top: y - (size + 8) / 2,
                  width: size + 8,
                }}
                title={title}
                type="button"
              >
                <ExecutionNodeAvatar
                  agent={owner}
                  current={current}
                  kind={node.kind}
                  selected={selected}
                  size={node.kind === "subagent" || node.kind === "tool"
                    ? "nested"
                    : "graph"}
                  status={status}
                  title={title}
                />
              </button>
            );
          })}

          {selectedLayoutNode ? (
            <ExecutionNodePopover
              attempt={selectedAttempt}
              directory={directory}
              item={selectedItem}
              layoutHeight={layout.height}
              layoutWidth={layout.width}
              node={selectedLayoutNode.node}
              taskRun={selectedTaskRun}
              x={selectedLayoutNode.x}
              y={selectedLayoutNode.y}
            />
          ) : null}
        </div>
      </div>
    </div>
  );
}

function ExecutionNodePopover({
  attempt,
  directory,
  item,
  layoutHeight,
  layoutWidth,
  node,
  taskRun,
  x,
  y,
}: {
  attempt: ExecutionAttemptView | null;
  directory: ExecutionAgentDirectory;
  item: ExecutionWorkItemView | null;
  layoutHeight: number;
  layoutWidth: number;
  node: ExecutionGraphNodeView;
  taskRun: ConversationTaskRun | null;
  x: number;
  y: number;
}) {
  const { t } = useI18n();
  const owner = node.kind === "tool"
    ? null
    : resolveExecutionAgent(
      directory,
      node.agent_id ?? item?.owner_agent_id,
    );
  const objectiveSource = item?.objective
    ?? node.description
    ?? node.name
    ?? "";
  const objective = compactExecutionNodeObjective(objectiveSource, owner?.name);
  const deliverable = item?.deliverable.trim() ?? "";
  const showDeliverable = deliverable
    && deliverable.toLocaleLowerCase() !== objective.toLocaleLowerCase();
  const status = resolveExecutionGraphNodeStatus(node, item);
  const statusLabel = attempt && node.kind === "subagent"
    ? t(ATTEMPT_STATUS_LABEL_KEY[attempt.status])
    : t(WORK_ITEM_STATUS_LABEL_KEY[status]);
  const heading = graphNodeHeading(node, item, t);
  const openRight = x <= layoutWidth / 2;
  const estimatedHeight = taskRun ? 214 : 104;
  const top = Math.max(8, Math.min(y - 42, layoutHeight - estimatedHeight));
  return (
    <article
      className="absolute z-20 w-[15rem] rounded-[10px] border border-(--surface-control-border) bg-(--surface-control-background) px-3 py-2.5 shadow-(--surface-control-shadow)"
      aria-label={`${t("execution.details")}: ${heading}`}
      data-execution-selected-node-detail={node.id}
      role="dialog"
      style={{
        left: openRight ? x + 34 : x - 34,
        top,
        transform: openRight ? undefined : "translateX(-100%)",
      }}
    >
      <div className="flex min-w-0 items-center gap-2">
        <h3 className="min-w-0 flex-1 truncate text-compact font-semibold text-(--text-strong)">
          {heading}
        </h3>
        <span className={cn(
          "shrink-0 text-[10px] font-medium",
          selectedStatusTone(status),
        )}>
          {statusLabel}
        </span>
      </div>
      {node.kind === "subagent" && item ? (
        <p className="mt-0.5 truncate text-[10px] text-(--text-soft)">
          {item.subject}
        </p>
      ) : null}
      {objective ? (
        <p className="mt-1 line-clamp-2 text-[11px] leading-4 text-(--text-default)">
          {objective}
        </p>
      ) : null}
      <p className="mt-1.5 flex min-w-0 gap-1 text-[10px] leading-4 text-(--text-soft)">
        <span className="shrink-0">
          {node.kind === "tool"
            ? node.name ?? t("execution.node_tool")
            : node.kind === "subagent"
            ? t("execution.attempt_subagent")
            : owner?.name ?? t("execution.owner_unassigned")}
        </span>
        {showDeliverable ? (
          <>
            <span aria-hidden="true">·</span>
            <span className="min-w-0 truncate text-(--text-default)">
              {deliverable}
            </span>
          </>
        ) : null}
      </p>
      {taskRun ? <ExecutionNodeTaskList run={taskRun} /> : null}
    </article>
  );
}

function graphNodeTitle(
  node: ExecutionGraphNodeView,
  item: ExecutionWorkItemView | null,
  ownerName: string | undefined,
  t: (key: TranslationKey) => string,
): string {
  if (node.kind === "tool") {
    return node.name?.trim() || t("execution.node_tool");
  }
  if (node.kind === "gate") {
    const subject = item?.subject ?? node.description?.trim();
    const gate = node.name === "objective_alignment"
      ? t("execution.node_alignment_gate")
      : t("execution.node_gate");
    return ownerName
      ? `${subject ? `${subject} · ` : ""}${gate} · ${ownerName}`
      : `${subject ? `${subject} · ` : ""}${gate}`;
  }
  if (node.kind === "subagent") {
    return item?.subject
      ? `${t("execution.attempt_subagent")} · ${item.subject}`
      : t("execution.attempt_subagent");
  }
  const subject = item?.subject
    ?? node.description?.trim()
    ?? ownerName
    ?? t("execution.owner_unassigned");
  return ownerName ? `${subject} · ${ownerName}` : subject;
}

function graphNodeHeading(
  node: ExecutionGraphNodeView,
  item: ExecutionWorkItemView | null,
  t: (key: TranslationKey) => string,
): string {
  if (node.kind === "tool") {
    return node.name?.trim() || t("execution.node_tool");
  }
  if (node.kind === "gate") {
    return node.name === "objective_alignment"
      ? t("execution.node_alignment_gate")
      : t("execution.node_gate");
  }
  if (node.kind === "subagent") {
    return t("execution.attempt_subagent");
  }
  return item?.subject
    ?? node.description?.trim()
    ?? node.name?.trim()
    ?? t("execution.owner_unassigned");
}

function selectedStatusTone(status: ExecutionWorkItemStatus): string {
  if (status === "accepted") {
    return "text-(--success)";
  }
  if (
    status === "blocked"
    || status === "changes_requested"
    || status === "failed"
  ) {
    return "text-(--warning)";
  }
  if (
    status === "running"
    || status === "submitted"
    || status === "ready"
    || status === "assigned"
  ) {
    return "text-(--primary)";
  }
  return "text-(--text-soft)";
}
