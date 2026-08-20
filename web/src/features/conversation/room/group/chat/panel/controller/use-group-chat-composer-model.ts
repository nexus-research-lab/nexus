/**
 * INPUT: Room Composer 会话、成员 Session、精确 Agent execution/slot/stopping 状态与发送资源。
 * OUTPUT: 带共享本机目录和点击时精确目标快照“全部停止”的 Room Composer 模型。
 * POS: Room 会话能力到共享 Composer Props 的唯一动作装配边界。
 */
import { useCallback } from "react";

import { prepareRoomConversationAttachments } from "@/features/conversation/shared/composer/attachments/composer-attachments";
import { useConversationComposerHandlers } from "@/features/conversation/shared/composer/use-conversation-composer-handlers";
import { ROOM_GOAL_SCOPE_LABEL } from "@/features/conversation/shared/goal/goal-continuation-hold";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import {
  buildComposerDraftScopeKey,
  buildComposerHistoryScopeKey,
} from "@/features/conversation/shared/composer/composer-draft-scope";
import { useI18n } from "@/shared/i18n/i18n-context";
import { buildRoomAgentSessionKey } from "@/lib/conversation/session-key";
import type { Agent } from "@/types/agent/agent";
import type { UseAgentConversationReturn } from "@/types/agent/agent-conversation";
import type { LoopCatalogItem } from "@/types/capability/loop";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

import type { GroupChatComposerModel } from "../view/group-chat-panel-view";
import {
  buildRoomLoopGoalMetadata,
  buildRoomLoopGoalObjective,
} from "../../room-goal-model";
import { projectRoomPendingInputQueueItems } from "./group-chat-panel-projection";
import type { RoomGoalComposerModel } from "./use-room-goal-composer";

type ComposerConversation = Pick<
  UseAgentConversationReturn,
  | "delete_input_queue_message"
  | "command_catalog"
  | "context_usage"
  | "context_usage_by_agent"
  | "enqueue_input_queue_message"
  | "guide_input_queue_message"
  | "input_queue_items"
  | "is_loading"
  | "pending_agent_slots"
  | "reorder_input_queue_messages"
  | "room_agent_execution_states"
  | "runtime_phase"
  | "send_message"
  | "set_goal"
  | "stop_generation"
  | "stopping_agent_round_ids"
>;

interface UseGroupChatComposerModelOptions {
  agentId: string | null;
  conversation: ComposerConversation;
  conversationId: string | null;
  goal: RoomGoalComposerModel;
  initialDraft: string | null;
  onInitialDraftConsumed?: () => void;
  roomId: string | null;
  roomMembers: Agent[];
  scrollToBottom: (behavior?: ScrollBehavior) => void;
  sessionKey: string | null;
  runtimeKind: AgentRuntimeKind;
}

