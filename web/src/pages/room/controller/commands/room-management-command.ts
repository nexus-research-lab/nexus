import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import {
  addRoomMember,
  removeRoomMember,
  updateRoom,
} from "@/lib/api/room-api";
import type { UpdateRoomParams } from "@/types/conversation/room";

import { buildRoomMembershipPlan } from "./room-management-command-model";

export async function saveRoomManagement(
  roomId: string,
  currentAgentIds: readonly string[],
  submission: RoomDialogSubmission,
): Promise<void> {
  const plan = buildRoomMembershipPlan(currentAgentIds, submission.agentIds);

  // 先补齐新群主等设置依赖的成员，再更新房间，最后移除旧成员。
  await applyMemberCommands(roomId, plan.addAgentIds, addRoomMember);
  await updateRoom(roomId, buildRoomUpdateParams(submission));
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
