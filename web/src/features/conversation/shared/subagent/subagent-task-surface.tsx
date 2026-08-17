"use client";

import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";

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
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
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
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const handledRequestKeyRef = useRef(0);
  const {
    data,
    error,
    isLoading,
    refresh,
    tasks,
  } = useSubagentTasks(source, selectedTaskId === null, hostAgentId);
  const visibleTasks = useMemo(
    () => filterSubagentTasksByHostAgent(tasks, hostAgentId),
    [hostAgentId, tasks],
  );
  const selectedTask = visibleTasks.find(
    (task) => task.task_id === selectedTaskId,
  ) ?? null;

  useEffect(() => {
    const requestedToolUseId = requestedTaskToolUseId?.trim() ?? "";
    if (
      requestKey <= 0
      || !requestedToolUseId
      || handledRequestKeyRef.current === requestKey
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
    handledRequestKeyRef.current = requestKey;
    setSelectedTaskId(requestedTask.task_id);
  }, [requestKey, requestedTaskToolUseId, visibleTasks]);

  useEffect(() => {
    if (selectedTaskId && data && !selectedTask) {
      setSelectedTaskId(null);
    }
  }, [data, selectedTask, selectedTaskId]);

  if (selectedTask) {
    return (
      <SubagentTaskThread
        layout={layout}
        onBack={() => {
          setSelectedTaskId(null);
          void refresh(true);
        }}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        source={source}
        task={selectedTask}
      />
    );
  }

  return (
    <SubagentTaskList
      data={data}
      error={error}
      headerLeading={headerLeading}
      isLoading={isLoading}
      onClose={onClose}
      onRefresh={() => void refresh()}
      onSelectTask={setSelectedTaskId}
      showTitle={layout === "mobile"}
      tasks={visibleTasks}
    />
  );
}
