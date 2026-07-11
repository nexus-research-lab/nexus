import type {
  ContentBlock,
  TaskProgressContent,
  ToolUseContent,
} from "@/types/conversation/message/content";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

import {
  ASK_USER_QUESTION_TOOL_NAME,
  type MessageActivityState,
  resolveToolActivityState,
} from "./message-activity-state";
import { findLastActivityBlock } from "./message-activity-blocks";

interface ContentActivityContext {
  fallback: MessageActivityState | null;
  hasStreamingText: boolean;
  pendingPermissionsByToolUseId: ReadonlyMap<string, PendingPermission>;
}

interface ContentActivityRule {
  matches: (block: ContentBlock) => boolean;
  resolve: (
    block: ContentBlock,
    context: ContentActivityContext,
  ) => MessageActivityState;
}

interface ActivityBlockVisibilityContext {
  block: ContentBlock;
  consumedBlockIndexes: ReadonlySet<number>;
  hiddenToolNames: ReadonlySet<string>;
  index: number;
}

interface ToolUseActivityFacts {
  fallback: MessageActivityState;
  hasPermission: boolean;
  isQuestion: boolean;
}

interface ToolUseActivityRule {
  matches: (facts: ToolUseActivityFacts) => boolean;
  resolve: (facts: ToolUseActivityFacts) => MessageActivityState;
}

const HIDDEN_ACTIVITY_BLOCK_RULES: ReadonlyArray<
  (context: ActivityBlockVisibilityContext) => boolean
> = [
  ({ consumedBlockIndexes, index }) => consumedBlockIndexes.has(index),
  ({ block, hiddenToolNames }) => block.type === "tool_use"
    && hiddenToolNames.has(block.name),
  ({ block }) => block.type === "text" && !block.text.trim(),
  ({ block }) => block.type === "thinking" && !block.thinking.trim(),
];

const CONTENT_ACTIVITY_RULES: ContentActivityRule[] = [
  defineContentActivityRule(
    "task_progress",
    (block) => resolveProgressActivityState(block),
  ),
  defineContentActivityRule(
    "tool_use",
    (block, context) => resolveToolUseActivityState(block, context),
  ),
  defineContentActivityRule("thinking", () => "thinking"),
  defineContentActivityRule(
    "text",
    (_block, context) => context.hasStreamingText
      ? "replying"
      : context.fallback ?? "replying",
  ),
  defineContentActivityRule(
    "workspace_file_artifact",
    (_block, context) => context.fallback ?? "executing",
  ),
];

const TOOL_USE_ACTIVITY_RULES: ToolUseActivityRule[] = [
  {
    matches: ({ hasPermission }) => hasPermission,
    resolve: ({ isQuestion }) => isQuestion
      ? "waiting_input"
      : "waiting_permission",
  },
  {
    matches: ({ isQuestion }) => isQuestion,
    resolve: ({ fallback }) => fallback,
  },
];

const EMPTY_PENDING_PERMISSIONS = new Map<string, PendingPermission>();
const EMPTY_STREAMING_BLOCK_INDEXES = new Set<number>();

export function resolveContentActivityState({
  consumedBlockIndexes,
  content,
  fallbackActivityState = null,
  hiddenToolNames,
  pendingPermissionsByToolUseId = EMPTY_PENDING_PERMISSIONS,
  resolvedToolUseIds,
  streamingBlockIndexes = EMPTY_STREAMING_BLOCK_INDEXES,
}: {
  consumedBlockIndexes: ReadonlySet<number>;
  content: readonly ContentBlock[];
  fallbackActivityState?: MessageActivityState | null;
  hiddenToolNames: ReadonlySet<string>;
  pendingPermissionsByToolUseId?: ReadonlyMap<string, PendingPermission>;
  resolvedToolUseIds: ReadonlySet<string>;
  streamingBlockIndexes?: ReadonlySet<number>;
}): MessageActivityState {
  const context: ContentActivityContext = {
    fallback: fallbackActivityState,
    hasStreamingText: hasStreamingTextBlock(content, streamingBlockIndexes),
    pendingPermissionsByToolUseId,
  };
  return resolveLatestPendingToolActivity({
    content,
    context,
    hiddenToolNames,
    resolvedToolUseIds,
  }) ?? resolveLatestVisibleBlockActivity({
    consumedBlockIndexes,
    content,
    context,
    hiddenToolNames,
  });
}

