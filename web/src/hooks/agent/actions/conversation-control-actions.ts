import { resolveAgentId } from "@/config/runtime-options";
import type { Message } from "@/types/conversation/message/entity";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";
import type { WebSocketMessage } from "@/types/system/websocket";

import {
  conversationContextError,
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
    context.setError(conversationContextError(result.reason));
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

interface PermissionResponsePlan {
  errorMessage?: string;
  removeRequestId?: string;
  response?: WebSocketMessage;
}

const PERMISSION_CONTEXT_ERRORS = {
  disconnected: "WebSocket未连接，无法提交权限决策",
  invalid_session: "当前会话的 session_key 非法，无法提交权限决策",
  missing_session: undefined,
} as const;

const PERMISSION_DECISION_MESSAGES = {
  allow: "",
  deny: "User denied permission",
} as const;

function getPermissionValidationError(
  pendingPermission: PendingPermission,
  payload: PermissionDecisionPayload,
): string | undefined {
  const requiresAnswers = pendingPermission.interaction_mode === "question"
    && payload.decision === "allow";
  return requiresAnswers && !payload.user_answers?.length
    ? "请先完成问题回答"
    : undefined;
}

function buildPermissionResponse(
  pendingPermission: PendingPermission,
  payload: PermissionDecisionPayload,
  context: AgentConversationActionContext,
  sessionKey: string,
): WebSocketMessage {
  const optionalFields = Object.fromEntries(
    [
      ["user_answers", payload.user_answers],
      ["updated_permissions", payload.updated_permissions],
    ].filter((entry) => Array.isArray(entry[1]) && entry[1].length > 0),
  );
  return {
    type: "permission_response",
    request_id: payload.request_id,
    session_key: sessionKey,
    agent_id: resolveAgentId(
      pendingPermission.agent_id || context.identity?.agent_id,
    ),
    decision: payload.decision,
    message: payload.message || PERMISSION_DECISION_MESSAGES[payload.decision],
    interrupt: payload.interrupt === true,
    ...optionalFields,
  };
}

function findPendingPermission(
  requestId: string,
  context: AgentConversationActionContext,
): PendingPermission | undefined {
  return context.pendingPermissions.find(
    (permission) => permission.request_id === requestId,
  );
}

function resolveCurrentPermissionSession(
  context: AgentConversationActionContext,
): string | null {
  const sessionKey = context.sessionKey || context.activeSessionKeyRef.current;
  return sessionKey && context.activeSessionKeyRef.current === sessionKey
    ? sessionKey
    : null;
}

function planPermissionResponse(
  payload: PermissionDecisionPayload,
  context: AgentConversationActionContext,
): PermissionResponsePlan {
  const pendingPermission = findPendingPermission(payload.request_id, context);
  if (!pendingPermission) {
    return {};
  }

  if (!resolveCurrentPermissionSession(context)) {
    return { removeRequestId: payload.request_id };
  }

  const actionContext = resolveConversationActionContext(context);
  if (!actionContext.ok) {
    return {
      errorMessage: PERMISSION_CONTEXT_ERRORS[actionContext.reason],
    };
  }

  const validationError = getPermissionValidationError(
    pendingPermission,
    payload,
  );
  if (validationError) {
    return { errorMessage: validationError };
  }
  return {
    response: buildPermissionResponse(
      pendingPermission,
      payload,
      context,
      actionContext.value.sessionKey,
    ),
  };
}

export function sendSessionPermissionResponse(
  payload: PermissionDecisionPayload,
  context: AgentConversationActionContext,
): boolean {
  const plan = planPermissionResponse(payload, context);
  if (plan.removeRequestId) {
    removePendingPermission(context, plan.removeRequestId);
  }
  if (plan.errorMessage) {
    context.setError(plan.errorMessage);
  }
  if (!plan.response) {
    return false;
  }
  if (context.wsSend(plan.response).disposition !== "sent") {
    context.setError("权限决策发送失败，请稍后重试");
    return false;
  }
  removePendingPermission(context, payload.request_id);
  context.setError(null);
  return true;
}
