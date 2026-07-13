import type { ToolUseContent } from "@/types/conversation/message/content";
import type { Message } from "@/types/conversation/message/entity";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

export interface PendingPermissionToolUseCandidate {
  tool_use_id: string;
  tool_name: string;
  tool_input: Record<string, unknown>;
  message_id: string;
}

export interface PendingPermissionMatchResult {
  matched_permissions_by_tool_use_id: Map<string, PendingPermission>;
  matched_request_ids: Set<string>;
  unmatched_permissions: PendingPermission[];
}

export function collectUnresolvedToolUseCandidates(
  messages: Message[],
): PendingPermissionToolUseCandidate[] {
  const ordered_candidates: PendingPermissionToolUseCandidate[] = [];
  const candidate_index_by_tool_use_id = new Map<string, number>();
  const resolved_tool_use_ids = new Set<string>();

  for (const message of messages) {
    if (message.role !== "assistant") {
      continue;
    }

    for (const block of message.content) {
      if (block.type === "tool_use") {
        const next_candidate = build_tool_use_candidate(block, message.message_id);
        const existing_index = candidate_index_by_tool_use_id.get(block.id);
        if (existing_index == null) {
          candidate_index_by_tool_use_id.set(block.id, ordered_candidates.length);
          ordered_candidates.push(next_candidate);
        } else {
          ordered_candidates[existing_index] = next_candidate;
        }
        continue;
      }

      if (block.type === "tool_result") {
        resolved_tool_use_ids.add(block.tool_use_id);
      }
    }
  }

  return ordered_candidates.filter((candidate) => !resolved_tool_use_ids.has(candidate.tool_use_id));
}

export function matchPendingPermissionsToToolUses(
  pending_permissions: PendingPermission[],
  candidates: PendingPermissionToolUseCandidate[],
): PendingPermissionMatchResult {
  const matched_permissions_by_tool_use_id = new Map<string, PendingPermission>();
  const matched_request_ids = new Set<string>();
  const candidate_queue_by_message_id = new Map<string, PendingPermissionToolUseCandidate[]>();

  for (const candidate of candidates) {
    const queue = candidate_queue_by_message_id.get(candidate.message_id) ?? [];
    queue.push(candidate);
    candidate_queue_by_message_id.set(candidate.message_id, queue);
  }

  for (const permission of pending_permissions) {
    const message_id = permission.message_id?.trim();
    if (!message_id) {
      continue;
    }

    const queue = candidate_queue_by_message_id.get(message_id);
    if (!queue?.length) {
      continue;
    }

    const matched_index = queue.findIndex((candidate) => (
      permission.tool_name === candidate.tool_name &&
      stable_stringify(permission.tool_input) === stable_stringify(candidate.tool_input)
    ));
    if (matched_index < 0) {
      continue;
    }

    const [candidate] = queue.splice(matched_index, 1);
    if (!candidate) {
      continue;
    }

    matched_permissions_by_tool_use_id.set(candidate.tool_use_id, permission);
    matched_request_ids.add(permission.request_id);
  }

  return {
    matched_permissions_by_tool_use_id,
    matched_request_ids,
    unmatched_permissions: pending_permissions.filter((permission) => !matched_request_ids.has(permission.request_id)),
  };
}

export function filterPendingPermissionsForStage(
  permissions: PendingPermission[],
  session_key: string | null,
  agent_id: string | null | undefined,
  projected_messages: Message[],
): PendingPermission[] {
  const projected_message_ids = new Set(projected_messages.map((message) => message.message_id));

  return permissions.filter((permission) => {
    if (agent_id && permission.agent_id && permission.agent_id !== agent_id) {
      return false;
    }

    if (!session_key) {
      return true;
    }

    if (permission.session_key) {
      return are_equivalent_stage_session_keys(permission.session_key, session_key);
    }

    if (permission.message_id) {
      return projected_message_ids.has(permission.message_id);
    }

    return false;
  });
}

function build_tool_use_candidate(
  block: ToolUseContent,
  message_id: string,
): PendingPermissionToolUseCandidate {
  return {
    tool_use_id: block.id,
    tool_name: block.name,
    tool_input: (block.input ?? {}) as Record<string, unknown>,
    message_id,
  };
}

function are_equivalent_stage_session_keys(
  left: string | null | undefined,
  right: string | null | undefined,
): boolean {
  const left_identity = get_stage_session_identity(left);
  const right_identity = get_stage_session_identity(right);
  return Boolean(left_identity && right_identity && left_identity === right_identity);
}

function get_stage_session_identity(session_key: string | null | undefined): string | null {
  const normalized_key = (session_key ?? "").trim();
  if (!normalized_key) {
    return null;
  }
  if (normalized_key.startsWith("room:group:")) {
    return `room:${normalized_key.slice("room:group:".length)}`;
  }
  return normalized_key;
}

function stable_stringify(value: unknown): string {
  if (value == null) {
    return "null";
  }
  if (typeof value === "string") {
    return JSON.stringify(value);
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  if (Array.isArray(value)) {
    return `[${value.map((item) => stable_stringify(item)).join(",")}]`;
  }
  if (typeof value === "object") {
    return `{${Object.keys(value as Record<string, unknown>)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${stable_stringify((value as Record<string, unknown>)[key])}`)
      .join(",")}}`;
  }
  return JSON.stringify(String(value));
}
