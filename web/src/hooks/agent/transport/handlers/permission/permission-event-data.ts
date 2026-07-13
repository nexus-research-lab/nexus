import {
  asUnknownRecord,
  readString,
  type UnknownRecord,
} from "@/lib/unknown-value";
import type {
  PendingPermission,
  PermissionInteractionMode,
  PermissionRiskLevel,
  PermissionUpdate,
} from "@/types/conversation/interaction/permission";
import type { EventMessage } from "@/types/generated/protocol";

const INTERACTION_MODES = new Set<PermissionInteractionMode>([
  "permission",
  "question",
]);
const DEFAULT_INTERACTION_MODES = new Map<string, PermissionInteractionMode>([
  ["AskUserQuestion", "question"],
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

export function decodePermissionRequest(
  event: EventMessage,
): PendingPermission | null {
  const requestId = readString(event.data, "request_id");
  const toolName = readString(event.data, "tool_name");
  if (!requestId || !toolName) {
    return null;
  }

  return {
    request_id: requestId,
    tool_name: toolName,
    tool_input: readToolInput(event.data.tool_input),
    session_key: readEventSessionKey(event),
    agent_id: readEventScope(event, "agent_id"),
    message_id: readEventScope(event, "message_id"),
    round_id: readEventScope(event, "round_id"),
    agent_round_id: readEventScope(event, "agent_round_id"),
    tool_use_id: readString(event.data, "tool_use_id"),
    interaction_mode: readInteractionMode(event.data, toolName),
    risk_level: readEnum(event.data, "risk_level", RISK_LEVELS),
    risk_label: readOptionalString(event.data, "risk_label"),
    summary: readOptionalString(event.data, "summary"),
    suggestions: readPermissionSuggestions(event.data.suggestions),
    expires_at: readOptionalString(event.data, "expires_at"),
  };
}

function readEventSessionKey(event: EventMessage): string | null {
  return event.session_key ?? null;
}

export function decodeResolvedPermissionRequestId(
  event: EventMessage,
): string | null {
  return readString(event.data, "request_id");
}

function readInteractionMode(
  data: UnknownRecord,
  toolName: string,
): PermissionInteractionMode {
  const explicitMode = readEnum(
    data,
    "interaction_mode",
    INTERACTION_MODES,
  );
  if (explicitMode) {
    return explicitMode;
  }
  return DEFAULT_INTERACTION_MODES.get(toolName) ?? "permission";
}

function readEnum<T extends string>(
  data: UnknownRecord,
  key: string,
  values: ReadonlySet<T>,
): T | undefined {
  const value = readString(data, key);
  if (!value) {
    return undefined;
  }
  return values.has(value as T) ? value as T : undefined;
}

function readEventScope(
  event: EventMessage,
  key: "agent_id" | "agent_round_id" | "message_id" | "round_id",
): string | null {
  return readString(event.data, key) ?? event[key] ?? null;
}

function readOptionalString(
  data: UnknownRecord,
  key: string,
): string | undefined {
  return readString(data, key) ?? undefined;
}

function readToolInput(value: unknown): UnknownRecord {
  return asUnknownRecord(value) ?? {};
}

function readPermissionSuggestions(value: unknown): PermissionUpdate[] {
  const suggestions = Array.isArray(value) ? value : [];
  return suggestions.filter(isPermissionUpdate);
}

function isPermissionUpdate(value: unknown): value is PermissionUpdate {
  const record = asUnknownRecord(value);
  return Boolean(
    record && readEnum(record, "type", PERMISSION_UPDATE_TYPES),
  );
}
