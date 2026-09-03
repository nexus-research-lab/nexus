/**
 * INPUT: Graph 节点类型、状态、当前标记、展示语义与可选持久或 runtime Agent identity。
 * OUTPUT: 带生命周期或实时工作状态的 Agent/Subagent 头像、动作语义 Tool 图标与 Gate 图标。
 * POS: Composer 节点轨迹与展开 Execution Graph 共用的节点视觉原语。
 */
"use client";

import {
  Bot,
  FilePenLine,
  FileSearch,
  Globe2,
  MousePointerClick,
  Plug,
  Search,
  Send,
  ShieldCheck,
  Sparkles,
  Terminal,
  Wrench,
  Workflow,
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import type {
  ExecutionGraphNodeKind,
  ExecutionWorkItemStatus,
} from "@/types/conversation/execution";

import type { ExecutionAgentIdentity } from "./execution-process-model";
import {
  resolveExecutionToolVisualKind,
  type ExecutionToolVisualKind,
} from "./execution-tool-visual";

const EXECUTION_TOOL_ICON: Record<ExecutionToolVisualKind, LucideIcon> = {
  browser: MousePointerClick,
  external: Plug,
  fetch: Globe2,
  generate: Sparkles,
  generic: Wrench,
  inspect: FileSearch,
  search: Search,
  send: Send,
  terminal: Terminal,
  workflow: Workflow,
  write: FilePenLine,
};

export function ExecutionNodeAvatar({
  agent,
  current = false,
  kind = "agent",
  selected = false,
  size = "compact",
  status,
  title,
  tone = "status",
  toolName,
}: {
  agent: ExecutionAgentIdentity | null;
  current?: boolean;
  kind?: ExecutionGraphNodeKind;
  selected?: boolean;
  size?: "compact" | "dock" | "graph" | "nested";
  status: ExecutionWorkItemStatus;
  title: string;
  tone?: "activity" | "status";
  toolName?: string;
}) {
  const graph = size === "graph";
  const dock = size === "dock";
  const nested = size === "nested";
  const toolVisualKind = resolveExecutionToolVisualKind(toolName);
  const ToolIcon = EXECUTION_TOOL_ICON[toolVisualKind];
  return (
    <span
      className={cn(
        "relative grid shrink-0 place-items-center border bg-(--surface-control-background) p-px transition-[border-color,box-shadow,transform]",
        graph
          ? "h-11 w-11 rounded-[13px]"
          : dock
          ? "h-6 w-6 rounded-[7px]"
          : nested
          ? "h-8.5 w-8.5 rounded-[11px]"
          : "h-6 w-6 rounded-[8px]",
        executionNodeFrameTone(status, tone),
        current
          && (tone === "activity"
            ? "scale-105 ring-2 ring-[color:color-mix(in_srgb,var(--success)_24%,transparent)] ring-offset-1 ring-offset-(--surface-panel-background)"
            : "scale-105 ring-2 ring-[color:var(--status-running-soft-border)] ring-offset-1 ring-offset-(--surface-panel-background)"),
        selected && graph && "ring-2 ring-(--primary)",
      )}
      data-execution-node-agent={agent?.id ?? ""}
      data-execution-node-current={current ? "true" : undefined}
      data-execution-node-kind={kind}
      data-execution-node-status={status}
      data-execution-node-tone={tone}
      data-execution-tool-visual={kind === "tool" ? toolVisualKind : undefined}
      title={title}
    >
      {kind === "tool" ? (
        <ToolIcon
          aria-hidden="true"
          className={cn(
            "text-(--icon-default)",
            dock ? "h-3.5 w-3.5" : nested ? "h-4 w-4" : graph ? "h-5 w-5" : "h-3 w-3",
          )}
          strokeWidth={1.8}
        />
      ) : kind === "gate" ? (
        <ShieldCheck
          aria-hidden="true"
          className={cn(
            "text-(--icon-default)",
            dock ? "h-3.5 w-3.5" : nested ? "h-4 w-4" : graph ? "h-5 w-5" : "h-3 w-3",
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
              : dock
              ? "h-5.5 w-5.5 rounded-[6px]"
              : nested
              ? "h-7 w-7 rounded-[8px]"
              : "h-5 w-5",
          )}
          imageClassName={graph
            ? "rounded-[9px]"
            : dock
            ? "rounded-[5px]"
            : nested
            ? "rounded-[7px]"
            : "rounded-[5px]"}
          name={agent.name}
          size={graph ? "md" : "xs"}
        />
      ) : kind === "subagent" ? (
        // 防御性 fallback；正常 WorkGraph 投影会为每个 Subagent 提供稳定头像。
        <Bot
          aria-hidden="true"
          className={cn(
            "text-(--icon-default)",
            dock ? "h-3.5 w-3.5" : nested ? "h-4 w-4" : graph ? "h-5 w-5" : "h-3 w-3",
          )}
          strokeWidth={1.8}
        />
      ) : (
        <span
          aria-hidden="true"
          className={cn(
            "rounded-full bg-(--icon-muted)",
            graph
              ? "h-3.5 w-3.5"
              : dock
              ? "h-2.5 w-2.5"
              : nested
              ? "h-3 w-3"
              : "h-2 w-2",
          )}
        />
      )}
      <span
        aria-hidden="true"
        className={cn(
          "absolute -bottom-0.5 -right-0.5 rounded-full border border-(--surface-control-background)",
          graph ? "h-2.5 w-2.5" : dock ? "h-1.5 w-1.5" : "h-2 w-2",
          executionNodeDotTone(status, tone),
        )}
      />
    </span>
  );
}

function executionNodeFrameTone(
  status: ExecutionWorkItemStatus,
  tone: "activity" | "status",
): string {
  if (tone === "activity") {
    if (status === "running") {
      return "border-(--success)";
    }
    if (
      status === "blocked"
      || status === "changes_requested"
      || status === "failed"
    ) {
      return "border-(--warning)";
    }
    return "border-(--surface-control-border)";
  }
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

function executionNodeDotTone(
  status: ExecutionWorkItemStatus,
  tone: "activity" | "status",
): string {
  if (tone === "activity") {
    if (status === "running") {
      return "bg-(--success)";
    }
    if (
      status === "blocked"
      || status === "changes_requested"
      || status === "failed"
    ) {
      return "bg-(--warning)";
    }
    return "bg-(--icon-muted)";
  }
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
