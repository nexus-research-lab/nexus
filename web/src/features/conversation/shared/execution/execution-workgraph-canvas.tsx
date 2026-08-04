/**
 * INPUT: 权威 Execution Graph、Agent 目录、当前 Graph 节点与精确 Agent round Task run。
 * OUTPUT: 在工作板网格上显示精简图标与方向边，并用节点旁悬浮检查器展示目标、结果、错误与子级运行事实。
 * POS: DM/Room 共用的 Execution Graph 主视图；子图只按结构化父身份分组，不从自由文本反推关系。
 */
"use client";

import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from "react";
import { X } from "lucide-react";

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
  normalizeExecutionNodeDisplayText,
  resolveExecutionGraphNodeAgent,
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

const NODE_INSPECTOR_WIDTH = 304;
const NODE_INSPECTOR_GAP = 12;
const NODE_INSPECTOR_EDGE_PADDING = 8;

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
  const loopMarkerId = `${markerId}-loop`;
  const retryMarkerId = `${markerId}-retry`;
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
  const selectedInspectorStyle = selectedLayoutNode
    ? resolveNodeInspectorStyle(
        layout.width,
        selectedLayoutNode.x,
        selectedLayoutNode.y,
        selectedLayoutNode.size,
      )
    : undefined;

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
    <div className="relative flex min-h-0 flex-1 overflow-hidden" data-execution-node-map>
      <div
        ref={viewportRef}
        className="soft-scrollbar min-h-0 min-w-0 flex-1 overflow-auto p-2"
        data-execution-board-grid
        style={{
          backgroundImage: "linear-gradient(to right, color-mix(in srgb, var(--divider-subtle-color) 58%, transparent) 1px, transparent 1px), linear-gradient(to bottom, color-mix(in srgb, var(--divider-subtle-color) 58%, transparent) 1px, transparent 1px)",
          backgroundPosition: "-1px -1px",
          backgroundSize: "24px 24px",
        }}
      >
        <div
          aria-label={t("execution.label")}
          className="relative mx-auto overflow-visible rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-panel-background)_42%,transparent)]"
          data-execution-workgraph-canvas
          data-execution-node-detail-mode="popover"
          role="group"
          style={{ height: layout.height, width: layout.width }}
        >
          {layout.groups.map((group) => (
            <div
              aria-hidden="true"
              className="pointer-events-none absolute z-0 rounded-[18px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_78%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-control-background)_48%,transparent)]"
              data-execution-subgraph-root={group.id}
              key={group.id}
              style={{
                height: group.height,
                left: group.x,
                top: group.y,
                width: group.width,
              }}
            />
          ))}
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
              <marker
                id={loopMarkerId}
                markerHeight="5"
                markerUnits="strokeWidth"
                markerWidth="5"
                orient="auto"
                refX="4"
                refY="2.5"
                viewBox="0 0 5 5"
              >
                <path d="M 0 0 L 5 2.5 L 0 5 z" fill="var(--warning)" />
              </marker>
              <marker
                id={retryMarkerId}
                markerHeight="5"
                markerUnits="strokeWidth"
                markerWidth="5"
                orient="auto"
                refX="4"
                refY="2.5"
                viewBox="0 0 5 5"
              >
                <path d="M 0 0 L 5 2.5 L 0 5 z" fill="var(--primary)" />
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
                  markerEnd={`url(#${edge.kind === "loop_back"
                    ? loopMarkerId
                    : edge.kind === "retry"
                    ? retryMarkerId
                    : markerId})`}
                  opacity={emphasized ? 0.96 : 0.68}
                  stroke={edge.kind === "loop_back"
                    ? "var(--warning)"
                    : edge.kind === "retry" || emphasized
                    ? "var(--primary)"
                    : "var(--divider-subtle-color)"}
                  strokeDasharray={edge.kind === "spawn" || edge.kind === "invoke"
                    ? "3 3"
                    : edge.kind === "loop_back" || edge.kind === "retry"
                    ? "5 3"
                    : undefined}
                  strokeLinecap="round"
                  strokeWidth={emphasized ? 1.7 : 1.4}
                />
              );
            })}
          </svg>

          {layout.nodes.map(({ item, node, size, x, y }) => {
            const owner = resolveExecutionGraphNodeAgent(directory, node, item);
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
          {selectedLayoutNode && selectedInspectorStyle ? (
            <ExecutionNodeInspector
              attempt={selectedAttempt}
              directory={directory}
              execution={execution}
              item={selectedItem}
              node={selectedLayoutNode.node}
              onClose={() => setSelectedId(null)}
              style={selectedInspectorStyle}
              taskRun={selectedTaskRun}
            />
          ) : null}
        </div>
      </div>
    </div>
  );
}

function ExecutionNodeInspector({
  attempt,
  directory,
  execution,
  item,
  node,
  onClose,
  style,
  taskRun,
}: {
  attempt: ExecutionAttemptView | null;
  directory: ExecutionAgentDirectory;
  execution: ExecutionView;
  item: ExecutionWorkItemView | null;
  node: ExecutionGraphNodeView;
  onClose: () => void;
  style: CSSProperties;
  taskRun: ConversationTaskRun | null;
}) {
  const { t } = useI18n();
  const parentNode = node.parent_node_id
    ? execution.graph?.nodes?.find((candidate) => candidate.id === node.parent_node_id)
      ?? null
    : null;
  const parentItem = parentNode
    ? execution.work_items?.find((candidate) => (
      candidate.id === parentNode.work_item_id
    )) ?? null
    : null;
  const owner = resolveExecutionGraphNodeAgent(directory, node, item)
    ?? (parentNode
      ? resolveExecutionGraphNodeAgent(directory, parentNode, parentItem)
      : null);
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
  const relatedSubject = node.kind === "agent" ? "" : item?.subject.trim() ?? "";
  const submission = item?.submission?.result_summary.trim() ?? "";
  const review = item?.acceptance?.feedback?.trim() ?? "";
  const resultSummary = node.result_summary?.trim() ?? "";
  const errorSummary = normalizeExecutionNodeDisplayText(
    node.error_summary || attempt?.failure_reason || "",
  );
  const visibleErrorSummary = errorSummary
    || (status === "failed" ? t("execution.error_summary_unavailable") : "");
  const retryEdges = (execution.graph?.edges ?? []).filter((edge) => (
    edge.kind === "retry"
    && (edge.source_node_id === node.id || edge.target_node_id === node.id)
  ));
  const controlReturnObserved = (execution.graph?.edges ?? []).some((edge) => (
    edge.kind === "loop_back" && edge.source_node_id === node.id
  ));
  const childNodes = (execution.graph?.nodes ?? [])
    .filter((candidate) => candidate.parent_node_id === node.id)
    .sort((left, right) => (
      left.position - right.position || left.id.localeCompare(right.id)
    ));
  return (
    <aside
      className="soft-scrollbar absolute z-30 max-h-[min(70vh,28rem)] w-[19rem] max-w-[calc(100%-1rem)] overflow-auto rounded-[14px] border border-(--surface-control-border) bg-(--surface-panel-background) shadow-(--surface-control-shadow)"
      aria-label={`${t("execution.details")}: ${heading}`}
      data-execution-selected-node-detail={node.id}
      data-execution-selected-node-detail-mode="popover"
      style={style}
    >
      <div className="sticky top-0 z-10 flex min-w-0 items-center gap-2 border-b dialog-divider bg-(--surface-panel-background) px-3 py-3">
        <ExecutionNodeAvatar
          agent={owner}
          current={status === "running"}
          kind={node.kind}
          size="graph"
          status={status}
          title={heading}
        />
        <div className="min-w-0 flex-1">
          <h3 className="truncate text-compact font-semibold text-(--text-strong)">
            {heading}
          </h3>
          <p className="mt-0.5 flex min-w-0 items-center gap-1 text-[10px] text-(--text-soft)">
            {owner ? <span className="truncate">{owner.name}</span> : null}
            {owner ? <span aria-hidden="true">·</span> : null}
            <span className={cn("shrink-0 font-medium", selectedStatusTone(status))}>
              {statusLabel}
            </span>
          </p>
        </div>
        <button
          aria-label={t("execution.close_node_details")}
          className="grid h-7 w-7 shrink-0 place-items-center rounded-[8px] text-(--icon-muted) transition-[background,color] hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--primary)"
          onClick={onClose}
          title={t("execution.close_node_details")}
          type="button"
        >
          <X aria-hidden="true" className="h-3.5 w-3.5" />
        </button>
      </div>
      <div className="space-y-3 px-3 py-3">
        {relatedSubject ? (
          <p className="text-[11px] font-medium leading-4 text-(--text-default)">
            {relatedSubject}
          </p>
        ) : null}
        {objective ? (
          <NodeDetailSection label={t("execution.objective")}>
            <p>{objective}</p>
          </NodeDetailSection>
        ) : null}
        {showDeliverable ? (
          <NodeDetailSection label={t("execution.deliverable")}>
            <p>{deliverable}</p>
          </NodeDetailSection>
        ) : null}
        {(item?.acceptance_criteria?.length ?? 0) > 0 ? (
          <NodeDetailSection label={t("execution.acceptance")}>
            <ul className="space-y-1">
              {item?.acceptance_criteria?.slice(0, 4).map((criterion) => (
                <li className="flex gap-2" key={criterion}>
                  <span aria-hidden="true" className="mt-[7px] h-1 w-1 shrink-0 rounded-full bg-(--icon-muted)" />
                  <span>{criterion}</span>
                </li>
              ))}
            </ul>
          </NodeDetailSection>
        ) : null}
        {item?.block_reason?.trim() ? (
          <NodeDetailSection label={t("execution.blocker")}>
            <p>{item.block_reason.trim()}</p>
          </NodeDetailSection>
        ) : null}
        {item?.needed_input?.trim() ? (
          <NodeDetailSection label={t("execution.needed_input")}>
            <p>{item.needed_input.trim()}</p>
          </NodeDetailSection>
        ) : null}
        {visibleErrorSummary ? (
          <NodeDetailSection label={t("execution.error_summary")}>
            <p>{visibleErrorSummary}</p>
            {node.error_code?.trim() ? (
              <p className="mt-1 font-mono text-[10px] text-(--text-soft)">
                {node.error_code.trim()}
              </p>
            ) : null}
          </NodeDetailSection>
        ) : null}
        {resultSummary ? (
          <NodeDetailSection label={t("execution.result_summary")}>
            <p>{resultSummary}</p>
            {node.summary_truncated ? (
              <p className="mt-1 text-[10px] text-(--text-soft)">
                {t("execution.summary_truncated")}
              </p>
            ) : null}
          </NodeDetailSection>
        ) : null}
        {(node.duration_ms ?? 0) > 0 ? (
          <NodeDetailSection label={t("execution.duration")}>
            <p>{formatNodeDuration(node.duration_ms ?? 0)}</p>
          </NodeDetailSection>
        ) : null}
        {controlReturnObserved ? (
          <NodeDetailSection label={t("execution.control_return")}>
            <p>{t("execution.control_return_observed")}</p>
          </NodeDetailSection>
        ) : null}
        {retryEdges.length > 0 ? (
          <NodeDetailSection label={t("execution.retry_relation")}>
            <p>{t("execution.retry_relation_count", { count: retryEdges.length })}</p>
          </NodeDetailSection>
        ) : null}
        {submission ? (
          <NodeDetailSection label={t("execution.submission")}>
            <p>{submission}</p>
          </NodeDetailSection>
        ) : null}
        {review ? (
          <NodeDetailSection label={t("execution.review")}>
            <p>{review}</p>
          </NodeDetailSection>
        ) : null}
        {taskRun ? <ExecutionNodeTaskList run={taskRun} /> : null}
        {childNodes.length > 0 ? (
          <ExecutionNodeRunList
            directory={directory}
            execution={execution}
            nodes={childNodes}
          />
        ) : null}
      </div>
    </aside>
  );
}

function NodeDetailSection({
  children,
  label,
}: {
  children: ReactNode;
  label: string;
}) {
  return (
    <section>
      <h4 className="mb-1 text-[10px] font-medium text-(--text-soft)">
        {label}
      </h4>
      <div className="text-[11px] leading-[1.55] text-(--text-default)">
        {children}
      </div>
    </section>
  );
}

function ExecutionNodeRunList({
  directory,
  execution,
  nodes,
}: {
  directory: ExecutionAgentDirectory;
  execution: ExecutionView;
  nodes: readonly ExecutionGraphNodeView[];
}) {
  const { t } = useI18n();
  return (
    <NodeDetailSection label={t("execution.runtime_activity")}>
      <ul className="space-y-1.5" data-execution-runtime-activity>
        {nodes.slice(0, 8).map((node) => {
          const item = execution.work_items?.find(
            (candidate) => candidate.id === node.work_item_id,
          ) ?? null;
          const owner = resolveExecutionGraphNodeAgent(directory, node, item);
          const status = resolveExecutionGraphNodeStatus(node, item);
          const summary = node.error_summary?.trim()
            || node.result_summary?.trim()
            || node.description?.trim()
            || "";
          return (
            <li
              className="flex min-w-0 gap-2 rounded-[9px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-control-background)_68%,transparent)] px-2 py-1.5"
              data-execution-runtime-node={node.id}
              key={node.id}
            >
              <ExecutionNodeAvatar
                agent={owner}
                current={status === "running"}
                kind={node.kind}
                size="nested"
                status={status}
                title={graphNodeHeading(node, item, t)}
              />
              <div className="min-w-0 flex-1">
                <div className="flex min-w-0 items-center gap-1.5">
                  <span className="truncate font-medium text-(--text-default)">
                    {graphNodeHeading(node, item, t)}
                  </span>
                  <span
                    aria-hidden="true"
                    className={cn(
                      "h-1.5 w-1.5 shrink-0 rounded-full bg-current",
                      selectedStatusTone(status),
                    )}
                  />
                </div>
                {summary ? (
                  <p className="mt-0.5 line-clamp-2 text-[10px] leading-4 text-(--text-soft)">
                    {summary}
                  </p>
                ) : null}
              </div>
            </li>
          );
        })}
      </ul>
      {nodes.length > 8 ? (
        <p className="mt-1 text-[10px] text-(--text-soft)">
          {t("execution.runtime_activity_more", { count: nodes.length - 8 })}
        </p>
      ) : null}
    </NodeDetailSection>
  );
}

function resolveNodeInspectorStyle(
  canvasWidth: number,
  x: number,
  y: number,
  nodeSize: number,
): CSSProperties {
  const width = Math.min(
    NODE_INSPECTOR_WIDTH,
    Math.max(240, canvasWidth - NODE_INSPECTOR_EDGE_PADDING * 2),
  );
  const right = x + nodeSize / 2 + NODE_INSPECTOR_GAP;
  const fitsRight = right + width
    <= canvasWidth - NODE_INSPECTOR_EDGE_PADDING;
  const left = fitsRight
    ? right
    : Math.max(
        NODE_INSPECTOR_EDGE_PADDING,
        x - nodeSize / 2 - NODE_INSPECTOR_GAP - width,
      );
  return {
    left,
    top: Math.max(NODE_INSPECTOR_EDGE_PADDING, y - 32),
    width,
  };
}

function formatNodeDuration(durationMS: number): string {
  if (durationMS < 1_000) {
    return `${Math.round(durationMS)}ms`;
  }
  const seconds = durationMS / 1_000;
  if (seconds < 60) {
    return `${seconds < 10 ? seconds.toFixed(1) : Math.round(seconds)}s`;
  }
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${Math.round(seconds % 60)}s`;
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
    const identity = ownerName || t("execution.attempt_subagent");
    return item?.subject ? `${identity} · ${item.subject}` : identity;
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
    return node.name?.trim() || t("execution.attempt_subagent");
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
