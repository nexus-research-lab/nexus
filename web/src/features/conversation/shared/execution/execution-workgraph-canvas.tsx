/**
 * INPUT: 当前 Execution、Agent 目录与当前节点。
 * OUTPUT: 按自身真实宽度重排的 Agent 节点、依赖边与单节点摘要。
 * POS: Execution 展开态的唯一 WorkGraph 主视图；快照更新即重排，不复制编排状态。
 */
"use client";

import { useEffect, useId, useMemo, useRef, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import type {
  ExecutionView,
  ExecutionWorkItemStatus,
  ExecutionWorkItemView,
} from "@/types/conversation/execution";

import { ExecutionNodeAvatar } from "./execution-node-avatar";
import {
  orderedExecutionItems,
  resolveExecutionAgent,
  type ExecutionAgentDirectory,
  WORK_ITEM_STATUS_LABEL_KEY,
} from "./execution-process-model";
import { buildExecutionGraphLayout } from "./execution-workgraph-layout";

export function ExecutionWorkGraphCanvas({
  currentId,
  directory,
  execution,
}: {
  currentId: string | null;
  directory: ExecutionAgentDirectory;
  execution: ExecutionView;
}) {
  const { t } = useI18n();
  const markerId = `execution-arrow-${useId().replace(/:/g, "")}`;
  const items = useMemo(() => orderedExecutionItems(execution), [execution]);
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const [availableWidth, setAvailableWidth] = useState<number | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const selectedItem = items.find((item) => item.id === selectedId) ?? null;
  const layout = useMemo(
    () => buildExecutionGraphLayout(execution, availableWidth ?? undefined),
    [availableWidth, execution],
  );
  const selectedNode = layout.nodes.find(
    (node) => node.item.id === selectedItem?.id,
  ) ?? null;

  useEffect(() => {
    if (selectedId && !items.some((item) => item.id === selectedId)) {
      setSelectedId(null);
    }
  }, [items, selectedId]);

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return;
    }
    const updateWidth = () => {
      const style = window.getComputedStyle(viewport);
      const horizontalPadding = (Number.parseFloat(style.paddingLeft) || 0)
        + (Number.parseFloat(style.paddingRight) || 0);
      const width = Math.max(
        0,
        viewport.clientWidth - horizontalPadding,
      );
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
          {selectedItem ? (
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
              const emphasized = edge.sourceId === selectedItem?.id
                || edge.targetId === selectedItem?.id
                || edge.targetId === currentId;
              return (
                <path
                  d={edge.path}
                  data-execution-edge-source={edge.sourceId}
                  data-execution-edge-target={edge.targetId}
                  fill="none"
                  key={edge.id}
                  markerEnd={`url(#${markerId})`}
                  opacity={emphasized ? 0.9 : 0.46}
                  stroke={emphasized
                    ? "var(--primary)"
                    : "var(--divider-subtle-color)"}
                  strokeLinecap="round"
                  strokeWidth={emphasized ? 1.6 : 1.25}
                />
              );
            })}
          </svg>

          {layout.nodes.map(({ item, x, y }) => {
            const owner = resolveExecutionAgent(directory, item.owner_agent_id);
            const selected = item.id === selectedItem?.id;
            const current = item.id === currentId;
            return (
              <button
                aria-label={`${t("execution.details")}: ${item.subject}`}
                aria-pressed={selected}
                className="absolute z-10 grid h-14 w-14 place-items-center rounded-[16px] transition-[left,top,transform] duration-300 hover:scale-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--primary)"
                data-execution-current-node={current ? "true" : undefined}
                data-execution-node-selected={selected ? "true" : undefined}
                data-execution-work-item-id={item.id}
                key={item.id}
                onClick={() => setSelectedId((currentId) => (
                  currentId === item.id ? null : item.id
                ))}
                style={{ left: x - 28, top: y - 28 }}
                title={`${item.subject} · ${
                  owner?.name ?? t("execution.owner_unassigned")
                }`}
                type="button"
              >
                <ExecutionNodeAvatar
                  agent={owner}
                  current={current}
                  selected={selected}
                  size="graph"
                  status={item.status}
                  title={`${item.subject} · ${
                    owner?.name ?? t("execution.owner_unassigned")
                  }`}
                />
              </button>
            );
          })}

          {selectedItem && selectedNode ? (
            <ExecutionNodePopover
              directory={directory}
              item={selectedItem}
              layoutHeight={layout.height}
              layoutWidth={layout.width}
              x={selectedNode.x}
              y={selectedNode.y}
            />
          ) : null}
        </div>
      </div>
    </div>
  );
}

function ExecutionNodePopover({
  directory,
  item,
  layoutHeight,
  layoutWidth,
  x,
  y,
}: {
  directory: ExecutionAgentDirectory;
  item: ExecutionWorkItemView;
  layoutHeight: number;
  layoutWidth: number;
  x: number;
  y: number;
}) {
  const { t } = useI18n();
  const owner = resolveExecutionAgent(directory, item.owner_agent_id);
  const openRight = x <= layoutWidth / 2;
  const top = Math.max(8, Math.min(y - 42, layoutHeight - 104));
  return (
    <article
      className="absolute z-20 w-[13rem] rounded-[10px] border border-(--surface-control-border) bg-(--surface-control-background) px-3 py-2.5 shadow-(--surface-control-shadow)"
      aria-label={`${t("execution.details")}: ${item.subject}`}
      data-execution-selected-node-detail={item.id}
      role="dialog"
      style={{
        left: openRight ? x + 34 : x - 34,
        top,
        transform: openRight ? undefined : "translateX(-100%)",
      }}
    >
      <div className="flex min-w-0 items-center gap-2">
        <h3 className="min-w-0 flex-1 truncate text-compact font-semibold text-(--text-strong)">
          {item.subject}
        </h3>
        <span className={cn(
          "shrink-0 text-[10px] font-medium",
          selectedStatusTone(item.status),
        )}>
          {t(WORK_ITEM_STATUS_LABEL_KEY[item.status])}
        </span>
      </div>
      <p className="mt-1 line-clamp-2 text-[11px] leading-4 text-(--text-default)">
        {item.objective}
      </p>
      <p className="mt-1.5 flex min-w-0 gap-1 text-[10px] leading-4 text-(--text-soft)">
        <span className="shrink-0">
          {owner?.name ?? t("execution.owner_unassigned")}
        </span>
        <span aria-hidden="true">·</span>
        <span className="min-w-0 truncate text-(--text-default)">
          {item.deliverable}
        </span>
      </p>
    </article>
  );
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
