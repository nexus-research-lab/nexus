/**
 * INPUT: 会话轮次、消息分组、可选本地前导内容、运行态与 Feed renderer/refs。
 * OUTPUT: canonical 业务轮次、同栈本地前导内容及跨 optimistic ACK 稳定的视图节点身份。
 * POS: DM 静态/虚拟 Feed 共用的纯投影契约。
 */
import type { ReactNode, RefObject } from "react";

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
  isBottomScrollActive?: () => boolean;
  isFollowingLatest?: () => boolean;
  isUserScrollActive?: () => boolean;
  roundScrollRef?: ConversationRoundScrollHandleRef;
  scrollRef?: RefObject<HTMLDivElement | null>;
}

export interface ConversationRoundRenderer {
  compact?: boolean;
  currentAgentAvatar?: string | null;
  currentAgentName: string | null;
  historyDividerLabel?: string;
  onEditLastUserMessage?: (messageId: string, newContent: string) => void;
  onForkRound?: (roundId: string) => Promise<void>;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
  workspaceAgentId?: string | null;
}

export interface ConversationRoundSource {
  liveLayoutActive: boolean;
  liveRoundIds: string[];
  messageGroups: Map<string, Message[]>;
  pendingPermissions: PendingPermission[];
  roundIds: string[];
  roundIndexItems?: SessionRoundIndexItem[];
  runtimePhase?: AgentConversationRuntimePhase | null;
  scopeKey: string | null;
}

export interface ConversationFeedProps {
  isMobileLayout: boolean;
  leadingContent?: ReactNode;
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
  nodeId: string;
  roundId: string;
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
    nodeId: resolveConversationRoundNodeId(messages, roundId),
    roundId,
  };
}

export function isConversationRoundActivelyGrowing(
  source: ConversationRoundSource,
  state: ConversationRoundState,
): boolean {
  if (state.isLive || (source.liveLayoutActive && state.isLast)) {
    return true;
  }
  return state.messages.some((message) => (
    message.role === "assistant"
    && (message.stream_status === "pending"
      || message.stream_status === "streaming")
    && message.is_complete !== true
    && !message.result_summary
  ));
}

/**
 * ACK 会把 optimistic round_id 换成服务端 canonical id，但会保留
 * client_message_id。Feed 节点必须沿用这条客户端身份，否则 React 会把同一轮
 * 当作删除后新增，造成闪白、入场动画重播和虚拟测量抖动。
 */
export function resolveConversationRoundNodeId(
  messages: Message[],
  roundId: string,
): string {
  const clientMessage = messages.find(
    (message) => message.role === "user" && message.client_message_id?.trim(),
  );
  return clientMessage?.role === "user"
    ? clientMessage.client_message_id?.trim() || roundId
    : roundId;
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
