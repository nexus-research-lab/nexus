/**
 * INPUT: 当前 Thread 精确执行轮目标与 Room 实时源。
 * OUTPUT: 仅含目标 agent_round 的 Thread 面板消息、权限和运行态。
 * POS: Room Thread 实时切片到面板 Props 的纯投影。
 */
import { getRoomAgentRoundEntry, isAgentRoundActive } from "../../round/round-agent-model";
import { getRoomThreadMessages } from "../../round/round-thread-model";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import type { ThreadTarget } from "../group-thread-state";
import type { RoomThreadLiveSource } from "./room-thread-live-store";

export interface RoomThreadPanelModel {
  agentAvatar: string | null;
  agentName: string;
  isLoading: boolean;
  messages: ReturnType<typeof getRoomThreadMessages>;
  onOpenWorkspaceFile?: RoomThreadLiveSource["onOpenWorkspaceFile"];
  onPermissionResponse: RoomThreadLiveSource["onPermissionResponse"];
  onStopMessage: RoomThreadLiveSource["onStopMessage"];
  pendingPermissions: ReturnType<typeof getThreadPendingPermissions>;
  userAvatar: string | null;
}

export function buildRoomThreadPanelModel(
  source: RoomThreadLiveSource | null,
  target: ThreadTarget | null,
): RoomThreadPanelModel | null {
  if (!source || !target) {
    return null;
  }
  const roundMessages = source.messageGroups.get(target.roundId) ?? [];
  const entry = getRoomAgentRoundEntry(
    roundMessages,
    target.agentId,
    source.pendingSlotGroups.get(target.roundId) ?? [],
    target.agentRoundId,
  );

  return {
    agentAvatar: source.agentAvatarMap[target.agentId] ?? null,
    agentName: source.agentNameMap[target.agentId] ?? target.agentId,
    isLoading: Boolean(entry && isAgentRoundActive(entry.status)),
    messages: getRoomThreadMessages(
      roundMessages,
      target.agentId,
      target.agentRoundId,
    ),
    onOpenWorkspaceFile: source.onOpenWorkspaceFile,
    onPermissionResponse: source.onPermissionResponse,
    onStopMessage: source.onStopMessage,
    pendingPermissions: getThreadPendingPermissions(
      target.roundId,
      target.agentId,
      target.agentRoundId,
      source.pendingPermissionGroups.get(target.roundId) ?? [],
    ),
    userAvatar: source.currentUserAvatar,
  };
}

function getThreadPendingPermissions(
  roundId: string,
  agentId: string,
  agentRoundId: string | null,
  pendingPermissions: PendingPermission[],
): PendingPermission[] {
  return pendingPermissions.filter(
    (permission) =>
      permission.agent_id === agentId
      && permission.round_id === roundId
      && (
        !agentRoundId
        || !permission.agent_round_id
        || permission.agent_round_id === agentRoundId
      ),
  );
}
