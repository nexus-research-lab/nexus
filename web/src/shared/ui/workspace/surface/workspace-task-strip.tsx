/**
 * INPUT: 当前会话 scope、只读任务列表、来源 Agent 与可选来源控件。
 * OUTPUT: 可换行任务摘要与共享非模态明细浮层；会话切换关闭，任务目录/来源变更清空旧详情。
 * POS: 锚在 Composer 顶边的 Workspace 会话级只读任务入口。
 */
"use client";

import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { ChevronDown, ChevronUp, Circle, CircleCheck, ListChecks } from "lucide-react";
import {
  type ReactNode,
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
} from "react";
import { createPortal } from "react-dom";

import { getTabbableElements } from "@/shared/lib/browser/focus-navigation";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveUiAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-layout";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "@/shared/ui/overlay/overlay-contract";
import { focusAfterAnchoredOverlayExit } from "@/shared/ui/overlay/overlay-focus-navigation";

import { cn } from "@/shared/ui/class-name";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "@/shared/ui/overlay/overlay-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { TodoItem } from "@/types/conversation/todo";

import { getConversationActivityChipClassName } from "./conversation-activity-chip-styles";
import { resolveWorkspaceTaskState } from "./workspace-task-strip-model";

interface WorkspaceTaskPanelProps {
  scopeKey?: string;
  todos: TodoItem[];
  className?: string;
  source?: WorkspaceTaskSource;
  sourceControl?: ReactNode;
}

export interface WorkspaceTaskSource {
  agentId: string;
  avatar: string | null;
  /** 可选的去歧义名称；头像缩写仍取原始展示姓名。 */
  label?: string;
  name: string;
}

