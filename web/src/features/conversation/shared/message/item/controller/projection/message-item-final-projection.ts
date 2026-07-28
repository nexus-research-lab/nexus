/**
 * INPUT: Assistant 内容模式、streamed ContentBlock、过程尾部与 canonical Room result。
 * OUTPUT: DM live/terminal 同 surface 的单调正文，以及保留非文本块身份的 Room 最终回复投影。
 * POS: MessageItem 最终回复与过程归档的纯投影真相源。
 */
import type {
  AssistantMessage,
  AgentMention,
  Message,
  ResultSummary,
} from "@/types/conversation/message/entity";
import type { ContentBlock } from "@/types/conversation/message/content";
import { extractTextFromContentBlocks } from "../../../message-content-model";
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
  room_result: resolveRoomResultFinalAssistantContent,
  room_thread: resolveHiddenFinalAssistantContent,
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
  const archivedProcessProjection = buildArchivedProcessProjection({
    finalAssistantTurn,
    finalTailEntries,
    streamingBlockIndexes,
    visibleOrderedAssistantEntries,
  });
  const fallbackFinalAssistantContent = resolveFallbackFinalAssistantContent(
    finalAssistantTurn,
    finalTailEntries,
  );
  const fallbackFinalAssistantStreamingIndexes =
    resolveFallbackFinalAssistantStreamingIndexes(
      finalAssistantTurn,
      finalTailEntries,
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
    finalAssistantTurn,
    finalTailEntries,
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
		finalAssistantTurn?.messageId ?? null,
	);

  return {
    directOrderedProjection,
    processProjection,
    finalAssistantContent,
    finalAssistantStreamingIndexes,
    finalAssistantText,
    finalAssistantMentions,
  };
}

function resolveFinalAssistantMentions(
	assistantMessages: Message[],
	messageId: string | null,
): AgentMention[] {
	if (!messageId) {
		return [];
	}
	const message = assistantMessages.find(
		(value): value is AssistantMessage =>
			value.role === "assistant" && value.message_id === messageId,
	);
	return message?.agent_mentions ?? [];
}

function resolveDirectOrderedProjection(
  mode: AssistantContentMode,
  orderedProjection: ContentProjection,
  archivedProcessProjection: ContentProjection,
): ContentProjection {
  if (mode === "dm_live") {
    // DM 的最终回复从首个流式字开始固定在 final surface；
    // direct 只承载会在终态折叠归档的思考、工具和系统过程。
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
  streamingBlockIndexes,
  visibleOrderedAssistantEntries,
}: {
  finalAssistantTurn: AssistantTurnEntry | null;
  finalTailEntries: OrderedAssistantEntry[];
  streamingBlockIndexes: Set<number>;
  visibleOrderedAssistantEntries: OrderedAssistantEntry[];
}) {
  // 最终回复由独立区域渲染（tail / turn 文本 / result 摘要），
  // 过程链无条件剥离它，避免同一段答案在过程和最终各出现一次。
  if (finalTailEntries.length > 0) {
    const tailIndexes = new Set(
      finalTailEntries.map((entry) => entry.mergedIndex),
    );
    return projectionFromOrderedEntries(
      visibleOrderedAssistantEntries.filter(
        (entry) => !tailIndexes.has(entry.mergedIndex),
      ),
      streamingBlockIndexes,
    );
  }

  if (finalAssistantTurn && finalAssistantTurn.textContent.length > 0) {
    const finalAssistantTextMergedIndexes = textEntryIndexesForTurn(
      finalAssistantTurn,
      visibleOrderedAssistantEntries,
    );
    return projectionFromOrderedEntries(
      visibleOrderedAssistantEntries.filter(
        (entry) =>
          entry.sourceMessageId !== finalAssistantTurn.messageId ||
          !finalAssistantTextMergedIndexes.has(entry.mergedIndex),
      ),
      streamingBlockIndexes,
    );
  }

  return projectionFromOrderedEntries(
    visibleOrderedAssistantEntries,
    streamingBlockIndexes,
  );
}

