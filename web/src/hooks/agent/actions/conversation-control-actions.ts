import { resolveAgentId } from "@/config/options";
import type { Message } from "@/types/conversation/message/entity";
import type { PermissionDecisionPayload } from "@/types/conversation/interaction/permission";
import type { WebSocketMessage } from "@/types/system/websocket";

import {
  resolveConversationActionContext,
  type AgentConversationActionContext,
} from "./conversation-action-context";
import { buildConversationAddress } from "./conversation-command-builders";

function getLatestUserRoundId(messages: Message[]): string | undefined {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index].role === "user") {
      return messages[index].round_id;
    }
  }
  return undefined;
}

export function stopSessionGeneration(
  context: AgentConversationActionContext,
  agentRoundId?: string,
): void {
  const result = resolveConversationActionContext(context);
  if (!result.ok) {
    if (result.reason === "invalid_session") {
      context.setError("当前会话的 session_key 非法，无法中断");
    }
    return;
  }

  const command: WebSocketMessage = {
    type: "interrupt",
    ...buildConversationAddress(result.value),
    round_id: getLatestUserRoundId(context.messages),
    ...(agentRoundId ? { agent_round_id: agentRoundId } : {}),
  } as WebSocketMessage;
  if (context.wsSend(command).disposition !== "sent") {
    context.setError("中断请求发送失败，请稍后重试");
    return;
  }
  context.setPendingPermissions([]);
}

function removePendingPermission(
  context: AgentConversationActionContext,
  requestId: string,
): void {
  context.setPendingPermissions((permissions) => permissions.filter(
    (permission) => permission.request_id !== requestId,
  ));
}

export function sendSessionPermissionResponse(
  payload: PermissionDecisionPayload,
  context: AgentConversationActionContext,
): boolean {
  const pendingPermission = context.pendingPermissions.find(
    (permission) => permission.request_id === payload.request_id,
  );
  if (!pendingPermission) {
    return false;
  }

  const sessionKey = context.sessionKey || context.activeSessionKeyRef.current;
  if (!sessionKey || context.activeSessionKeyRef.current !== sessionKey) {
    removePendingPermission(context, payload.request_id);
    return false;
  }

  const actionContext = resolveConversationActionContext(context);
  const validationRules: Array<[boolean, string]> = [
    [
      !actionContext.ok && actionContext.reason === "invalid_session",
      "当前会话的 session_key 非法，无法提交权限决策",
    ],
    [
      !actionContext.ok && actionContext.reason === "disconnected",
      "WebSocket未连接，无法提交权限决策",
    ],
    [
      pendingPermission.interaction_mode === "question" &&
        payload.decision === "allow" &&
        !payload.user_answers?.length,
      "请先完成问题回答",
    ],
  ];
  const validationError = validationRules.find(([failed]) => failed)?.[1];
  if (validationError || !actionContext.ok) {
    if (validationError) {
      context.setError(validationError);
    }
    return false;
  }

  const response: WebSocketMessage = {
    type: "permission_response",
    request_id: payload.request_id,
    session_key: actionContext.value.sessionKey,
    agent_id: resolveAgentId(
      pendingPermission.agent_id || context.identity?.agent_id,
    ),
    decision: payload.decision,
    message: payload.message || (
      payload.decision === "deny" ? "User denied permission" : ""
    ),
    interrupt: payload.interrupt ?? false,
    ...(payload.user_answers?.length
      ? { user_answers: payload.user_answers }
      : {}),
    ...(payload.updated_permissions?.length
      ? { updated_permissions: payload.updated_permissions }
      : {}),
  };
  if (context.wsSend(response).disposition !== "sent") {
    context.setError("权限决策发送失败，请稍后重试");
    return false;
  }
  removePendingPermission(context, payload.request_id);
  context.setError(null);
  return true;
}
