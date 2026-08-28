/**
 * INPUT: Assistant 内容模式、streamed ContentBlock、过程尾部与 canonical Room result。
 * OUTPUT: DM 单调正文、正文后独立 WorkGraph 交付物、Room 公区最终回复与 Thread 纯过程投影。
 * POS: MessageItem 最终回复与过程归档的纯投影真相源。
 */
import type {
  AssistantMessage,
  AgentMention,
  Message,
  PublicHandoffReply,
  ResultSummary,
} from "@/types/conversation/message/entity";
import type { ContentBlock } from "@/types/conversation/message/content";
import { isGenerativeUIWidgetToolName } from "@/lib/conversation/generative-ui";
import { extractTextFromContentBlocks } from "../../../message-content-model";
import { normalizePublicHandoffReply } from "../../../public-handoff-reply-model";
import { getResultSummaryDisplayText } from "./message-item-stats";
import {
  projectionFromOrderedEntries,
  resolveAssistantResponseSurface,
  type AssistantContentMode,
  type AssistantTurnEntry,
  type ContentProjection,
  type OrderedAssistantEntry,
} from "../../message-item-projection";

interface FinalProjectionInput {
  assistantContentMode: AssistantContentMode;
  assistantMessages: Message[];
  orderedProjection: ContentProjection;
  resultSummary: ResultSummary | undefined;
  roundId: string;
  /** 本轮 durable user message id；新协议下顶层 assistant 的 parent_id 指向它。 */
  userMessageId?: string | null;
  streamingBlockIndexes: Set<number>;
  visibleAssistantTurns: AssistantTurnEntry[];
  visibleOrderedAssistantEntries: OrderedAssistantEntry[];
}

interface FinalAssistantContentContext {
  fallbackFinalAssistantContent: ContentBlock[] | null;
  finalAssistantTurn: AssistantTurnEntry | null;
  finalTailEntries: OrderedAssistantEntry[];
  generativeUIContent: ContentBlock[];
  resultText: string | null;
}

interface RoomResultFinalAssistantContentInput {
  fallbackFinalAssistantContent: ContentBlock[] | null;
  resultText: string | null;
}

type FinalAssistantContentResolver = (
  context: FinalAssistantContentContext,
) => string | ContentBlock[] | null;

const FINAL_ASSISTANT_CONTENT_RESOLVERS: Readonly<Record<
  AssistantContentMode,
  FinalAssistantContentResolver
>> = {
  dm_archived: resolveArchivedFinalAssistantContent,
  dm_live: resolveArchivedFinalAssistantContent,
  room_result: resolveRoomResultFinalAssistantContentFromContext,
  room_thread: resolveHiddenFinalAssistantContent,
  room_thread_process: resolveHiddenFinalAssistantContent,
};

