import type {
  AgentEventHandler,
  AgentEventHandlerMap,
} from "../agent-event-context";
import { withCurrentSessionEvent } from "./handler-scope";

const handlePermissionRequest: AgentEventHandler = withCurrentSessionEvent((
  event,
  context,
) => {
  const data = event.data || {};
  context.state.setPendingPermissions((currentPermissions) => {
    const nextPermission = {
      request_id: data.request_id,
      tool_name: data.tool_name,
      tool_input: data.tool_input || {},
      session_key: event.session_key || null,
      agent_id: data.agent_id ?? event.agent_id ?? null,
      message_id: data.message_id ?? event.message_id ?? null,
      round_id: data.round_id ?? event.round_id ?? null,
      agent_round_id: data.agent_round_id ?? event.agent_round_id ?? null,
      tool_use_id: data.tool_use_id ?? null,
      interaction_mode: data.interaction_mode ?? (
        data.tool_name === "AskUserQuestion" ? "question" : "permission"
      ),
      risk_level: data.risk_level,
      risk_label: data.risk_label,
      summary: data.summary,
      suggestions: data.suggestions || [],
      expires_at: data.expires_at,
    };
    return [
      ...currentPermissions.filter(
        (permission) => permission.request_id !== data.request_id,
      ),
      nextPermission,
    ];
  });
});

const handlePermissionResolved: AgentEventHandler = withCurrentSessionEvent((
  event,
  context,
) => {
  const requestId = typeof event.data?.request_id === "string"
    ? event.data.request_id
    : "";
  if (!requestId) {
    return;
  }
  context.state.setPendingPermissions((currentPermissions) => {
    const nextPermissions = currentPermissions.filter(
      (permission) => permission.request_id !== requestId,
    );
    return nextPermissions.length === currentPermissions.length
      ? currentPermissions
      : nextPermissions;
  });
});

export const AGENT_PERMISSION_EVENT_HANDLERS: AgentEventHandlerMap = {
  permission_request: handlePermissionRequest,
  permission_request_resolved: handlePermissionResolved,
};
