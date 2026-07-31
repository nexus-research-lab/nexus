/**
 * INPUT: Work Item 状态、当前节点标记与可选责任 Agent。
 * OUTPUT: 带状态边框和状态点的 Agent 节点；未分配时显示中性空节点。
 * POS: Composer 节点轨迹与展开 WorkGraph 共用的责任节点视觉原语。
 */
"use client";

import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import type { ExecutionWorkItemStatus } from "@/types/conversation/execution";

import type { ExecutionAgentIdentity } from "./execution-process-model";

export function ExecutionNodeAvatar({
  agent,
  current = false,
  selected = false,
  size = "compact",
  status,
  title,
}: {
  agent: ExecutionAgentIdentity | null;
  current?: boolean;
  selected?: boolean;
  size?: "compact" | "graph";
  status: ExecutionWorkItemStatus;
  title: string;
}) {
  const graph = size === "graph";
  return (
    <span
      className={cn(
        "relative grid shrink-0 place-items-center border bg-(--surface-control-background) p-px transition-[border-color,box-shadow,transform]",
        graph ? "h-11 w-11 rounded-[13px]" : "h-6 w-6 rounded-[8px]",
        executionNodeFrameTone(status),
        current
          && "scale-105 ring-2 ring-[color:var(--status-running-soft-border)] ring-offset-1 ring-offset-(--surface-panel-background)",
        selected && graph && "ring-2 ring-(--primary)",
      )}
      data-execution-node-agent={agent?.id ?? ""}
      data-execution-node-current={current ? "true" : undefined}
      data-execution-node-status={status}
      title={title}
    >
      {agent ? (
        <UiAgentAvatar
          avatar={agent.avatar}
          className={cn(
            "border-0 shadow-none",
            graph ? "h-9.5 w-9.5 rounded-[10px]" : "h-5 w-5",
          )}
          imageClassName={graph ? "rounded-[9px]" : "rounded-[5px]"}
          name={agent.name}
          size={graph ? "md" : "xs"}
        />
      ) : (
        <span
          aria-hidden="true"
          className={cn(
            "rounded-full bg-(--icon-muted)",
            graph ? "h-3.5 w-3.5" : "h-2 w-2",
          )}
        />
      )}
      <span
        aria-hidden="true"
        className={cn(
          "absolute -bottom-0.5 -right-0.5 rounded-full border border-(--surface-control-background)",
          graph ? "h-2.5 w-2.5" : "h-2 w-2",
          executionNodeDotTone(status),
        )}
      />
    </span>
  );
}

function executionNodeFrameTone(status: ExecutionWorkItemStatus): string {
  if (status === "accepted") {
    return "border-(--success)";
  }
  if (
    status === "blocked"
    || status === "changes_requested"
    || status === "failed"
  ) {
    return "border-(--warning)";
  }
  if (
    status === "running"
    || status === "submitted"
    || status === "ready"
    || status === "assigned"
  ) {
    return "border-(--primary)";
  }
  return "border-(--surface-control-border)";
}

function executionNodeDotTone(status: ExecutionWorkItemStatus): string {
  if (status === "accepted") {
    return "bg-(--success)";
  }
  if (
    status === "blocked"
    || status === "changes_requested"
    || status === "failed"
  ) {
    return "bg-(--warning)";
  }
  if (
    status === "running"
    || status === "submitted"
    || status === "ready"
    || status === "assigned"
  ) {
    return "bg-(--primary)";
  }
  return "bg-(--icon-muted)";
}