export function resolveMessageItemFinalProjection({
  assistantContentMode,
  assistantMessages,
  orderedProjection,
  resultSummary,
  roundId,
  userMessageId,
  streamingBlockIndexes,
  visibleAssistantTurns,
  visibleOrderedAssistantEntries,
}: FinalProjectionInput) {
  const finalAssistantTurn = resolveFinalAssistantTurn(
    assistantMessages,
    roundId,
    userMessageId ?? null,
    visibleAssistantTurns,
  );
  const finalTailEntries = resolveFinalTailEntries(
    finalAssistantTurn,
    visibleOrderedAssistantEntries,
  );
  const generativeUIEntries = resolveGenerativeUIEntries(
    visibleOrderedAssistantEntries,
  );
  const finalSurfaceAssistantTurn = resolveFinalSurfaceAssistantTurn({
    assistantContentMode,
    finalAssistantTurn,
    finalTailEntries,
    generativeUIEntries,
  });
  const archivedProcessProjection = buildArchivedProcessProjection({
    finalAssistantTurn: finalSurfaceAssistantTurn,
    finalTailEntries,
    generativeUIEntries,
    streamingBlockIndexes,
    visibleOrderedAssistantEntries,
  });
  const fallbackFinalAssistantContent = resolveFallbackFinalAssistantContent(
    finalSurfaceAssistantTurn,
    finalTailEntries,
    generativeUIEntries,
  );
  const fallbackFinalAssistantStreamingIndexes =
    resolveFallbackFinalAssistantStreamingIndexes(
      finalSurfaceAssistantTurn,
      finalTailEntries,
      generativeUIEntries,
      streamingBlockIndexes,
    );

  const directOrderedProjection = resolveDirectOrderedProjection(
    assistantContentMode,
    orderedProjection,
    archivedProcessProjection,
  );
  const processProjection = resolveProcessProjection(
    assistantContentMode,
    archivedProcessProjection,
  );
  const finalAssistantContent = resolveFinalAssistantContent({
    assistantContentMode,
    fallbackFinalAssistantContent,
    finalAssistantTurn: finalSurfaceAssistantTurn,
    finalTailEntries,
    generativeUIContent: generativeUIEntries.map((entry) => entry.block),
    resultSummary,
  });
  const finalAssistantStreamingIndexes = resolveFinalStreamingIndexes(
    assistantContentMode,
    finalAssistantContent,
    fallbackFinalAssistantStreamingIndexes,
  );
  const finalAssistantText = resolveFinalAssistantText(finalAssistantContent);
  const finalAssistantMentions = resolveFinalAssistantMentions(
    assistantMessages,
    finalSurfaceAssistantTurn?.messageId ?? null,
    countLeadingGenerativeUIEntries(generativeUIEntries),
  );
  const finalAssistantHandoffReply = resolveFinalAssistantHandoffReply(
    assistantMessages,
    finalAssistantTurn?.messageId ?? null,
  );

  return {
    directOrderedProjection,
    processProjection,
    finalAssistantContent,
    finalAssistantStreamingIndexes,
    finalAssistantText,
    finalAssistantMentions,
    finalAssistantHandoffReply,
  };
}

function resolveFinalSurfaceAssistantTurn({
  assistantContentMode,
  finalAssistantTurn,
  finalTailEntries,
  generativeUIEntries,
}: {
  assistantContentMode: AssistantContentMode;
  finalAssistantTurn: AssistantTurnEntry | null;
  finalTailEntries: OrderedAssistantEntry[];
  generativeUIEntries: OrderedAssistantEntry[];
}): AssistantTurnEntry | null {
  if (
    assistantContentMode !== "dm_live"
    || !finalAssistantTurn
    || finalTailEntries.length > 0
  ) {
    return finalAssistantTurn;
  }
  const lastTextIndex = finalAssistantTurn.content.findLastIndex(
    (block) => block.type === "text" && Boolean(block.text.trim()),
  );
  if (lastTextIndex < 0) {
    return finalAssistantTurn;
  }
  const promotedBlocks = new Set(
    generativeUIEntries.map((entry) => entry.block),
  );
  const processContinues = finalAssistantTurn.content
    .slice(lastTextIndex + 1)
    .some((block) => !promotedBlocks.has(block));
  // 普通工具跟在正文后时，这段正文是过程边界，必须留在 direct 时间线。
  return processContinues ? null : finalAssistantTurn;
}

function resolveFinalAssistantHandoffReply(
  assistantMessages: Message[],
  messageId: string | null,
): PublicHandoffReply | null {
  if (!messageId) {
    return null;
  }
  const message = assistantMessages.find(
    (value): value is AssistantMessage =>
      value.role === "assistant" && value.message_id === messageId,
  );
  return normalizePublicHandoffReply(message?.handoff_reply);
}

function resolveFinalAssistantMentions(
  assistantMessages: Message[],
  messageId: string | null,
  contentBlockOffset: number,
): AgentMention[] {
  if (!messageId) {
    return [];
  }
  const message = assistantMessages.find(
    (value): value is AssistantMessage =>
      value.role === "assistant" && value.message_id === messageId,
  );
  if (!message) {
    return [];
  }
  const textBlockIndexes = new Map<number, number>();
  message.content.forEach((block, index) => {
    if (block.type === "text" && block.text.trim()) {
      textBlockIndexes.set(index, textBlockIndexes.size);
    }
  });
  // 最终回复会剥离 thinking 等过程块，mention 必须同步投影到正文块索引。
  return (message.agent_mentions ?? []).flatMap((mention) => {
    const textBlockIndex = textBlockIndexes.get(mention.content_block_index);
    return textBlockIndex == null ? [] : [{
      ...mention,
      content_block_index: textBlockIndex + contentBlockOffset,
    }];
  });
}

