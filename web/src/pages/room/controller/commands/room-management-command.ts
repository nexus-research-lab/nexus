import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import {
  addRoomMember,
  removeRoomMember,
  setRoomMemberParticipation,
  updateRoom,
} from "@/lib/api/conversation/room-command-api";
import type { UpdateRoomParams } from "@/types/conversation/room";

import {
  buildRoomMembershipPlan,
  buildRoomParticipationPlan,
} from "./room-management-command-model";

export async function saveRoomManagement(
  roomId: string,
  currentMembers: readonly {
    agent_id: string;
    room_participation_paused?: boolean;
  }[],
  submission: RoomDialogSubmission,
): Promise<void> {
  const plan = buildRoomMembershipPlan(
    currentMembers.map((member) => member.agent_id),
    submission.agentIds,
  );
  const participationPlan = buildRoomParticipationPlan(
    currentMembers,
    submission.agentIds,
    submission.pausedAgentIds,
  );

  // 先补齐新群主等设置依赖的成员，再更新房间，最后移除旧成员。
  await applyMemberCommands(roomId, plan.addAgentIds, addRoomMember);
  await updateRoom(roomId, buildRoomUpdateParams(submission));
  for (const mutation of participationPlan) {
    await setRoomMemberParticipation(
      roomId,
      mutation.agentId,
      mutation.paused,
    );
  }
  await applyMemberCommands(roomId, plan.removeAgentIds, removeRoomMember);
}

async function applyMemberCommands(
  roomId: string,
  agentIds: readonly string[],
  command: (scopeRoomId: string, agentId: string) => Promise<unknown>,
): Promise<void> {
  for (const agentId of agentIds) {
    await command(roomId, agentId);
  }
}

function buildRoomUpdateParams(
  submission: RoomDialogSubmission,
): UpdateRoomParams {
  return {
    avatar: submission.avatar,
    host_agent_id: submission.hostAgentId,
    host_auto_reply_enabled: submission.hostAutoReplyEnabled,
    name: submission.name,
    private_messages_enabled: submission.privateMessagesEnabled,
    skill_names: submission.skillNames,
  };
}