function resolveFallbackFinalAssistantContent(
  finalAssistantTurn: AssistantTurnEntry | null,
  finalTailEntries: OrderedAssistantEntry[],
) {
  if (finalTailEntries.length > 0) {
    return finalTailEntries.map((entry) => entry.block);
  }
  if (!finalAssistantTurn) {
    return null;
  }
  if (finalAssistantTurn.textContent.length > 0) {
    return finalAssistantTurn.textContent;
  }
  if (finalAssistantTurn.content.length > 0) {
    return finalAssistantTurn.content;
  }
  return null;
}

function resolveFallbackFinalAssistantStreamingIndexes(
  finalAssistantTurn: AssistantTurnEntry | null,
  finalTailEntries: OrderedAssistantEntry[],
  streamingBlockIndexes: Set<number>,
) {
  if (finalTailEntries.length > 0) {
    const nextIndexes = new Set<number>();
    finalTailEntries.forEach((entry, index) => {
      if (streamingBlockIndexes.has(entry.mergedIndex)) {
        nextIndexes.add(index);
      }
    });
    return nextIndexes;
  }
  if (!finalAssistantTurn) {
    return new Set<number>();
  }
  if (finalAssistantTurn.textContent.length > 0) {
    return finalAssistantTurn.textStreamingIndexes;
  }
  return finalAssistantTurn.streamingIndexes;
}

function resolveFinalAssistantContent({
  assistantContentMode,
  fallbackFinalAssistantContent,
  finalAssistantTurn,
  finalTailEntries,
  resultSummary,
}: {
  assistantContentMode: AssistantContentMode;
  fallbackFinalAssistantContent: ContentBlock[] | null;
  finalAssistantTurn: AssistantTurnEntry | null;
  finalTailEntries: OrderedAssistantEntry[];
  resultSummary: ResultSummary | undefined;
}) {
  return FINAL_ASSISTANT_CONTENT_RESOLVERS[assistantContentMode]({
    fallbackFinalAssistantContent,
    finalAssistantTurn,
    finalTailEntries,
    resultText: getResultSummaryDisplayText(resultSummary),
  });
}

function resolveArchivedFinalAssistantContent({
  finalAssistantTurn,
  finalTailEntries,
  resultText,
}: FinalAssistantContentContext): string | ContentBlock[] | null {
  // 归档回复优先使用已从过程链剥离的正文，result 只补齐缺失正文。
  if (finalTailEntries.length > 0) {
    return finalTailEntries.map((entry) => entry.block);
  }
  if (finalAssistantTurn?.textContent.length) {
    return finalAssistantTurn.textContent;
  }
  return resultText || null;
}

export function resolveRoomResultFinalAssistantContent({
  fallbackFinalAssistantContent,
  resultText,
}: RoomResultFinalAssistantContentInput): ContentBlock[] | null {
  const canonicalText = resultText?.trim() ?? "";
  if (!canonicalText) {
    return fallbackFinalAssistantContent;
  }
  if (!fallbackFinalAssistantContent?.length) {
    return [{ type: "text", text: canonicalText }];
  }

  const fallbackText = extractTextFromContentBlocks(
    fallbackFinalAssistantContent,
  );
  if (
    fallbackText === canonicalText
    || fallbackText.startsWith(canonicalText)
  ) {
    // result 摘要相同或更短时保留已经显示的 ContentBlock 身份，禁止终态回缩。
    return fallbackFinalAssistantContent;
  }
  if (fallbackText && canonicalText.startsWith(fallbackText)) {
    const lastTextIndex = fallbackFinalAssistantContent.findLastIndex(
      (block) =>
        block.type === "text"
        && Boolean(extractTextFromContentBlocks([block])),
    );
    if (lastTextIndex >= 0) {
      const suffix = canonicalText.slice(fallbackText.length);
      return fallbackFinalAssistantContent.map((block, index) => (
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
    fallbackFinalAssistantContent,
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
