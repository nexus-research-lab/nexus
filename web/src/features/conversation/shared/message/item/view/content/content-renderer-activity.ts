import type {
  ContentBlock,
  TaskProgressContent,
  ToolUseContent,
} from "@/types/conversation/message/content";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

import type { MessageActivityState } from "../../../ui/message-primitives";
import type { ToolUseProjection } from "./content-renderer-model";

const BROWSING_TOOLS = new Set([
  "Read",
  "Glob",
  "LS",
  "Grep",
  "WebSearch",
  "WebFetch",
]);

export function resolveActivityState({
  consumedBlockIndexes,
  content,
  fallbackActivityState,
  hiddenToolNames,
  pendingPermissionsByToolUseId,
  streamingBlockIndexes,
  toolUseById,
}: {
  consumedBlockIndexes: ReadonlySet<number>;
  content: ContentBlock[];
  fallbackActivityState?: MessageActivityState | null;
  hiddenToolNames: ReadonlySet<string>;
  pendingPermissionsByToolUseId?: ReadonlyMap<string, PendingPermission>;
  streamingBlockIndexes?: ReadonlySet<number>;
  toolUseById: ReadonlyMap<string, ToolUseProjection>;
}): MessageActivityState {
  const latestPendingTool = findLatestPendingToolUse(
    content,
    toolUseById,
    hiddenToolNames,
  );
  if (latestPendingTool) {
    const pendingPermission = pendingPermissionsByToolUseId?.get(latestPendingTool.id);
    if (pendingPermission) {
      return latestPendingTool.name === "AskUserQuestion"
        ? "waiting_input"
        : "waiting_permission";
    }
    if (latestPendingTool.name === "AskUserQuestion") {
      return fallbackActivityState ?? "thinking";
    }
    return mapToolNameToActivityState(latestPendingTool.name);
  }

  const latestVisibleBlock = findLatestVisibleBlock(
    content,
    consumedBlockIndexes,
    hiddenToolNames,
  );
  if (!latestVisibleBlock) {
    return fallbackActivityState ?? "thinking";
  }

  switch (latestVisibleBlock.type) {
    case "task_progress":
      return mapProgressToActivityState(latestVisibleBlock);
    case "tool_use":
      if (latestVisibleBlock.name !== "AskUserQuestion") {
        return mapToolNameToActivityState(latestVisibleBlock.name);
      }
      return pendingPermissionsByToolUseId?.has(latestVisibleBlock.id)
        ? "waiting_input"
        : (fallbackActivityState ?? "thinking");
    case "thinking":
      return "thinking";
    case "text":
      return hasStreamingTextBlock(content, streamingBlockIndexes)
        ? "replying"
        : (fallbackActivityState ?? "replying");
    case "workspace_file_artifact":
      return fallbackActivityState ?? "executing";
    default:
      return fallbackActivityState ?? "thinking";
  }
}

function findLatestPendingToolUse(
  content: ContentBlock[],
  toolUseById: ReadonlyMap<string, ToolUseProjection>,
  hiddenToolNames: ReadonlySet<string>,
): ToolUseContent | null {
  for (let index = content.length - 1; index >= 0; index -= 1) {
    const block = content[index];
    if (block?.type !== "tool_use" || hiddenToolNames.has(block.name)) {
      continue;
    }
    if (!toolUseById.get(block.id)?.result) {
      return block;
    }
  }
  return null;
}

function findLatestVisibleBlock(
  content: ContentBlock[],
  consumedBlockIndexes: ReadonlySet<number>,
  hiddenToolNames: ReadonlySet<string>,
): ContentBlock | null {
  for (let index = content.length - 1; index >= 0; index -= 1) {
    const block = content[index];
    if (!block || consumedBlockIndexes.has(index)) {
      continue;
    }
    if (block.type === "tool_use" && hiddenToolNames.has(block.name)) {
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

function mapProgressToActivityState(
  block: TaskProgressContent,
): MessageActivityState {
  return mapToolNameToActivityState(block.last_tool_name ?? null);
}

function mapToolNameToActivityState(
  toolName?: string | null,
): MessageActivityState {
  if (!toolName) {
    return "executing";
  }
  return BROWSING_TOOLS.has(toolName) ? "browsing" : "executing";
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
