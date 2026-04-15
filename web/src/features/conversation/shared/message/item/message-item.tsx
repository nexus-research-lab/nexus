/**
 * Message Component
 *
 *
 */

"use client";

import { memo, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { prepare, layout } from "@chenglou/pretext";
import { Bot, Check, ChevronDown, ChevronRight, Copy, Edit2, RotateCcw, Square, User, Wrench } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAssistantContentMerge } from "@/hooks/conversation/use-assistant-content-merge";
import { useScrollAnchoredState } from "@/hooks/conversation/use-scroll-anchored-state";
import { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import { AssistantMessage, ContentBlock, get_system_message_display_meta, Message, SystemMessage } from "@/types/conversation/message";
import {
  collect_unresolved_tool_use_candidates,
  match_pending_permissions_to_tool_uses,
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";
import { ToolBlock } from "../blocks/tool-block";
import { MessageStats } from "../ui/message-stats";
import { ContentRenderer } from "./content-renderer";
import {
  AssistantContentMode,
  AssistantTurnEntry,
  ContentProjection,
  extract_text_from_content_blocks,
  find_latest_streaming_block,
  get_system_message_container_class_name,
  get_system_message_icon_class_name,
  has_timed_out_ask_user_question,
  map_runtime_phase_to_activity_state,
  OrderedAssistantEntry,
  projection_from_ordered_entries,
} from "./message-item-support";
import {
  MessageActionButton,
  MessageActivityStatus,
  MessageAvatar,
  MessageShell,
} from "../ui/message-primitives";

interface MessageItemProps {
  compact?: boolean;
  current_agent_name?: string | null;
  current_agent_avatar?: string | null;
  round_id: string;
  messages: Message[];
  is_last_round?: boolean;
  is_loading?: boolean;
  runtime_phase?: AgentConversationRuntimePhase | null;
  pending_permissions?: PendingPermission[];
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  can_respond_to_permissions?: boolean;
  permission_read_only_reason?: string;
  hidden_tool_names?: string[];
  on_edit_user_message?: (message_id: string, new_content: string) => void;
  on_open_workspace_file?: (path: string) => void;
  /** Called when user clicks the per-message stop button in Room mode. */
  on_stop_message?: (msg_id: string) => void;
  /** 初始化时 process 区域是否默认展开 */
  default_process_expanded?: boolean;
  /** 助手头部右侧附加操作，例如查看 Thread */
  assistant_header_action?: ReactNode;
  /** 助手内容渲染模式。 */
  assistant_content_mode?: AssistantContentMode;
  class_name?: string;
}

function MessageItemInner(
  {
    compact = false,
    current_agent_name,
    current_agent_avatar,
    round_id,
    messages,
    is_last_round,
    is_loading,
    runtime_phase,
    pending_permissions = [],
    on_permission_response,
    can_respond_to_permissions = true,
    permission_read_only_reason,
    hidden_tool_names = ['TodoWrite'],
    on_edit_user_message,
    on_open_workspace_file,
    on_stop_message,
    default_process_expanded = false,
    assistant_header_action,
    assistant_content_mode = "dm_archived",
    class_name,
  }: MessageItemProps) {
  const [copiedUser, setCopiedUser] = useState(false);
  const [copiedAssistant, setCopiedAssistant] = useState(false);
  const {
    is_open: isProcessExpanded,
    toggle: toggleProcessExpanded,
    set_open: setIsProcessExpanded,
    anchor_ref: processAnchorRef,
  } = useScrollAnchoredState(default_process_expanded);

  // 分离消息 + 合并内容
  const {
    user_message,
    assistant_messages,
    result_message,
    merged_content,
    merged_content_source_message_ids,
    streaming_block_indexes,
  } = useAssistantContentMerge({
    messages,
    is_last_round,
    is_loading,
  });
  const systemMessages = useMemo(() => (
    messages.filter((message): message is SystemMessage => (
      message.role === "system"
      && typeof message.content === "string"
      && Boolean(message.content.trim())
    ))
  ), [messages]);

  // 元数据
  const firstAssistant = assistant_messages[0];
  const model = firstAssistant && 'model' in firstAssistant ? firstAssistant.model : undefined;
  const timestamp = firstAssistant?.timestamp || systemMessages[0]?.timestamp || result_message?.timestamp;

  // Room 并发场景：读取 stream_status（仅 AssistantMessage 携带此字段）
  const stream_status = useMemo(() => {
    const a = firstAssistant as AssistantMessage | undefined;
    return a?.stream_status ?? null;
  }, [firstAssistant]);

  // 统计信息
  const stats = useMemo(() => {
    if (!result_message) return null;
    const cacheHit = result_message.usage?.cache_read_input_tokens;
    return {
      duration: result_message.duration_ms >= 1000
        ? `${(result_message.duration_ms / 1000).toFixed(1)}s`
        : `${result_message.duration_ms}ms`,
      tokens: result_message.usage
        ? `↑ ${result_message.usage.input_tokens} ↓ ${result_message.usage.output_tokens}`
        : null,
      cost: result_message.total_cost_usd !== undefined
        ? `$ ${result_message.total_cost_usd ? result_message.total_cost_usd.toFixed(4) : null}`
        : null,
      cache_hit: cacheHit && cacheHit > 0 ? `💾 ${cacheHit}` : null,
    };
  }, [result_message]);

  // 状态
  const userContent = useMemo(() => {
    if (!user_message || user_message.role !== 'user') return '';
    return typeof user_message.content === 'string' ? user_message.content : '';
  }, [user_message]);

  const {
    matchedPendingPermissionsByToolUseId,
    unmatchedPendingPermissions,
  } = useMemo(() => {
    if (pending_permissions.length === 0) {
      return {
        matchedPendingPermissionsByToolUseId: new Map<string, PendingPermission>(),
        unmatchedPendingPermissions: [] as PendingPermission[],
      };
    }

    const unresolved_tool_use_candidates = collect_unresolved_tool_use_candidates(messages);
    const permission_match_result = match_pending_permissions_to_tool_uses(
      pending_permissions,
      unresolved_tool_use_candidates,
    );
    const matched_permissions_by_tool_use_id = new Map(
      permission_match_result.matched_permissions_by_tool_use_id,
    );

    const unmatched_question_permissions = permission_match_result.unmatched_permissions.filter(
      (permission) => (
        permission.interaction_mode === "question"
        || permission.tool_name === "AskUserQuestion"
      ),
    );
    const unresolved_question_candidates = unresolved_tool_use_candidates.filter(
      (candidate) => (
        candidate.tool_name === "AskUserQuestion"
        && !matched_permissions_by_tool_use_id.has(candidate.tool_use_id)
      ),
    );

    // Room 场景下 AskUserQuestion 的 permission_request 绑定的是占位槽位 msg_id，
    // 不是实际 tool_use 所在的 assistant message_id，精确匹配会天然落空。
    // 这里只对问答工具做安全回退：优先按 round_id 唯一匹配，否则在“单权限 + 单候选”时绑定。
    for (const permission of unmatched_question_permissions) {
      const candidates_by_round = unresolved_question_candidates.filter(
        (candidate) => !matched_permissions_by_tool_use_id.has(candidate.tool_use_id)
          && (
            !permission.caused_by
            || candidate.round_id === permission.caused_by
          ),
      );

      if (candidates_by_round.length === 1) {
        matched_permissions_by_tool_use_id.set(candidates_by_round[0].tool_use_id, permission);
        continue;
      }

      const remaining_candidates = unresolved_question_candidates.filter(
        (candidate) => !matched_permissions_by_tool_use_id.has(candidate.tool_use_id),
      );
      if (remaining_candidates.length === 1 && unmatched_question_permissions.length === 1) {
        matched_permissions_by_tool_use_id.set(remaining_candidates[0].tool_use_id, permission);
      }
    }

    return {
      matchedPendingPermissionsByToolUseId: matched_permissions_by_tool_use_id,
      unmatchedPendingPermissions: permission_match_result.unmatched_permissions.filter(
        (permission) => (
          permission.interaction_mode !== "question"
          && permission.tool_name !== "AskUserQuestion"
        ),
      ),
    };
  }, [messages, pending_permissions]);

  const hiddenToolUseIds = useMemo(() => {
    const nextIds = new Set<string>();
    for (const block of merged_content) {
      if (block.type === "tool_use" && hidden_tool_names.includes(block.name)) {
        nextIds.add(block.id);
      }
    }
    return nextIds;
  }, [merged_content, hidden_tool_names]);

  const visibleOrderedAssistantEntries = useMemo<OrderedAssistantEntry[]>(() => {
    const entries: OrderedAssistantEntry[] = [];

    merged_content.forEach((block, mergedIndex) => {
      if (block.type === "text") {
        if (block.text.trim()) {
          entries.push({
            block,
            merged_index: mergedIndex,
            source_message_id: merged_content_source_message_ids[mergedIndex] || "",
          });
        }
        return;
      }
      if (block.type === "thinking") {
        if (block.thinking?.trim()) {
          entries.push({
            block,
            merged_index: mergedIndex,
            source_message_id: merged_content_source_message_ids[mergedIndex] || "",
          });
        }
        return;
      }
      if (block.type === "tool_use") {
        if (!hidden_tool_names.includes(block.name)) {
          entries.push({
            block,
            merged_index: mergedIndex,
            source_message_id: merged_content_source_message_ids[mergedIndex] || "",
          });
        }
        return;
      }
      if (block.type === "tool_result") {
        if (!hiddenToolUseIds.has(block.tool_use_id)) {
          entries.push({
            block,
            merged_index: mergedIndex,
            source_message_id: merged_content_source_message_ids[mergedIndex] || "",
          });
        }
        return;
      }
      if (block.type === "task_progress") {
        entries.push({
          block,
          merged_index: mergedIndex,
          source_message_id: merged_content_source_message_ids[mergedIndex] || "",
        });
      }
    });

    return entries;
  }, [hiddenToolUseIds, hidden_tool_names, merged_content, merged_content_source_message_ids]);

  const visibleOrderedAssistantContent = useMemo(() => {
    return visibleOrderedAssistantEntries.map((entry) => entry.block);
  }, [visibleOrderedAssistantEntries]);

  const orderedAssistantStreamingIndexes = useMemo(() => {
    const nextIndexes = new Set<number>();

    visibleOrderedAssistantEntries.forEach((entry, visibleIndex) => {
      if (streaming_block_indexes.has(entry.merged_index)) {
        nextIndexes.add(visibleIndex);
      }
    });

    return nextIndexes;
  }, [streaming_block_indexes, visibleOrderedAssistantEntries]);

  const visibleAssistantTurns = useMemo<AssistantTurnEntry[]>(() => {
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

    visibleOrderedAssistantEntries.forEach((entry) => {
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
      .filter((turn): turn is AssistantTurnEntry => Boolean(turn && turn.content.length > 0));
  }, [assistant_messages, streaming_block_indexes, visibleOrderedAssistantEntries]);

  const orderedProjection = useMemo<ContentProjection>(() => ({
    content: visibleOrderedAssistantContent,
    streaming_indexes: orderedAssistantStreamingIndexes,
  }), [orderedAssistantStreamingIndexes, visibleOrderedAssistantContent]);

  const lastAssistantTurn = useMemo(
    () => visibleAssistantTurns.at(-1) ?? null,
    [visibleAssistantTurns],
  );

  const finalAssistantTurn = useMemo(() => {
    for (let index = assistant_messages.length - 1; index >= 0; index -= 1) {
      const message = assistant_messages[index] as AssistantMessage;
      if (!message.parent_id || message.parent_id === round_id) {
        return visibleAssistantTurns.find((turn) => turn.message_id === message.message_id) ?? null;
      }
    }
    return lastAssistantTurn;
  }, [assistant_messages, lastAssistantTurn, round_id, visibleAssistantTurns]);

  const finalTailEntries = useMemo<OrderedAssistantEntry[]>(() => {
    if (!finalAssistantTurn) {
      return [];
    }

    const tail_entries: OrderedAssistantEntry[] = [];
    for (let index = visibleOrderedAssistantEntries.length - 1; index >= 0; index -= 1) {
      const entry = visibleOrderedAssistantEntries[index];
      if (entry.source_message_id !== finalAssistantTurn.message_id) {
        break;
      }
      if (entry.block.type !== "text" || !entry.block.text.trim()) {
        break;
      }
      tail_entries.unshift(entry);
    }
    return tail_entries;
  }, [finalAssistantTurn, visibleOrderedAssistantEntries]);

  const finalTailText = useMemo(() => {
    return finalTailEntries
      .map((entry) => entry.block)
      .filter((block): block is Extract<ContentBlock, { type: "text" }> => block.type === "text")
      .map((block) => block.text)
      .join("\n\n")
      .trim();
  }, [finalTailEntries]);

  const finalAssistantTextMergedIndexes = useMemo(() => {
    if (!finalAssistantTurn || finalAssistantTurn.text_content.length === 0) {
      return new Set<number>();
    }

    const next_indexes = new Set<number>();
    for (const entry of visibleOrderedAssistantEntries) {
      if (entry.source_message_id !== finalAssistantTurn.message_id) {
        continue;
      }
      if (entry.block.type !== "text" || !entry.block.text.trim()) {
        continue;
      }
      next_indexes.add(entry.merged_index);
    }
    return next_indexes;
  }, [finalAssistantTurn, visibleOrderedAssistantEntries]);

  const archivedProcessProjection = useMemo<ContentProjection>(() => {
    const result_text = result_message?.result?.trim();
    const should_strip_tail = finalTailEntries.length > 0
      && (
        !result_text
        || finalTailText === result_text
        || finalTailEntries
          .map((entry) => entry.block)
          .filter((block): block is Extract<ContentBlock, { type: "text" }> => block.type === "text")
          .map((block) => block.text)
          .join("")
          .trim() === result_text
      );

    if (should_strip_tail) {
      const tail_indexes = new Set(finalTailEntries.map((entry) => entry.merged_index));
      return projection_from_ordered_entries(
        visibleOrderedAssistantEntries.filter((entry) => !tail_indexes.has(entry.merged_index)),
        streaming_block_indexes,
      );
    }

    if (!result_text && finalAssistantTurn) {
      return projection_from_ordered_entries(
        visibleOrderedAssistantEntries.filter((entry) => (
          entry.source_message_id !== finalAssistantTurn.message_id
          || !finalAssistantTextMergedIndexes.has(entry.merged_index)
        )),
        streaming_block_indexes,
      );
    }

    return projection_from_ordered_entries(visibleOrderedAssistantEntries, streaming_block_indexes);
  }, [
    finalTailEntries,
    finalTailText,
    finalAssistantTurn,
    finalAssistantTextMergedIndexes,
    result_message,
    streaming_block_indexes,
    visibleOrderedAssistantEntries,
  ]);

  const fallbackFinalAssistantContent = useMemo(() => {
    if (finalTailEntries.length > 0) {
      return finalTailEntries.map((entry) => entry.block);
    }
    if (!finalAssistantTurn) {
      return null;
    }
    if (finalAssistantTurn.text_content.length > 0) {
      return finalAssistantTurn.text_content;
    }
    if (finalAssistantTurn.content.length > 0) {
      return finalAssistantTurn.content;
    }
    return null;
  }, [finalTailEntries, finalAssistantTurn]);

  const fallbackFinalAssistantStreamingIndexes = useMemo(() => {
    if (finalTailEntries.length > 0) {
      const next_indexes = new Set<number>();
      finalTailEntries.forEach((entry, index) => {
        if (streaming_block_indexes.has(entry.merged_index)) {
          next_indexes.add(index);
        }
      });
      return next_indexes;
    }
    if (!finalAssistantTurn) {
      return new Set<number>();
    }
    if (finalAssistantTurn.text_content.length > 0) {
      return finalAssistantTurn.text_streaming_indexes;
    }
    return finalAssistantTurn.streaming_indexes;
  }, [finalAssistantTurn, finalTailEntries, streaming_block_indexes]);

  const directOrderedProjection = useMemo<ContentProjection>(() => {
    if (assistant_content_mode === "dm_live" || assistant_content_mode === "room_thread") {
      return orderedProjection;
    }
    return { content: [], streaming_indexes: new Set<number>() };
  }, [assistant_content_mode, orderedProjection]);

  const processProjection = useMemo<ContentProjection>(() => {
    if (assistant_content_mode === "dm_archived") {
      return archivedProcessProjection;
    }
    return { content: [], streaming_indexes: new Set<number>() };
  }, [archivedProcessProjection, assistant_content_mode]);

  const finalAssistantContent = useMemo<string | ContentBlock[] | null>(() => {
    if (assistant_content_mode === "dm_live" || assistant_content_mode === "room_thread") {
      return null;
    }

    const result_text = result_message?.result?.trim();
    if (result_text) {
      return result_text;
    }

    if (assistant_content_mode === "dm_archived") {
      if (finalTailEntries.length > 0) {
        return finalTailEntries.map((entry) => entry.block);
      }
      if (finalAssistantTurn?.text_content.length) {
        return finalAssistantTurn.text_content;
      }
      return null;
    }

    return fallbackFinalAssistantContent;
  }, [
    assistant_content_mode,
    fallbackFinalAssistantContent,
    finalAssistantTurn,
    finalTailEntries,
    result_message,
  ]);

  const finalAssistantStreamingIndexes = useMemo(() => {
    if (assistant_content_mode === "dm_live" || assistant_content_mode === "room_thread") {
      return new Set<number>();
    }
    if (typeof finalAssistantContent === "string") {
      return new Set<number>();
    }
    return fallbackFinalAssistantStreamingIndexes;
  }, [assistant_content_mode, fallbackFinalAssistantStreamingIndexes, finalAssistantContent]);

  const finalAssistantText = useMemo(() => {
    if (typeof finalAssistantContent === "string") {
      return finalAssistantContent;
    }
    return extract_text_from_content_blocks(finalAssistantContent);
  }, [finalAssistantContent]);
  const shouldRenderDirectAssistantContent = directOrderedProjection.content.length > 0;
  const hasVisibleProcess = processProjection.content.length > 0 || unmatchedPendingPermissions.length > 0;
  const shouldRenderProcessCallchain = assistant_content_mode === "dm_archived" && hasVisibleProcess;
  const hasTimedOutQuestionInProcess = useMemo(
    () => has_timed_out_ask_user_question(processProjection.content),
    [processProjection.content],
  );

  const processSummary = useMemo(() => {
    let toolCount = 0;
    let thinkingCount = 0;
    let errorCount = 0;
    let progressCount = 0;

    for (const block of processProjection.content) {
      if (block.type === "thinking") {
        thinkingCount += 1;
        continue;
      }
      if (block.type === "tool_use") {
        toolCount += 1;
        continue;
      }
      if (block.type === "tool_result" && block.is_error) {
        errorCount += 1;
        continue;
      }
      if (block.type === "task_progress") {
        progressCount += 1;
      }
    }

    if (pending_permissions.length > 0) {
      return "等待你的确认后继续";
    }

    const summaryParts: string[] = [];
    if (thinkingCount > 0) {
      summaryParts.push(`${thinkingCount} 段思路`);
    }
    if (toolCount > 0) {
      summaryParts.push(`${toolCount} 次动作`);
    }
    if (errorCount > 0) {
      summaryParts.push(`${errorCount} 个异常`);
    }
    if (progressCount > 0) {
      summaryParts.push(`${progressCount} 条进度`);
    }

    return summaryParts.length > 0 ? summaryParts.join(" · ") : "查看过程";
  }, [pending_permissions.length, processProjection.content]);

  const liveActivityState = useMemo(() => {
    if (!is_last_round || !is_loading) {
      return null;
    }

    if (pending_permissions.length > 0) {
      return pending_permissions.some((permission) => (
        permission.interaction_mode === "question" || permission.tool_name === "AskUserQuestion"
      ))
        ? "waiting_input"
        : "waiting_permission";
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

    const has_visible_reply_text = merged_content.some((block) => (
      block.type === "text" && Boolean(block.text.trim())
    ));
    if (has_visible_reply_text && (stream_status === "streaming" || assistant_messages.length > 0)) {
      return "replying";
    }

    if (stream_status === "pending") {
      return "thinking";
    }

    return map_runtime_phase_to_activity_state(runtime_phase)
      ?? (assistant_messages.length > 0 ? "replying" : "thinking");
  }, [
    assistant_messages.length,
    is_last_round,
    is_loading,
    merged_content,
    pending_permissions,
    runtime_phase,
    stream_status,
    streaming_block_indexes,
  ]);

  const shouldHideAssistantContent = useMemo(() => {
    if (liveActivityState) {
      return false;
    }

    if (unmatchedPendingPermissions.length > 0) {
      return false;
    }

    if (systemMessages.length > 0) {
      return false;
    }

    if (
      stream_status === 'pending'
      || stream_status === 'streaming'
      || stream_status === 'cancelled'
      || stream_status === 'error'
    ) {
      return false;
    }

    if (directOrderedProjection.content.length > 0) {
      return false;
    }
    if (processProjection.content.length > 0) {
      return false;
    }
    if (typeof finalAssistantContent === "string") {
      return !finalAssistantContent.trim();
    }
    if (finalAssistantContent && finalAssistantContent.length > 0) {
      return false;
    }
    return !result_message;
  }, [
    directOrderedProjection.content.length,
    finalAssistantContent,
    liveActivityState,
    processProjection.content.length,
    result_message,
    stream_status,
    systemMessages.length,
    unmatchedPendingPermissions.length,
  ]);

  const shouldRenderAssistantText = Boolean(
    typeof finalAssistantContent === "string"
      ? finalAssistantContent.trim()
      : finalAssistantContent?.length,
  );

  const shouldRenderStandaloneActivityStatus = Boolean(
    liveActivityState
    && !shouldRenderDirectAssistantContent
    && !shouldRenderProcessCallchain
    && !shouldRenderAssistantText
  );

  useEffect(() => {
    if (pending_permissions.length > 0) {
      setIsProcessExpanded(true);
    }
  }, [pending_permissions.length, setIsProcessExpanded]);

  useEffect(() => {
    if (hasTimedOutQuestionInProcess) {
      setIsProcessExpanded(true);
    }
  }, [hasTimedOutQuestionInProcess, setIsProcessExpanded]);

  // 操作
  const handleCopyUser = useCallback(async () => {
    if (!userContent) return;
    try {
      await navigator.clipboard.writeText(userContent);
      setCopiedUser(true);
      setTimeout(() => setCopiedUser(false), 2000);
    } catch (error) {
      console.error('Failed to copy:', error);
    }
  }, [userContent]);

  const handleCopyAssistant = useCallback(async () => {
    const text = finalAssistantText;
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      setCopiedAssistant(true);
      setTimeout(() => setCopiedAssistant(false), 2000);
    } catch (error) {
      console.error('Failed to copy:', error);
    }
  }, [finalAssistantText]);

  const showCursor = Boolean(
    is_last_round
    && is_loading
    && (
      streaming_block_indexes.size > 0
      || assistant_messages.length > 0
      || pending_permissions.length > 0
      || stream_status === 'pending'
      || stream_status === 'streaming'
    ),
  );
  const finalAssistantIsStreaming = Boolean(
    showCursor
    && typeof finalAssistantContent !== "string"
    && finalAssistantStreamingIndexes.size > 0,
  );
  const canCopyAssistant = Boolean(finalAssistantText?.trim());
  const shouldShowAssistantFooter = (
    assistant_content_mode === "dm_archived" || assistant_content_mode === "room_result"
  ) && !is_loading && (Boolean(stats) || canCopyAssistant);

  // Per-message stop: visible when this bubble is actively pending/streaming
  const can_stop_message = on_stop_message && (stream_status === 'pending' || stream_status === 'streaming');
  const handle_stop_message = useCallback(() => {
    if (!on_stop_message || !firstAssistant) return;
    on_stop_message(firstAssistant.message_id);
  }, [on_stop_message, firstAssistant]);
  const is_room_thread_mode = assistant_content_mode === "room_thread";
  const pendingPermissionBlock = unmatchedPendingPermissions.length > 0 ? (
    <div className={cn(
      "mt-3 flex flex-col gap-3",
      is_room_thread_mode
        ? "border-t border-(--divider-subtle-color) pt-3"
        : "rounded-2xl bg-(--surface-inset-background) p-3",
    )}>
      {unmatchedPendingPermissions.map((permission) => (
        <ToolBlock
          key={permission.request_id}
          tool_use={{
            type: "tool_use",
            id: `pending_${permission.request_id}`,
            name: permission.tool_name,
            input: permission.tool_input,
          }}
          status="waiting_permission"
          permission_request={{
            request_id: permission.request_id,
            tool_input: permission.tool_input,
            risk_level: permission.risk_level,
            risk_label: permission.risk_label,
            summary: permission.summary,
            suggestions: permission.suggestions,
            expires_at: permission.expires_at,
            on_allow: (updated_permissions) => on_permission_response?.({
              request_id: permission.request_id,
              decision: "allow",
              updated_permissions,
            }),
            on_deny: (updated_permissions) => on_permission_response?.({
              request_id: permission.request_id,
              decision: "deny",
              updated_permissions,
            }),
          }}
          interaction_disabled={!can_respond_to_permissions}
          interaction_disabled_reason={permission_read_only_reason}
        />
      ))}
    </div>
  ) : null;

  // Pretext-based streaming min-height: measure the current assistant text
  // and hold the container at that height so scroll doesn't jump on each token.
  // Throttled to run at most once every 150ms — pretext layout is fast but
  // calling it on every token (100/sec) would still burn meaningful CPU.
  const contentAreaRef = useRef<HTMLDivElement>(null);
  const streamingMinHeight = useRef(60);
  const layoutThrottleRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    const layout_text = assistant_content_mode === "dm_live" || assistant_content_mode === "room_thread"
      ? extract_text_from_content_blocks(directOrderedProjection.content)
      : finalAssistantText;

    if (!showCursor || !layout_text) return;
    if (layoutThrottleRef.current !== null) return; // already scheduled

    layoutThrottleRef.current = setTimeout(() => {
      layoutThrottleRef.current = null;
      const el = contentAreaRef.current;
      if (!el) return;
      try {
        const width = el.offsetWidth || 640;
        const prepared = prepare(layout_text, "400 14px ui-sans-serif, system-ui, sans-serif");
        const result = layout(prepared, width, 28);
        streamingMinHeight.current = Math.max(streamingMinHeight.current, result.height);
      } catch { /* keep previous estimate */ }
    }, 150);
  }, [assistant_content_mode, directOrderedProjection.content, finalAssistantText, showCursor]);

  // Reset on new stream; cancel any pending throttled layout
  useEffect(() => {
    if (!showCursor) {
      streamingMinHeight.current = 60;
      if (layoutThrottleRef.current !== null) {
        clearTimeout(layoutThrottleRef.current);
        layoutThrottleRef.current = null;
      }
    }
  }, [showCursor]);

  // 格式化时间
  const format_time = (ts: number) => {
    const date = new Date(ts);
    return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
  };

  return (
    <MessageShell
      class_name={cn(
        "animate-in fade-in slide-in-from-bottom-2 space-y-2 py-3 duration-300",
        class_name,
      )}
      separated={!compact}
    >

      {/* ═══════════════════════ 用户消息 ═══════════════════════ */}
      {user_message && (
        <div className={cn("w-full", compact ? "px-0" : "px-2 sm:px-3")}>
          <div className="w-full">
            <div className={cn(
              "group flex min-w-0 justify-end",
              compact ? "" : "gap-3",
            )}>
              <div className="relative ml-auto min-w-0 max-w-[min(100%,720px)]">
                {/* 头部 */}
                <div className={cn(
                  "flex items-center justify-end gap-2",
                  compact ? "h-6" : "h-7",
                )}>
                  <div className="flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
                    {on_edit_user_message ? (
                      <MessageActionButton
                        aria-label="编辑消息"
                        onClick={() => {
                          const newContent = prompt('编辑消息:', userContent);
                          if (newContent && newContent !== userContent) {
                            on_edit_user_message(user_message.message_id, newContent);
                          }
                        }}
                        tone="default"
                      >
                        <Edit2 className="w-3 h-3" />
                      </MessageActionButton>
                    ) : null}
                    <MessageActionButton
                      aria-label="复制消息"
                      onClick={handleCopyUser}
                      tone={copiedUser ? "success" : "default"}
                    >
                      {copiedUser ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                    </MessageActionButton>
                  </div>

                  <span className="hidden shrink-0 text-xs text-(--text-muted) sm:inline">
                    {user_message.timestamp ? format_time(user_message.timestamp) : "--:--"}
                  </span>
                  <span className="shrink-0 text-sm font-bold text-(--text-strong)">你</span>
                  <MessageAvatar class_name="shrink-0" size={compact ? "compact" : "full"}>
                    <User className={compact ? "h-3 w-3" : "h-4 w-4"} />
                  </MessageAvatar>
                </div>

                {/* 内容 */}
                <div className="rounded-2xl bg-[color-mix(in_srgb,var(--primary)_6%,var(--material-card-background))] px-4 py-3">
                  <p className={cn(
                    "w-full",
                    "message-cjk-font whitespace-pre-wrap text-left text-(--text-strong) wrap-anywhere",
                    compact ? "text-[15px] leading-6" : "text-[16px] leading-7",
                  )}>
                    {userContent}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ═══════════════════════ 助手消息 ═══════════════════════ */}
      {!shouldHideAssistantContent && (
        <div className={cn("w-full", compact ? "px-0" : "px-2 sm:px-3")}>
          <div className={cn("w-full", compact ? "max-w-full" : "max-w-[980px]")}>
            <div className={cn(
              "group grid min-w-0",
              compact ? "grid-cols-[minmax(0,1fr)]" : "grid-cols-[40px_minmax(0,1fr)] gap-3",
            )}>
              {!compact ? (
                <MessageAvatar avatar_url={current_agent_avatar}>
                  {!current_agent_avatar && <Bot className="h-4 w-4" />}
                </MessageAvatar>
              ) : null}

              <div className="relative min-w-0">
                {/* 优雅的头部栏 */}
                <div className={cn(
                  "flex min-w-0 items-center gap-2",
                  compact ? "min-h-6 pb-0" : "h-7 pb-0.5",
                )}>
                  {compact ? (
                    <MessageAvatar class_name="shrink-0" size="compact" avatar_url={current_agent_avatar}>
                      {!current_agent_avatar && <Bot className="h-3 w-3" />}
                    </MessageAvatar>
                  ) : null}
                  <span className="shrink-0 text-sm font-bold text-(--text-strong)">
                    {current_agent_name || "协作成员"}
                  </span>

                  {/* 时间 */}
                  <span className="hidden shrink-0 text-xs text-(--text-muted) sm:inline">
                    {timestamp ? format_time(timestamp) : "--:--"}
                  </span>

                  {/* 模型 */}
                  {model ? <span className="min-w-0 truncate text-xs text-(--text-soft)">{model}</span> : null}

                  <div className="flex-1" />

                  {assistant_header_action ? (
                    <div className="shrink-0">
                      {assistant_header_action}
                    </div>
                  ) : null}

                  {/* Per-message stop button (Room 并发模式) */}
                  {can_stop_message && (
                    <MessageActionButton
                      type="button"
                      aria-label="停止生成"
                      onClick={handle_stop_message}
                      class_name="flex items-center gap-1 px-1.5 py-0.5 text-xs"
                      tone="default"
                    >
                      <Square className="h-3 w-3 fill-current" />
                      <span>停止</span>
                    </MessageActionButton>
                  )}

                </div>

                {/* 内容区 */}
                <div
                  ref={contentAreaRef}
                  className={cn(
                    "min-w-0 max-w-full overflow-x-hidden pb-2 pt-1 text-left",
                    compact ? "text-[15px] leading-6" : "text-[16px] leading-7",
                  )}
                  style={showCursor ? { minHeight: streamingMinHeight.current } : undefined}
                >
                  {shouldRenderStandaloneActivityStatus ? (
                    <MessageActivityStatus class_name="py-1" state={liveActivityState!} />
                  ) : null}

                  {systemMessages.length > 0 ? (
                    <div className="mb-3 flex flex-col gap-2">
                      {systemMessages.map((message) => {
                        const display_meta = get_system_message_display_meta(message);
                        return (
                          <div
                            key={message.message_id}
                            className={cn(
                              "flex items-start gap-2 rounded-2xl px-3 py-2.5",
                              get_system_message_container_class_name(display_meta.tone),
                              is_room_thread_mode && display_meta.tone === "neutral"
                                ? "border border-(--divider-subtle-color) bg-transparent text-(--text-default)"
                                : null,
                            )}
                          >
                            <RotateCcw
                              className={cn(
                                "mt-0.5 h-3.5 w-3.5 shrink-0",
                                get_system_message_icon_class_name(display_meta.tone),
                              )}
                            />
                            <div className="min-w-0">
                              <p className="text-[11px] font-semibold uppercase tracking-[0.14em]">
                                {display_meta.label}
                              </p>
                              <p className="mt-0.5 whitespace-pre-wrap text-[13px] leading-6 wrap-anywhere">
                                {message.content}
                              </p>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  ) : null}

                  {/* Room 并发：已取消标记 */}
                  {stream_status === 'cancelled' && merged_content.length === 0 && (
                    <span className="text-xs italic text-(--text-soft)">已停止</span>
                  )}

                  {stream_status === 'error' && merged_content.length === 0 && (
                    <span className="text-xs text-rose-500 italic">执行失败</span>
                  )}

                  {shouldRenderDirectAssistantContent ? (
                    <div>
                      <ContentRenderer
                        content={directOrderedProjection.content}
                        is_streaming={showCursor}
                        streaming_block_indexes={directOrderedProjection.streaming_indexes}
                        fallback_activity_state={liveActivityState}
                        pending_permissions_by_tool_use_id={matchedPendingPermissionsByToolUseId}
                        on_permission_response={on_permission_response}
                        can_respond_to_permissions={can_respond_to_permissions}
                        permission_read_only_reason={permission_read_only_reason}
                        on_open_workspace_file={on_open_workspace_file}
                        hidden_tool_names={hidden_tool_names}
                      />
                      {pendingPermissionBlock}
                    </div>
                  ) : null}

                  {shouldRenderProcessCallchain ? (
                    <div ref={processAnchorRef as React.RefObject<HTMLDivElement>}>
                      <button
                        className="flex w-full items-center gap-2 py-1.5 text-left text-(--text-muted) transition-colors duration-(--motion-duration-fast) hover:text-(--text-strong)"
                        onClick={toggleProcessExpanded}
                        type="button"
                      >
                        <Wrench className="h-3 w-3 shrink-0 text-(--icon-muted)" />
                        <div className="min-w-0 flex-1 truncate text-[12px] font-medium text-(--text-muted)">
                          {processSummary}
                        </div>
                        <div className="text-(--icon-muted)">
                          {isProcessExpanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
                        </div>
                      </button>

                      {isProcessExpanded ? (
                        <div className="pt-1">
                          <ContentRenderer
                            content={processProjection.content}
                            is_streaming={showCursor}
                            streaming_block_indexes={processProjection.streaming_indexes}
                            fallback_activity_state={liveActivityState}
                            pending_permissions_by_tool_use_id={matchedPendingPermissionsByToolUseId}
                            on_permission_response={on_permission_response}
                            can_respond_to_permissions={can_respond_to_permissions}
                            permission_read_only_reason={permission_read_only_reason}
                            on_open_workspace_file={on_open_workspace_file}
                            hidden_tool_names={hidden_tool_names}
                          />

                          {pendingPermissionBlock}
                        </div>
                      ) : null}
                    </div>
                  ) : null}

                  {shouldRenderAssistantText ? (
                    <div className={cn(shouldRenderProcessCallchain)}>
                      <ContentRenderer
                        content={finalAssistantContent ?? []}
                        is_streaming={finalAssistantIsStreaming}
                        streaming_block_indexes={finalAssistantStreamingIndexes}
                        fallback_activity_state={liveActivityState}
                        on_open_workspace_file={on_open_workspace_file}
                      />
                    </div>
                  ) : null}

                  {!shouldRenderDirectAssistantContent && !shouldRenderProcessCallchain && pendingPermissionBlock ? (
                    <div className="pt-2">
                      {pendingPermissionBlock}
                    </div>
                  ) : null}
                </div>

                {/* 底部统计栏（完成后显示） */}
                {shouldShowAssistantFooter && (
                  <MessageStats
                    stats={stats || undefined}
                    show_cursor={showCursor}
                    compact={compact}
                    copied_assistant={copiedAssistant}
                    on_copy_assistant={canCopyAssistant ? handleCopyAssistant : undefined}
                  />
                )}

              </div>
            </div>
          </div>
        </div>
      )}
    </MessageShell>
  );
}

// 仅在影响视觉输出的关键属性变化时重新渲染，避免流式阶段产生无效更新。
export const MessageItem = memo(MessageItemInner, (prev, next) => {
  if (prev.round_id !== next.round_id) return false;
  if (prev.is_last_round !== next.is_last_round) return false;
  if (prev.is_loading !== next.is_loading) return false;
  if (prev.runtime_phase !== next.runtime_phase) return false;
  if (prev.compact !== next.compact) return false;
  if (prev.current_agent_name !== next.current_agent_name) return false;
  if (prev.pending_permissions !== next.pending_permissions) return false;
  if (prev.can_respond_to_permissions !== next.can_respond_to_permissions) return false;
  if (prev.permission_read_only_reason !== next.permission_read_only_reason) return false;
  if (prev.assistant_header_action !== next.assistant_header_action) return false;
  if (prev.assistant_content_mode !== next.assistant_content_mode) return false;
  if (prev.class_name !== next.class_name) return false;
  // 消息数组按引用比较，上游流式合并会返回新数组，足以标记内容变化。
  if (prev.messages !== next.messages) return false;
  // 回调由上游 useCallback 保持稳定，这里不做深比较以避免额外开销。
  return true;
});

export default MessageItem;
