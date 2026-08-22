/**
 * INPUT: 已投影的结构化内容、工具完成态与消息级活动 fallback。
 * OUTPUT: 仅在没有可见叶子块持有运行状态时返回消息级尾随活动。
 * POS: ContentRenderer 的状态所有权边界；ToolBlock 自己展示状态时不得再叠加 Agent 活动。
 */
import type {
  ContentBlock,
  TaskProgressContent,
  ToolUseContent,
} from "@/types/conversation/message/content";

import {
  type MessageActivityState,
  resolveToolActivityState,
} from "./message-activity-state";
import { findLastActivityBlock } from "./message-activity-blocks";
import { isRecoverableToolUse } from "../../message-content-model";

interface ContentActivityContext {
  fallback: MessageActivityState | null;
  hasStreamingText: boolean;
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

const HIDDEN_ACTIVITY_BLOCK_RULES: ReadonlyArray<
  (context: ActivityBlockVisibilityContext) => boolean
> = [
  ({ consumedBlockIndexes, index }) => consumedBlockIndexes.has(index),
  ({ block, hiddenToolNames }) => block.type === "tool_use"
    && (hiddenToolNames.has(block.name) || isRecoverableToolUse(block)),
  ({ block }) => block.type === "text" && !block.text.trim(),
  ({ block }) => block.type === "thinking" && !block.thinking.trim(),
];

const CONTENT_ACTIVITY_RULES: ContentActivityRule[] = [
  defineContentActivityRule(
    "task_progress",
    (block) => resolveProgressActivityState(block),
  ),
  defineContentActivityRule("thinking", () => "thinking"),
  defineContentActivityRule(
    "text",
    (_block, context) => context.hasStreamingText
      ? "replying"
      : context.fallback ?? "replying",
  ),
  defineContentActivityRule(
    "workgraph_artifact",
    (_block, context) => context.fallback ?? "executing",
  ),
  defineContentActivityRule(
    "workspace_file_artifact",
    (_block, context) => context.fallback ?? "executing",
  ),
];

const EMPTY_STREAMING_BLOCK_INDEXES = new Set<number>();

export function resolveContentActivityState({
  consumedBlockIndexes,
  content,
  fallbackActivityState = null,
  hiddenToolNames,
  resolvedToolUseIds,
  streamingBlockIndexes = EMPTY_STREAMING_BLOCK_INDEXES,
}: {
  consumedBlockIndexes: ReadonlySet<number>;
  content: readonly ContentBlock[];
  fallbackActivityState?: MessageActivityState | null;
  hiddenToolNames: ReadonlySet<string>;
  resolvedToolUseIds: ReadonlySet<string>;
  streamingBlockIndexes?: ReadonlySet<number>;
}): MessageActivityState | null {
  const context: ContentActivityContext = {
    fallback: fallbackActivityState,
    hasStreamingText: hasStreamingTextBlock(content, streamingBlockIndexes),
  };
  if (findLatestPendingToolUse(content, resolvedToolUseIds, hiddenToolNames)) {
    // 可见 ToolBlock 已展示 running / waiting；消息层不得再叠加同义状态。
    return null;
  }
  return resolveLatestVisibleBlockActivity({
    consumedBlockIndexes,
    content,
    context,
    hiddenToolNames,
    resolvedToolUseIds,
  });
}

function resolveLatestVisibleBlockActivity({
  consumedBlockIndexes,
  content,
  context,
  hiddenToolNames,
  resolvedToolUseIds,
}: {
  consumedBlockIndexes: ReadonlySet<number>;
  content: readonly ContentBlock[];
  context: ContentActivityContext;
  hiddenToolNames: ReadonlySet<string>;
  resolvedToolUseIds: ReadonlySet<string>;
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
  if (block.type === "tool_use") {
    // 已完成工具后仍在流式表示 Agent 已回到下一步推理；未完成工具的
    // 状态由上面的 ToolBlock 自己持有。
    return resolvedToolUseIds.has(block.id) ? "thinking" : fallback;
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
      && !isRecoverableToolUse(block)
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
