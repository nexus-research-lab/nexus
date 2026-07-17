import type { RefObject } from "react";

import type {
  AgentConversationRuntimePhase,
} from "@/types/agent/agent-conversation";
import type { Message } from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";
import type { SessionRoundIndexItem } from "@/types/conversation/history";
import type { ConversationRoundScrollHandleRef } from "@/features/conversation/shared/timeline/scroll/round-scroll";

interface GroupConversationFeedRefs {
  bottomAnchorRef: RefObject<HTMLDivElement | null>;
  feedRef?: RefObject<HTMLDivElement | null>;
  roundScrollRef?: ConversationRoundScrollHandleRef;
  scrollRef?: RefObject<HTMLDivElement | null>;
}

export interface GroupConversationRoundSource {
  liveRoundIds: string[];
  messageGroups: Map<string, Message[]>;
  pendingPermissionGroups: Map<string, PendingPermission[]>;
  pendingSlotGroups: Map<string, RoomPendingAgentSlotState[]>;
  rootRoundIds?: Map<string, string>;
  roundIds: string[];
  roundIndexItems?: SessionRoundIndexItem[];
}

export interface GroupConversationRoundRenderer {
  agentAvatarMap: Record<string, string | null>;
  agentNameMap: Record<string, string>;
  compact?: boolean;
  currentAgentAvatar: string | null;
  currentAgentName: string | null;
  currentUserAvatar: string | null;
  isLastRoundPendingPermissions: PendingPermission[];
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
  onStopMessage: (msgId: string) => void;
  runtimePhase: AgentConversationRuntimePhase | null;
}

export interface GroupConversationFeedProps {
  isMobileLayout: boolean;
  refs: GroupConversationFeedRefs;
  renderer: GroupConversationRoundRenderer;
  source: GroupConversationRoundSource;
}

export interface GroupConversationRoundState {
  index: number;
  isLast: boolean;
  isLive: boolean;
  isLoaded: boolean;
  messages: Message[];
  pendingPermissions: PendingPermission[];
  pendingSlots: RoomPendingAgentSlotState[];
  roundId: string;
  rootRoundId: string;
}

export function resolveGroupConversationRound(
  source: GroupConversationRoundSource,
  index: number,
): GroupConversationRoundState {
  const roundId = source.roundIds[index];
  const rootRoundId = source.rootRoundIds?.get(roundId) ?? roundId;
  const messages = source.messageGroups.get(roundId) ?? [];
  const pendingPermissions =
    source.pendingPermissionGroups.get(roundId) ?? [];
  const pendingSlots = source.pendingSlotGroups.get(roundId) ?? [];
  const isLast = index === source.roundIds.length - 1;
  const isLive = isLast && source.liveRoundIds.includes(rootRoundId);

  return {
    index,
    isLast,
    isLive,
    isLoaded:
      messages.length > 0 ||
      pendingPermissions.length > 0 ||
      pendingSlots.length > 0 ||
      isLive,
    messages,
    pendingPermissions,
    pendingSlots,
    roundId,
    rootRoundId,
  };
}

/** canonical root 导航到它保留的 root node；无 root 正文时落到首个 Agent node。 */
export function buildGroupConversationRoundAliases(
  source: GroupConversationRoundSource,
): Map<string, string> {
  const aliases = new Map<string, string>();
  for (const nodeId of source.roundIds) {
    const rootRoundId = source.rootRoundIds?.get(nodeId) ?? nodeId;
    if (!aliases.has(rootRoundId) || nodeId === rootRoundId) {
      aliases.set(rootRoundId, nodeId);
    }
  }
  return aliases;
}

function resolveRoundAgentId(messages: Message[]): string | null {
  const assistantMessage = messages.find(
    (message) => message.role === "assistant",
  );
  if (
    !assistantMessage ||
    !("agent_id" in assistantMessage) ||
    !assistantMessage.agent_id
  ) {
    return null;
  }
  return assistantMessage.agent_id;
}

export function resolveRoundAgent(
  messages: Message[],
  renderer: GroupConversationRoundRenderer,
): {
  avatar: string | null | undefined;
  id: string | null;
  name: string | null | undefined;
} {
  const id = resolveRoundAgentId(messages);
  return {
    avatar: id
      ? renderer.agentAvatarMap[id] ?? renderer.currentAgentAvatar
      : renderer.currentAgentAvatar,
    id,
    name: id
      ? renderer.agentNameMap[id] ?? renderer.currentAgentName
      : renderer.currentAgentName,
  };
}
