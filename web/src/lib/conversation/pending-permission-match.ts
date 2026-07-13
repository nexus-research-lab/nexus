import type { Message } from "@/types/conversation/message/entity";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

interface ToolUseCandidate {
  messageId: string;
  toolInput: Record<string, unknown>;
  toolName: string;
  toolUseId: string;
}

interface CandidateIndex {
  byMessageId: Map<string, ToolUseCandidate[]>;
  byToolUseId: Map<string, ToolUseCandidate>;
}

/**
 * 权限只能绑定当前快照中尚未收口的工具调用；旧事件仅允许在同一消息内精确匹配载荷。
 */
export function matchPendingPermissionsToMessages(
  messages: Message[],
  pendingPermissions: PendingPermission[],
) {
  const candidates = collectUnresolvedToolUses(messages);
  const candidateIndex = buildCandidateIndex(candidates);
  const matchedByToolUseId = new Map<string, PendingPermission>();
  const matchedRequestIds = new Set<string>();

  for (const permission of pendingPermissions) {
    const candidate = findPermissionCandidate(permission, candidateIndex);
    if (!candidate) {
      continue;
    }
    consumeCandidate(candidateIndex, candidate);
    matchedByToolUseId.set(candidate.toolUseId, permission);
    matchedRequestIds.add(permission.request_id);
  }

  return {
    matchedByToolUseId,
    matchedRequestIds,
    unmatchedPermissions: pendingPermissions.filter(
      (permission) => !matchedRequestIds.has(permission.request_id),
    ),
  };
}

function collectUnresolvedToolUses(messages: Message[]): ToolUseCandidate[] {
  const candidates: ToolUseCandidate[] = [];
  const candidatePosition = new Map<string, number>();
  const resolvedToolUseIds = new Set<string>();

  for (const message of messages) {
    if (message.role !== "assistant") {
      continue;
    }
    collectMessageToolUses(message, candidates, candidatePosition, resolvedToolUseIds);
  }

  return candidates.filter((candidate) => !resolvedToolUseIds.has(candidate.toolUseId));
}

function collectMessageToolUses(
  message: Extract<Message, { role: "assistant" }>,
  candidates: ToolUseCandidate[],
  candidatePosition: Map<string, number>,
  resolvedToolUseIds: Set<string>,
): void {
  for (const block of message.content) {
    if (block.type === "tool_result") {
      resolvedToolUseIds.add(block.tool_use_id);
      continue;
    }
    if (block.type !== "tool_use") {
      continue;
    }

    const candidate = {
      messageId: message.message_id,
      toolInput: (block.input ?? {}) as Record<string, unknown>,
      toolName: block.name,
      toolUseId: block.id,
    };
    const position = candidatePosition.get(block.id);
    if (position === undefined) {
      candidatePosition.set(block.id, candidates.length);
      candidates.push(candidate);
      continue;
    }
    candidates[position] = candidate;
  }
}

function buildCandidateIndex(candidates: ToolUseCandidate[]): CandidateIndex {
  const index: CandidateIndex = {
    byMessageId: new Map(),
    byToolUseId: new Map(),
  };
  for (const candidate of candidates) {
    index.byToolUseId.set(candidate.toolUseId, candidate);
    const messageCandidates = index.byMessageId.get(candidate.messageId) ?? [];
    messageCandidates.push(candidate);
    index.byMessageId.set(candidate.messageId, messageCandidates);
  }
  return index;
}

function findPermissionCandidate(
  permission: PendingPermission,
  index: CandidateIndex,
): ToolUseCandidate | undefined {
  const toolUseId = permission.tool_use_id?.trim();
  if (toolUseId) {
    return index.byToolUseId.get(toolUseId);
  }

  const messageId = permission.message_id?.trim();
  return messageId
    ? index.byMessageId.get(messageId)?.find(
      (candidate) => isSameToolInvocation(permission, candidate),
    )
    : undefined;
}

function consumeCandidate(index: CandidateIndex, candidate: ToolUseCandidate): void {
  index.byToolUseId.delete(candidate.toolUseId);
  const remaining = index.byMessageId.get(candidate.messageId)?.filter(
    (entry) => entry !== candidate,
  );
  index.byMessageId.set(candidate.messageId, remaining ?? []);
}

function isSameToolInvocation(
  permission: PendingPermission,
  candidate: ToolUseCandidate,
): boolean {
  return permission.tool_name === candidate.toolName
    && stableStringify(permission.tool_input) === stableStringify(candidate.toolInput);
}

function stableStringify(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map(stableStringify).join(",")}]`;
  }
  if (value !== null && typeof value === "object") {
    const record = value as Record<string, unknown>;
    return `{${Object.keys(record)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${stableStringify(record[key])}`)
      .join(",")}}`;
  }
  return JSON.stringify(value) ?? JSON.stringify(String(value));
}
