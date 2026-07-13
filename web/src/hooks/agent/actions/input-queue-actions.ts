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
): void {
  if (!content.trim() && attachments.length === 0) {
    return;
  }
  sendInputQueueCommand(context, {
    action: "enqueue",
    content,
    delivery_policy: deliveryPolicy,
    ...(attachments.length > 0 ? { attachments } : {}),
  });
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