export function WorkspaceTaskPanel({
  todos,
  scopeKey,
  className,
  source,
  sourceControl,
}: WorkspaceTaskPanelProps) {
  const { t } = useI18n();
  const taskState = useMemo(
    () => resolveWorkspaceTaskState(todos),
    [todos],
  );
  const normalizedTodos = taskState?.todos ?? [];
  const hasTasks = taskState !== null;
  const [isExpanded, setIsExpanded] = useResettableState(false, JSON.stringify([scopeKey, hasTasks]));
  // Todo 没有稳定条目 ID；结构变化时关闭旧详情，不按相同行号猜测新任务身份。
  const contentCounts = new Map<string, number>();
  for (const todo of normalizedTodos) contentCounts.set(todo.content, (contentCounts.get(todo.content) ?? 0) + 1);
  const detailScope = JSON.stringify([scopeKey, source?.agentId, isExpanded, normalizedTodos.map((todo) => [
    todo.content, Boolean(taskDetail(todo)),
    contentCounts.get(todo.content)! > 1 ? [todo.active_form, todo.status] : null,
  ])]);
  const [expandedTaskIndex, setExpandedTaskIndex] = useResettableState<number | null>(null, detailScope);
  const summaryId = useId();
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const focusedControlRef = useRef<HTMLElement | null>(null);
  const closePanel = useCallback(() => setIsExpanded(false), [setIsExpanded]);
  const { overlayId: panelId, overlayPosition, overlayRef, overlayStyle, portalContainer, updateOverlayPosition } = useAnchoredOverlayLayer({
    anchorRef: triggerRef,
    disabled: !hasTasks,
    estimatePosition: resolveTaskPopoverPosition,
    isOpen: isExpanded,
    onClose: closePanel,
  });
  const collapsePanel = useCallback(() => {
    triggerRef.current?.focus();
    closePanel();
  }, [closePanel]);
  useEffect(() => {
    if (!isExpanded) return;
    const handleTab = (event: KeyboardEvent) => {
      const root = overlayRef.current;
      if (!root || event.key !== "Tab" || event.defaultPrevented || isImeKeyboardEvent(event)
        || !(event.target instanceof Node) || !root.contains(event.target)) return;
      const controls = getTabbableElements(root);
      const current = document.activeElement;
      const exitsBackward = event.shiftKey && (current === root || current === controls[0]);
      const exitsForward = !event.shiftKey && (current === controls.at(-1) || controls.length === 0);
      if (!exitsBackward && !exitsForward) return;
      event.preventDefault();
      event.stopPropagation();
      collapsePanel();
      if (exitsForward) focusAfterAnchoredOverlayExit([root], false);
    };
    document.addEventListener("keydown", handleTab);
    return () => document.removeEventListener("keydown", handleTab);
  }, [collapsePanel, isExpanded, overlayRef]);
  const isPositioned = overlayPosition !== null;
  useLayoutEffect(() => {
    if (isExpanded) updateOverlayPosition();
  }, [isExpanded, source?.label, source?.name, t, taskState, updateOverlayPosition]);
  useEffect(() => {
    if (isExpanded && isPositioned) overlayRef.current?.focus({ preventScroll: true });
  }, [isExpanded, isPositioned, overlayRef]);
  useLayoutEffect(() => {
    const previous = focusedControlRef.current;
    if (isExpanded && previous && !previous.isConnected && document.activeElement === document.body) {
      overlayRef.current?.focus({ preventScroll: true });
    }
  }, [detailScope, isExpanded, overlayRef]);

  if (taskState === null) {
    return null;
  }

  const {
    completedCount,
    currentStep,
    hasRunningTask,
    summary,
    totalCount,
  } = taskState.summary;

  const taskStatusLabel = (status: TodoItem["status"]) => {
    if (status === "completed") {
      return t("tasks.status_completed");
    }
    if (status === "in_progress") {
      return t("tasks.status_in_progress");
    }
    return t("tasks.status_pending");
  };

  const renderStatusMarker = (status: TodoItem["status"]) => {
    if (status === "completed") {
      return <CircleCheck aria-hidden="true" className="h-3.5 w-3.5 text-(--success)" />;
    }
    if (status === "in_progress") {
      return <Circle aria-hidden="true" className="h-2.5 w-2.5 fill-current text-(--status-running-soft-text)" />;
    }
    return <Circle aria-hidden="true" className="h-2.5 w-2.5 text-(--icon-muted)" />;
  };


  return (
    <aside
      aria-label={t("tasks.label")}
      aria-live="polite"
      data-workspace-task-panel
      className={cn(
        "pointer-events-none relative flex min-w-0 max-w-[min(42rem,calc(100vw-4rem))] justify-center",
        className,
      )}
    >
      <button
        ref={triggerRef}
        aria-controls={isExpanded ? panelId : undefined}
        aria-describedby={summaryId}
        aria-haspopup="dialog"
        aria-expanded={isExpanded}
        aria-label={isExpanded ? t("tasks.collapse_panel") : t("tasks.expand_panel")}
        className="group pointer-events-auto flex min-h-11 min-w-0 max-w-full items-center focus-visible:outline-none"
        data-workspace-task-summary={summary}
        data-workspace-task-trigger
        onClick={() => setIsExpanded((current) => !current)}
        type="button"
      >
        <span
          className={getConversationActivityChipClassName("inline-flex min-w-0 max-w-full items-center gap-1.5 px-2 py-1 transition-[background,color] duration-(--motion-duration-fast) group-hover:bg-(--surface-control-hover-background) group-hover:text-(--text-strong) group-focus-visible:bg-(--surface-control-hover-background) group-focus-visible:ring-2 group-focus-visible:ring-[color:var(--ring)]", "plain")}
          data-workspace-task-visual
          id={summaryId}
        >
          {source ? (
            <WorkspaceTaskSourceIdentity source={source} />
          ) : null}
          <span className="grid h-3.5 w-3.5 shrink-0 place-items-center">
            {hasRunningTask ? (
              <LoadingOrb />
            ) : completedCount === totalCount ? (
              <CircleCheck aria-hidden="true" className="h-3.5 w-3.5 text-(--success)" />
            ) : (
              <ListChecks aria-hidden="true" className="h-3.5 w-3.5 text-(--icon-muted)" />
            )}
          </span>
          <span className={cn(
            "shrink-0 tabular-nums",
            getUiTypographyClassName({ role: "metadata", tone: "strong", weight: "medium" }),
          )}>
            {t("tasks.step_progress", {
              current: currentStep,
              total: totalCount,
            })}
          </span>
          <span className={cn(
            "min-w-0 whitespace-normal break-words text-left",
            getUiTypographyClassName({ role: "metadata", tone: "muted" }),
          )}>
            {summary}
          </span>
          <ChevronDown
            className={cn(
              "h-3.5 w-3.5 shrink-0 text-(--icon-muted) transition-transform duration-200",
              isExpanded && "rotate-180",
            )}
          />
        </span>
      </button>
      {isExpanded && portalContainer ? createPortal(
        <div
          ref={overlayRef}
          aria-label={t("tasks.label")}
          aria-modal={false}
          className={cn(
            "pointer-events-auto fixed ui-layer-popover flex origin-bottom flex-col overflow-hidden outline-none",
            OVERLAY_SURFACE_CLASS_NAME,
            ANCHORED_OVERLAY_MOTION_CLASS_NAME,
          )}
          data-placement={overlayPosition?.placement ?? "top"}
          id={panelId}
          onFocusCapture={(event) => {
            if (event.target instanceof HTMLElement && event.currentTarget.contains(event.target)) {
              focusedControlRef.current = event.target;
            }
          }}
          role="dialog"
          style={overlayStyle}
          tabIndex={-1}
          {...OPEN_OVERLAY_DATA_ATTRIBUTES}
        >
          <div className="flex h-10 shrink-0 items-center gap-2 px-3">
            {sourceControl || source ? (
              <>
                <div
                  className="min-w-0 max-w-[9rem]"
                  data-workspace-task-expanded-source
                >
                  {sourceControl ?? (
                    source ? <WorkspaceTaskSourceIdentity source={source} /> : null
                  )}
                </div>
                <span
                  aria-hidden="true"
                  className="h-3.5 w-px shrink-0 bg-(--divider-subtle-color)"
                />
              </>
            ) : null}
              <span className={cn(
                "shrink-0",
                getUiTypographyClassName({ role: "metadata", tone: "strong", weight: "semibold" }),
              )}>
                {t("tasks.label")}
              </span>
              <span
                className={cn(
                  "shrink-0 tabular-nums",
                  getUiTypographyClassName({ role: "metadata", tone: "soft" }),
                )}
                data-workspace-task-progress-label
              >
                {completedCount}/{totalCount}
              </span>
              <span className="min-w-0 flex-1" />
              {hasRunningTask ? <LoadingOrb /> : null}
              <UiIconButton
                aria-label={t("tasks.collapse_panel")}
                className="shrink-0"
                onClick={collapsePanel}
                size="xs"
              >
                <ChevronUp className="h-3.5 w-3.5" />
              </UiIconButton>
            </div>

            <ol className="soft-scrollbar min-h-0 overflow-y-auto pb-1.5">
              {normalizedTodos.map((todo, index) => {
                const detailText = taskDetail(todo);
                const hasDetail = detailText.length > 0;
                const taskId = `${panelId}-task-${index}`;
                const detailId = `${taskId}-detail`;
                const isDetailExpanded = expandedTaskIndex === index;

                return (
                  <li
                    className="flex min-w-0 items-start gap-2 px-3 py-1.5"
                    key={JSON.stringify([todo.content, contentCounts.get(todo.content)! > 1 ? index : null])}
                  >
                    <span className="flex h-5 w-5 shrink-0 items-center justify-center">
                      {renderStatusMarker(todo.status)}
                      <span className="sr-only">{taskStatusLabel(todo.status)}</span>
                    </span>
                    <div className="min-w-0 flex-1">
                      <p
                        id={taskId}
                        className={cn(
                          "break-words",
                          getUiTypographyClassName({ role: "metadata", tone: "default" }),
                          todo.status === "completed" && "text-(--text-soft) line-through",
                        )}
                      >
                        {todo.content}
                      </p>
                      {isDetailExpanded && hasDetail ? (
                        <p id={detailId} className={cn(
                          "mt-0.5 break-words border-l border-(--divider-subtle-color) pl-2",
                          getUiTypographyClassName({ role: "caption", tone: "muted" }),
                        )}>
                          {detailText}
                        </p>
                      ) : null}
                    </div>
                    {hasDetail ? (
                      <UiIconButton
                        aria-controls={isDetailExpanded ? detailId : undefined}
                        aria-describedby={taskId}
                        aria-expanded={isDetailExpanded}
                        aria-label={isDetailExpanded ? t("tasks.collapse_detail") : t("tasks.expand_detail")}
                        className="shrink-0"
                        onClick={() => setExpandedTaskIndex((currentIndex) => (
                          currentIndex === index ? null : index
                        ))}
                        size="xs"
                      >
                        <ChevronDown
                          className={cn(
                            "h-3.5 w-3.5 transition-transform duration-200",
                            isDetailExpanded && "rotate-180",
                          )}
                        />
                      </UiIconButton>
                    ) : null}
                  </li>
                );
              })}
            </ol>
          </div>,
          portalContainer,
      ) : null}
    </aside>
  );
}

function WorkspaceTaskSourceIdentity({
  source,
}: {
  source: WorkspaceTaskSource;
}) {
  return (
    <UiTooltip label={source.label ?? source.name}><span
      className="flex min-w-0 max-w-[7.5rem] items-center gap-1.5"
      data-workspace-task-agent-id={source.agentId}

    >
      <UiAgentAvatar
        aria-hidden="true"
        avatar={source.avatar}
        className="shadow-none"
        name={source.name}
        size="xs"
      />
      <span className={cn(
        "min-w-0 truncate",
        getUiTypographyClassName({ role: "caption", tone: "default", weight: "medium" }),
      )}>
        {source.label ?? source.name}
      </span>
    </span></UiTooltip>
  );
}

function taskDetail(todo: TodoItem): string {
  const detail = todo.active_form?.trim() ?? "";
  return detail !== todo.content.trim() ? detail : "";
}

function resolveTaskPopoverPosition(anchor: HTMLButtonElement) {
  return resolveUiAnchoredOverlayPosition({ anchor, placement: "top", align: "center", preset: "reference-list" });
}
