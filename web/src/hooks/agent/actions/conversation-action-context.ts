/**
 * INPUT: 当前 Session/transport/action 状态与结构化 reliability controller。
 * OUTPUT: 经身份校验的命令作用域、发送结果和无原始错误泄露的失败分类。
 * POS: Conversation 用户命令进入 WebSocket 前的统一守卫。
 */
import type { Dispatch, RefObject, SetStateAction } from "react";

import { resolveAgentId } from "@/config/runtime-options";
import { isStructuredSessionKey } from "@/lib/conversation/session-key";
import type { Message } from "@/types/conversation/message/entity";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import type {
  WebSocketMessage,
  WebSocketSendResult,
  WebSocketState,
} from "@/types/system/websocket";
import type { ConversationFailureCode } from "@/types/agent/agent-conversation-reliability";
import type { ConversationReliabilityController } from "../reliability/use-conversation-reliability";

export interface AgentConversationActionContext {
  acknowledgePermissionRequest: (requestId: string) => void;
  activeSessionKeyRef: RefObject<string | null>;
  identity: AgentConversationIdentity | null;
  messages: Message[];
  pendingPermissions: PendingPermission[];
  sessionKey: string | null;
  reliability: ConversationReliabilityController;
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

export function conversationContextError(
  reason: ConversationContextFailure,
): string {
  return CONVERSATION_CONTEXT_ERRORS[reason];
}

interface ConversationContextGuard {
  rejects: (candidate: ConversationContextCandidate) => boolean;
  reason: ConversationContextFailure;
}

interface ConversationContextCandidate {
  sessionKey: string;
  wsState: WebSocketState;
}

const CONVERSATION_CONTEXT_GUARDS: readonly ConversationContextGuard[] = [
  {
    reason: "missing_session",
    rejects: ({ sessionKey }) => sessionKey === "",
  },
  {
    reason: "invalid_session",
    rejects: ({ sessionKey }) => !isStructuredSessionKey(sessionKey),
  },
  {
    reason: "disconnected",
    rejects: ({ wsState }) => wsState !== "connected",
  },
];

function buildResolvedConversationActionContext(
  context: AgentConversationActionContext,
  sessionKey: string,
): ResolvedConversationActionContext {
  return {
    agentId: resolveAgentId(context.identity?.agent_id),
    chatType: context.identity?.chat_type ?? "dm",
    conversationId: context.identity?.conversation_id ?? null,
    roomId: context.identity?.room_id ?? null,
    sessionKey,
  };
}

export function resolveConversationActionContext(
  context: AgentConversationActionContext,
): ConversationContextResult {
  const candidate: ConversationContextCandidate = {
    sessionKey: context.sessionKey || context.activeSessionKeyRef.current || "",
    wsState: context.wsState,
  };
  const failedGuard = CONVERSATION_CONTEXT_GUARDS.find(({ rejects }) =>
    rejects(candidate),
  );
  if (failedGuard) {
    return { ok: false, reason: failedGuard.reason };
  }
  return {
    ok: true,
    value: buildResolvedConversationActionContext(
      context,
      candidate.sessionKey,
    ),
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
  const message = conversationContextError(result.reason);
  const sessionKey = context.sessionKey || context.activeSessionKeyRef.current;
  if (sessionKey) {
    context.reliability.reportFailure({
      code: result.reason === "disconnected"
        ? "connection_unavailable"
        : "validation_failed",
      session_key: sessionKey,
    });
  }
  throw new Error(message);
}

export function failConversationAction(
  context: AgentConversationActionContext,
  message: string,
  code: ConversationFailureCode = "validation_failed",
): never {
  const sessionKey = context.sessionKey || context.activeSessionKeyRef.current;
  if (sessionKey) {
    context.reliability.reportFailure({ code, session_key: sessionKey });
  }
  throw new Error(message);
}

export function sendConversationCommand(
  context: AgentConversationActionContext,
  command: WebSocketMessage,
  failureMessage: string,
): void {
  if (context.wsSend(command).disposition === "sent") {
    const sessionKey = context.sessionKey || context.activeSessionKeyRef.current;
    if (sessionKey) {
      context.reliability.observeRecovery({
        kind: "submission_started",
        session_key: sessionKey,
      });
    }
    return;
  }
  failConversationAction(context, failureMessage, "connection_unavailable");
}
