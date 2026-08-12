/**
 * INPUT: Goal Composer objective、Room lead/Loop options 与会话 WebSocket 上下文。
 * OUTPUT: 独立 set_goal 控制消息及可由 durable Room record/ACK 收口的 optimistic user item。
 * POS: Goal UI transport；不复用普通 chat 发送路径。
 */
import type {
  AgentConversationGoalOptions,
  AgentConversationSendOptions,
} from "@/types/agent/agent-conversation";
import type { WebSocketMessage } from "@/types/system/websocket";

import { upsertMessage } from "../message/message-collection-model";
import {
  requireConversationActionContext,
  sendConversationCommand,
  type AgentConversationActionContext,
} from "./conversation-action-context";
import { buildConversationScope } from "./conversation-command-builders";
import { buildOptimisticUserMessage } from "./conversation-chat-actions";
import {
  createOutboundRequestDescriptor,
  type OutboundRequestDescriptor,
} from "./outbound-request";

export async function setSessionGoal(
  objective: string,
  context: AgentConversationActionContext,
  options: AgentConversationGoalOptions = {},
): Promise<OutboundRequestDescriptor | null> {
  const normalizedObjective = objective.trim();
  if (!normalizedObjective) {
    return null;
  }
  const actionContext = requireConversationActionContext(context);
  const request = createOutboundRequestDescriptor();
  const messageOptions: AgentConversationSendOptions = {
    delivery_policy: "auto",
    target_agent_ids: options.target_agent_ids,
  };
  const optimisticMessage = {
    ...buildOptimisticUserMessage(
      `/goal ${normalizedObjective}`,
      actionContext,
      request,
      messageOptions,
    ),
    metadata: { subtype: "goal_set" },
  };
  sendConversationCommand(context, {
    type: "set_goal",
    objective: normalizedObjective,
    ...buildConversationScope(actionContext),
    client_request_id: request.client_request_id,
    client_message_id: request.client_message_id,
    ...(options.target_agent_ids?.length
      ? { target_agent_ids: options.target_agent_ids }
      : {}),
    goal_options: {
      ...(options.metadata ? { metadata: options.metadata } : {}),
      replace_existing: options.replace_existing ?? true,
      token_budget: options.token_budget ?? null,
    },
  } as WebSocketMessage, "Goal 未发送到后端，请检查连接后重试");
  context.setMessages((messages) => upsertMessage(messages, optimisticMessage));
  return request;
}
