/**
 * INPUT: 有序 Work Item、依赖关系、Agent 目录与当前选择。
 * OUTPUT: 带责任人、状态、依赖边提示和键盘可选中的 WorkGraph 列表。
 * POS: Execution 展开面板左侧责任流。
 */
"use client";

import {
  AlertTriangle,
  Circle,
  CircleCheck,
  CircleDot,
  GitBranch,
  Send,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import type {
  ExecutionView,
  ExecutionWorkItemStatus,
  ExecutionWorkItemView,
} from "@/types/conversation/execution";

import {
  resolveExecutionAgent,
  resolveWorkItemDepths,
  type ExecutionAgentDirectory,
  WORK_ITEM_KIND_LABEL_KEY,
  WORK_ITEM_STATUS_LABEL_KEY,
} from "./execution-process-model";

export function ExecutionWorkItemList({
  directory,
  execution,
  onSelect,
  selectedId,
}: {
  directory: ExecutionAgentDirectory;
  execution: ExecutionView;
  onSelect: (workItemId: string) => void;
  selectedId: string | null;
}) {
  const { t } = useI18n();
  const items = execution.work_items ?? [];
  const itemById = new Map(items.map((item) => [item.id, item]));
  const depthById = resolveWorkItemDepths(execution);
  return (
    <div className="flex min-h-0 flex-col border-b border-(--divider-subtle-color) md:border-r md:border-b-0">
      <div className="flex h-10 shrink-0 items-center justify-between px-3">
        <span className="text-compact font-semibold text-(--text-strong)">
          {t("execution.work_items")}
        </span>
        <span className="text-xs tabular-nums text-(--text-soft)">
          {execution.progress.accepted}/{execution.progress.total}
        </span>
      </div>
      <div
        className="soft-scrollbar min-h-0 overflow-y-auto px-2 pb-2"
        role="listbox"
      >
        {items.map((item) => {
          const owner = resolveExecutionAgent(directory, item.owner_agent_id);
          const dependencies = (item.dependency_ids ?? [])
            .map((id) => itemById.get(id))
            .filter((value): value is ExecutionWorkItemView => Boolean(value));
          const selected = selectedId === item.id;
          const depth = depthById[item.id] ?? 0;
          const indent = depth * 12;
          return (
            <button
              aria-selected={selected}
              className={cn(
                "relative mb-1 flex w-full min-w-0 items-start gap-2 rounded-[10px] border px-2.5 py-2.5 text-left transition-[background,border-color,box-shadow]",
                selected
                  ? "border-(--surface-control-hover-border) bg-(--surface-control-hover-background) shadow-(--surface-control-shadow)"
                  : "border-transparent hover:border-(--surface-control-border) hover:bg-(--surface-interactive-hover-background)",
              )}
              data-execution-work-item-id={item.id}
              data-execution-work-item-depth={depth}
              key={item.id}
              onClick={() => onSelect(item.id)}
              role="option"
              style={{
                marginLeft: `${indent}px`,
                width: `calc(100% - ${indent}px)`,
              }}
              type="button"
            >
              {depth > 0 ? (
                <span
                  aria-hidden="true"
                  className="absolute -left-2 top-0 h-1/2 w-2 rounded-bl-[6px] border-b border-l border-(--divider-subtle-color)"
                />
              ) : null}
              <WorkItemStatusMarker status={item.status} />
              <span className="min-w-0 flex-1">
                <span className="flex min-w-0 items-center gap-1.5">
                  <span className="min-w-0 flex-1 truncate text-compact font-semibold text-(--text-default)">
                    {item.subject}
                  </span>
                  {item.terminal ? (
                    <span className="shrink-0 rounded-full bg-(--surface-interactive-hover-background) px-1.5 py-0.5 text-[10px] text-(--text-muted)">
                      {t("execution.terminal")}
                    </span>
                  ) : null}
                </span>
                <span className="mt-0.5 flex min-w-0 items-center gap-1.5 text-xs text-(--text-soft)">
                  <span className="shrink-0">
                    {t(WORK_ITEM_KIND_LABEL_KEY[item.kind])}
                  </span>
                  <span aria-hidden="true">·</span>
                  <span className="min-w-0 truncate">
                    {t(WORK_ITEM_STATUS_LABEL_KEY[item.status])}
                  </span>
                </span>
                {dependencies.length > 0 ? (
                  <span className="mt-1.5 flex min-w-0 items-center gap-1 text-[10px] text-(--text-muted)">
                    <GitBranch className="h-3 w-3 shrink-0" />
                    <span className="min-w-0 truncate">
                      {dependencies.map((dependency) => dependency.subject).join(" · ")}
                    </span>
                  </span>
                ) : null}
              </span>
              <span className="flex shrink-0 flex-col items-end gap-1">
                {owner ? (
                  <UiAgentAvatar
                    avatar={owner.avatar}
                    name={owner.name}
                    size="xs"
                    title={owner.name}
                  />
                ) : (
                  <span
                    className="grid h-5.5 w-5.5 place-items-center rounded-[6px] border border-dashed border-(--surface-control-border) text-[9px] text-(--text-soft)"
                    title={t("execution.owner_unassigned")}
                  >
                    —
                  </span>
                )}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

export function WorkItemStatusMarker({
  status,
}: {
  status: ExecutionWorkItemStatus;
}) {
  const baseClassName = "mt-0.5 grid h-5 w-5 shrink-0 place-items-center";
  switch (status) {
  case "accepted":
    return (
      <span className={baseClassName}>
        <CircleCheck className="h-[18px] w-[18px] text-(--success)" />
      </span>
    );
  case "running":
    return <span className={baseClassName}><LoadingOrb /></span>;
  case "blocked":
  case "changes_requested":
  case "failed":
    return (
      <span className={baseClassName}>
        <AlertTriangle className="h-4 w-4 text-(--warning)" />
      </span>
    );
  case "submitted":
    return (
      <span className={baseClassName}>
        <Send className="h-3.5 w-3.5 text-(--primary)" />
      </span>
    );
  case "ready":
  case "assigned":
    return (
      <span className={baseClassName}>
        <CircleDot className="h-4 w-4 text-(--primary)" />
      </span>
    );
  default:
    return (
      <span className={baseClassName}>
        <Circle className="h-3 w-3 text-(--icon-muted)" />
      </span>
    );
  }
}
