/**
 * INPUT: 结构化内容块、流式身份与 tool 关联状态。
 * OUTPUT: 内容块消费投影、工具状态及稳定挂载判定。
 * POS: 内容块视图的纯模型层，不持有 React 身份或消息时间线状态。
 */
import type {
  ContentBlock,
  TaskProgressContent,
  ToolResultContent,
  ToolUseContent,
} from "@/types/conversation/message/content";

import type { ToolBlockStatus } from "../../../blocks/tool/tool-block-types";
import {
  isRejectedToolResult,
  isSupersededToolResult,
} from "../../../tool-result-semantic-model";

export interface ToolUseProjection {
  index: number;
  result?: ToolResultContent;
  use: ToolUseContent;
}

export interface StructuredContentProjection {
  consumedBlockIndexes: ReadonlySet<number>;
  resolvedToolUseIds: ReadonlySet<string>;
  taskProgressByToolUseId: ReadonlyMap<string, TaskProgressContent>;
  toolUseById: ReadonlyMap<string, ToolUseProjection>;
}

export function projectStructuredContent(
  content: ContentBlock[],
): StructuredContentProjection {
  const toolUseById = new Map<string, ToolUseProjection>();
  const taskProgressByToolUseId = new Map<string, TaskProgressContent>();

  content.forEach((block, index) => {
    if (block.type === "tool_use") {
      toolUseById.set(block.id, { index, use: block });
    }
    if (block.type === "task_progress" && block.tool_use_id) {
      taskProgressByToolUseId.set(block.tool_use_id, block);
    }
  });

  const consumedBlockIndexes = new Set<number>();
  const resolvedToolUseIds = new Set<string>();
  content.forEach((block, index) => {
    if (block.type !== "tool_result") {
      return;
    }
    const toolUse = toolUseById.get(block.tool_use_id);
    if (!toolUse) {
      return;
    }
    toolUse.result = block;
    resolvedToolUseIds.add(block.tool_use_id);
    consumedBlockIndexes.add(index);
  });

  return {
    consumedBlockIndexes,
    resolvedToolUseIds,
    taskProgressByToolUseId,
    toolUseById,
  };
}

export function shouldMountTextContentBlock(
  content: string,
  streaming: boolean,
): boolean {
  // live 空块先建立 MarkdownRenderer 身份，首个非空快照才会进入已有 hook 的 backlog；
  // 历史或恢复消息首挂已有正文，仍由 hook 直接呈现，不会重播。
  return streaming || Boolean(content.trim());
}

export function resolveToolBlockStatus(
  toolUse: ToolUseProjection | undefined,
  waitingForPermission: boolean,
  unresolvedToolStatus?: Extract<ToolBlockStatus, "error" | "stopped">,
): ToolBlockStatus {
  if (toolUse?.result) {
    if (toolUse.result.is_error) {
      return "error";
    }
    if (isRejectedToolResult(toolUse.result)) {
      return "rejected";
    }
    return isSupersededToolResult(toolUse.result)
      ? "superseded"
      : "success";
  }
  if (unresolvedToolStatus) {
    return unresolvedToolStatus;
  }
  return waitingForPermission ? "waiting_permission" : "running";
}
