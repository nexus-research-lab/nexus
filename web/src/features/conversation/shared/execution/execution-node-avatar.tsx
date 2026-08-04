/**
 * INPUT: Graph 节点类型、状态、当前标记与可选持久 Agent。
 * OUTPUT: 带状态环的 Agent 头像或轻量 Subagent 图标。
 * POS: Composer 节点轨迹与展开 Execution Graph 共用的节点视觉原语。
 */
"use client";

import { Bot, ShieldCheck, Wrench } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import type {
  ExecutionGraphNodeKind,
  ExecutionWorkItemStatus,
} from "@/types/conversation/execution";

import type { ExecutionAgentIdentity } from "./execution-process-model";

export function ExecutionNodeAvatar({
  agent,
  current = false,
  kind = "agent",
  selected = false,
  size = "compact",
  status,
  title,
}: {
  agent: ExecutionAgentIdentity | null;
  current?: boolean;
  kind?: ExecutionGraphNodeKind;
  selected?: boolean;
  size?: "compact" | "graph" | "nested";
  status: ExecutionWorkItemStatus;
  title: string;
}) {
  const graph = size === "graph";
  const nested = size === "nested";
  return (
    <span
      className={cn(
        "relative grid shrink-0 place-items-center border bg-(--surface-control-background) p-px transition-[border-color,box-shadow,transform]",
        graph
          ? "h-11 w-11 rounded-[13px]"
          : nested
          ? "h-8.5 w-8.5 rounded-[11px]"
          : "h-6 w-6 rounded-[8px]",
        executionNodeFrameTone(status),
        current
          && "scale-105 ring-2 ring-[color:var(--status-running-soft-border)] ring-offset-1 ring-offset-(--surface-panel-background)",
        selected && graph && "ring-2 ring-(--primary)",
      )}
      data-execution-node-agent={agent?.id ?? ""}
      data-execution-node-current={current ? "true" : undefined}
      data-execution-node-kind={kind}
      data-execution-node-status={status}
      title={title}
    >
      {kind === "tool" ? (
        <Wrench
          aria-hidden="true"
          className={cn(
            "text-(--icon-default)",
            nested ? "h-4 w-4" : graph ? "h-5 w-5" : "h-3 w-3",
          )}
          strokeWidth={1.8}
        />
      ) : agent ? (
        <UiAgentAvatar
          avatar={agent.avatar}
          className={cn(
            "border-0 shadow-none",
            graph
              ? "h-9.5 w-9.5 rounded-[10px]"
              : nested
              ? "h-7 w-7 rounded-[8px]"
              : "h-5 w-5",
          )}
          imageClassName={graph
            ? "rounded-[9px]"
            : nested
            ? "rounded-[7px]"
            : "rounded-[5px]"}
          name={agent.name}
          size={graph ? "md" : "xs"}
        />
      ) : kind === "subagent" ? (
        <Bot
          aria-hidden="true"
          className={cn(
            "text-(--icon-default)",
            nested ? "h-4 w-4" : graph ? "h-5 w-5" : "h-3 w-3",
          )}
          strokeWidth={1.8}
        />
      ) : kind === "gate" ? (
        <ShieldCheck
          aria-hidden="true"
          className={cn(
            "text-(--icon-default)",
            nested ? "h-4 w-4" : graph ? "h-5 w-5" : "h-3 w-3",
          )}
          strokeWidth={1.8}
        />
      ) : (
        <span
          aria-hidden="true"
          className={cn(
            "rounded-full bg-(--icon-muted)",
            graph ? "h-3.5 w-3.5" : nested ? "h-3 w-3" : "h-2 w-2",
          )}
        />
      )}
      <span
        aria-hidden="true"
        className={cn(
          "absolute -bottom-0.5 -right-0.5 rounded-full border border-(--surface-control-background)",
          graph ? "h-2.5 w-2.5" : nested ? "h-2 w-2" : "h-2 w-2",
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
