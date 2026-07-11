import type { PendingPermission } from "@/types/conversation/interaction/permission";

export type MessageActivityState =
  | "sending"
  | "thinking"
  | "replying"
  | "browsing"
  | "executing"
  | "waiting_permission"
  | "waiting_input";

export const ASK_USER_QUESTION_TOOL_NAME = "AskUserQuestion";

const BROWSING_TOOL_NAMES = new Set([
  "Read",
  "Glob",
  "LS",
  "Grep",
  "WebSearch",
  "WebFetch",
]);

export function resolveToolActivityState(
  toolName?: string | null,
): MessageActivityState {
  return toolName && BROWSING_TOOL_NAMES.has(toolName)
    ? "browsing"
    : "executing";
}

export function resolvePermissionActivityState(
  permissions: readonly PendingPermission[],
): MessageActivityState | null {
  if (permissions.length === 0) {
    return null;
  }
  return permissions.some(isQuestionPermission)
    ? "waiting_input"
    : "waiting_permission";
}

function isQuestionPermission(permission: PendingPermission): boolean {
  return permission.interaction_mode === "question"
    || permission.tool_name === ASK_USER_QUESTION_TOOL_NAME;
}
