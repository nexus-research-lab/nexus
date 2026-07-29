/**
 * INPUT: runtime pending permission 快照与当前可见消息工具调用。
 * OUTPUT: 同 request 的最新快照、首次出现顺序、terminal Room execution 过滤，以及只绑定未收口工具调用的精确匹配。
 * POS: DM/Room 权限渲染、运行态恢复与虚拟估高共用的人工介入身份真相源。
 */
import type { Message } from "@/types/conversation/message/entity";
import type { RoomAgentExecutionState } from "@/types/agent/agent-conversation";
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

interface MatchPendingPermissionsOptions {
  visibleToolUseIds?: ReadonlySet<string>;
}

export function coalescePendingPermissions(
  permissions: readonly PendingPermission[],
): PendingPermission[] {
  const order: string[] = [];
  const latestByRequestId = new Map<string, PendingPermission>();
  for (const permission of permissions) {
    if (!latestByRequestId.has(permission.request_id)) {
      order.push(permission.request_id);
    }
    latestByRequestId.set(permission.request_id, permission);
  }
  return order.flatMap((requestId) => {
    const permission = latestByRequestId.get(requestId);
    return permission ? [permission] : [];
  });
}

/**
 * 权威 lifecycle 已收口的精确 Room execution 不再接受迟到交互。
 * 缺少 root 或 agent_round 身份的旧请求无法安全归属，必须保留。
 */
export function filterPendingPermissionsForTerminalRoomExecutions(
  permissions: PendingPermission[],
  executionStates: readonly RoomAgentExecutionState[],
): PendingPermission[] {
  const terminalExecutionKeys = new Set(executionStates.flatMap((state) => (
    state.phase === "terminal"
      ? [`${state.round_id}\u001f${state.agent_round_id}`]
      : []
  )));
  if (terminalExecutionKeys.size === 0) {
    return permissions;
  }
  const next = permissions.filter((permission) => {
    const roundId = permission.round_id?.trim();
    const agentRoundId = permission.agent_round_id?.trim();
    return !roundId
      || !agentRoundId
      || !terminalExecutionKeys.has(`${roundId}\u001f${agentRoundId}`);
  });
  return next.length === permissions.length ? permissions : next;
}

/**
 * 权限只能绑定当前快照中尚未收口的工具调用；旧事件仅允许在同一消息内精确匹配载荷。
 */
export function matchPendingPermissionsToMessages(
  messages: Message[],
  pendingPermissions: PendingPermission[],
  options: MatchPendingPermissionsOptions = {},
) {
  const candidates = collectUnresolvedToolUses(
    messages,
    options.visibleToolUseIds,
  );
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

function collectUnresolvedToolUses(
  messages: Message[],
  visibleToolUseIds?: ReadonlySet<string>,
): ToolUseCandidate[] {
  const candidates: ToolUseCandidate[] = [];
  const candidatePosition = new Map<string, number>();
  const resolvedToolUseIds = new Set<string>();

  for (const message of messages) {
    if (message.role !== "assistant") {
      continue;
    }
    collectMessageToolUses(
      message,
      candidates,
      candidatePosition,
      resolvedToolUseIds,
      visibleToolUseIds,
    );
  }

  return candidates.filter((candidate) => !resolvedToolUseIds.has(candidate.toolUseId));
}

function collectMessageToolUses(
  message: Extract<Message, { role: "assistant" }>,
  candidates: ToolUseCandidate[],
  candidatePosition: Map<string, number>,
  resolvedToolUseIds: Set<string>,
  visibleToolUseIds?: ReadonlySet<string>,
): void {
  for (const block of message.content) {
    if (block.type === "tool_result") {
      if (!visibleToolUseIds || visibleToolUseIds.has(block.tool_use_id)) {
        resolvedToolUseIds.add(block.tool_use_id);
      }
      continue;
    }
    if (
      block.type !== "tool_use"
      || (visibleToolUseIds && !visibleToolUseIds.has(block.id))
    ) {
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