function resolveLatestPendingToolActivity({
  content,
  context,
  hiddenToolNames,
  resolvedToolUseIds,
}: {
  content: readonly ContentBlock[];
  context: ContentActivityContext;
  hiddenToolNames: ReadonlySet<string>;
  resolvedToolUseIds: ReadonlySet<string>;
}): MessageActivityState | null {
  const toolUse = findLatestPendingToolUse(
    content,
    resolvedToolUseIds,
    hiddenToolNames,
  );
  return toolUse ? resolveToolUseActivityState(toolUse, context) : null;
}

function resolveLatestVisibleBlockActivity({
  consumedBlockIndexes,
  content,
  context,
  hiddenToolNames,
}: {
  consumedBlockIndexes: ReadonlySet<number>;
  content: readonly ContentBlock[];
  context: ContentActivityContext;
  hiddenToolNames: ReadonlySet<string>;
}): MessageActivityState {
  const block = findLatestVisibleBlock(
    content,
    consumedBlockIndexes,
    hiddenToolNames,
  );
  const fallback = context.fallback ?? "thinking";
  if (!block) {
    return fallback;
  }
  const rule = CONTENT_ACTIVITY_RULES.find(
    (candidate) => candidate.matches(block),
  );
  return rule ? rule.resolve(block, context) : fallback;
}

function findLatestPendingToolUse(
  content: readonly ContentBlock[],
  resolvedToolUseIds: ReadonlySet<string>,
  hiddenToolNames: ReadonlySet<string>,
): ToolUseContent | null {
  return findLastActivityBlock(
    content,
    (block): block is ToolUseContent => block.type === "tool_use"
      && !hiddenToolNames.has(block.name)
      && !resolvedToolUseIds.has(block.id),
  );
}

function findLatestVisibleBlock(
  content: readonly ContentBlock[],
  consumedBlockIndexes: ReadonlySet<number>,
  hiddenToolNames: ReadonlySet<string>,
): ContentBlock | null {
  return findLastActivityBlock(
    content,
    (block, index): block is ContentBlock => !HIDDEN_ACTIVITY_BLOCK_RULES.some(
      (rule) => rule({
        block,
        consumedBlockIndexes,
        hiddenToolNames,
        index,
      }),
    ),
  );
}

function resolveProgressActivityState(
  block: TaskProgressContent,
): MessageActivityState {
  return resolveToolActivityState(block.last_tool_name);
}

function resolveToolUseActivityState(
  block: ToolUseContent,
  context: ContentActivityContext,
): MessageActivityState {
  const facts: ToolUseActivityFacts = {
    fallback: context.fallback ?? "thinking",
    hasPermission: context.pendingPermissionsByToolUseId.has(block.id),
    isQuestion: block.name === ASK_USER_QUESTION_TOOL_NAME,
  };
  const rule = TOOL_USE_ACTIVITY_RULES.find(
    (candidate) => candidate.matches(facts),
  );
  return rule ? rule.resolve(facts) : resolveToolActivityState(block.name);
}

function hasStreamingTextBlock(
  content: readonly ContentBlock[],
  streamingBlockIndexes: ReadonlySet<number>,
): boolean {
  return Array.from(streamingBlockIndexes).some((index) => {
    const block = content[index];
    return block?.type === "text" && Boolean(block.text.trim());
  });
}

function defineContentActivityRule<Type extends ContentBlock["type"]>(
  type: Type,
  resolve: (
    block: Extract<ContentBlock, { type: Type }>,
    context: ContentActivityContext,
  ) => MessageActivityState,
): ContentActivityRule {
  return {
    matches: (block) => block.type === type,
    resolve: (block, context) => resolve(
      block as Extract<ContentBlock, { type: Type }>,
      context,
    ),
  };
}
