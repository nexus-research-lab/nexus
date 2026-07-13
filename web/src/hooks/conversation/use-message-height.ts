import { prepare, layout } from "@chenglou/pretext";
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

interface MutableRoundHeightMetrics {
  textParts: string[];
  toolBlockCount: number;
}

interface RoundHeightMetrics {
  text: string;
  toolBlockCount: number;
}

type ContentBlockMetricCollectorMap = {
  [Type in ContentBlockType]: (
    block: ContentBlockOf<Type>,
    metrics: MutableRoundHeightMetrics,
  ) => void;
};
type MessageMetricCollectorMap = {
  [Role in MessageRole]: (
    message: MessageOf<Role>,
    metrics: MutableRoundHeightMetrics,
  ) => void;
};

// 与 Markdown 正文的字号和行高保持一致，避免虚拟列表初始估高跳动。
const PROSE_FONT = "400 14px ui-sans-serif, system-ui, sans-serif";
const PROSE_LINE_HEIGHT = 28;

// 每轮固定结构：用户头部、Agent 头部、内边距和分隔线。
const ROUND_CHROME_HEIGHT = 96;

const BLOCK_PADDING = 16;

const CODE_LINE_HEIGHT = 22;
const CODE_BLOCK_MIN_HEIGHT = 80;

const TOOL_BLOCK_HEIGHT = 60;

const ignoreHeightMetrics = () => undefined;

const CONTENT_BLOCK_METRIC_COLLECTORS = {
  image: ignoreHeightMetrics,
  system_event: ignoreHeightMetrics,
  task_progress: (block, metrics) => {
    metrics.textParts.push(block.description);
  },
  text: (block, metrics) => {
    metrics.textParts.push(block.text);
  },
  thinking: ignoreHeightMetrics,
  tool_result: ignoreHeightMetrics,
  tool_use: (_block, metrics) => {
    metrics.toolBlockCount += 1;
  },
  tool_use_error: ignoreHeightMetrics,
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
  const metrics: MutableRoundHeightMetrics = {
    textParts: [],
    toolBlockCount: 0,
  };
  for (const message of messages) {
    const collectMetrics = MESSAGE_METRIC_COLLECTORS[message.role] as (
      value: Message,
      target: MutableRoundHeightMetrics,
    ) => void;
    collectMetrics(message, metrics);
  }
  return {
    text: metrics.textParts.join("\n"),
    toolBlockCount: metrics.toolBlockCount,
  };
}

function collectUserMessageMetrics(
  message: MessageOf<"user">,
  metrics: MutableRoundHeightMetrics,
): void {
  if (!isAutomationTriggerUserMessage(message)) {
    metrics.textParts.push(message.content);
  }
}

function collectAssistantMessageMetrics(
  message: MessageOf<"assistant">,
  metrics: MutableRoundHeightMetrics,
): void {
  for (const block of message.content) {
    const collectMetrics = CONTENT_BLOCK_METRIC_COLLECTORS[block.type] as (
      value: ContentBlock,
      target: MutableRoundHeightMetrics,
    ) => void;
    collectMetrics(block, metrics);
  }
}

function estimateCodeBlockHeight(text: string): number {
  const blocks = text.match(/```[\s\S]*?```/g) ?? [];
  return blocks.reduce((sum, block) => {
    const lines = block.split("\n").length;
    return sum + Math.max(CODE_BLOCK_MIN_HEIGHT, lines * CODE_LINE_HEIGHT);
  }, 0);
}

/**
 * 批量估算轮次高度，共享 pretext 缓存并避免逐项触发 DOM 测量。
 */
export function estimateRoundHeights(
  roundIds: string[],
  messageGroups: Map<string, Message[]>,
  containerWidth: number,
): Map<string, number> {
  const result = new Map<string, number>();

  if (containerWidth <= 0) {
    roundIds.forEach((id) => result.set(id, 200));
    return result;
  }

  for (const id of roundIds) {
    const messages = messageGroups.get(id) ?? [];
    const metrics = projectRoundHeightMetrics(messages);
    const codeBlockHeight = estimateCodeBlockHeight(metrics.text);
    const proseText = metrics.text.replace(/```[\s\S]*?```/g, "");
    const proseHeight = estimateTextHeight(proseText, containerWidth);

    const height = Math.max(
      80,
      ROUND_CHROME_HEIGHT +
        proseHeight +
        codeBlockHeight +
        metrics.toolBlockCount * TOOL_BLOCK_HEIGHT,
    );
    result.set(id, height);
  }

  return result;
}
