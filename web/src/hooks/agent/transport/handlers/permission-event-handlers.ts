import { asUnknownRecord, readString } from "@/lib/unknown-value";
import type {
  PendingPermission,
  PermissionInteractionMode,
  PermissionRiskLevel,
  PermissionUpdate,
} from "@/types/conversation/interaction/permission";
import type { EventMessage } from "@/types/generated/protocol";

import type {
  AgentEventHandler,
  AgentEventHandlerMap,
} from "../agent-event-context";
import { withCurrentSessionEvent } from "./handler-scope";

const INTERACTION_MODES = new Set<PermissionInteractionMode>([
  "permission",
  "question",
]);
const RISK_LEVELS = new Set<PermissionRiskLevel>(["high", "low", "medium"]);
const PERMISSION_UPDATE_TYPES = new Set<PermissionUpdate["type"]>([
  "addDirectories",
  "addRules",
  "removeDirectories",
  "removeRules",
  "replaceRules",
  "setMode",
]);

function isPermissionUpdate(value: unknown): value is PermissionUpdate {
  const record = asUnknownRecord(value);
  const type = record ? readString(record, "type") : null;
  return Boolean(
    type
    && PERMISSION_UPDATE_TYPES.has(type as PermissionUpdate["type"]),
  );
}

function parsePermissionRequest(event: EventMessage): PendingPermission | null {
  const data = event.data;
  const requestId = readString(data, "request_id");
  const toolName = readString(data, "tool_name");
  if (!requestId || !toolName) {
    return null;
  }

  const interactionModeValue = readString(data, "interaction_mode");
  const interactionMode = interactionModeValue
    && INTERACTION_MODES.has(interactionModeValue as PermissionInteractionMode)
    ? interactionModeValue as PermissionInteractionMode
    : toolName === "AskUserQuestion" ? "question" : "permission";
  const riskLevelValue = readString(data, "risk_level");
  const riskLevel = riskLevelValue
    && RISK_LEVELS.has(riskLevelValue as PermissionRiskLevel)
    ? riskLevelValue as PermissionRiskLevel
    : undefined;
  const suggestions = Array.isArray(data.suggestions)
    ? data.suggestions.filter(isPermissionUpdate)
    : [];
  const riskLabel = readString(data, "risk_label");
  const summary = readString(data, "summary");
  const expiresAt = readString(data, "expires_at");

  return {
    request_id: requestId,
    tool_name: toolName,
    tool_input: asUnknownRecord(data.tool_input) ?? {},
    session_key: event.session_key ?? null,
    agent_id: readString(data, "agent_id") ?? event.agent_id ?? null,
    message_id: readString(data, "message_id") ?? event.message_id ?? null,
    round_id: readString(data, "round_id") ?? event.round_id ?? null,
    agent_round_id: readString(data, "agent_round_id")
      ?? event.agent_round_id
      ?? null,
    tool_use_id: readString(data, "tool_use_id"),
    interaction_mode: interactionMode,
    ...(riskLevel ? { risk_level: riskLevel } : {}),
    ...(riskLabel ? { risk_label: riskLabel } : {}),
    ...(summary ? { summary } : {}),
    suggestions,
    ...(expiresAt ? { expires_at: expiresAt } : {}),
  };
}

const handlePermissionRequest: AgentEventHandler = withCurrentSessionEvent((
  event,
  context,
) => {
  const nextPermission = parsePermissionRequest(event);
  if (!nextPermission) {
    return;
  }
  context.state.setPendingPermissions((currentPermissions) => {
    return [
      ...currentPermissions.filter(
        (permission) => permission.request_id !== nextPermission.request_id,
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
