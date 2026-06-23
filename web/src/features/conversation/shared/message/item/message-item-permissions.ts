import type { Message } from "@/types/conversation/message";
import {
  collect_unresolved_tool_use_candidates,
  match_pending_permissions_to_tool_uses,
  type PendingPermission,
} from "@/types/conversation/permission";

export interface MessageItemPermissionMatch {
  matched_pending_permissions_by_tool_use_id: Map<string, PendingPermission>;
  unmatched_pending_permissions: PendingPermission[];
}

export function resolve_message_item_permissions(
  messages: Message[],
  pending_permissions: PendingPermission[],
): MessageItemPermissionMatch {
  if (pending_permissions.length === 0) {
    return {
      matched_pending_permissions_by_tool_use_id: new Map(),
      unmatched_pending_permissions: [],
    };
  }

  const unresolved_tool_use_candidates =
    collect_unresolved_tool_use_candidates(messages);
  const permission_match_result = match_pending_permissions_to_tool_uses(
    pending_permissions,
    unresolved_tool_use_candidates,
  );
  const matched_permissions_by_tool_use_id = new Map(
    permission_match_result.matched_permissions_by_tool_use_id,
  );

  const unmatched_question_permissions =
    permission_match_result.unmatched_permissions.filter(
      (permission) =>
        permission.interaction_mode === "question" ||
        permission.tool_name === "AskUserQuestion",
    );
  const unresolved_question_candidates =
    unresolved_tool_use_candidates.filter(
      (candidate) =>
        candidate.tool_name === "AskUserQuestion" &&
        !matched_permissions_by_tool_use_id.has(candidate.tool_use_id),
    );

  // Room 场景下 AskUserQuestion 的 permission_request 会先绑定占位槽位，
  // 这里按 round_id 和单候选规则做一次安全补配，避免问答块丢失交互能力。
  for (const permission of unmatched_question_permissions) {
    const candidates_by_round = unresolved_question_candidates.filter(
      (candidate) =>
        !matched_permissions_by_tool_use_id.has(candidate.tool_use_id) &&
        (!permission.caused_by ||
          candidate.round_id === permission.caused_by),
    );

    if (candidates_by_round.length === 1) {
      matched_permissions_by_tool_use_id.set(
        candidates_by_round[0].tool_use_id,
        permission,
      );
      continue;
    }

    const remaining_candidates = unresolved_question_candidates.filter(
      (candidate) =>
        !matched_permissions_by_tool_use_id.has(candidate.tool_use_id),
    );
    if (
      remaining_candidates.length === 1 &&
      unmatched_question_permissions.length === 1
    ) {
      matched_permissions_by_tool_use_id.set(
        remaining_candidates[0].tool_use_id,
        permission,
      );
    }
  }

  return {
    matched_pending_permissions_by_tool_use_id:
      matched_permissions_by_tool_use_id,
    unmatched_pending_permissions:
      permission_match_result.unmatched_permissions.filter(
        (permission) =>
          permission.interaction_mode !== "question" &&
          permission.tool_name !== "AskUserQuestion",
      ),
  };
}
