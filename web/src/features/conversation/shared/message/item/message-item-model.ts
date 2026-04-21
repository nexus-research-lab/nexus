/**
 * =====================================================
 * @File   ：use-message-item-state.ts
 * @Date   ：2026-04-16 15:54
 * @Author ：leemysw
 * 2026-04-16 15:54   Create
 * =====================================================
 */

"use client";

import { prepare, layout } from "@chenglou/pretext";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
} from "react";

import { useAssistantContentMerge } from "@/hooks/conversation/use-assistant-content-merge";
import { useScrollAnchoredState } from "@/hooks/conversation/use-scroll-anchored-state";
import {
  get_system_message_display_meta,
  type AssistantMessage,
  type ContentBlock,
  type SystemEventContent,
  type SystemMessage,
} from "@/types/conversation/message";
import {
  collect_unresolved_tool_use_candidates,
  match_pending_permissions_to_tool_uses,
} from "@/types/conversation/permission";

import type {
  MessageItemProps,
  MessageItemState,
  MessageStatsData,
} from "./message-item-types";
import {
  extract_text_from_content_blocks,
  find_latest_streaming_block,
  has_timed_out_ask_user_question,
  map_runtime_phase_to_activity_state,
  projection_from_ordered_entries,
  type AssistantTurnEntry,
  type ContentProjection,
  type OrderedAssistantEntry,
} from "./message-item-support";

