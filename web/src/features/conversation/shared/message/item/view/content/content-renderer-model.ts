import type {
  ContentBlock,
  SystemEventContent,
  TaskProgressContent,
  ToolResultContent,
  ToolUseContent,
} from "@/types/conversation/message";

import type { ToolBlockStatus } from "../../../blocks/tool/tool-block-types";

const API_RETRY_VISIBLE_ATTEMPT = 4;

export interface ToolUseProjection {
  index: number;
  result?: ToolResultContent;
  use: ToolUseContent;
}

export interface StructuredContentProjection {
  consumedBlockIndexes: ReadonlySet<number>;
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
  content.forEach((block, index) => {
    if (block.type !== "tool_result") {
      return;
    }
    const toolUse = toolUseById.get(block.tool_use_id);
    if (!toolUse) {
      return;
    }
    toolUse.result = block;
    consumedBlockIndexes.add(index);
  });

  return {
    consumedBlockIndexes,
    taskProgressByToolUseId,
    toolUseById,
  };
}

export function resolveToolBlockStatus(
  toolUse: ToolUseProjection | undefined,
  waitingForPermission: boolean,
): ToolBlockStatus {
  if (waitingForPermission) {
    return "waiting_permission";
  }
  if (!toolUse?.result) {
    return "running";
  }
  return toolUse.result.is_error ? "error" : "success";
}

export function isHiddenSystemEvent(block: SystemEventContent): boolean {
  return block.subtype === "api_retry" &&
    typeof block.attempt === "number" &&
    block.attempt < API_RETRY_VISIBLE_ATTEMPT;
}
