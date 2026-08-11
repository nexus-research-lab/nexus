/**
 * INPUT: Room 身份、成员、宿主 Agent、当前 Session 与 Goal 创建动作。
 * OUTPUT: 按 Session 隔离的 Goal 负责人选择、创建能力与刷新序列。
 * POS: Room Composer Goal 模式的领域控制器。
 */
import { useCallback, useEffect, useMemo, useState } from "react";

import { buildComposerDraftScopeKey } from "@/features/conversation/shared/composer/composer-draft-scope";
import { useComposerDraftStore } from "@/features/conversation/shared/composer/composer-draft-store";
import { createGoalApi } from "@/lib/api/conversation/goal-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";
import type { LoopCatalogItem } from "@/types/capability/loop";

import {
  buildRoomGoalMetadata,
  buildRoomLoopGoalMetadata,
  buildRoomLoopGoalObjective,
  resolveDefaultRoomGoalLead,
} from "../../room-goal-model";

interface UseRoomGoalComposerOptions {
  roomId: string | null;
  roomHostAgentId: string | null;
  roomMembers: Agent[];
  sessionKey: string | null;
}

export interface RoomGoalComposerModel {
  createDisabledReason: string | null;
  leadAgentId: string;
  onCreateGoal: (objective: string) => Promise<void>;
  onCreateLoopGoal: (loop: LoopCatalogItem) => Promise<void>;
  refresh: () => void;
  refreshSequence: number;
  setLeadAgentId: (agentId: string) => void;
}

export function useRoomGoalComposer({
  roomId,
  roomHostAgentId,
  roomMembers,
  sessionKey,
}: UseRoomGoalComposerOptions): RoomGoalComposerModel {
  const { t } = useI18n();
  const draftScopeKey = useMemo(
    () => buildComposerDraftScopeKey({ roomId, sessionKey }),
    [roomId, sessionKey],
  );
  const defaultLeadAgentId = useMemo(
    () => resolveDefaultRoomGoalLead(roomMembers, roomHostAgentId),
    [roomHostAgentId, roomMembers],
  );
  const storedLeadAgentId = useComposerDraftStore(
    (state) => (
      state.drafts_by_scope[draftScopeKey]?.goalLeadAgentId ?? null
    ),
  );
  const updateComposerDraft = useComposerDraftStore(
    (state) => state.update_composer_draft,
  );
  const leadAgentId = storedLeadAgentId ?? defaultLeadAgentId;
  const setLeadAgentId = useCallback((agentId: string) => {
    updateComposerDraft(draftScopeKey, (current) => ({
      ...current,
      goalLeadAgentId: agentId,
    }));
  }, [draftScopeKey, updateComposerDraft]);
  const [refreshSequence, setRefreshSequence] = useState(0);

  useEffect(() => {
    if (storedLeadAgentId === null || storedLeadAgentId.trim() === "") {
      return;
    }
    const isCurrentMember = roomMembers.some(
      (agent) => agent.agent_id === storedLeadAgentId,
    );
    if (!isCurrentMember) {
      setLeadAgentId(defaultLeadAgentId);
    }
  }, [
    defaultLeadAgentId,
    roomMembers,
    setLeadAgentId,
    storedLeadAgentId,
  ]);

  const refresh = useCallback(() => {
    setRefreshSequence((value) => value + 1);
  }, []);
  const createGoal = useCallback(
    async (
      objective: string,
      metadata: Record<string, unknown>,
      roomLeadAgentId: string,
    ) => {
      if (!sessionKey) {
        throw new Error(t("room.goal_session_not_ready"));
      }
      await createGoalApi({
        metadata,
        objective,
        replace_existing: true,
        room_lead_agent_id: roomLeadAgentId,
        session_key: sessionKey,
        token_budget: null,
      });
      refresh();
    },
    [refresh, sessionKey, t],
  );
  const requireLeadAgentId = useCallback(() => {
    const normalized = leadAgentId.trim();
    if (!normalized) {
      throw new Error(t("room.goal_lead_required"));
    }
    return normalized;
  }, [leadAgentId, t]);
  const onCreateGoal = useCallback(
    async (objective: string) => {
      const leadAgent = requireLeadAgentId();
      await createGoal(
        objective,
        buildRoomGoalMetadata(roomMembers),
        leadAgent,
      );
    },
    [createGoal, requireLeadAgentId, roomMembers],
  );
  const onCreateLoopGoal = useCallback(
    async (loop: LoopCatalogItem) => {
      const leadAgent = requireLeadAgentId();
      await createGoal(
        buildRoomLoopGoalObjective(loop),
        buildRoomLoopGoalMetadata(roomMembers, loop),
        leadAgent,
      );
    },
    [createGoal, requireLeadAgentId, roomMembers],
  );

  return {
    createDisabledReason: resolveCreateDisabledReason(
      roomMembers,
      leadAgentId,
      t("room.goal_no_assignable_agent"),
      t("room.goal_lead_required"),
    ),
    leadAgentId,
    onCreateGoal,
    onCreateLoopGoal,
    refresh,
    refreshSequence,
    setLeadAgentId,
  };
}

function resolveCreateDisabledReason(
  roomMembers: Agent[],
  leadAgentId: string,
  noAssignableAgentMessage: string,
  leadRequiredMessage: string,
): string | null {
  if (roomMembers.length === 0) {
    return noAssignableAgentMessage;
  }
  return leadAgentId.trim() === "" ? leadRequiredMessage : null;
}
