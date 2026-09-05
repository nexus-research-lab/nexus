/**
 * INPUT: Room 成员、按 Agent 隔离的进程集合与会话 scope。
 * OUTPUT: 默认跟随最近进程、允许用户稳定切换 Agent 的 Workspace Task 面板。
 * POS: Room 多 Agent 任务投影到共享任务面板与成员切换器之间的视图适配层。
 */
"use client";

import { useMemo } from "react";

import type { ConversationTodoProcess } from "@/features/conversation/shared/todos/todo-projection-model";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  WorkspaceTaskPanel,
  type WorkspaceTaskSource,
} from "@/shared/ui/workspace/surface/workspace-task-strip";
import type { Agent } from "@/types/agent/agent";

import { RoomAgentSwitcher } from "../../../../surface/room-agent-switcher";
import { resolveRoomTaskSelection } from "./room-workspace-task-model";

interface RoomWorkspaceTaskPanelProps {
  processes: ConversationTodoProcess[];
  roomMembers: Agent[];
  scopeKey: string;
}

export function RoomWorkspaceTaskPanel({
  processes,
  roomMembers,
  scopeKey,
}: RoomWorkspaceTaskPanelProps) {
  const { t } = useI18n();
  const [selectedAgentId, setSelectedAgentId] = useResettableState<
    string | null
  >(null, scopeKey);
  const selection = useMemo(
    () => resolveRoomTaskSelection(processes, roomMembers, selectedAgentId),
    [processes, roomMembers, selectedAgentId],
  );

  if (!selection) {
    return null;
  }

  const source: WorkspaceTaskSource = {
    agentId: selection.process.agentId,
    avatar: selection.member.avatar ?? null,
    name: selection.member.name,
  };
  const sourceControl = selection.members.length > 1 ? (
    <RoomAgentSwitcher
      ariaLabel={t("tasks.switch_agent")}
      members={selection.members}
      onSelect={setSelectedAgentId}
      selectedId={selection.process.agentId}
      variant="task"
    />
  ) : undefined;

  return (
    <WorkspaceTaskPanel
      source={source}
      sourceControl={sourceControl}
      todos={selection.process.todos}
    />
  );
}
