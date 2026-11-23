/**
 * INPUT: 已投影的结构化内容、工具完成态与消息级活动 fallback。
 * OUTPUT: 仅在没有可见叶子块持有运行状态时返回消息级尾随活动。
 * POS: ContentRenderer 的状态所有权边界；ToolBlock 自己展示状态时不得再叠加 Agent 活动。Thought 只是内容，思考态始终由活动行表达。
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
  const latestIndex = findLatestVisibleBlockIndex(
    content,
    consumedBlockIndexes,
    hiddenToolNames,
  );
  return resolveLatestVisibleBlockActivity({
    block: content[latestIndex] ?? null,
    context,
    resolvedToolUseIds,
  });
}

function resolveLatestVisibleBlockActivity({
  block,
  context,
  resolvedToolUseIds,
}: {
  block: ContentBlock | null;
  context: ContentActivityContext;
  resolvedToolUseIds: ReadonlySet<string>;
}): MessageActivityState {
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

function findLatestVisibleBlockIndex(
  content: readonly ContentBlock[],
  consumedBlockIndexes: ReadonlySet<number>,
  hiddenToolNames: ReadonlySet<string>,
): number {
  for (let index = content.length - 1; index >= 0; index -= 1) {
    const block = content[index];
    if (block && !HIDDEN_ACTIVITY_BLOCK_RULES.some((rule) => rule({
      block,
      consumedBlockIndexes,
      hiddenToolNames,
      index,
    }))) {
      return index;
    }
  }
  return -1;
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
