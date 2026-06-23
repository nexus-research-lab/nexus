/**
 * =====================================================
 * @File   ：use-message-item-state.ts
 * @Date   ：2026-04-16 15:54
 * @Author ：leemysw
 * 2026-04-16 15:54   Create
 * =====================================================
 */

"use client";

import { useCallback, useEffect, useMemo } from "react";

import { useAssistantContentMerge } from "@/hooks/conversation/use-assistant-content-merge";
import { useScrollAnchoredState } from "@/hooks/conversation/use-scroll-anchored-state";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import {
  get_system_message_display_meta,
  type AssistantMessage,
  type SystemEventContent,
  type SystemMessage,
} from "@/types/conversation/message";

import type {
  MessageItemProps,
  MessageItemState,
} from "./message-item-types";
import {
  build_process_summary,
  resolve_live_activity_state,
} from "./message-item-activity";
import {
  build_visible_assistant_turns,
  build_visible_ordered_assistant_entries,
} from "./message-item-ordering";
import { resolve_message_item_permissions } from "./message-item-permissions";
import { build_message_stats } from "./message-item-stats";
import { resolve_message_item_final_projection } from "./message-item-final-projection";
import {
  has_timed_out_ask_user_question,
  type AssistantTurnEntry,
  type ContentProjection,
  type OrderedAssistantEntry,
} from "./message-item-support";
import { useMessageItemStreamingLayout } from "./message-item-streaming-layout";

export function useMessageItemState({
  is_last_round,
  is_loading,
  runtime_phase,
  messages,
  pending_permissions = [],
  hidden_tool_names = ["TodoWrite"],
  on_stop_message,
  round_id,
  default_process_expanded = false,
  assistant_content_mode = "dm_archived",
}: MessageItemProps): MessageItemState {
  const { copied: copied_user, copy: copy_user } = useCopyToClipboard();
  const { copied: copied_assistant, copy: copy_assistant } = useCopyToClipboard();
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
    return messages.filter(
      (message): message is SystemMessage =>
        message.role === "system" &&
        typeof message.content === "string" &&
        Boolean(message.content.trim()) &&
        (
          (is_last_round && is_loading) ||
          message.metadata?.subtype === "guided_input"
        ),
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
  const assistant_agent_id = first_assistant?.agent_id ?? null;
  const model = first_assistant?.model;
  const timestamp =
    first_assistant?.timestamp ||
    system_event_blocks[0]?.timestamp ||
    result_summary?.timestamp;

  const stream_status = useMemo(() => {
    return first_assistant?.stream_status ?? null;
  }, [first_assistant]);

  const stats = useMemo(
    () => build_message_stats(result_summary),
    [result_summary],
  );

  const user_content = useMemo(() => {
    if (!user_message || user_message.role !== "user") {
      return "";
    }
    return typeof user_message.content === "string" ? user_message.content : "";
  }, [user_message]);
  const user_attachments = useMemo(() => {
    if (!user_message || user_message.role !== "user") {
      return [];
    }
    return user_message.attachments ?? [];
  }, [user_message]);

  const {
    matched_pending_permissions_by_tool_use_id,
    unmatched_pending_permissions,
  } = useMemo(
    () => resolve_message_item_permissions(messages, pending_permissions),
    [messages, pending_permissions],
  );

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
  >(
    () => build_visible_ordered_assistant_entries({
      hidden_tool_names,
      hidden_tool_use_ids,
      is_loading,
      merged_content,
      merged_content_source_message_ids,
      source_message_order_by_id,
      system_event_blocks,
    }),
    [
      hidden_tool_names,
      hidden_tool_use_ids,
      is_loading,
      merged_content,
      merged_content_source_message_ids,
      source_message_order_by_id,
      system_event_blocks,
    ],
  );

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

  const visible_assistant_turns = useMemo<AssistantTurnEntry[]>(
    () => build_visible_assistant_turns({
      assistant_messages,
      streaming_block_indexes,
      visible_ordered_assistant_entries,
    }),
    [
      assistant_messages,
      streaming_block_indexes,
      visible_ordered_assistant_entries,
    ],
  );

  const ordered_projection = useMemo<ContentProjection>(
    () => ({
      content: visible_ordered_assistant_content,
      streaming_indexes: ordered_assistant_streaming_indexes,
    }),
    [ordered_assistant_streaming_indexes, visible_ordered_assistant_content],
  );

  const {
    direct_ordered_projection,
    process_projection,
    final_assistant_content,
    final_assistant_streaming_indexes,
    final_assistant_text,
  } = useMemo(
    () =>
      resolve_message_item_final_projection({
        assistant_content_mode,
        assistant_messages,
        ordered_projection,
        result_summary,
        round_id,
        streaming_block_indexes,
        visible_assistant_turns,
        visible_ordered_assistant_entries,
      }),
    [
      assistant_content_mode,
      assistant_messages,
      ordered_projection,
      result_summary,
      round_id,
      streaming_block_indexes,
      visible_assistant_turns,
      visible_ordered_assistant_entries,
    ],
  );

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

  const process_summary = useMemo(
    () => build_process_summary({
      pending_permission_count: pending_permissions.length,
      process_content: process_projection.content,
    }),
    [pending_permissions.length, process_projection.content],
  );

  const live_activity_state = useMemo(
    () => resolve_live_activity_state({
      is_last_round,
      is_loading,
      merged_content,
      pending_permissions,
      runtime_phase,
      stream_status,
      streaming_block_indexes,
    }),
    [
      is_last_round,
      is_loading,
      merged_content,
      pending_permissions,
      runtime_phase,
      stream_status,
      streaming_block_indexes,
    ],
  );

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
    await copy_user(user_content);
  }, [copy_user, user_content]);

  const handle_copy_assistant = useCallback(async () => {
    if (!final_assistant_text) {
      return;
    }
    await copy_assistant(final_assistant_text);
  }, [copy_assistant, final_assistant_text]);

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

  const { content_area_ref, content_area_style } =
    useMessageItemStreamingLayout({
      assistant_content_mode,
      direct_content: direct_ordered_projection.content,
      final_assistant_text,
      show_cursor,
    });

  return {
    copied_user,
    copied_assistant,
    user_message,
    user_content,
    user_attachments,
    assistant_agent_id,
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
