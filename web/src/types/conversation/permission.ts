/**
 * 权限类型定义
 *
 * [INPUT]: 无
 * [OUTPUT]: 对外提供权限请求、建议与响应类型
 * [POS]: types 模块的权限类型中心，被 hook 与权限弹窗消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { Message } from './message';
import { UserQuestionAnswer } from './ask-user-question';

export type PermissionRiskLevel = 'low' | 'medium' | 'high';
export type PermissionDecision = 'allow' | 'deny';
export type PermissionInteractionMode = 'permission' | 'question';
export type PermissionUpdateType =
  | 'addRules'
  | 'replaceRules'
  | 'removeRules'
  | 'setMode'
  | 'addDirectories'
  | 'removeDirectories';
export type PermissionBehavior = 'allow' | 'deny' | 'ask';
export type PermissionDestination = 'userSettings' | 'projectSettings' | 'localSettings' | 'session';

export interface PermissionRule {
  tool_name: string;
  rule_content?: string | null;
}

export interface PermissionUpdate {
  type: PermissionUpdateType;
  rules?: PermissionRule[];
  behavior?: PermissionBehavior;
  mode?: 'default' | 'acceptEdits' | 'plan' | 'bypassPermissions';
  directories?: string[];
  destination?: PermissionDestination;
}

export interface PendingPermission {
  request_id: string;
  tool_name: string;
  tool_input: Record<string, any>;
  session_key?: string | null;
  agent_id?: string | null;
  message_id?: string | null;
  caused_by?: string | null;
  interaction_mode?: PermissionInteractionMode;
  risk_level?: PermissionRiskLevel;
  risk_label?: string;
  summary?: string;
  suggestions?: PermissionUpdate[];
  expires_at?: string;
}

export interface PermissionDecisionPayload {
  request_id: string;
  decision: PermissionDecision;
  user_answers?: UserQuestionAnswer[];
  updated_permissions?: PermissionUpdate[];
  message?: string;
  interrupt?: boolean;
}

export interface PendingPermissionToolUseCandidate {
  tool_use_id: string;
  tool_name: string;
  tool_input: Record<string, unknown>;
  message_id: string;
  round_id: string;
}

export interface PendingPermissionMatchResult {
  matched_permissions_by_tool_use_id: Map<string, PendingPermission>;
  matched_request_ids: Set<string>;
  unmatched_permissions: PendingPermission[];
}

/**
 * 从消息快照中提取仍未完成的 toolUse。
 *
 * 中文注释：权限请求的唯一上游来源是后端事件里的 `messageId / causedBy`。
 * 这里先把当前 assistant 消息里仍未被 toolResult 收口的 toolUse 全部抽出来，
 * 后续所有权限绑定都只允许在这批候选里做精确匹配，不再跨消息猜测。
 */
export function collectUnresolvedToolUseCandidates(
  messages: Message[],
): PendingPermissionToolUseCandidate[] {
  const orderedCandidates: PendingPermissionToolUseCandidate[] = [];
  const candidateIndexByToolUseId = new Map<string, number>();
  const resolvedToolUseIds = new Set<string>();

  for (const message of messages) {
    if (message.role !== 'assistant') {
      continue;
    }

    for (const block of message.content) {
      if (block.type === 'tool_use') {
        const nextCandidate: PendingPermissionToolUseCandidate = {
          tool_use_id: block.id,
          tool_name: block.name,
          tool_input: (block.input ?? {}) as Record<string, unknown>,
          message_id: message.message_id,
          round_id: message.round_id,
        };
        const existingIndex = candidateIndexByToolUseId.get(block.id);
        if (existingIndex == null) {
          candidateIndexByToolUseId.set(block.id, orderedCandidates.length);
          orderedCandidates.push(nextCandidate);
        } else {
          orderedCandidates[existingIndex] = nextCandidate;
        }
        continue;
      }

      if (block.type === 'tool_result') {
        resolvedToolUseIds.add(block.tool_use_id);
      }
    }
  }

  return orderedCandidates.filter((candidate) => !resolvedToolUseIds.has(candidate.tool_use_id));
}

/**
 * 将 pending permission 精确绑定到唯一的 toolUse。
 *
 * 中文注释：绑定主键只认 `permission.messageId`。
 * 同一条 assistant message 内如果有多个 toolUse，再用后端原样携带的工具载荷做精确定位；
 * 一旦缺少 `messageId` 或载荷不一致，就保留成未匹配卡片，不走跨消息签名兜底。
 */
export function matchPendingPermissionsToToolUses(
  pendingPermissions: PendingPermission[],
  candidates: PendingPermissionToolUseCandidate[],
): PendingPermissionMatchResult {
  const matchedPermissionsByToolUseId = new Map<string, PendingPermission>();
  const matchedRequestIds = new Set<string>();
  const candidateQueueByMessageId = new Map<string, PendingPermissionToolUseCandidate[]>();

  for (const candidate of candidates) {
    const queue = candidateQueueByMessageId.get(candidate.message_id) ?? [];
    queue.push(candidate);
    candidateQueueByMessageId.set(candidate.message_id, queue);
  }

  for (const permission of pendingPermissions) {
    const messageId = permission.message_id?.trim();
    if (!messageId) {
      continue;
    }

    const queue = candidateQueueByMessageId.get(messageId);
    if (!queue?.length) {
      continue;
    }

    const matchedIndex = queue.findIndex((candidate) => isSameToolInvocation(permission, candidate));
    if (matchedIndex < 0) {
      continue;
    }

    const [candidate] = queue.splice(matchedIndex, 1);
    if (!candidate) {
      continue;
    }

    matchedPermissionsByToolUseId.set(candidate.tool_use_id, permission);
    matchedRequestIds.add(permission.request_id);
  }

  return {
    matched_permissions_by_tool_use_id: matchedPermissionsByToolUseId,
    matched_request_ids: matchedRequestIds,
    unmatched_permissions: pendingPermissions.filter(
      (permission) => !matchedRequestIds.has(permission.request_id),
    ),
  };
}

function isSameToolInvocation(
  permission: PendingPermission,
  candidate: PendingPermissionToolUseCandidate,
): boolean {
  return (
    permission.tool_name === candidate.tool_name
    && stableStringify(permission.tool_input) === stableStringify(candidate.tool_input)
  );
}

function stableStringify(value: unknown): string {
  if (value == null) {
    return 'null';
  }
  if (typeof value === 'string') {
    return JSON.stringify(value);
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  if (Array.isArray(value)) {
    return `[${value.map((item) => stableStringify(item)).join(',')}]`;
  }
  if (typeof value === 'object') {
    return `{${Object.keys(value as Record<string, unknown>)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${stableStringify((value as Record<string, unknown>)[key])}`)
      .join(',')}}`;
  }
  return JSON.stringify(String(value));
}
