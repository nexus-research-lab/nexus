/**
 * INPUT: 虚拟列表当前轮次的消息、活动状态和列宽。
 * OUTPUT: 供虚拟列表使用的各轮次初始高度。
 * POS: DM 与群聊共用的轮次估高逻辑。
 */
import { prepare, layout } from "@chenglou/pretext";
import { useMemo } from "react";

import { isGenerativeUIWidgetToolName } from "@/lib/conversation/generative-ui";
import { isAutomationTriggerUserMessage } from "@/types/conversation/automation-message";
import type { ContentBlock } from "@/types/conversation/message/content";
import type { Message } from "@/types/conversation/message/entity";

type ContentBlockType = ContentBlock["type"];
type ContentBlockOf<Type extends ContentBlockType> = Extract<
  ContentBlock,
  { type: Type }
>;
type MessageRole = Message["role"];
type MessageOf<Role extends MessageRole> = Extract<Message, { role: Role }>;

interface RoundHeightMetrics {
  textParts: string[];
  toolBlockCount: number;
}

interface RoundTextEstimate {
  parts: string[];
  height: number;
}

type ContentBlockMetricCollectorMap = {
  [Type in ContentBlockType]: (
    block: ContentBlockOf<Type>,
    metrics: RoundHeightMetrics,
  ) => void;
};
type MessageMetricCollectorMap = {
  [Role in MessageRole]: (
    message: MessageOf<Role>,
    metrics: RoundHeightMetrics,
  ) => void;
};

// 与默认聊天排版（14px / 1.7）保持一致，避免虚拟列表初始估高跳动。
const PROSE_FONT =
  '400 14px -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif';
const PROSE_LINE_HEIGHT = 24;

// 每轮固定结构：用户头部、Agent 头部、内边距和分隔线。
const ROUND_CHROME_HEIGHT = 96;

const BLOCK_PADDING = 16;

const CODE_LINE_HEIGHT = 22;
const CODE_BLOCK_MIN_HEIGHT = 80;

const TOOL_BLOCK_HEIGHT = 60;

// live 正文的 backend target 可能远快于 Markdown reveal；此时只给未测量节点
// 一个保守起点，让真实 DOM 以正向 delta 驱动增长，不能提前预支整段目标高度。
export const ACTIVE_ROUND_ESTIMATED_HEIGHT = 96;

const ignoreHeightMetrics = () => undefined;

const CONTENT_BLOCK_METRIC_COLLECTORS = {
  document: ignoreHeightMetrics,
  image: ignoreHeightMetrics,
  progress_update: ignoreHeightMetrics,
  redacted_thinking: ignoreHeightMetrics,
  resource_link: ignoreHeightMetrics,
  search_result: ignoreHeightMetrics,
  system_event: ignoreHeightMetrics,
  task_progress: (block, metrics) => {
    metrics.textParts.push(block.description);
  },
  text: (block, metrics) => {
    metrics.textParts.push(block.text);
  },
  thinking: ignoreHeightMetrics,
  tool_result: ignoreHeightMetrics,
  tool_use: (block, metrics) => {
    metrics.toolBlockCount += isGenerativeUIWidgetToolName(block.name) ? 6 : 1;
  },
  tool_use_error: ignoreHeightMetrics,
  unsupported: ignoreHeightMetrics,
  workgraph_artifact: (_block, metrics) => {
    // 草图卡片约等于六个紧凑工具行；先给虚拟列表合理估高，
    // 实际 DOM ResizeObserver 再校正，避免 FOLLOW 锚点在卡片出现时下跳。
    metrics.toolBlockCount += 6;
  },
  workspace_file_artifact: ignoreHeightMetrics,
} satisfies ContentBlockMetricCollectorMap;

const MESSAGE_METRIC_COLLECTORS = {
  assistant: collectAssistantMessageMetrics,
  system: ignoreHeightMetrics,
  user: collectUserMessageMetrics,
} satisfies MessageMetricCollectorMap;

function estimateTextHeight(text: string, containerWidth: number): number {
  if (!text.trim()) return 0;
  try {
    const prepared = prepare(text, PROSE_FONT);
    const result = layout(prepared, containerWidth, PROSE_LINE_HEIGHT);
    return result.height + BLOCK_PADDING;
  } catch {
    // 字体测量失败时按平均字符宽度估算，保证虚拟列表仍可工作。
    const charsPerLine = Math.max(1, Math.floor(containerWidth / 8.4));
    const lines = Math.ceil(text.length / charsPerLine);
    return lines * PROSE_LINE_HEIGHT + BLOCK_PADDING;
  }
}