function resolveDirectOrderedProjection(
  mode: AssistantContentMode,
  orderedProjection: ContentProjection,
  archivedProcessProjection: ContentProjection,
): ContentProjection {
  if (
    mode === "dm_live"
    || mode === "room_result"
    || mode === "room_thread_process"
  ) {
    // DM/Room Result 的最终回复固定在 final surface；Room Thread 的最终回复固定在主 Feed。
    // 这些 direct surface 只承载思考、工具和系统过程，供同一个工具批次视图折叠。
    return archivedProcessProjection;
  }
  return resolveAssistantResponseSurface(mode) === "direct"
    ? orderedProjection
    : emptyProjection();
}

function resolveProcessProjection(
  mode: AssistantContentMode,
  archivedProcessProjection: ContentProjection,
): ContentProjection {
  return mode === "dm_archived"
    ? archivedProcessProjection
    : emptyProjection();
}

function resolveFinalStreamingIndexes(
  mode: AssistantContentMode,
  content: string | ContentBlock[] | null,
  fallbackStreamingIndexes: Set<number>,
): Set<number> {
  if (
    content == null
    || (Array.isArray(content) && content.length === 0)
    ||
    resolveAssistantResponseSurface(mode) === "direct"
    || typeof content === "string"
  ) {
    return new Set<number>();
  }
  return fallbackStreamingIndexes;
}

function resolveFinalAssistantText(
  content: string | ContentBlock[] | null,
): string {
  return typeof content === "string"
    ? content
    : extractTextFromContentBlocks(content);
}

function resolveFinalAssistantTurn(
  assistantMessages: Message[],
  roundId: string,
  userMessageId: string | null,
  visibleAssistantTurns: AssistantTurnEntry[],
) {
  // 顶层 assistant 的 parent 指向本轮 user message（旧数据指向 round_id）；
  // 其他 parent（tool_use / slot msg）属于子执行，不能当最终回复。
  const isTopLevelParent = (parentId: string | undefined) =>
    !parentId ||
    parentId === roundId ||
    (userMessageId != null && parentId === userMessageId);
  for (let index = assistantMessages.length - 1; index >= 0; index -= 1) {
    const message = assistantMessages[index] as AssistantMessage;
    if (isTopLevelParent(message.parent_id)) {
      return (
        visibleAssistantTurns.find(
          (turn) => turn.messageId === message.message_id,
        ) ?? null
      );
    }
  }
  return visibleAssistantTurns.at(-1) ?? null;
}

function resolveFinalTailEntries(
  finalAssistantTurn: AssistantTurnEntry | null,
  visibleOrderedAssistantEntries: OrderedAssistantEntry[],
) {
  if (!finalAssistantTurn) {
    return [];
  }

  const tailEntries: OrderedAssistantEntry[] = [];
  for (
    let index = visibleOrderedAssistantEntries.length - 1;
    index >= 0;
    index -= 1
  ) {
    const entry = visibleOrderedAssistantEntries[index];
    if (entry.sourceMessageId !== finalAssistantTurn.messageId) {
      break;
    }
    if (entry.block.type !== "text" || !entry.block.text.trim()) {
      break;
    }
    tailEntries.unshift(entry);
  }
  return tailEntries;
}

