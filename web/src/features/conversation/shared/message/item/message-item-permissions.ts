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
  pendingPermissions: PendingPermission[],
): MessageItemPermissionMatch {
  if (pendingPermissions.length === 0) {
    return {
      matched_pending_permissions_by_tool_use_id: new Map(),
      unmatched_pending_permissions: [],
    };
  }

  const unresolvedToolUseCandidates =
    collect_unresolved_tool_use_candidates(messages);
  const permissionMatchResult = match_pending_permissions_to_tool_uses(
    pendingPermissions,
    unresolvedToolUseCandidates,
  );
  const matchedPermissionsByToolUseId = new Map(
    permissionMatchResult.matched_permissions_by_tool_use_id,
  );

  const unmatchedQuestionPermissions =
    permissionMatchResult.unmatched_permissions.filter(
      (permission) =>
        permission.interaction_mode === "question" ||
        permission.tool_name === "AskUserQuestion",
    );
  const unresolvedQuestionCandidates =
    unresolvedToolUseCandidates.filter(
      (candidate) =>
        candidate.tool_name === "AskUserQuestion" &&
        !matchedPermissionsByToolUseId.has(candidate.tool_use_id),
    );

  // Room 场景下 AskUserQuestion 的 permission_request 会先绑定占位槽位，
  // 这里按 round_id 和单候选规则做一次安全补配，避免问答块丢失交互能力。
  for (const permission of unmatchedQuestionPermissions) {
    const candidatesByRound = unresolvedQuestionCandidates.filter(
      (candidate) =>
        !matchedPermissionsByToolUseId.has(candidate.tool_use_id) &&
        (!permission.caused_by ||
          candidate.round_id === permission.caused_by),
    );

    if (candidatesByRound.length === 1) {
      matchedPermissionsByToolUseId.set(
        candidatesByRound[0].tool_use_id,
        permission,
      );
      continue;
    }

    const remainingCandidates = unresolvedQuestionCandidates.filter(
      (candidate) =>
        !matchedPermissionsByToolUseId.has(candidate.tool_use_id),
    );
    if (
      remainingCandidates.length === 1 &&
      unmatchedQuestionPermissions.length === 1
    ) {
      matchedPermissionsByToolUseId.set(
        remainingCandidates[0].tool_use_id,
        permission,
      );
    }
  }

  return {
    matched_pending_permissions_by_tool_use_id:
      matchedPermissionsByToolUseId,
    unmatched_pending_permissions:
      permissionMatchResult.unmatched_permissions.filter(
        (permission) =>
          permission.interaction_mode !== "question" &&
          permission.tool_name !== "AskUserQuestion",
      ),
  };
}