export function useGroupChatComposerModel({
  agentId,
  conversation,
  conversationId,
  goal,
  initialDraft,
  onInitialDraftConsumed,
  roomId,
  roomMembers,
  scrollToBottom,
  sessionKey,
  runtimeKind,
}: UseGroupChatComposerModelOptions): GroupChatComposerModel {
  const { t } = useI18n();
  const defaultDeliveryPolicy = useDefaultChatDeliveryPolicy();
  const draftScopeKey = buildComposerDraftScopeKey({ roomId, sessionKey });
  const historyScopeKey = buildComposerHistoryScopeKey({ roomId });
  const localDirectoryAgentId = agentId ?? roomMembers[0]?.agent_id ?? null;
  const prepareAttachments = useCallback(
    async (files: File[]) => {
      if (!roomId || !conversationId) {
        throw new Error(t("room.attachment_session_not_ready"));
      }
      return prepareRoomConversationAttachments(roomId, conversationId, files);
    },
    [conversationId, roomId, t],
  );
  const handlers = useConversationComposerHandlers({
    canSendInitialDraft: true,
    initialDraft,
    initialDraftLogLabel: "room",
    isLoading: conversation.is_loading,
    onInitialDraftConsumed,
    prepareAttachments,
    scrollToBottom,
    sendMessage: conversation.send_message,
    sessionKey,
  });
  const {
    pending_agent_slots: pendingAgentSlots,
    room_agent_execution_states: roomAgentExecutionStates,
    set_goal: setGoal,
    stop_generation: stopGeneration,
    stopping_agent_round_ids: stoppingAgentRoundIds,
  } = conversation;
  const activeAgentRoundIds = collectActiveRoomAgentRoundIds({
    pending_agent_slots: pendingAgentSlots,
    room_agent_execution_states: roomAgentExecutionStates,
    stopping_agent_round_ids: stoppingAgentRoundIds,
  });
  const stopAllAgentOutputs = useCallback(() => {
    stopRoomAgentOutputs(
      collectActiveRoomAgentRoundIds({
        pending_agent_slots: pendingAgentSlots,
        room_agent_execution_states: roomAgentExecutionStates,
        stopping_agent_round_ids: stoppingAgentRoundIds,
      }),
      stopGeneration,
    );
  }, [
    pendingAgentSlots,
    roomAgentExecutionStates,
    stopGeneration,
    stoppingAgentRoundIds,
  ]);
  const createGoal = useCallback(async (
    objective: string,
    metadata?: Record<string, unknown>,
  ) => {
    if (!sessionKey) {
      throw new Error(t("room.goal_session_not_ready"));
    }
    const leadAgentId = goal.leadAgentId.trim();
    if (!leadAgentId) {
      throw new Error(t("room.goal_lead_required"));
    }
    await setGoal(objective, {
      ...(metadata ? { metadata } : {}),
      replace_existing: true,
      target_agent_ids: [leadAgentId],
      token_budget: null,
    });
  }, [goal.leadAgentId, sessionKey, setGoal, t]);
  const createLoopGoal = useCallback(async (loop: LoopCatalogItem) => {
    await createGoal(
      buildRoomLoopGoalObjective(loop),
      buildRoomLoopGoalMetadata(loop),
    );
  }, [createGoal]);

  return {
    commandCatalog: conversation.command_catalog,
    contextUsage: conversation.context_usage,
    contextUsageItems: roomMembers.map((member) => ({
      agentId: member.agent_id,
      avatar: member.avatar,
      name: member.name,
      usage: conversation.context_usage_by_agent[member.agent_id] ?? null,
    })),
    defaultDeliveryPolicy,
    draftScopeKey,
    enableLoops: true,
    goalCreateDisabledReason: goal.createDisabledReason,
    goalScopeLabel: ROOM_GOAL_SCOPE_LABEL,
    historyScopeKey,
    inputQueueItems: projectRoomPendingInputQueueItems(
      conversation.input_queue_items,
    ),
    isLoading: conversation.is_loading,
    localDirectorySessionKey: conversationId && localDirectoryAgentId
      ? buildRoomAgentSessionKey(
          conversationId,
          localDirectoryAgentId,
        )
      : undefined,
    onCreateGoal: sessionKey
      ? (objective: string) => createGoal(objective)
      : undefined,
    onCreateLoopGoal: sessionKey ? createLoopGoal : undefined,
    onDeleteQueuedMessage: conversation.delete_input_queue_message,
    onEnqueueMessage: conversation.enqueue_input_queue_message,
    onGuideQueuedMessage: conversation.guide_input_queue_message,
    onPrepareAttachments: handlers.handlePrepareAttachments,
    onReorderQueueMessages: conversation.reorder_input_queue_messages,
    onSendMessage: handlers.handleSendMessage,
    onStop: activeAgentRoundIds.length > 0
      ? stopAllAgentOutputs
      : undefined,
    queueWhenSessionBusy: true,
    roomMembers,
    runtimePhase: conversation.runtime_phase,
    runtimeKind,
    stopLabel: t("room.stop_all_outputs"),
    sessionSettings: conversationId && roomMembers.length > 0
      ? {
          initialTargetId: agentId ?? roomMembers[0].agent_id,
          runtimeKind,
          targets: roomMembers.map((member) => ({
            agentId: member.agent_id,
            avatar: member.avatar,
            defaultConnectorIds: member.options.connector_ids,
            defaultModel: member.options.model,
            defaultPermissionMode: member.options.permission_mode,
            defaultProvider: member.options.provider,
            name: member.name,
            sessionKey: buildRoomAgentSessionKey(
              conversationId,
              member.agent_id,
            ),
          })),
        }
      : undefined,
    tourAnchor: CONVERSATION_TOUR_ANCHORS.composer,
  };
}

type RoomStopConversation = Pick<
  ComposerConversation,
  | "pending_agent_slots"
  | "room_agent_execution_states"
  | "stopping_agent_round_ids"
>;

/** 按 execution 首见顺序冻结当前可停止目标，并用 slot 补齐 ACK 前空窗。 */
export function collectActiveRoomAgentRoundIds(
  conversation: RoomStopConversation,
): string[] {
  const stopping = new Set(
    conversation.stopping_agent_round_ids.map((value) => value.trim()),
  );
  const result: string[] = [];
  const seen = new Set<string>();
  const append = (agentRoundId: string): void => {
    const normalizedAgentRoundId = agentRoundId.trim();
    if (
      !normalizedAgentRoundId
      || stopping.has(normalizedAgentRoundId)
      || seen.has(normalizedAgentRoundId)
    ) {
      return;
    }
    seen.add(normalizedAgentRoundId);
    result.push(normalizedAgentRoundId);
  };
  for (const state of conversation.room_agent_execution_states) {
    if (state.phase !== "terminal") {
      append(state.agent_round_id);
    }
  }
  for (const slot of conversation.pending_agent_slots) {
    if (slot.status === "pending" || slot.status === "streaming") {
      append(slot.agent_round_id);
    }
  }
  return result;
}

/** 点击后立即复制目标，某个同步停止回调不得改变本批后续成员。 */
export function stopRoomAgentOutputs(
  activeAgentRoundIds: readonly string[],
  stopGeneration: (agentRoundId: string) => void,
): void {
  const targetSnapshot = [...activeAgentRoundIds];
  for (const agentRoundId of targetSnapshot) {
    stopGeneration(agentRoundId);
  }
}
