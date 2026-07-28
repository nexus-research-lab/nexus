"use client";

import type { SubagentTask, SubagentTaskSource } from "@/types/conversation/subagent-task";

import { SubagentTaskThreadView } from "./subagent-task-thread-view";
import { useSubagentTaskThread } from "./use-subagent-task-thread";

interface SubagentTaskThreadProps {
  layout?: "desktop" | "mobile";
  onBack: () => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  source: SubagentTaskSource;
  task: SubagentTask;
}

export function SubagentTaskThread({
  layout = "desktop",
  onBack,
  onOpenWorkspaceFile,
  source,
  task,
}: SubagentTaskThreadProps) {
  const thread = useSubagentTaskThread({ source, task });

  return (
    <SubagentTaskThreadView
      layout={layout}
      model={{
        ...thread,
        onRetry: () => void thread.refresh(),
      }}
      onBack={onBack}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
    />
  );
}
