import type { Message } from "@/types/conversation/message";
import {
  collectUnresolvedToolUseCandidates,
  matchPendingPermissionsToToolUses,
  type PendingPermission,
} from "@/types/conversation/permission";

export interface MessageItemPermissionMatch {
  matchedPendingPermissionsByToolUseId: Map<string, PendingPermission>;
  unmatchedPendingPermissions: PendingPermission[];
}

export function resolveMessageItemPermissions(
  messages: Message[],
  pendingPermissions: PendingPermission[],
): MessageItemPermissionMatch {
  if (pendingPermissions.length === 0) {
    return {
      matchedPendingPermissionsByToolUseId: new Map(),
      unmatchedPendingPermissions: [],
    };
  }

  const unresolvedToolUseCandidates =
    collectUnresolvedToolUseCandidates(messages);
  const permissionMatchResult = matchPendingPermissionsToToolUses(
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

  // Room 场景下 AskUserQuestion 的 permissionRequest 会先绑定占位槽位，
  // 这里按 roundId 和单候选规则做一次安全补配，避免问答块丢失交互能力。
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
    matchedPendingPermissionsByToolUseId: matchedPermissionsByToolUseId,
    unmatchedPendingPermissions:
      permissionMatchResult.unmatched_permissions.filter(
        (permission) =>
          permission.interaction_mode !== "question" &&
          permission.tool_name !== "AskUserQuestion",
      ),
  };
}
