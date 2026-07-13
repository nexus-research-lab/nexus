import { generateUuid } from "@/lib/uuid";
import type { Message } from "@/types/conversation/message/entity";
import type {
  AgentConversationSendOptions,
} from "@/types/agent/agent-conversation";
import type { WebSocketMessage } from "@/types/system/websocket";

import { upsertMessage } from "../message/message-collection-model";
import {
  failConversationAction,
  requireConversationActionContext,
  sendConversationCommand,
  type AgentConversationActionContext,
  type ResolvedConversationActionContext,
} from "./conversation-action-context";
import { buildConversationScope } from "./conversation-command-builders";

export interface OutboundChatRequest {
  client_message_id: string;
  client_request_id: string;
}

function createOutboundChatRequest(): OutboundChatRequest {
  return {
    client_message_id: `local_msg_${generateUuid()}`,
    client_request_id: `req_${generateUuid()}`,
  };
}

function buildOptimisticUserMessage(
  content: string,
  actionContext: ResolvedConversationActionContext,
  request: OutboundChatRequest,
  options: AgentConversationSendOptions,
): Message {
  const attachments = options.attachments ?? [];
  return {
    message_id: request.client_message_id,
    session_key: actionContext.sessionKey,
    round_id: request.client_message_id,
    agent_id: actionContext.agentId,
    role: "user",
    content,
    timestamp: Date.now(),
    delivery_policy: options.delivery_policy ?? "queue",
    ...(attachments.length > 0 ? { attachments } : {}),
    ...(actionContext.chatType === "group"
      ? {
          room_id: actionContext.roomId ?? undefined,
          conversation_id: actionContext.conversationId ?? undefined,
        }
      : {}),
  };
}

function buildChatCommand(
  content: string,
  actionContext: ResolvedConversationActionContext,
  request: OutboundChatRequest,
  options: AgentConversationSendOptions,
): WebSocketMessage {
  const attachments = options.attachments ?? [];
  return {
    type: "chat",
    content,
    ...buildConversationScope(actionContext),
    client_request_id: request.client_request_id,
    client_message_id: request.client_message_id,
    delivery_policy: options.delivery_policy ?? "queue",
    ...(attachments.length > 0 ? { attachments } : {}),
  } as WebSocketMessage;
}

/** 后端 mint round_id；前端请求 ID 只服务乐观消息与 ACK 关联。 */
export async function sendSessionMessage(
  content: string,
  context: AgentConversationActionContext,
  options: AgentConversationSendOptions = {},
): Promise<OutboundChatRequest | null> {
  if (!content.trim() && !options.attachments?.length) {
    return null;
  }
  const actionContext = requireConversationActionContext(context);
  const request = createOutboundChatRequest();
  const optimisticMessage = buildOptimisticUserMessage(
    content,
    actionContext,
    request,
    options,
  );
  sendConversationCommand(
    context,
    buildChatCommand(content, actionContext, request, options),
    "消息未发送到后端，请检查连接后重试",
  );
  context.setMessages((messages) => upsertMessage(messages, optimisticMessage));
  return request;
}

export async function rewriteLastUserMessage(
  targetRoundId: string,
  content: string,
  context: AgentConversationActionContext,
): Promise<OutboundChatRequest | null> {
  if (!content.trim()) {
    return null;
  }
  if (!targetRoundId.trim()) {
    failConversationAction(context, "找不到要编辑的消息，请刷新后重试");
  }

  const actionContext = requireConversationActionContext(context);
  if (actionContext.chatType === "group") {
    failConversationAction(context, "Room 会话暂不支持编辑重跑");
  }
  const request = createOutboundChatRequest();
  sendConversationCommand(context, {
    type: "chat_rewrite_last",
    content,
    ...buildConversationScope(actionContext),
    target_round_id: targetRoundId,
    client_request_id: request.client_request_id,
    client_message_id: request.client_message_id,
  } as WebSocketMessage, "消息未发送到后端，请检查连接后重试");
  return request;
}
