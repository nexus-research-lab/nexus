import type { RefObject } from "react";

import type { ConversationRoundScrollHandleRef } from "../timeline/scroll/round-scroll";
import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import type { Message } from "@/types/conversation/message/entity";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";
import type { SessionRoundIndexItem } from "@/types/conversation/history";

interface ConversationFeedRefs {
  bottomAnchorRef: RefObject<HTMLDivElement | null>;
  feedRef?: RefObject<HTMLDivElement | null>;
  roundScrollRef?: ConversationRoundScrollHandleRef;
  scrollRef?: RefObject<HTMLDivElement | null>;
}

export interface ConversationRoundRenderer {
  compact?: boolean;
  currentAgentAvatar?: string | null;
  currentAgentName: string | null;
  currentUserAvatar?: string | null;
  onEditLastUserMessage?: (messageId: string, newContent: string) => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
  workspaceAgentId?: string | null;
}

export interface ConversationRoundSource {
  liveRoundIds: string[];
  messageGroups: Map<string, Message[]>;
  pendingPermissions: PendingPermission[];
  roundIds: string[];
  roundIndexItems?: SessionRoundIndexItem[];
  runtimePhase?: AgentConversationRuntimePhase | null;
}

export interface ConversationFeedProps {
  isMobileLayout: boolean;
  refs: ConversationFeedRefs;
  renderer: ConversationRoundRenderer;
  source: ConversationRoundSource;
}

export interface ConversationRoundState {
  index: number;
  isLast: boolean;
  isLive: boolean;
  isLoaded: boolean;
  messages: Message[];
  roundId: string;
}

export function buildRoundIndexItemMap(
  items: SessionRoundIndexItem[] | undefined,
): Map<string, SessionRoundIndexItem> {
  return new Map(
    (items ?? [])
      .filter((item) => item.roundId.trim() !== "")
      .map((item) => [item.roundId, item]),
  );
}

export function resolveConversationRound(
  source: ConversationRoundSource,
  index: number,
): ConversationRoundState {
  const roundId = source.roundIds[index];
  const messages = source.messageGroups.get(roundId) ?? [];
  const isLast = index === source.roundIds.length - 1;
  const isLive = isLast && source.liveRoundIds.includes(roundId);
  return {
    index,
    isLast,
    isLive,
    isLoaded: messages.length > 0 || isLive,
    messages,
    roundId,
  };
}

export function resolveRoundWorkspaceAgentId(
  messages: Message[],
  fallbackAgentId?: string | null,
): string | null {
  const assistantMessage = messages.find(
    (message) => message.role === "assistant",
  );
  if (
    assistantMessage
    && "agent_id" in assistantMessage
    && assistantMessage.agent_id
  ) {
    return assistantMessage.agent_id;
  }
  return fallbackAgentId ?? null;
}
