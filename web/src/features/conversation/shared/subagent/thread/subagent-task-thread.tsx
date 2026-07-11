"use client";

import type { SubagentTask, SubagentTaskSource } from "@/types/conversation/subagent-task";

import { SubagentTaskThreadView } from "./subagent-task-thread-view";
import { useSubagentTaskThread } from "./use-subagent-task-thread";

interface SubagentTaskThreadProps {
  layout?: "desktop" | "mobile";
  onBack: () => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  onRefreshTasks: () => void;
  source: SubagentTaskSource;
  task: SubagentTask;
}

export function SubagentTaskThread({
  layout = "desktop",
  onBack,
  onOpenWorkspaceFile,
  onRefreshTasks,
  source,
  task,
}: SubagentTaskThreadProps) {
  const thread = useSubagentTaskThread({ onRefreshTasks, source, task });

  return (
    <SubagentTaskThreadView
      layout={layout}
      model={{
        ...thread,
        onRetry: () => void thread.refresh(),
        onSend: () => void thread.sendMessage(),
        onStop: () => void thread.stop(),
      }}
      onBack={onBack}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
    />
  );
}
