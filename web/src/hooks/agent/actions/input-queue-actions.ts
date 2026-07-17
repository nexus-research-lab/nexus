import type {
  AgentConversationDeliveryPolicy,
  AgentConversationSendOptions,
} from "@/types/agent/agent-conversation";
import type { WebSocketMessage } from "@/types/system/websocket";

import {
  requireConversationActionContext,
  sendConversationCommand,
  type AgentConversationActionContext,
} from "./conversation-action-context";
import { buildConversationScope } from "./conversation-command-builders";
import {
  createOutboundClientMessageId,
  createOutboundRequestDescriptor,
  type OutboundRequestDescriptor,
} from "./outbound-request";

export type OutboundInputQueueRequest = OutboundRequestDescriptor;

export function createInputQueueDraftFingerprint(
  content: string,
  deliveryPolicy: AgentConversationDeliveryPolicy,
  attachments: AgentConversationSendOptions["attachments"] = [],
  targetAgentIDs: string[] = [],
): string {
  return JSON.stringify({
    attachments: attachments.map((attachment) => ({
      conversation_id: attachment.conversation_id ?? null,
      file_name: attachment.file_name,
      kind: attachment.kind,
      mime_type: attachment.mime_type ?? null,
      room_id: attachment.room_id ?? null,
      scope: attachment.scope ?? null,
      size: attachment.size ?? null,
      workspace_agent_id: attachment.workspace_agent_id ?? null,
      workspace_path: attachment.workspace_path,
    })),
    content,
    delivery_policy: deliveryPolicy,
    target_agent_ids: targetAgentIDs,
  });
}

export function resolveInputQueueClientMessageId(
  requestIDs: Map<string, string>,
  fingerprint: string,
): string {
  const existingID = requestIDs.get(fingerprint);
  if (existingID) {
    return existingID;
  }
  const clientMessageID = createOutboundClientMessageId();
  requestIDs.set(fingerprint, clientMessageID);
  return clientMessageID;
}

function sendInputQueueCommand(
  context: AgentConversationActionContext,
  command: Record<string, unknown>,
): void {
  const actionContext = requireConversationActionContext(context);
  sendConversationCommand(context, {
    type: "input_queue",
    ...buildConversationScope(actionContext),
    ...command,
  } as WebSocketMessage, "队列请求未发送到后端，请检查连接后重试");
}

export function enqueueInputQueueMessage(
  content: string,
  context: AgentConversationActionContext,
  deliveryPolicy: AgentConversationDeliveryPolicy = "queue",
  attachments: AgentConversationSendOptions["attachments"] = [],
  targetAgentIDs: string[] = [],
  clientMessageId?: string,
): OutboundInputQueueRequest | null {
  if (!content.trim() && attachments.length === 0) {
    return null;
  }
  const request = createOutboundRequestDescriptor(clientMessageId);
  sendInputQueueCommand(context, {
    action: "enqueue",
    client_message_id: request.client_message_id,
    client_request_id: request.client_request_id,
    content,
    delivery_policy: deliveryPolicy,
    ...(attachments.length > 0 ? { attachments } : {}),
    ...(targetAgentIDs.length > 0 ? { target_agent_ids: targetAgentIDs } : {}),
  });
  return request;
}

export function deleteInputQueueMessage(
  itemId: string,
  context: AgentConversationActionContext,
): void {
  if (itemId.trim()) {
    sendInputQueueCommand(context, { action: "delete", item_id: itemId });
  }
}

export function guideInputQueueMessage(
  itemId: string,
  context: AgentConversationActionContext,
): void {
  if (itemId.trim()) {
    sendInputQueueCommand(context, { action: "guide", item_id: itemId });
  }
}

export function reorderInputQueueMessages(
  orderedIds: string[],
  context: AgentConversationActionContext,
): void {
  sendInputQueueCommand(context, {
    action: "reorder",
    ordered_ids: orderedIds,
  });
}
