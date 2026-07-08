import type {
  ContentBlock,
  TaskProgressContent,
  ToolResultContent,
  ToolUseContent,
} from "@/types/conversation/message";
import type { PendingPermission } from "@/types/conversation/permission";

import type { MessageActivityState } from "../ui/message-primitives";

export function resolveActivityState({
  content,
  streamingBlockIndexes,
  toolUseMap,
  renderedIndices,
  fallbackActivityState,
  pendingPermissionsByToolUseId,
  hiddenToolNames,
}: {
  content: ContentBlock[];
  streamingBlockIndexes?: ReadonlySet<number>;
  toolUseMap: ReadonlyMap<string, {
    use: ToolUseContent;
    result?: ToolResultContent;
    index: number;
  }>;
  renderedIndices: ReadonlySet<number>;
  fallbackActivityState?: MessageActivityState | null;
  pendingPermissionsByToolUseId?: ReadonlyMap<string, PendingPermission>;
  hiddenToolNames: string[];
}): MessageActivityState {
  const latestPendingTool = findLatestPendingToolUse(
    content,
    toolUseMap,
    hiddenToolNames,
  );
  if (latestPendingTool) {
    const pendingPermission = pendingPermissionsByToolUseId?.get(latestPendingTool.id);
    if (pendingPermission) {
      if (latestPendingTool.name === "AskUserQuestion") {
        return "waiting_input";
      }
      return "waiting_permission";
    }

    if (latestPendingTool.name === "AskUserQuestion") {
      return fallbackActivityState ?? "thinking";
    }

    return mapToolNameToActivityState(latestPendingTool.name);
  }

  const latestVisibleBlock = findLatestVisibleBlock(
    content,
    renderedIndices,
    hiddenToolNames,
  );
  if (!latestVisibleBlock) {
    return fallbackActivityState ?? "thinking";
  }

  if (latestVisibleBlock.type === "task_progress") {
    return mapProgressToActivityState(latestVisibleBlock);
  }

  if (latestVisibleBlock.type === "tool_use") {
    if (latestVisibleBlock.name === "AskUserQuestion") {
      return pendingPermissionsByToolUseId?.has(latestVisibleBlock.id)
        ? "waiting_input"
        : (fallbackActivityState ?? "thinking");
    }
    return mapToolNameToActivityState(latestVisibleBlock.name);
  }

  if (latestVisibleBlock.type === "thinking") {
    return "thinking";
  }

  if (latestVisibleBlock.type === "text") {
    return hasStreamingTextBlock(content, streamingBlockIndexes) ? "replying" : (fallbackActivityState ?? "replying");
  }

  if (latestVisibleBlock.type === "workspace_file_artifact") {
    return fallbackActivityState ?? "executing";
  }

  return fallbackActivityState ?? "thinking";
}

function findLatestPendingToolUse(
  content: ContentBlock[],
  toolUseMap: ReadonlyMap<string, {
    use: ToolUseContent;
    result?: ToolResultContent;
    index: number;
  }>,
  hiddenToolNames: string[],
): ToolUseContent | null {
  for (let index = content.length - 1; index >= 0; index -= 1) {
    const block = content[index];
    if (block?.type !== "tool_use") {
      continue;
    }
    if (hiddenToolNames.includes(block.name)) {
      continue;
    }

    const toolData = toolUseMap.get(block.id);
    if (!toolData?.result) {
      return block;
    }
  }

  return null;
}

function findLatestVisibleBlock(
  content: ContentBlock[],
  renderedIndices: ReadonlySet<number>,
  hiddenToolNames: string[],
): ContentBlock | null {
  for (let index = content.length - 1; index >= 0; index -= 1) {
    const block = content[index];
    if (!block) {
      continue;
    }
    if (renderedIndices.has(index)) {
      continue;
    }
    if (block.type === "tool_use" && hiddenToolNames.includes(block.name)) {
      continue;
    }
    if (block.type === "text" && !block.text.trim()) {
      continue;
    }
    if (block.type === "thinking" && !block.thinking.trim()) {
      continue;
    }
    return block;
  }

  return null;
}

function mapProgressToActivityState(block: TaskProgressContent): MessageActivityState {
  return mapToolNameToActivityState(block.last_tool_name ?? null);
}

function mapToolNameToActivityState(toolName?: string | null): MessageActivityState {
  if (!toolName) {
    return "executing";
  }

  const browsingTools = new Set([
    "Read",
    "Glob",
    "LS",
    "Grep",
    "WebSearch",
    "WebFetch",
  ]);

  if (browsingTools.has(toolName)) {
    return "browsing";
  }

  return "executing";
}

function hasStreamingTextBlock(
  content: ContentBlock[],
  streamingBlockIndexes?: ReadonlySet<number>,
): boolean {
  if (!streamingBlockIndexes?.size) {
    return false;
  }

  for (const index of streamingBlockIndexes) {
    const block = content[index];
    if (block?.type === "text" && block.text.trim()) {
      return true;
    }
  }

  return false;
}
