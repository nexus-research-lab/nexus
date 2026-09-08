// INPUT: Exact Session/Room source, optional caller filter, task navigation request and shared file actions.
// OUTPUT: Source/caller-isolated navigation, truthful missing-target feedback and focus handoff between directory/detail.
// POS: Shared subagent surface assembly; member identity belongs to the Room adapter, queries to resource hooks.
"use client";

import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  type ReactNode,
} from "react";

import type { WorkspaceFileOpenHandler } from "@/lib/workspace-file-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";

import { SubagentTaskList } from "./subagent-task-list";
import { filterSubagentTasksByHostAgent } from "./subagent-task-list-model";
import {
  findSubagentTaskByToolUseId,
  subagentTaskSourceKey,
} from "./subagent-task-model";
import { SubagentTaskThread } from "./thread/subagent-task-thread";
import { useSubagentTasks } from "./use-subagent-tasks";

interface SubagentTaskSurfaceProps {
  headerLeading?: ReactNode;
  hostAgentId?: string | null;
  layout?: "desktop" | "mobile";
  onClose: () => void;
  onOpenWorkspaceFile?: WorkspaceFileOpenHandler;
  requestKey?: number;
  requestedTaskToolUseId?: string | null;
  source: SubagentTaskSource;
}

export function SubagentTaskSurface({
  headerLeading,
  hostAgentId,
  layout = "desktop",
  onClose,
  onOpenWorkspaceFile,
  requestKey = 0,
  requestedTaskToolUseId,
  source,
}: SubagentTaskSurfaceProps) {
  const sourceKey = subagentTaskSourceKey(source);
  return (
    <SubagentTaskSourceSurface
      key={sourceKey}
      headerLeading={headerLeading}
      hostAgentId={hostAgentId}
      layout={layout}
      onClose={onClose}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
      requestKey={requestKey}
      requestedTaskToolUseId={requestedTaskToolUseId}
      source={source}
    />
  );
}

function SubagentTaskSourceSurface({
  headerLeading,
  hostAgentId,
  layout = "desktop",
  onClose,
  onOpenWorkspaceFile,
  requestKey = 0,
  requestedTaskToolUseId,
  source,
}: SubagentTaskSurfaceProps) {
  const hostFilterKey = hostAgentId?.trim() ?? null;
  const { t } = useI18n();
  const rootRef = useRef<HTMLDivElement>(null);
  const previousTaskRef = useRef<string | null>(null);
  const [selectedTaskId, setSelectedTaskId] = useResettableState<string | null>(null, hostFilterKey);
  const handledRequestRef = useRef<string | null>(null);
  const requestedToolUseId = requestedTaskToolUseId?.trim() ?? "";
  const requestIdentity = Number.isSafeInteger(requestKey) && requestKey > 0 && requestedToolUseId
    ? JSON.stringify([requestKey, requestedToolUseId])
    : null;
  const {
    data,
    error,
    isLoading,
    refresh,
    tasks,
  } = useSubagentTasks(source, selectedTaskId === null && hostFilterKey !== "", hostAgentId);
  const visibleTasks = useMemo(
    () => data && !data.capabilities.observe ? [] : filterSubagentTasksByHostAgent(tasks, hostAgentId),
    [data, hostAgentId, tasks],
  );
  const selectedTask = visibleTasks.find(
    (task) => task.task_id === selectedTaskId,
  ) ?? null;

  useLayoutEffect(() => {
    const previousTaskId = previousTaskRef.current;
    const currentTaskId = selectedTask?.task_id ?? null;
    previousTaskRef.current = currentTaskId;
    const root = rootRef.current;
    if (!root || previousTaskId === currentTaskId) return;
    const returnedRow = Array.from(root.querySelectorAll<HTMLElement>("[data-subagent-task-id]"))
      .find((row) => row.dataset.subagentTaskId === previousTaskId);
    const target = currentTaskId
      ? root.querySelector<HTMLElement>("header button")
      : returnedRow;
    // Returning to a lower catalog row should let the browser reveal it in the
    // directory scroller; header/fallback focus does not move scroll positions.
    (target ?? root.querySelector<HTMLElement>("button, [role='button']") ?? root).focus({ preventScroll: !returnedRow });
  }, [selectedTask?.task_id]);

  useEffect(() => {
    if (
      !requestIdentity
      || handledRequestRef.current === requestIdentity
    ) {
      return;
    }
    const requestedTask = findSubagentTaskByToolUseId(
      visibleTasks,
      requestedToolUseId,
    );
    if (!requestedTask) {
      return;
    }
    handledRequestRef.current = requestIdentity;
    setSelectedTaskId(requestedTask.task_id);
  }, [requestIdentity, requestedToolUseId, setSelectedTaskId, visibleTasks]);

  useEffect(() => {
    if (selectedTaskId && data && !selectedTask) {
      setSelectedTaskId(null);
    }
  }, [data, selectedTask, selectedTaskId, setSelectedTaskId]);

  return (
    <div
      ref={rootRef}
      role="group"
      aria-label={t("subagents.panel_title")}
      tabIndex={-1}
      className="flex min-h-0 min-w-0 flex-1 flex-col"
    >
      {selectedTask ? (
        <SubagentTaskThread
          layout={layout}
          onBack={() => {
            if (requestIdentity) handledRequestRef.current = requestIdentity;
            setSelectedTaskId(null);
            void refresh(true);
          }}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          source={source}
          task={selectedTask}
        />
      ) : (
        <SubagentTaskList
          data={data}
          error={error}
          headerLeading={headerLeading}
          isLoading={isLoading}
          onClose={onClose}
          onRefresh={() => void refresh()}
          requestedTaskUnavailable={Boolean(
            requestIdentity && handledRequestRef.current !== requestIdentity
            && data?.capabilities.observe && !findSubagentTaskByToolUseId(visibleTasks, requestedToolUseId),
      )}
      onSelectTask={(taskId) => {
        if (requestIdentity) handledRequestRef.current = requestIdentity;
        setSelectedTaskId(taskId);
      }}
      showTitle={layout === "mobile"}
      tasks={visibleTasks}
    />
      )}
    </div>
  );
}