function format_compact_count(value: number): string {
  if (!Number.isFinite(value)) {
    return "0";
  }
  if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(value >= 10_000_000 ? 0 : 1)}m`;
  }
  if (value >= 1_000) {
    return `${(value / 1_000).toFixed(value >= 10_000 ? 0 : 1)}k`;
  }
  return `${value}`;
}

function get_result_summary_display_text(
  result_summary: AssistantMessage["result_summary"] | undefined,
): string | null {
  const result_text = result_summary?.result?.trim();
  if (result_text) {
    return result_text;
  }
  if (!result_summary) {
    return null;
  }
  if (result_summary.subtype === "interrupted") {
    return null;
  }
  if (result_summary.subtype === "error" || result_summary.is_error) {
    return "执行失败";
  }
  return null;
}

export function useMessageItemState({
  is_last_round,
  is_loading,
  runtime_phase,
  messages,
  pending_permissions = [],
  on_permission_response,
  hidden_tool_names = ["TodoWrite"],
  on_stop_message,
  round_id,
  default_process_expanded = false,
  assistant_content_mode = "dm_archived",
}: MessageItemProps): MessageItemState {
  const [copied_user, set_copied_user] = useState(false);
  const [copied_assistant, set_copied_assistant] = useState(false);
  const {
    is_open: is_process_expanded,
    toggle: toggle_process_expanded,
    set_open: set_is_process_expanded,
    anchor_ref: process_anchor_ref,
  } = useScrollAnchoredState(default_process_expanded);

  const {
    user_message,
    assistant_messages,
    result_summary,
    merged_content,
    merged_content_source_message_ids,
    streaming_block_indexes,
  } = useAssistantContentMerge({
    messages,
    is_last_round,
    is_loading,
  });

  const system_messages = useMemo(() => {
    if (!is_last_round || !is_loading) {
      return [];
    }

    return messages.filter(
      (message): message is SystemMessage =>
        message.role === "system" &&
        typeof message.content === "string" &&
        Boolean(message.content.trim()),
    );
  }, [is_last_round, is_loading, messages]);
  const system_event_blocks = useMemo<SystemEventContent[]>(
    () =>
      system_messages.map((message) => {
        const display_meta = get_system_message_display_meta(message);
        return {
          type: "system_event",
          content: message.content,
          label: display_meta.label,
          tone: display_meta.tone,
          icon: display_meta.icon,
          source_message_id: message.message_id,
          timestamp: message.timestamp,
          subtype: message.metadata?.subtype,
          tool_use_id:
            typeof message.metadata?.tool_use_id === "string"
              ? message.metadata.tool_use_id
              : null,
        };
      }),
    [system_messages],
  );
  const source_message_order_by_id = useMemo(() => {
    const next_order = new Map<string, number>();
    messages.forEach((message, index) => {
      next_order.set(message.message_id, index);
    });
    return next_order;
  }, [messages]);

  const first_assistant = assistant_messages[0] as AssistantMessage | undefined;
  const model = first_assistant?.model;
  const timestamp =
    first_assistant?.timestamp ||
    system_event_blocks[0]?.timestamp ||
    result_summary?.timestamp;

  const stream_status = useMemo(() => {
    return first_assistant?.stream_status ?? null;
  }, [first_assistant]);

  const stats = useMemo<MessageStatsData | null>(() => {
    const usage = result_summary?.usage;
    const duration = result_summary
      ? result_summary.duration_ms > 0
        ? `${(result_summary.duration_ms / 1000).toFixed(1)}s`
        : "0s"
      : null;
    const cost =
      result_summary?.total_cost_usd !== undefined
        ? `$${result_summary.total_cost_usd.toFixed(4)}`
        : null;
    const cache_hit = usage?.cache_read_input_tokens;
    const tokens = usage
      ? `${format_compact_count(usage.input_tokens)}↑ ${format_compact_count(usage.output_tokens)}↓`
      : null;

    if (!duration && !tokens && !cost && !cache_hit) {
      return null;
    }

    return {
      duration,
      tokens,
      cost,
      cache_hit:
        cache_hit && cache_hit > 0
          ? `缓存 ${format_compact_count(cache_hit)}`
          : null,
    };
  }, [result_summary]);

  const user_content = useMemo(() => {
    if (!user_message || user_message.role !== "user") {
      return "";
    }
    return typeof user_message.content === "string" ? user_message.content : "";
  }, [user_message]);

  const {
    matched_pending_permissions_by_tool_use_id,
    unmatched_pending_permissions,
  } = useMemo(() => {
    if (pending_permissions.length === 0) {
      return {
        matched_pending_permissions_by_tool_use_id: new Map(),
        unmatched_pending_permissions: [],
      };
    }

    const unresolved_tool_use_candidates =
      collect_unresolved_tool_use_candidates(messages);
    const permission_match_result = match_pending_permissions_to_tool_uses(
      pending_permissions,
      unresolved_tool_use_candidates,
    );
    const matched_permissions_by_tool_use_id = new Map(
      permission_match_result.matched_permissions_by_tool_use_id,
    );

    const unmatched_question_permissions =
      permission_match_result.unmatched_permissions.filter(
        (permission) =>
          permission.interaction_mode === "question" ||
          permission.tool_name === "AskUserQuestion",
      );
    const unresolved_question_candidates =
      unresolved_tool_use_candidates.filter(
        (candidate) =>
          candidate.tool_name === "AskUserQuestion" &&
          !matched_permissions_by_tool_use_id.has(candidate.tool_use_id),
      );

    // Room 场景下 AskUserQuestion 的 permission_request 会先绑定占位槽位，
    // 这里按 round_id 和单候选规则做一次安全补配，避免问答块丢失交互能力。
    for (const permission of unmatched_question_permissions) {
      const candidates_by_round = unresolved_question_candidates.filter(
        (candidate) =>
          !matched_permissions_by_tool_use_id.has(candidate.tool_use_id) &&
          (!permission.caused_by ||
            candidate.round_id === permission.caused_by),
      );

      if (candidates_by_round.length === 1) {
        matched_permissions_by_tool_use_id.set(
          candidates_by_round[0].tool_use_id,
          permission,
        );
        continue;
      }

      const remaining_candidates = unresolved_question_candidates.filter(
        (candidate) =>
          !matched_permissions_by_tool_use_id.has(candidate.tool_use_id),
      );
      if (
        remaining_candidates.length === 1 &&
        unmatched_question_permissions.length === 1
      ) {
        matched_permissions_by_tool_use_id.set(
          remaining_candidates[0].tool_use_id,
          permission,
        );
      }
    }

    return {
      matched_pending_permissions_by_tool_use_id:
        matched_permissions_by_tool_use_id,
      unmatched_pending_permissions:
        permission_match_result.unmatched_permissions.filter(
          (permission) =>
            permission.interaction_mode !== "question" &&
            permission.tool_name !== "AskUserQuestion",
        ),
    };
  }, [messages, pending_permissions]);

  const hidden_tool_use_ids = useMemo(() => {
    const next_ids = new Set<string>();
    for (const block of merged_content) {
      if (block.type === "tool_use" && hidden_tool_names.includes(block.name)) {
        next_ids.add(block.id);
      }
    }
    return next_ids;
  }, [hidden_tool_names, merged_content]);

  const visible_ordered_assistant_entries = useMemo<
    OrderedAssistantEntry[]
  >(() => {
    const assistant_entries: OrderedAssistantEntry[] = [];
    const resolve_source_order = (source_message_id: string) =>
      source_message_order_by_id.get(source_message_id) ??
      Number.MAX_SAFE_INTEGER;

    merged_content.forEach((block, merged_index) => {
      const source_message_id =
        merged_content_source_message_ids[merged_index] || "";
      const source_order = resolve_source_order(source_message_id);

      if (block.type === "text") {
        if (block.text.trim()) {
          assistant_entries.push({
            block,
            merged_index,
            source_message_id,
            source_order,
          });
        }
        return;
      }

      if (block.type === "thinking") {
        if (block.thinking?.trim()) {
          assistant_entries.push({
            block,
            merged_index,
            source_message_id,
            source_order,
          });
        }
        return;
      }

      if (block.type === "tool_use") {
        if (!hidden_tool_names.includes(block.name)) {
          assistant_entries.push({
            block,
            merged_index,
            source_message_id,
            source_order,
          });
        }
        return;
      }

      if (block.type === "tool_result") {
        if (!hidden_tool_use_ids.has(block.tool_use_id)) {
          assistant_entries.push({
            block,
            merged_index,
            source_message_id,
            source_order,
          });
        }
        return;
      }

      if (block.type === "task_progress") {
        assistant_entries.push({
          block,
          merged_index,
          source_message_id,
          source_order,
        });
      }
    });

    const ordered_entries: OrderedAssistantEntry[] = [];
    const system_blocks_by_tool_use_id = new Map<
      string,
      SystemEventContent[]
    >();
    const unmatched_system_blocks: SystemEventContent[] = [];

    system_event_blocks.forEach((block) => {
      if (block.tool_use_id) {
        const existing_blocks =
          system_blocks_by_tool_use_id.get(block.tool_use_id) ?? [];
        existing_blocks.push(block);
        system_blocks_by_tool_use_id.set(block.tool_use_id, existing_blocks);
        return;
      }
      unmatched_system_blocks.push(block);
    });

    assistant_entries.forEach((entry) => {
      ordered_entries.push(entry);
      if (entry.block.type !== "tool_use") {
        return;
      }

      const matched_system_blocks = system_blocks_by_tool_use_id.get(
        entry.block.id,
      );
      if (!matched_system_blocks?.length) {
        return;
      }

      matched_system_blocks.forEach((block) => {
        ordered_entries.push({
          block,
          merged_index: -1,
          source_message_id: block.source_message_id,
          source_order: resolve_source_order(block.source_message_id),
        });
      });
      system_blocks_by_tool_use_id.delete(entry.block.id);
    });

    system_blocks_by_tool_use_id.forEach((blocks) => {
      unmatched_system_blocks.push(...blocks);
    });
    const unmatched_ordered_entries = unmatched_system_blocks
      .map((block) => ({
        block,
        merged_index: -1,
        source_message_id: block.source_message_id,
        source_order: resolve_source_order(block.source_message_id),
      }))
      .sort((left, right) => {
        if (left.source_order !== right.source_order) {
          return left.source_order - right.source_order;
        }
        const left_timestamp =
          left.block.type === "system_event" ? left.block.timestamp : 0;
        const right_timestamp =
          right.block.type === "system_event" ? right.block.timestamp : 0;
        return left_timestamp - right_timestamp;
      });

    if (unmatched_ordered_entries.length === 0) {
      return ordered_entries;
    }

    const merged_entries: OrderedAssistantEntry[] = [];
    let system_index = 0;
    ordered_entries.forEach((entry) => {
      while (
        system_index < unmatched_ordered_entries.length &&
        unmatched_ordered_entries[system_index].source_order <
          entry.source_order
      ) {
        merged_entries.push(unmatched_ordered_entries[system_index]);
        system_index += 1;
      }
      merged_entries.push(entry);
    });
    while (system_index < unmatched_ordered_entries.length) {
      merged_entries.push(unmatched_ordered_entries[system_index]);
      system_index += 1;
    }

    return merged_entries;
  }, [
    hidden_tool_use_ids,
    hidden_tool_names,
    merged_content,
    merged_content_source_message_ids,
    source_message_order_by_id,
    system_event_blocks,
  ]);

  const visible_ordered_assistant_content = useMemo(() => {
    return visible_ordered_assistant_entries.map((entry) => entry.block);
  }, [visible_ordered_assistant_entries]);

  const ordered_assistant_streaming_indexes = useMemo(() => {
    const next_indexes = new Set<number>();

    visible_ordered_assistant_entries.forEach((entry, visible_index) => {
      if (streaming_block_indexes.has(entry.merged_index)) {
        next_indexes.add(visible_index);
      }
    });

    return next_indexes;
  }, [streaming_block_indexes, visible_ordered_assistant_entries]);

  const visible_assistant_turns = useMemo<AssistantTurnEntry[]>(() => {
    const turn_map = new Map<string, AssistantTurnEntry>();
    assistant_messages.forEach((message) => {
      turn_map.set(message.message_id, {
        message_id: message.message_id,
        content: [],
        text_content: [],
        streaming_indexes: new Set<number>(),
        text_streaming_indexes: new Set<number>(),
      });
    });

    visible_ordered_assistant_entries.forEach((entry) => {
      const turn = turn_map.get(entry.source_message_id);
      if (!turn) {
        return;
      }

      const content_index = turn.content.length;
      turn.content.push(entry.block);
      if (streaming_block_indexes.has(entry.merged_index)) {
        turn.streaming_indexes.add(content_index);
      }

      if (entry.block.type === "text" && entry.block.text.trim()) {
        const text_index = turn.text_content.length;
        turn.text_content.push(entry.block);
        if (streaming_block_indexes.has(entry.merged_index)) {
          turn.text_streaming_indexes.add(text_index);
        }
      }
    });

    return assistant_messages
      .map((message) => turn_map.get(message.message_id))
      .filter((turn): turn is AssistantTurnEntry =>
        Boolean(turn && turn.content.length > 0),
      );
  }, [
    assistant_messages,
    streaming_block_indexes,
    visible_ordered_assistant_entries,
  ]);

  const ordered_projection = useMemo<ContentProjection>(
    () => ({
      content: visible_ordered_assistant_content,
      streaming_indexes: ordered_assistant_streaming_indexes,
    }),
    [ordered_assistant_streaming_indexes, visible_ordered_assistant_content],
  );

  const last_assistant_turn = useMemo(
    () => visible_assistant_turns.at(-1) ?? null,
    [visible_assistant_turns],
  );

  const final_assistant_turn = useMemo(() => {
    for (let index = assistant_messages.length - 1; index >= 0; index -= 1) {
      const message = assistant_messages[index] as AssistantMessage;
      if (!message.parent_id || message.parent_id === round_id) {
        return (
          visible_assistant_turns.find(
            (turn) => turn.message_id === message.message_id,
          ) ?? null
        );
      }
    }
    return last_assistant_turn;
  }, [
    assistant_messages,
    last_assistant_turn,
    round_id,
    visible_assistant_turns,
  ]);

  const final_tail_entries = useMemo<OrderedAssistantEntry[]>(() => {
    if (!final_assistant_turn) {
      return [];
    }

    const tail_entries: OrderedAssistantEntry[] = [];
    for (
      let index = visible_ordered_assistant_entries.length - 1;
      index >= 0;
      index -= 1
    ) {
      const entry = visible_ordered_assistant_entries[index];
      if (entry.source_message_id !== final_assistant_turn.message_id) {
        break;
      }
      if (entry.block.type !== "text" || !entry.block.text.trim()) {
        break;
      }
      tail_entries.unshift(entry);
    }
    return tail_entries;
  }, [final_assistant_turn, visible_ordered_assistant_entries]);

  const final_tail_text = useMemo(() => {
    return final_tail_entries
      .map((entry) => entry.block)
      .filter(
        (block): block is Extract<ContentBlock, { type: "text" }> =>
          block.type === "text",
      )
      .map((block) => block.text)
      .join("\n\n")
      .trim();
  }, [final_tail_entries]);

  const final_assistant_text_merged_indexes = useMemo(() => {
    if (
      !final_assistant_turn ||
      final_assistant_turn.text_content.length === 0
    ) {
      return new Set<number>();
    }

    const next_indexes = new Set<number>();
    for (const entry of visible_ordered_assistant_entries) {
      if (entry.source_message_id !== final_assistant_turn.message_id) {
        continue;
      }
      if (entry.block.type !== "text" || !entry.block.text.trim()) {
        continue;
      }
      next_indexes.add(entry.merged_index);
    }
    return next_indexes;
  }, [final_assistant_turn, visible_ordered_assistant_entries]);

  const archived_process_projection = useMemo<ContentProjection>(() => {
    const result_text = result_summary?.result?.trim();
    const should_strip_tail =
      final_tail_entries.length > 0 &&
      (!result_text ||
        final_tail_text === result_text ||
        final_tail_entries
          .map((entry) => entry.block)
          .filter(
            (block): block is Extract<ContentBlock, { type: "text" }> =>
              block.type === "text",
          )
          .map((block) => block.text)
          .join("")
          .trim() === result_text);

    if (should_strip_tail) {
      const tail_indexes = new Set(
        final_tail_entries.map((entry) => entry.merged_index),
      );
      return projection_from_ordered_entries(
        visible_ordered_assistant_entries.filter(
          (entry) => !tail_indexes.has(entry.merged_index),
        ),
        streaming_block_indexes,
      );
    }

    if (!result_text && final_assistant_turn) {
      return projection_from_ordered_entries(
        visible_ordered_assistant_entries.filter(
          (entry) =>
            entry.source_message_id !== final_assistant_turn.message_id ||
            !final_assistant_text_merged_indexes.has(entry.merged_index),
        ),
        streaming_block_indexes,
      );
    }

    return projection_from_ordered_entries(
      visible_ordered_assistant_entries,
      streaming_block_indexes,
    );
  }, [
    final_assistant_text_merged_indexes,
    final_assistant_turn,
    final_tail_entries,
    final_tail_text,
    result_summary,
    streaming_block_indexes,
    visible_ordered_assistant_entries,
  ]);

  const fallback_final_assistant_content = useMemo(() => {
    if (final_tail_entries.length > 0) {
      return final_tail_entries.map((entry) => entry.block);
    }
    if (!final_assistant_turn) {
      return null;
    }
    if (final_assistant_turn.text_content.length > 0) {
      return final_assistant_turn.text_content;
    }
    if (final_assistant_turn.content.length > 0) {
      return final_assistant_turn.content;
    }
    return null;
  }, [final_assistant_turn, final_tail_entries]);

  const fallback_final_assistant_streaming_indexes = useMemo(() => {
    if (final_tail_entries.length > 0) {
      const next_indexes = new Set<number>();
      final_tail_entries.forEach((entry, index) => {
        if (streaming_block_indexes.has(entry.merged_index)) {
          next_indexes.add(index);
        }
      });
      return next_indexes;
    }
    if (!final_assistant_turn) {
      return new Set<number>();
    }
    if (final_assistant_turn.text_content.length > 0) {
      return final_assistant_turn.text_streaming_indexes;
    }
    return final_assistant_turn.streaming_indexes;
  }, [final_assistant_turn, final_tail_entries, streaming_block_indexes]);

  const direct_ordered_projection = useMemo<ContentProjection>(() => {
    if (
      assistant_content_mode === "dm_live" ||
      assistant_content_mode === "room_thread"
    ) {
      return ordered_projection;
    }
    return { content: [], streaming_indexes: new Set<number>() };
  }, [assistant_content_mode, ordered_projection]);

  const process_projection = useMemo<ContentProjection>(() => {
    if (assistant_content_mode === "dm_archived") {
      return archived_process_projection;
    }
    return { content: [], streaming_indexes: new Set<number>() };
  }, [archived_process_projection, assistant_content_mode]);

  const final_assistant_content = useMemo<
    string | ContentBlock[] | null
  >(() => {
    if (
      assistant_content_mode === "dm_live" ||
      assistant_content_mode === "room_thread"
    ) {
      return null;
    }

    const result_text = get_result_summary_display_text(result_summary);
    if (result_text) {
      return result_text;
    }

    if (assistant_content_mode === "dm_archived") {
      if (final_tail_entries.length > 0) {
        return final_tail_entries.map((entry) => entry.block);
      }
      if (final_assistant_turn?.text_content.length) {
        return final_assistant_turn.text_content;
      }
      return null;
    }

    return fallback_final_assistant_content;
  }, [
    assistant_content_mode,
    fallback_final_assistant_content,
    final_assistant_turn,
    final_tail_entries,
    result_summary,
  ]);

  const final_assistant_streaming_indexes = useMemo(() => {
    if (
      assistant_content_mode === "dm_live" ||
      assistant_content_mode === "room_thread"
    ) {
      return new Set<number>();
    }
    if (typeof final_assistant_content === "string") {
      return new Set<number>();
    }
    return fallback_final_assistant_streaming_indexes;
  }, [
    assistant_content_mode,
    fallback_final_assistant_streaming_indexes,
    final_assistant_content,
  ]);

  const final_assistant_text = useMemo(() => {
    if (typeof final_assistant_content === "string") {
      return final_assistant_content;
    }
    return extract_text_from_content_blocks(final_assistant_content);
  }, [final_assistant_content]);

  const should_render_direct_assistant_content =
    direct_ordered_projection.content.length > 0;
  const has_visible_process =
    process_projection.content.length > 0 ||
    unmatched_pending_permissions.length > 0;
  const should_render_process_callchain =
    assistant_content_mode === "dm_archived" && has_visible_process;

  const has_timed_out_question_in_process = useMemo(
    () => has_timed_out_ask_user_question(process_projection.content),
    [process_projection.content],
  );

  const process_summary = useMemo(() => {
    let tool_count = 0;
    let thinking_count = 0;
    let error_count = 0;
    let progress_count = 0;

    for (const block of process_projection.content) {
      if (block.type === "thinking") {
        thinking_count += 1;
        continue;
      }
      if (block.type === "tool_use") {
        tool_count += 1;
        continue;
      }
      if (block.type === "tool_result" && block.is_error) {
        error_count += 1;
        continue;
      }
      if (block.type === "task_progress") {
        progress_count += 1;
      }
    }

    if (pending_permissions.length > 0) {
      return "等待你的确认后继续";
    }

    const summary_parts: string[] = [];
    if (thinking_count > 0) {
      summary_parts.push(`${thinking_count} 段思路`);
    }
    if (tool_count > 0) {
      summary_parts.push(`${tool_count} 次动作`);
    }
    if (error_count > 0) {
      summary_parts.push(`${error_count} 个异常`);
    }
    if (progress_count > 0) {
      summary_parts.push(`${progress_count} 条进度`);
    }

    return summary_parts.length > 0 ? summary_parts.join(" · ") : "查看过程";
  }, [pending_permissions.length, process_projection.content]);

  const live_activity_state = useMemo(() => {
    if (!is_last_round || !is_loading) {
      return null;
    }

    if (pending_permissions.length > 0) {
      return pending_permissions.some(
        (permission) =>
          permission.interaction_mode === "question" ||
          permission.tool_name === "AskUserQuestion",
      )
        ? "waiting_input"
        : "waiting_permission";
    }

    const runtime_activity_state =
      map_runtime_phase_to_activity_state(runtime_phase);
    if (runtime_activity_state === "sending") {
      return "sending";
    }

    const latest_streaming_block = find_latest_streaming_block(
      merged_content,
      streaming_block_indexes,
    );
    if (latest_streaming_block?.type === "thinking") {
      return "thinking";
    }
    if (latest_streaming_block?.type === "text") {
      return "replying";
    }

    const has_visible_reply_text = merged_content.some(
      (block) => block.type === "text" && Boolean(block.text.trim()),
    );
    if (has_visible_reply_text && stream_status === "streaming") {
      return "replying";
    }

    if (stream_status === "pending") {
      return "thinking";
    }

    return runtime_activity_state;
  }, [
    is_last_round,
    is_loading,
    merged_content,
    pending_permissions,
    runtime_phase,
    stream_status,
    streaming_block_indexes,
  ]);

  const should_hide_assistant_content = useMemo(() => {
    if (live_activity_state) {
      return false;
    }
    if (unmatched_pending_permissions.length > 0) {
      return false;
    }
    if (
      stream_status === "pending" ||
      stream_status === "streaming" ||
      stream_status === "cancelled" ||
      stream_status === "error"
    ) {
      return false;
    }
    if (direct_ordered_projection.content.length > 0) {
      return false;
    }
    if (process_projection.content.length > 0) {
      return false;
    }
    if (typeof final_assistant_content === "string") {
      return !final_assistant_content.trim();
    }
    if (final_assistant_content && final_assistant_content.length > 0) {
      return false;
    }
    return !result_summary;
  }, [
    direct_ordered_projection.content.length,
    final_assistant_content,
    live_activity_state,
    process_projection.content.length,
    result_summary,
    stream_status,
    unmatched_pending_permissions.length,
  ]);

  const should_render_assistant_text = Boolean(
    typeof final_assistant_content === "string"
      ? final_assistant_content.trim()
      : final_assistant_content?.length,
  );

  const should_render_standalone_activity_status = Boolean(
    live_activity_state &&
    !should_render_direct_assistant_content &&
    !should_render_process_callchain &&
    !should_render_assistant_text,
  );

  useEffect(() => {
    if (pending_permissions.length > 0) {
      set_is_process_expanded(true);
    }
  }, [pending_permissions.length, set_is_process_expanded]);

  useEffect(() => {
    if (has_timed_out_question_in_process) {
      set_is_process_expanded(true);
    }
  }, [has_timed_out_question_in_process, set_is_process_expanded]);

  const handle_copy_user = useCallback(async () => {
    if (!user_content) {
      return;
    }
    try {
      await navigator.clipboard.writeText(user_content);
      set_copied_user(true);
      setTimeout(() => set_copied_user(false), 2000);
    } catch (error) {
      console.error("Failed to copy:", error);
    }
  }, [user_content]);

  const handle_copy_assistant = useCallback(async () => {
    if (!final_assistant_text) {
      return;
    }
    try {
      await navigator.clipboard.writeText(final_assistant_text);
      set_copied_assistant(true);
      setTimeout(() => set_copied_assistant(false), 2000);
    } catch (error) {
      console.error("Failed to copy:", error);
    }
  }, [final_assistant_text]);

  const show_cursor = Boolean(
    is_last_round &&
    is_loading &&
    (streaming_block_indexes.size > 0 ||
      assistant_messages.length > 0 ||
      pending_permissions.length > 0 ||
      stream_status === "pending" ||
      stream_status === "streaming"),
  );

  const final_assistant_is_streaming = Boolean(
    show_cursor &&
    typeof final_assistant_content !== "string" &&
    final_assistant_streaming_indexes.size > 0,
  );

  const can_copy_assistant = Boolean(final_assistant_text.trim());
  const should_show_assistant_footer =
    (assistant_content_mode === "dm_archived" ||
      assistant_content_mode === "room_result") &&
    (Boolean(stats) || (!is_loading && can_copy_assistant));

  const can_stop_message = Boolean(
    on_stop_message &&
    (stream_status === "pending" || stream_status === "streaming"),
  );
  const handle_stop_message = useCallback(() => {
    if (!on_stop_message || !first_assistant) {
      return;
    }
    on_stop_message(first_assistant.message_id);
  }, [first_assistant, on_stop_message]);

  const content_area_ref = useRef<HTMLDivElement>(null);
  const streaming_min_height = useRef(60);
  const layout_throttle_ref = useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );

  useEffect(() => {
    const layout_text =
      assistant_content_mode === "dm_live" ||
      assistant_content_mode === "room_thread"
        ? extract_text_from_content_blocks(direct_ordered_projection.content)
        : final_assistant_text;

    if (!show_cursor || !layout_text) {
      return;
    }
    if (layout_throttle_ref.current !== null) {
      return;
    }

    layout_throttle_ref.current = setTimeout(() => {
      layout_throttle_ref.current = null;
      const element = content_area_ref.current;
      if (!element) {
        return;
      }
      try {
        const width = element.offsetWidth || 640;
        const prepared = prepare(
          layout_text,
          "400 14px ui-sans-serif, system-ui, sans-serif",
        );
        const result = layout(prepared, width, 28);
        streaming_min_height.current = Math.max(
          streaming_min_height.current,
          result.height,
        );
      } catch {
        // 这里只保留上一次可用高度，避免流式阶段因为排版测量失败产生闪动。
      }
    }, 150);
  }, [
    assistant_content_mode,
    direct_ordered_projection.content,
    final_assistant_text,
    show_cursor,
  ]);

  useEffect(() => {
    if (!show_cursor) {
      streaming_min_height.current = 60;
      if (layout_throttle_ref.current !== null) {
        clearTimeout(layout_throttle_ref.current);
        layout_throttle_ref.current = null;
      }
    }
  }, [show_cursor]);

  const content_area_style: CSSProperties | undefined = show_cursor
    ? { minHeight: streaming_min_height.current }
    : undefined;

  return {
    copied_user,
    copied_assistant,
    user_message,
    user_content,
    model,
    timestamp,
    stream_status,
    stats,
    matched_pending_permissions_by_tool_use_id,
    unmatched_pending_permissions,
    direct_ordered_projection,
    process_projection,
    final_assistant_content,
    final_assistant_streaming_indexes,
    final_assistant_text,
    should_render_direct_assistant_content,
    should_render_process_callchain,
    should_render_assistant_text,
    should_render_standalone_activity_status,
    should_show_assistant_footer,
    show_cursor,
    final_assistant_is_streaming,
    should_hide_assistant_content,
    process_summary,
    live_activity_state,
    is_process_expanded,
    toggle_process_expanded,
    process_anchor_ref,
    can_copy_assistant,
    can_stop_message,
    handle_copy_user,
    handle_copy_assistant,
    handle_stop_message,
    content_area_ref,
    content_area_style,
    merged_content_length: merged_content.length,
  };
}
