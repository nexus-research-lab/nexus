import type { Dispatch, RefObject, SetStateAction } from "react";

import type {
  AgentRoundStatusEventPayload,
  ChatAckData,
  RoundLifecycleStatus,
  StreamMessage,
} from "@/types/conversation/message/event";
import type {
  AssistantMessage,
  AssistantMessageStatus,
  Message,
} from "@/types/conversation/message/entity";
import type {
  CommandCatalogData,
  ContextUsageData,
  EventMessage,
  SessionStatusData,
} from "@/types/generated/protocol";
import type {
  AgentConversationRuntimeStatus,
  InputQueueItem,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import type { WorkspaceEventPayload } from "@/types/app/workspace-live";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import type {
  WebSocketMessage,
  WebSocketSendResult,
  WebSocketState,
} from "@/types/system/websocket";
import type { ConversationReliabilityController } from "../reliability/use-conversation-reliability";

type ConversationSocketSend = (
  payload: WebSocketMessage,
) => WebSocketSendResult;

interface AgentEventScope {
  agentId: string | null;
  chatType: "dm" | "group";
  conversationId: string | null;
  roomId: string | null;
  sessionKey: string | null;
  isCurrentRoomEvent: (roomId?: string | null) => boolean;
  isCurrentSessionEvent: (sessionKey?: string | null) => boolean;
}

interface AgentEventTransport {
  roomSeqCursorRef: RefObject<number>;
  sessionSeqCursorRef: RefObject<number>;
  wsSendRef: RefObject<ConversationSocketSend>;
  wsStateRef: RefObject<WebSocketState>;
  reloadCurrentSession: () => Promise<Message[] | null>;
}

interface AgentEventState {
  reliability: ConversationReliabilityController;
  setCommandCatalog: Dispatch<SetStateAction<CommandCatalogData>>;
  setContextUsageByAgent: Dispatch<
    SetStateAction<Record<string, ContextUsageData>>
  >;
  setInputQueueItems: Dispatch<SetStateAction<InputQueueItem[]>>;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  setPendingPermissions: Dispatch<SetStateAction<PendingPermission[]>>;
}

interface AgentEventRuntime {
  acknowledgePermissionRequest: (requestId: string) => void;
  applyAgentRoundStatus: (payload: AgentRoundStatusEventPayload) => void;
  applyRoundStatus: (
    roundId: string,
    status: RoundLifecycleStatus,
  ) => void;
  rejectPendingRequestAck: (
    clientRequestId: string,
    reason: string,
  ) => boolean;
  resolvePendingRequestAck: (clientRequestId?: string | null) => boolean;
  removeRewrittenRound: (roundId: string) => void;
  syncSessionStatus: (payload: SessionStatusData) => void;
  setRuntimeStatus: (status: AgentConversationRuntimeStatus) => void;
  trackAssistantMessage: (message: AssistantMessage) => void;
  trackChatAck: (ack: ChatAckData) => void;
  trackStreamExecution: (stream: StreamMessage) => void;
  updateMessageStatus: (
    messageId: string,
    status: AssistantMessageStatus,
    roundId?: string | null,
  ) => void;
}

interface AgentEventCallbacks {
  applyWorkspaceEvent: (payload: WorkspaceEventPayload) => void;
  enqueueStreamPayload: (payload: StreamMessage) => void;
  flushStreamPayloads: () => void;
  settleLiveMessageSnapshot: (message: Message) => void;
  onBackgroundMessage: (sessionKey: string, message: Message) => void;
  onRoomEvent: (eventType: string, data: RoomEventPayload) => void;
  settleAgentWorkspaceWrites: (agentId: string) => void;
}

export interface AgentEventContext {
  callbacks: AgentEventCallbacks;
  runtime: AgentEventRuntime;
  scope: AgentEventScope;
  state: AgentEventState;
  transport: AgentEventTransport;
}

export type AgentEventHandler = (
  event: EventMessage,
  context: AgentEventContext,
) => void;

export type AgentEventHandlerMap = Record<string, AgentEventHandler>;
