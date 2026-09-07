/**
 * INPUT: Room 成员、按 Agent 隔离的进程集合与会话 scope。
 * OUTPUT: 默认跟随最近进程、保留有效手动选择的任务面板；缺项提交有效回退，会话切换关闭旧详情。
 * POS: Room 多 Agent 任务投影到共享任务面板与成员切换器之间的视图适配层。
 */
"use client";

import { useMemo } from "react";

import { getAgentDisplayName } from "@/lib/agent-display-name";
import { buildAgentSelectionOptions } from "@/lib/agent-selection-options";
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

  // 手动选择失效后提交当前有效回退，避免旧成员/进程恢复时又夺回选择权。
  if (selectedAgentId !== null && selectedAgentId !== (selection?.process.agentId ?? null)) {
    setSelectedAgentId(selection?.process.agentId ?? null);
  }

  if (!selection) {
    return null;
  }

  const source: WorkspaceTaskSource = {
    agentId: selection.process.agentId,
    avatar: selection.member.avatar ?? null,
    label: buildAgentSelectionOptions([selection.member], t, roomMembers)[0].label,
    name: getAgentDisplayName(selection.member.name, t),
  };
  const sourceControl = selection.members.length > 1 ? (
    <RoomAgentSwitcher
      ariaLabel={t("tasks.switch_agent")}
      directory={roomMembers}
      members={selection.members}
      onSelect={setSelectedAgentId}
      selectedId={selection.process.agentId}
      variant="task"
    />
  ) : undefined;

  return (
    <WorkspaceTaskPanel
      scopeKey={scopeKey}
      source={source}
      sourceControl={sourceControl}
      todos={selection.process.todos}
    />
  );
}
