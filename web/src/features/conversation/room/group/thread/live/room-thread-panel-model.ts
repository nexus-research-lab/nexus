/**
 * INPUT: 当前 Thread 精确执行轮目标、Room 实时源与当前语言。
 * OUTPUT: 仅含目标 agent_round 的消息、权限、运行态和可读名称。
 * POS: Room Thread 实时切片到面板 Props 的纯投影。
 */
import { getRoomAgentRoundEntry, isAgentRoundActive } from "../../round/round-agent-model";
import { getRoomThreadMessages } from "../../round/round-thread-model";
import { getAgentDisplayName } from "@/lib/agent-display-name";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import type { ThreadTarget } from "../group-thread-state";
import type { RoomThreadLiveSource } from "./room-thread-live-store";
import type { UnresolvedToolStatus } from "@/features/conversation/shared/message/item/view/content/content-renderer-contract";

export interface RoomThreadPanelModel {
  agentAvatar: string | null;
  agentName: string;
  isLoading: boolean;
  messages: ReturnType<typeof getRoomThreadMessages>;
  onOpenWorkspaceFile?: RoomThreadLiveSource["onOpenWorkspaceFile"];
  onPermissionResponse: RoomThreadLiveSource["onPermissionResponse"];
  pendingPermissions: ReturnType<typeof getThreadPendingPermissions>;
  unresolvedToolStatus?: UnresolvedToolStatus;
}

export function buildRoomThreadPanelModel(
  source: RoomThreadLiveSource | null,
  target: ThreadTarget | null,
  t: I18nContextValue["t"],
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
    source.roomAgentExecutionStateGroups.get(target.roundId) ?? [],
  );

  return {
    agentAvatar: source.agentAvatarMap[target.agentId] ?? null,
    agentName: getAgentDisplayName(source.agentNameMap[target.agentId], t),
    isLoading: Boolean(entry && isAgentRoundActive(entry.status)),
    messages: getRoomThreadMessages(
      roundMessages,
      target.agentId,
      target.agentRoundId,
    ),
    onOpenWorkspaceFile: source.onOpenWorkspaceFile,
    onPermissionResponse: source.onPermissionResponse,
    pendingPermissions: getThreadPendingPermissions(
      target.roundId,
      target.agentId,
      target.agentRoundId,
      source.pendingPermissionGroups.get(target.roundId) ?? [],
    ),
    unresolvedToolStatus: entry?.status === "cancelled"
      ? "stopped"
      : entry?.status === "error" ? "error" : undefined,
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
