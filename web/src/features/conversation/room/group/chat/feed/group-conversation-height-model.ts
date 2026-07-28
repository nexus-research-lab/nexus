/**
 * INPUT: 共享消息估高、Room pending slot 与人工介入投影。
 * OUTPUT: 把尚未写入消息的 Agent 外壳和问答/审批面计入虚拟 feed 初始高度。
 * POS: Room 虚拟列表专属的运行态估高修正，不参与滚动策略。
 */
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import type { Message } from "@/types/conversation/message/entity";

import { parseAskUserQuestions } from "@/features/conversation/shared/message/blocks/question/ask-user-question-model";
import { ASK_USER_QUESTION_TOOL_NAME } from "@/features/conversation/shared/message/message-tool-names";
import {
  coalescePendingPermissions,
  matchPendingPermissionsToMessages,
} from "@/lib/conversation/pending-permission-match";

interface ProjectGroupRoundHeightOptions {
  baseHeights: ReadonlyMap<string, number>;
  containerWidth: number;
  messageGroups: ReadonlyMap<string, Message[]>;
  pendingPermissionGroups: ReadonlyMap<string, PendingPermission[]>;
  pendingSlotGroups: ReadonlyMap<string, RoomPendingAgentSlotState[]>;
  roundIds: readonly string[];
}

const SLOT_ONLY_SHELL_HEIGHT = 112;
const PERMISSION_STACK_GAP = 12;
const TOOL_PERMISSION_HEIGHT = 156;
const QUESTION_FALLBACK_HEIGHT = 260;
const QUESTION_FRAME_HEIGHT = 136;
const QUESTION_PROMPT_LINE_HEIGHT = 24;
const QUESTION_OPTION_HEIGHT = 52;
const QUESTION_CUSTOM_ANSWER_HEIGHT = 64;
const SHARED_TOOL_USE_ESTIMATED_HEIGHT = 60;

export function projectGroupRoundHeights({
  baseHeights,
  containerWidth,
  messageGroups,
  pendingPermissionGroups,
  pendingSlotGroups,
  roundIds,
}: ProjectGroupRoundHeightOptions): Map<string, number> {
  const result = new Map(baseHeights);
  for (const roundId of roundIds) {
    const messages = messageGroups.get(roundId) ?? [];
    const permissions = coalescePendingPermissions(
      pendingPermissionGroups.get(roundId) ?? [],
    );
    const slots = pendingSlotGroups.get(roundId) ?? [];
    const baseHeight = baseHeights.get(roundId) ?? SLOT_ONLY_SHELL_HEIGHT;
    const slotShellHeight = hasAssistantMessage(messages)
      ? 0
      : slots.length * SLOT_ONLY_SHELL_HEIGHT;
    const interactionHeight = estimatePendingInteractionHeight(
      permissions,
      containerWidth,
    );
    const matchedToolUseHeight = countMatchedPendingToolUses(
      messages,
      permissions,
    ) * SHARED_TOOL_USE_ESTIMATED_HEIGHT;
    const contentHeight = Math.max(0, baseHeight - matchedToolUseHeight);
    result.set(
      roundId,
      Math.max(contentHeight, slotShellHeight) + interactionHeight,
    );
  }
  return result;
}

function countMatchedPendingToolUses(
  messages: Message[],
  permissions: PendingPermission[],
): number {
  if (messages.length === 0 || permissions.length === 0) {
    return 0;
  }
  return matchPendingPermissionsToMessages(
    messages,
    permissions,
  ).matchedByToolUseId.size;
}

function hasAssistantMessage(messages: readonly Message[]): boolean {
  return messages.some((message) => message.role === "assistant");
}

function estimatePendingInteractionHeight(
  permissions: readonly PendingPermission[],
  containerWidth: number,
): number {
  if (permissions.length === 0) {
    return 0;
  }
  return permissions.reduce((height, permission) => (
    height
    + PERMISSION_STACK_GAP
    + estimatePendingPermissionHeight(permission, containerWidth)
  ), 0);
}

function estimatePendingPermissionHeight(
  permission: PendingPermission,
  containerWidth: number,
): number {
  if (
    permission.interaction_mode !== "question"
    && permission.tool_name !== ASK_USER_QUESTION_TOOL_NAME
  ) {
    return TOOL_PERMISSION_HEIGHT;
  }
  const questions = parseAskUserQuestions(permission.tool_input);
  if (questions.length === 0) {
    return QUESTION_FALLBACK_HEIGHT;
  }
  const contentWidth = Math.max(containerWidth - 120, 180);
  const charsPerLine = Math.max(Math.floor(contentWidth / 8.5), 18);
  return QUESTION_FRAME_HEIGHT + questions.reduce((height, question) => {
    const promptLines = Math.max(
      1,
      Math.ceil(question.question.length / charsPerLine),
    );
    const optionHeight = Math.max(question.options.length, 1)
      * QUESTION_OPTION_HEIGHT;
    return height
      + promptLines * QUESTION_PROMPT_LINE_HEIGHT
      + optionHeight
      + QUESTION_CUSTOM_ANSWER_HEIGHT;
  }, 0);
}
