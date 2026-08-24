import { getAgentWsUrl } from "@/config/runtime-endpoints";
import type {
  AgentConversationChatType,
  AgentConversationIdentity,
  InputQueueItem,
  RoomAgentExecutionState,
  RoomPendingAgentSlotState,
  UseAgentConversationOptions,
  UseAgentConversationReturn,
} from "@/types/agent/agent-conversation";
import type { Message } from "@/types/conversation/message/entity";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import type {
  CommandCatalogData,
  ContextUsageData,
} from "@/types/generated/protocol";
import type { WebSocketState } from "@/types/system/websocket";
import type { ConversationReliabilitySnapshot } from "@/types/agent/agent-conversation-reliability";

import type { AgentConversationRuntimeSnapshot } from "./runtime/model/conversation-runtime-state";

const EMPTY_AGENT_CONVERSATION_IDENTITY: AgentConversationIdentity = {
  chat_type: "dm",
  session_key: null,
};

export interface AgentConversationConfig {
  agentId: string | null;
  chatType: AgentConversationChatType;
  conversationId: string | null;
  identity: AgentConversationIdentity | null;
  identitySessionKey: string | null;
  onError: UseAgentConversationOptions["on_error"];
  onRoomEvent: UseAgentConversationOptions["on_room_event"];
  roomId: string | null;
  wsUrl: string;
}

export function resolveAgentConversationConfig(
  options: UseAgentConversationOptions,
): AgentConversationConfig {
  const identity = options.identity ?? null;
  const identitySource = identity ?? EMPTY_AGENT_CONVERSATION_IDENTITY;
  return {
    agentId: identitySource.agent_id ?? null,
    chatType: identitySource.chat_type,
    conversationId: identitySource.conversation_id ?? null,
    identity,
    identitySessionKey: identitySource.session_key?.trim() || null,
    onError: options.on_error,
    onRoomEvent: options.on_room_event,
    roomId: identitySource.room_id ?? null,
    wsUrl: options.ws_url || getAgentWsUrl(),
  };
}

interface AgentConversationPublicActions {
  deleteQueueMessage: UseAgentConversationReturn["delete_input_queue_message"];
  enqueueQueueMessage: UseAgentConversationReturn["enqueue_input_queue_message"];
  guideQueueMessage: UseAgentConversationReturn["guide_input_queue_message"];
  reorderQueueMessages: UseAgentConversationReturn["reorder_input_queue_messages"];
  rewriteLastMessage: UseAgentConversationReturn["rewrite_last_user_message"];
  sendMessage: UseAgentConversationReturn["send_message"];
  sendPermissionResponse: UseAgentConversationReturn["send_permission_response"];
  setGoal: UseAgentConversationReturn["set_goal"];
  stopGeneration: UseAgentConversationReturn["stop_generation"];
}

interface AgentConversationPublicRuntime {
  pendingAgentSlots: RoomPendingAgentSlotState[];
  pendingPermissions: PendingPermission[];
  roomAgentExecutionStates: RoomAgentExecutionState[];
  snapshot: AgentConversationRuntimeSnapshot;
  stoppingAgentRoundIds: string[];
}

interface AgentConversationPublicSession {
  bindSessionKey: UseAgentConversationReturn["bind_session_key"];
  clearSession: UseAgentConversationReturn["clear_session"];
  hasMoreHistory: boolean;
  historyPrependToken: number;
  inputQueueItems: InputQueueItem[];
  isHistoryLoading: boolean;
  isSessionLoading: boolean;
  loadOlderMessages: UseAgentConversationReturn["load_older_messages"];
  loadRoundWindow: UseAgentConversationReturn["load_round_window"];
  loadSession: UseAgentConversationReturn["load_session"];
  resolvedHistoryRoundIds: string[];
  resetSession: UseAgentConversationReturn["reset_session"];
  sessionKey: string | null;
  startSession: UseAgentConversationReturn["start_session"];
}

interface BuildAgentConversationResultOptions {
  actions: AgentConversationPublicActions;
  commandCatalog: CommandCatalogData;
  contextUsage: ContextUsageData | null;
  contextUsageByAgent: Readonly<Record<string, ContextUsageData>>;
  messages: Message[];
  reliability: ConversationReliabilitySnapshot;
  runtime: AgentConversationPublicRuntime;
  session: AgentConversationPublicSession;
  wsState: WebSocketState;
}

export function buildAgentConversationResult({
  actions,
  commandCatalog,
  contextUsage,
  contextUsageByAgent,
  messages,
  reliability,
  runtime,
  session,
  wsState,
}: BuildAgentConversationResultOptions): UseAgentConversationReturn {
  return {
    bind_session_key: session.bindSessionKey,
    clear_session: session.clearSession,
    command_catalog: commandCatalog,
    context_usage: contextUsage,
    context_usage_by_agent: contextUsageByAgent,
    delete_input_queue_message: actions.deleteQueueMessage,
    enqueue_input_queue_message: actions.enqueueQueueMessage,
    guide_input_queue_message: actions.guideQueueMessage,
    has_more_history: session.hasMoreHistory,
    history_prepend_token: session.historyPrependToken,
    input_queue_items: session.inputQueueItems,
    is_history_loading: session.isHistoryLoading,
    is_loading: runtime.snapshot.isLoading,
    is_session_loading: session.isSessionLoading,
    live_round_ids: runtime.snapshot.liveRoundIds,
    load_older_messages: session.loadOlderMessages,
    load_round_window: session.loadRoundWindow,
    load_session: session.loadSession,
    messages,
    pending_agent_slots: runtime.pendingAgentSlots,
    pending_permissions: runtime.pendingPermissions,
    reliability,
    room_agent_execution_states: runtime.roomAgentExecutionStates,
    resolved_history_round_ids: session.resolvedHistoryRoundIds,
    reorder_input_queue_messages: actions.reorderQueueMessages,
    reset_session: session.resetSession,
    rewrite_last_user_message: actions.rewriteLastMessage,
    runtime_phase: runtime.snapshot.phase,
    send_message: actions.sendMessage,
    send_permission_response: actions.sendPermissionResponse,
    set_goal: actions.setGoal,
    session_key: session.sessionKey,
    start_session: session.startSession,
    stop_generation: actions.stopGeneration,
    stopping_agent_round_ids: runtime.stoppingAgentRoundIds,
    ws_state: wsState,
  };
}
