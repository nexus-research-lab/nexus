/**
 * INPUT: 当前/最近 ExecutionView、Agent 目录与 terminal dismiss 动作。
 * OUTPUT: Composer 上方紧凑进程胶囊及向上展开的责任/依赖/交付/验收 WorkGraph。
 * POS: DM 与 Room 共用的权威 Execution UI；存在时替代 legacy Todo 进程。
 */
"use client";

import {
  AlertTriangle,
  ChevronDown,
  ChevronUp,
  CircleCheck,
  GitBranch,
  ListTree,
  PauseCircle,
  X,
} from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "@/shared/ui/overlay/overlay-styles";
import type { ExecutionView } from "@/types/conversation/execution";

import {
  buildExecutionSummaryParts,
  EXECUTION_STATUS_LABEL_KEY,
  isTerminalExecutionStatus,
  resolveExecutionAgent,
  resolveSelectedWorkItem,
  type ExecutionAgentDirectory,
} from "./execution-process-model";
import { ExecutionWorkItemDetail } from "./execution-work-item-detail";
import { ExecutionWorkItemList } from "./execution-work-item-list";

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
  const [selectedWorkItemId, setSelectedWorkItemId] = useState<string | null>(
    null,
  );
  const panelId = useId();
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const selectedItem = resolveSelectedWorkItem(execution, selectedWorkItemId);
  const coordinator = resolveExecutionAgent(
    directory,
    execution.coordinator_agent_id,
  );
  const summaryParts = buildExecutionSummaryParts(execution);
  const terminal = isTerminalExecutionStatus(execution.status);

  useEffect(() => {
    setSelectedWorkItemId(null);
  }, [execution.id]);

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

  const collapsePanel = () => {
    triggerRef.current?.focus();
    setIsExpanded(false);
  };

  return (
    <aside
      aria-label={t("execution.label")}
      aria-live="polite"
      className={cn(
        "pointer-events-none relative flex min-w-0 max-w-[min(40rem,calc(100vw-3.5rem))] justify-center",
        className,
      )}
      data-execution-process-panel
      data-execution-status={execution.status}
    >
      <button
        ref={triggerRef}
        aria-controls={panelId}
        aria-expanded={isExpanded}
        aria-label={
          isExpanded
            ? t("execution.collapse_panel")
            : t("execution.expand_panel")
        }
        className={cn(
          "pointer-events-auto inline-flex h-11 min-w-0 max-w-full items-center gap-2 rounded-full px-3.5 text-xs text-(--text-default) transition-[background,border-color,color,box-shadow] hover:border-(--surface-control-hover-border) hover:bg-(--surface-control-hover-background) hover:text-(--text-strong)",
          EXECUTION_TRIGGER_CLASS_NAME,
        )}
        data-execution-process-trigger
        onClick={() => setIsExpanded((current) => !current)}
        type="button"
      >
        {coordinator ? (
          <>
            <span
              className="flex min-w-0 max-w-[7.5rem] items-center gap-1.5"
              title={`${t("execution.coordinator")} · ${coordinator.name}`}
            >
              <UiAgentAvatar
                avatar={coordinator.avatar}
                name={coordinator.name}
                size="xs"
              />
              <span className="min-w-0 truncate text-compact font-semibold">
                {coordinator.name}
              </span>
            </span>
            <span
              aria-hidden="true"
              className="h-3.5 w-px shrink-0 bg-(--divider-subtle-color)"
            />
          </>
        ) : null}
        <ExecutionStatusIcon execution={execution} />
        <span className="shrink-0 font-medium">
          {t(EXECUTION_STATUS_LABEL_KEY[execution.status])}
        </span>
        <span aria-hidden="true" className="text-(--text-soft)">·</span>
        <span className="shrink-0 font-medium tabular-nums">
          {t("execution.progress", {
            accepted: execution.progress.accepted,
            total: execution.progress.total,
          })}
        </span>
        {summaryParts.map((part) => (
          <span
            className="hidden shrink-0 text-(--text-soft) sm:inline"
            key={part.key}
          >
            · {t(part.key, { count: part.count })}
          </span>
        ))}
        <span className="hidden min-w-0 truncate text-(--text-soft) md:inline">
          · {execution.objective}
        </span>
        <ChevronDown
          className={cn(
            "h-3.5 w-3.5 shrink-0 text-(--icon-muted) transition-transform duration-200",
            isExpanded && "rotate-180",
          )}
        />
      </button>

      {isExpanded ? (
        <div className="pointer-events-none absolute bottom-[calc(100%+0.5rem)] left-1/2 -translate-x-1/2">
          <section
            aria-label={t("execution.label")}
            className={cn(
              "pointer-events-auto flex max-h-[min(640px,72dvh)] w-[min(800px,calc(100vw-2rem))] origin-bottom flex-col overflow-hidden",
              OVERLAY_SURFACE_CLASS_NAME,
              ANCHORED_OVERLAY_MOTION_CLASS_NAME,
            )}
            data-execution-workgraph
            data-placement="top"
            id={panelId}
          >
            <ExecutionPanelHeader
              execution={execution}
              onCollapse={collapsePanel}
              onDismiss={terminal
                ? () => {
                    setIsExpanded(false);
                    onDismiss();
                  }
                : null}
            />
            {execution.plan && (execution.work_items?.length ?? 0) > 0 ? (
              <div className="grid min-h-0 flex-1 grid-rows-[minmax(130px,0.72fr)_minmax(220px,1.28fr)] md:grid-cols-[minmax(250px,0.82fr)_minmax(360px,1.18fr)] md:grid-rows-1">
                <ExecutionWorkItemList
                  directory={directory}
                  execution={execution}
                  onSelect={setSelectedWorkItemId}
                  selectedId={selectedItem?.id ?? null}
                />
                {selectedItem ? (
                  <ExecutionWorkItemDetail
                    directory={directory}
                    execution={execution}
                    item={selectedItem}
                  />
                ) : null}
              </div>
            ) : (
              <div className="grid min-h-40 place-items-center px-6 py-8 text-center">
                <div>
                  <ListTree className="mx-auto h-6 w-6 text-(--icon-muted)" />
                  <p className="mt-2 text-xs text-(--text-soft)">
                    {t("execution.no_plan")}
                  </p>
                </div>
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
  onCollapse,
  onDismiss,
}: {
  execution: ExecutionView;
  onCollapse: () => void;
  onDismiss: (() => void) | null;
}) {
  const { t } = useI18n();
  const acceptedRatio = execution.progress.total > 0
    ? execution.progress.accepted / execution.progress.total
    : 0;
  return (
    <header className="shrink-0 border-b border-(--divider-subtle-color) px-3.5 pb-3 pt-3">
      <div className="flex items-start gap-2.5">
        <span className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-[8px] border border-(--surface-control-border) bg-(--surface-interactive-hover-background) text-(--icon-default)">
          <GitBranch className="h-4 w-4" />
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="text-xs font-semibold text-(--text-strong)">
              {t("execution.label")}
            </span>
            <span className="rounded-full bg-(--surface-interactive-hover-background) px-2 py-0.5 text-[10px] font-medium text-(--text-muted)">
              {t(EXECUTION_STATUS_LABEL_KEY[execution.status])}
            </span>
            {execution.plan ? (
              <span className="text-[10px] text-(--text-soft)">
                {t("execution.plan_revision", {
                  revision: execution.plan.revision,
                })}
              </span>
            ) : null}
            {execution.goal_id ? (
              <span className="rounded-full border border-[color:color-mix(in_srgb,var(--primary)_20%,transparent)] px-2 py-0.5 text-[10px] text-(--primary)">
                {t("execution.goal_bound")}
              </span>
            ) : null}
          </div>
          <h2 className="mt-1 text-sm font-semibold leading-5 text-(--text-strong)">
            {execution.objective}
          </h2>
          <div className="mt-2 flex items-center gap-2">
            <div className="h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-(--surface-interactive-hover-background)">
              <div
                className="h-full rounded-full bg-(--success) transition-[width] duration-300"
                style={{
                  width: `${Math.max(0, Math.min(100, acceptedRatio * 100))}%`,
                }}
              />
            </div>
            <span className="shrink-0 text-[10px] tabular-nums text-(--text-soft)">
              {execution.progress.accepted}/{execution.progress.total}
            </span>
          </div>
        </div>
        {onDismiss ? (
          <button
            aria-label={t("execution.dismiss")}
            className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-[7px] text-(--icon-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
            onClick={onDismiss}
            title={t("execution.dismiss")}
            type="button"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        ) : null}
        <button
          aria-label={t("execution.collapse_panel")}
          className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-[7px] text-(--icon-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
          onClick={onCollapse}
          title={t("execution.collapse_panel")}
          type="button"
        >
          <ChevronUp className="h-3.5 w-3.5" />
        </button>
      </div>
    </header>
  );
}

function ExecutionStatusIcon({ execution }: { execution: ExecutionView }) {
  if (execution.status === "active" || execution.status === "waiting") {
    return <span className="grid h-4 w-4 shrink-0 place-items-center"><LoadingOrb /></span>;
  }
  if (execution.status === "completed") {
    return <CircleCheck className="h-4 w-4 shrink-0 text-(--success)" />;
  }
  if (execution.status === "paused") {
    return <PauseCircle className="h-4 w-4 shrink-0 text-(--warning)" />;
  }
  return <AlertTriangle className="h-4 w-4 shrink-0 text-(--warning)" />;
}