function buildArchivedProcessProjection({
  finalAssistantTurn,
  finalTailEntries,
  generativeUIEntries,
  streamingBlockIndexes,
  visibleOrderedAssistantEntries,
}: {
  finalAssistantTurn: AssistantTurnEntry | null;
  finalTailEntries: OrderedAssistantEntry[];
  generativeUIEntries: OrderedAssistantEntry[];
  streamingBlockIndexes: Set<number>;
  visibleOrderedAssistantEntries: OrderedAssistantEntry[];
}) {
  const generativeUIIndexes = new Set(
    generativeUIEntries.map((entry) => entry.mergedIndex),
  );
  const processEntries = visibleOrderedAssistantEntries.filter(
    (entry) => !generativeUIIndexes.has(entry.mergedIndex),
  );
  // 最终回复由独立区域渲染（tail / turn 文本 / result 摘要），
  // 生成式 UI 也属于答案本体；过程链无条件剥离，避免重复渲染。
  if (finalTailEntries.length > 0) {
    const tailIndexes = new Set(
      finalTailEntries.map((entry) => entry.mergedIndex),
    );
    return projectionFromOrderedEntries(
      processEntries.filter(
        (entry) => !tailIndexes.has(entry.mergedIndex),
      ),
      streamingBlockIndexes,
    );
  }

  if (finalAssistantTurn && finalAssistantTurn.textContent.length > 0) {
    const finalAssistantTextMergedIndexes = textEntryIndexesForTurn(
      finalAssistantTurn,
      processEntries,
    );
    return projectionFromOrderedEntries(
      processEntries.filter(
        (entry) =>
          entry.sourceMessageId !== finalAssistantTurn.messageId ||
          !finalAssistantTextMergedIndexes.has(entry.mergedIndex),
      ),
      streamingBlockIndexes,
    );
  }

  return projectionFromOrderedEntries(
    processEntries,
    streamingBlockIndexes,
  );
}

function resolveFallbackFinalAssistantContent(
  finalAssistantTurn: AssistantTurnEntry | null,
  finalTailEntries: OrderedAssistantEntry[],
  generativeUIEntries: OrderedAssistantEntry[],
) {
  const generativeUIContent = generativeUIEntries.map((entry) => entry.block);
  const promotedBlocks = new Set(generativeUIContent);
  let fallbackContent: ContentBlock[] | null = null;
  if (finalTailEntries.length > 0) {
    fallbackContent = finalTailEntries.map((entry) => entry.block);
  } else if (finalAssistantTurn?.textContent.length) {
    fallbackContent = finalAssistantTurn.textContent;
  } else if (finalAssistantTurn?.content.length) {
    fallbackContent = finalAssistantTurn.content.filter(
      (block) => !promotedBlocks.has(block),
    );
  }
  return generativeUIContent.length > 0
    ? composeFinalAssistantContent(
        generativeUIContent,
        fallbackContent ?? [],
      )
    : fallbackContent;
}

function resolveFallbackFinalAssistantStreamingIndexes(
  finalAssistantTurn: AssistantTurnEntry | null,
  finalTailEntries: OrderedAssistantEntry[],
  generativeUIEntries: OrderedAssistantEntry[],
  streamingBlockIndexes: Set<number>,
) {
  const nextIndexes = new Set<number>();
  const { leading, trailing } = partitionGenerativeUIEntries(
    generativeUIEntries,
  );
  leading.forEach((entry, index) => {
    if (streamingBlockIndexes.has(entry.mergedIndex)) {
      nextIndexes.add(index);
    }
  });
  const offset = leading.length;
  if (finalTailEntries.length > 0) {
    finalTailEntries.forEach((entry, index) => {
      if (streamingBlockIndexes.has(entry.mergedIndex)) {
        nextIndexes.add(offset + index);
      }
    });
    appendTrailingStreamingIndexes({
      nextIndexes,
      offset: offset + finalTailEntries.length,
      streamingBlockIndexes,
      trailing,
    });
    return nextIndexes;
  }
  if (!finalAssistantTurn) {
    appendTrailingStreamingIndexes({
      nextIndexes,
      offset,
      streamingBlockIndexes,
      trailing,
    });
    return nextIndexes;
  }
  if (finalAssistantTurn.textContent.length > 0) {
    finalAssistantTurn.textStreamingIndexes.forEach(
      (index) => nextIndexes.add(offset + index),
    );
    appendTrailingStreamingIndexes({
      nextIndexes,
      offset: offset + finalAssistantTurn.textContent.length,
      streamingBlockIndexes,
      trailing,
    });
    return nextIndexes;
  }
  appendTrailingStreamingIndexes({
    nextIndexes,
    offset,
    streamingBlockIndexes,
    trailing,
  });
  return generativeUIEntries.length > 0
    ? nextIndexes
    : finalAssistantTurn.streamingIndexes;
}

