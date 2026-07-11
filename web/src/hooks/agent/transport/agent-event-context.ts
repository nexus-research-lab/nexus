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
  EventMessage,
  SessionStatusData,
} from "@/types/generated/protocol";
import type {
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

type ConversationSocketSend = (
  payload: WebSocketMessage,
) => WebSocketSendResult;

interface AgentEventScope {
  agentId: string | null;
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
  reloadCurrentSession: () => Promise<void>;
}

interface AgentEventState {
  setError: Dispatch<SetStateAction<string | null>>;
  setInputQueueItems: Dispatch<SetStateAction<InputQueueItem[]>>;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  setPendingPermissions: Dispatch<SetStateAction<PendingPermission[]>>;
}

interface AgentEventRuntime {
  applyAgentRoundStatus: (payload: AgentRoundStatusEventPayload) => void;
  applyRoundStatus: (
    roundId: string,
    status: RoundLifecycleStatus,
  ) => void;
  rejectChatAck: (clientRequestId: string, reason: string) => boolean;
  removeRewrittenRound: (roundId: string) => void;
  syncSessionStatus: (payload: SessionStatusData) => void;
  trackAssistantMessage: (message: AssistantMessage) => void;
  trackChatAck: (ack: ChatAckData) => void;
  updateMessageStatus: (
    messageId: string,
    status: AssistantMessageStatus,
    roundId?: string | null,
  ) => void;
}

interface AgentEventCallbacks {
  applyWorkspaceEvent: (payload: WorkspaceEventPayload) => void;
  enqueueStreamPayload: (payload: StreamMessage) => void;
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
