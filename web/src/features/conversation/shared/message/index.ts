/**
 * 消息组件统一导出
 */

// 统一消息组件 - 主要推荐使用
export { default as MessageItem } from "./item/message-item";

// 专用消息组件
export { ToolBlock } from "./blocks/tool-block";

// 类型定义
export type {
  Message,
  MessageRole,
  ContentBlock,
  TextContent,
  ToolUseContent,
  ToolResultContent,
  ThinkingContent,
  UserMessage,
  AssistantMessage,
  SystemMessage,
  ResultMessage,
} from "@/types/conversation/message";