function resolveFinalAssistantContent({
  assistantContentMode,
  fallbackFinalAssistantContent,
  finalAssistantTurn,
  finalTailEntries,
  generativeUIContent,
  resultSummary,
}: {
  assistantContentMode: AssistantContentMode;
  fallbackFinalAssistantContent: ContentBlock[] | null;
  finalAssistantTurn: AssistantTurnEntry | null;
  finalTailEntries: OrderedAssistantEntry[];
  generativeUIContent: ContentBlock[];
  resultSummary: ResultSummary | undefined;
}) {
  return FINAL_ASSISTANT_CONTENT_RESOLVERS[assistantContentMode]({
    fallbackFinalAssistantContent,
    finalAssistantTurn,
    finalTailEntries,
    generativeUIContent,
    resultText: getResultSummaryDisplayText(resultSummary),
  });
}

function resolveArchivedFinalAssistantContent({
  finalAssistantTurn,
  finalTailEntries,
  generativeUIContent,
  resultText,
}: FinalAssistantContentContext): string | ContentBlock[] | null {
  // 归档回复优先使用已从过程链剥离的正文，result 只补齐缺失正文。
  const narrativeContent = finalTailEntries.length > 0
    ? finalTailEntries.map((entry) => entry.block)
    : finalAssistantTurn?.textContent.length
    ? finalAssistantTurn.textContent
    : null;
  if (generativeUIContent.length > 0) {
    return composeFinalAssistantContent(
      generativeUIContent,
      narrativeContent ?? (resultText
        ? [{ type: "text" as const, text: resultText }]
        : []),
    );
  }
  return narrativeContent ?? resultText ?? null;
}

function resolveRoomResultFinalAssistantContentFromContext({
  finalAssistantTurn,
  finalTailEntries,
  generativeUIContent,
  resultText,
}: FinalAssistantContentContext): ContentBlock[] | null {
  const narrativeContent = finalTailEntries.length > 0
    ? finalTailEntries.map((entry) => entry.block)
    : finalAssistantTurn?.textContent.length
    ? finalAssistantTurn.textContent
    : null;
  const fallbackFinalAssistantContent = generativeUIContent.length > 0
    ? composeFinalAssistantContent(
        generativeUIContent,
        narrativeContent ?? [],
      )
    : narrativeContent;
  return resolveRoomResultFinalAssistantContent({
    fallbackFinalAssistantContent,
    resultText,
  });
}

function resolveGenerativeUIEntries(
  entries: OrderedAssistantEntry[],
): OrderedAssistantEntry[] {
  const toolUseIds = new Set(
    entries.flatMap(({ block }) => (
      block.type === "tool_use" && isGenerativeUIWidgetToolName(block.name)
        ? [block.id]
        : []
    )),
  );
  const generativeUIEntries = entries.filter(({ block }) => (
    (block.type === "tool_use" && toolUseIds.has(block.id))
    || (block.type === "tool_result" && toolUseIds.has(block.tool_use_id))
  ));
  // 同一轮可能先 get 再 revise；最终回复只展示最后一张完整图快照，
  // 其余命令仍留在过程区，避免对话里连续堆叠旧版本卡片。
  const latestWorkGraphArtifact = entries.findLast(
    ({ block }) => block.type === "workgraph_artifact",
  );
  return latestWorkGraphArtifact
    ? [...generativeUIEntries, latestWorkGraphArtifact].sort(
        (left, right) => left.mergedIndex - right.mergedIndex,
      )
    : generativeUIEntries;
}

function countLeadingGenerativeUIEntries(
  entries: OrderedAssistantEntry[],
): number {
  return partitionGenerativeUIEntries(entries).leading.length;
}

function composeFinalAssistantContent(
  promotedContent: ContentBlock[],
  narrativeContent: ContentBlock[],
): ContentBlock[] {
  const leading = promotedContent.filter(
    (block) => block.type !== "workgraph_artifact",
  );
  const trailing = promotedContent.filter(
    (block) => block.type === "workgraph_artifact",
  );
  // 可交互 Widget 沿用回复前置语义；完整工作图是交付物，固定放在正文后。
  // 这样 Thought/Tool 折叠区、最终叙述和工作图卡片各自拥有稳定层级。
  return [...leading, ...narrativeContent, ...trailing];
}

