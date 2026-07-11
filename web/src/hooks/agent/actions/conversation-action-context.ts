import type { Dispatch, RefObject, SetStateAction } from "react";

import { resolveAgentId } from "@/config/options";
import { isStructuredSessionKey } from "@/lib/conversation/session-key";
import type { Message } from "@/types/conversation/message/entity";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import type {
  WebSocketMessage,
  WebSocketSendResult,
  WebSocketState,
} from "@/types/system/websocket";

export interface AgentConversationActionContext {
  activeSessionKeyRef: RefObject<string | null>;
  identity: AgentConversationIdentity | null;
  messages: Message[];
  pendingPermissions: PendingPermission[];
  sessionKey: string | null;
  setError: Dispatch<SetStateAction<string | null>>;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  setPendingPermissions: Dispatch<SetStateAction<PendingPermission[]>>;
  wsSend: (message: WebSocketMessage) => WebSocketSendResult;
  wsState: WebSocketState;
}

export interface ResolvedConversationActionContext {
  agentId: string;
  chatType: "dm" | "group";
  conversationId: string | null;
  roomId: string | null;
  sessionKey: string;
}

type ConversationContextFailure =
  | "missing_session"
  | "invalid_session"
  | "disconnected";

export type ConversationContextResult =
  | { ok: true; value: ResolvedConversationActionContext }
  | { ok: false; reason: ConversationContextFailure };

const CONVERSATION_CONTEXT_ERRORS: Record<
  ConversationContextFailure,
  string
> = {
  disconnected: "WebSocket未连接，请稍候重试",
  invalid_session: "当前会话的 session_key 非法，请刷新后重试",
  missing_session: "请先选择或创建会话",
};

export function resolveConversationActionContext(
  context: AgentConversationActionContext,
): ConversationContextResult {
  const sessionKey = context.sessionKey || context.activeSessionKeyRef.current;
  if (!sessionKey) {
    return { ok: false, reason: "missing_session" };
  }
  if (!isStructuredSessionKey(sessionKey)) {
    return { ok: false, reason: "invalid_session" };
  }
  if (context.wsState !== "connected") {
    return { ok: false, reason: "disconnected" };
  }
  return {
    ok: true,
    value: {
      agentId: resolveAgentId(context.identity?.agent_id),
      chatType: context.identity?.chat_type ?? "dm",
      conversationId: context.identity?.conversation_id ?? null,
      roomId: context.identity?.room_id ?? null,
      sessionKey,
    },
  };
}

export function requireConversationActionContext(
  context: AgentConversationActionContext,
): ResolvedConversationActionContext {
  const result = resolveConversationActionContext(context);
  if (result.ok) {
    context.activeSessionKeyRef.current = result.value.sessionKey;
    return result.value;
  }
  const message = CONVERSATION_CONTEXT_ERRORS[result.reason];
  context.setError(message);
  throw new Error(message);
}

export function failConversationAction(
  context: AgentConversationActionContext,
  message: string,
): never {
  context.setError(message);
  throw new Error(message);
}

export function sendConversationCommand(
  context: AgentConversationActionContext,
  command: WebSocketMessage,
  failureMessage: string,
): void {
  if (context.wsSend(command).disposition === "sent") {
    context.setError(null);
    return;
  }
  failConversationAction(context, failureMessage);
}