function projectRoundHeightMetrics(messages: Message[]): RoundHeightMetrics {
  const metrics: RoundHeightMetrics = {
    textParts: [],
    toolBlockCount: 0,
  };
  for (const message of messages) {
    const collectMetrics = MESSAGE_METRIC_COLLECTORS[message.role] as (
      value: Message,
      target: RoundHeightMetrics,
    ) => void;
    collectMetrics(message, metrics);
  }
  return metrics;
}

function collectUserMessageMetrics(
  message: MessageOf<"user">,
  metrics: RoundHeightMetrics,
): void {
  if (!isAutomationTriggerUserMessage(message)) {
    metrics.textParts.push(message.content);
  }
}

function collectAssistantMessageMetrics(
  message: MessageOf<"assistant">,
  metrics: RoundHeightMetrics,
): void {
  for (const block of message.content) {
    const collectMetrics = CONTENT_BLOCK_METRIC_COLLECTORS[block.type] as ((
      value: ContentBlock,
      target: RoundHeightMetrics,
    ) => void) | undefined;
    collectMetrics?.(block, metrics);
  }
}

function estimateCodeBlockHeight(text: string): number {
  const blocks = text.match(/```[\s\S]*?```/g) ?? [];
  return blocks.reduce((sum, block) => {
    const lines = block.split("\n").length;
    return sum + Math.max(CODE_BLOCK_MIN_HEIGHT, lines * CODE_LINE_HEIGHT);
  }, 0);
}

function estimateRoundText(
  parts: string[],
  containerWidth: number,
  previous: RoundTextEstimate | undefined,
): RoundTextEstimate {
  if (
    previous && previous.parts.length === parts.length
    && parts.every((part, index) => part === previous.parts[index])
  ) {
    return previous;
  }
  const text = parts.join("\n");
  const prose = text.replace(/```[\s\S]*?```/g, "");
  return {
    parts,
    height: estimateCodeBlockHeight(text) + estimateTextHeight(prose, containerWidth),
  };
}

function createRoundHeightEstimator(containerWidth: number) {
  // 渲染期写入闭包是有意为之：缓存只按正文片段逐段相等命中，被丢弃的并发渲染
  // 或 React 丢弃 memo 只会降低命中率，不会返回过期高度。不要在此缓存其他状态。
  let previous = new Map<string, RoundTextEstimate>();
  return (
    roundIds: string[],
    messageGroups: Map<string, Message[]>,
    activeRoundIds: ReadonlySet<string>,
  ): Map<string, number> => {
    if (containerWidth <= 0) return new Map(roundIds.map((id) => [id, 200]));
    const heights = new Map<string, number>();
    const next = new Map<string, RoundTextEstimate>();
    for (const id of roundIds) {
      if (activeRoundIds.has(id)) {
        heights.set(id, ACTIVE_ROUND_ESTIMATED_HEIGHT);
        continue;
      }
      const { textParts, toolBlockCount } = projectRoundHeightMetrics(
        messageGroups.get(id) ?? [],
      );
      let height = ROUND_CHROME_HEIGHT + toolBlockCount * TOOL_BLOCK_HEIGHT;
      if (textParts.length > 0) {
        const estimate = estimateRoundText(textParts, containerWidth, previous.get(id));
        next.set(id, estimate);
        height += estimate.height;
      }
      heights.set(id, Math.max(80, height));
    }
    // 只保留当前 Feed 的终态轮次，历史窗口淘汰的正文片段可随即释放。
    previous = next;
    return heights;
  };
}

export function useRoundHeights(
  roundIds: string[],
  messageGroups: Map<string, Message[]>,
  containerWidth: number,
  activeRoundIds: ReadonlySet<string>,
): Map<string, number> {
  const estimate = useMemo(
    () => createRoundHeightEstimator(containerWidth),
    [containerWidth],
  );
  return useMemo(
    () => estimate(roundIds, messageGroups, activeRoundIds),
    [estimate, roundIds, messageGroups, activeRoundIds],
  );
}
