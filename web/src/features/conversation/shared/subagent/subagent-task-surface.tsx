"use client";

import { useCallback, useEffect, useState } from "react";

import type { SubagentTaskSource } from "@/types/conversation/subagent-task";

import { SubagentTaskList } from "./subagent-task-list";
import { subagentTaskSourceKey } from "./subagent-task-model";
import { SubagentTaskThread } from "./thread/subagent-task-thread";
import { useSubagentTasks } from "./use-subagent-tasks";

interface SubagentTaskSurfaceProps {
  layout?: "desktop" | "mobile";
  onClose: () => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  source: SubagentTaskSource;
}

export function SubagentTaskSurface({
  layout = "desktop",
  onClose,
  onOpenWorkspaceFile,
  source,
}: SubagentTaskSurfaceProps) {
  const sourceKey = subagentTaskSourceKey(source);
  return (
    <SubagentTaskSourceSurface
      key={sourceKey}
      layout={layout}
      onClose={onClose}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
      source={source}
    />
  );
}

function SubagentTaskSourceSurface({
  layout = "desktop",
  onClose,
  onOpenWorkspaceFile,
  source,
}: SubagentTaskSurfaceProps) {
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const {
    data,
    error,
    isLoading,
    refresh,
    tasks,
  } = useSubagentTasks(source, true);
  const selectedTask = tasks.find((task) => task.task_id === selectedTaskId) ?? null;

  useEffect(() => {
    if (selectedTaskId && data && !selectedTask) {
      setSelectedTaskId(null);
    }
  }, [data, selectedTask, selectedTaskId]);

  const refreshTasks = useCallback(() => {
    void refresh(true);
  }, [refresh]);

  if (selectedTask) {
    return (
      <SubagentTaskThread
        layout={layout}
        onBack={() => setSelectedTaskId(null)}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        onRefreshTasks={refreshTasks}
        source={source}
        task={selectedTask}
      />
    );
  }

  return (
    <SubagentTaskList
      data={data}
      error={error}
      isLoading={isLoading}
      onClose={onClose}
      onRefresh={() => void refresh()}
      onSelectTask={setSelectedTaskId}
      showTitle={layout === "mobile"}
      tasks={tasks}
    />
  );
}