function partitionGenerativeUIEntries(
  entries: OrderedAssistantEntry[],
): {
  leading: OrderedAssistantEntry[];
  trailing: OrderedAssistantEntry[];
} {
  return {
    leading: entries.filter(
      ({ block }) => block.type !== "workgraph_artifact",
    ),
    trailing: entries.filter(
      ({ block }) => block.type === "workgraph_artifact",
    ),
  };
}

function appendTrailingStreamingIndexes({
  nextIndexes,
  offset,
  streamingBlockIndexes,
  trailing,
}: {
  nextIndexes: Set<number>;
  offset: number;
  streamingBlockIndexes: Set<number>;
  trailing: OrderedAssistantEntry[];
}) {
  trailing.forEach((entry, index) => {
    if (streamingBlockIndexes.has(entry.mergedIndex)) {
      nextIndexes.add(offset + index);
    }
  });
}

export function resolveRoomResultFinalAssistantContent({
  fallbackFinalAssistantContent,
  resultText,
}: RoomResultFinalAssistantContentInput): ContentBlock[] | null {
  const visibleFallbackContent = fallbackFinalAssistantContent?.some(
    (block) => block.type === "thinking",
  )
    ? fallbackFinalAssistantContent.filter(
        (block) => block.type !== "thinking",
      )
    : fallbackFinalAssistantContent;
  const canonicalText = resultText?.trim() ?? "";
  if (!canonicalText) {
    return visibleFallbackContent?.length ? visibleFallbackContent : null;
  }
  if (!visibleFallbackContent?.length) {
    return [{ type: "text", text: canonicalText }];
  }

  const fallbackText = extractTextFromContentBlocks(
    visibleFallbackContent,
  );
  if (
    fallbackText === canonicalText
    || fallbackText.startsWith(canonicalText)
  ) {
    // result 摘要相同或更短时保留已经显示的 ContentBlock 身份，禁止终态回缩。
    return visibleFallbackContent;
  }
  if (fallbackText && canonicalText.startsWith(fallbackText)) {
    const lastTextIndex = visibleFallbackContent.findLastIndex(
      (block) =>
        block.type === "text"
        && Boolean(extractTextFromContentBlocks([block])),
    );
    if (lastTextIndex >= 0) {
      const suffix = canonicalText.slice(fallbackText.length);
      return visibleFallbackContent.map((block, index) => (
        index === lastTextIndex && block.type === "text"
          ? {
              ...block,
              // 比较边界来自去空白后的可见文本；先移除原始块尾部空白，
              // 再接 canonical suffix，避免空格或换行被重复一次。
              text: block.text.replace(/\s+$/u, "") + suffix,
            }
          : block
      ));
    }
  }

  // result 确实修正正文时只重建文本槽位，过程、附件等非文本块完整保留。
  return replaceFallbackTextPreservingNonText(
    visibleFallbackContent,
    canonicalText,
  );
}

function replaceFallbackTextPreservingNonText(
  fallbackContent: ContentBlock[],
  canonicalText: string,
): ContentBlock[] {
  const lastTextIndex = fallbackContent.findLastIndex(
    (block) => block.type === "text",
  );
  if (lastTextIndex < 0) {
    // 没有正文槽位时，终态答案位于既有过程/附件之后。
    return [...fallbackContent, { type: "text", text: canonicalText }];
  }

  const nextContent: ContentBlock[] = [];
  fallbackContent.forEach((block, index) => {
    if (block.type !== "text") {
      nextContent.push(block);
      return;
    }
    if (index === lastTextIndex) {
      nextContent.push({ ...block, text: canonicalText });
    }
  });
  return nextContent;
}

function resolveHiddenFinalAssistantContent(): null {
  return null;
}

function textEntryIndexesForTurn(
  finalAssistantTurn: AssistantTurnEntry,
  visibleOrderedAssistantEntries: OrderedAssistantEntry[],
) {
  const nextIndexes = new Set<number>();
  for (const entry of visibleOrderedAssistantEntries) {
    if (entry.sourceMessageId !== finalAssistantTurn.messageId) {
      continue;
    }
    if (entry.block.type !== "text" || !entry.block.text.trim()) {
      continue;
    }
    nextIndexes.add(entry.mergedIndex);
  }
  return nextIndexes;
}

function emptyProjection(): ContentProjection {
  return { content: [], streamingIndexes: new Set<number>() };
}
