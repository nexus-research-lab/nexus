import type {
  ContentBlock,
  TaskProgressContent,
  ToolResultContent,
  ToolUseContent,
} from "@/types/conversation/message";
import type { PendingPermission } from "@/types/conversation/permission";

import type { MessageActivityState } from "../ui/message-primitives";

export function resolve_activity_state({
  content,
  streaming_block_indexes,
  tool_use_map,
  rendered_indices,
  fallback_activity_state,
  pending_permissions_by_tool_use_id,
  hidden_tool_names,
}: {
  content: ContentBlock[];
  streaming_block_indexes?: ReadonlySet<number>;
  tool_use_map: ReadonlyMap<string, {
    use: ToolUseContent;
    result?: ToolResultContent;
    index: number;
  }>;
  rendered_indices: ReadonlySet<number>;
  fallback_activity_state?: MessageActivityState | null;
  pending_permissions_by_tool_use_id?: ReadonlyMap<string, PendingPermission>;
  hidden_tool_names: string[];
}): MessageActivityState {
  const latest_pending_tool = find_latest_pending_tool_use(
    content,
    tool_use_map,
    hidden_tool_names,
  );
  if (latest_pending_tool) {
    const pending_permission = pending_permissions_by_tool_use_id?.get(latest_pending_tool.id);
    if (pending_permission) {
      if (latest_pending_tool.name === "AskUserQuestion") {
        return "waiting_input";
      }
      return "waiting_permission";
    }

    if (latest_pending_tool.name === "AskUserQuestion") {
      return fallback_activity_state ?? "thinking";
    }

    return map_tool_name_to_activity_state(latest_pending_tool.name);
  }

  const latest_visible_block = find_latest_visible_block(
    content,
    rendered_indices,
    hidden_tool_names,
  );
  if (!latest_visible_block) {
    return fallback_activity_state ?? "thinking";
  }

  if (latest_visible_block.type === "task_progress") {
    return map_progress_to_activity_state(latest_visible_block);
  }

  if (latest_visible_block.type === "tool_use") {
    if (latest_visible_block.name === "AskUserQuestion") {
      return pending_permissions_by_tool_use_id?.has(latest_visible_block.id)
        ? "waiting_input"
        : (fallback_activity_state ?? "thinking");
    }
    return map_tool_name_to_activity_state(latest_visible_block.name);
  }

  if (latest_visible_block.type === "thinking") {
    return "thinking";
  }

  if (latest_visible_block.type === "text") {
    return has_streaming_text_block(content, streaming_block_indexes) ? "replying" : (fallback_activity_state ?? "replying");
  }

  if (latest_visible_block.type === "workspace_file_artifact") {
    return fallback_activity_state ?? "executing";
  }

  return fallback_activity_state ?? "thinking";
}

function find_latest_pending_tool_use(
  content: ContentBlock[],
  tool_use_map: ReadonlyMap<string, {
    use: ToolUseContent;
    result?: ToolResultContent;
    index: number;
  }>,
  hidden_tool_names: string[],
): ToolUseContent | null {
  for (let index = content.length - 1; index >= 0; index -= 1) {
    const block = content[index];
    if (block?.type !== "tool_use") {
      continue;
    }
    if (hidden_tool_names.includes(block.name)) {
      continue;
    }

    const tool_data = tool_use_map.get(block.id);
    if (!tool_data?.result) {
      return block;
    }
  }

  return null;
}

function find_latest_visible_block(
  content: ContentBlock[],
  rendered_indices: ReadonlySet<number>,
  hidden_tool_names: string[],
): ContentBlock | null {
  for (let index = content.length - 1; index >= 0; index -= 1) {
    const block = content[index];
    if (!block) {
      continue;
    }
    if (rendered_indices.has(index)) {
      continue;
    }
    if (block.type === "tool_use" && hidden_tool_names.includes(block.name)) {
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

function map_progress_to_activity_state(block: TaskProgressContent): MessageActivityState {
  return map_tool_name_to_activity_state(block.last_tool_name ?? null);
}

function map_tool_name_to_activity_state(tool_name?: string | null): MessageActivityState {
  if (!tool_name) {
    return "executing";
  }

  const browsing_tools = new Set([
    "Read",
    "Glob",
    "LS",
    "Grep",
    "WebSearch",
    "WebFetch",
  ]);

  if (browsing_tools.has(tool_name)) {
    return "browsing";
  }

  return "executing";
}

function has_streaming_text_block(
  content: ContentBlock[],
  streaming_block_indexes?: ReadonlySet<number>,
): boolean {
  if (!streaming_block_indexes?.size) {
    return false;
  }

  for (const index of streaming_block_indexes) {
    const block = content[index];
    if (block?.type === "text" && block.text.trim()) {
      return true;
    }
  }

  return false;
}
