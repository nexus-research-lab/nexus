// INPUT: 当前群聊成员、设置、Agent 目录与异步保存命令。
// OUTPUT: 创建弹窗的管理模式装配；保存成功后关闭，失败和提交锁由弹窗持有。
// POS: Room 成员管理入口；不直接读取目录或写入持久化。
import type { Agent } from "@/types/agent/agent";

import {
  CreateRoomDialog,
  type RoomDialogSubmission,
} from "./create-room-dialog";

interface RoomMemberManagerDialogProps {
  availableRoomAgents: Agent[];
  initialAvatar?: string | null;
  initialHostAgentId?: string | null;
  initialHostAutoReplyEnabled: boolean;
  initialName: string;
  initialPrivateMessagesEnabled: boolean;
  initialRoomSkillNames: string[];
  isOpen: boolean;
  onClose: () => void;
  onManageRoom: (submission: RoomDialogSubmission) => Promise<void>;
  roomMembers: Agent[];
}

export function RoomMemberManagerDialog({
  availableRoomAgents,
  initialAvatar,
  initialHostAgentId,
  initialHostAutoReplyEnabled,
  initialName,
  initialPrivateMessagesEnabled,
  initialRoomSkillNames,
  isOpen,
  onClose,
  onManageRoom,
  roomMembers,
}: RoomMemberManagerDialogProps) {
  return (
    <CreateRoomDialog
      agents={buildRoomAgentCatalog(roomMembers, availableRoomAgents)}
      initialAvatar={initialAvatar ?? ""}
      initialHostAgentId={initialHostAgentId ?? null}
      initialHostAutoReplyEnabled={initialHostAutoReplyEnabled}
      initialName={initialName}
      initialPausedAgentIds={roomMembers
        .filter((member) => member.room_participation_paused)
        .map((member) => member.agent_id)}
      initialPrivateMessagesEnabled={initialPrivateMessagesEnabled}
      initialRoomSkillNames={initialRoomSkillNames}
      initialSelectedAgentIds={roomMembers.map((member) => member.agent_id)}
      isOpen={isOpen}
      mode="manage"
      onCancel={onClose}
      onConfirm={async (submission) => {
        await onManageRoom(submission);
        onClose();
      }}
    />
  );
}

function buildRoomAgentCatalog(
  members: Agent[],
  availableAgents: Agent[],
): Agent[] {
  const memberAgentIds = new Set(members.map((member) => member.agent_id));
  return [
    ...members,
    ...availableAgents.filter((agent) => !memberAgentIds.has(agent.agent_id)),
  ];
}
