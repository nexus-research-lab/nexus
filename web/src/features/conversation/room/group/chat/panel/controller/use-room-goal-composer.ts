/**
 * INPUT: Room 身份、成员、宿主 Agent、当前 Session 与 Goal 事件刷新信号。
 * OUTPUT: 按 Session 隔离的 Goal 负责人选择、创建能力与刷新序列。
 * POS: Room Composer Goal 负责人控制器；创建由独立宿主 Goal 控制链负责。
 */
import { useCallback, useEffect, useMemo, useState } from "react";

import { buildComposerDraftScopeKey } from "@/features/conversation/shared/composer/composer-draft-scope";
import { useComposerDraftStore } from "@/features/conversation/shared/composer/composer-draft-store";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";

import {
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
  const [refreshSequence, setRefreshSequence] = useState(0);
  const setLeadAgentId = useCallback((agentId: string) => {
    updateComposerDraft(draftScopeKey, (current) => ({
      ...current,
      goalLeadAgentId: agentId,
    }));
  }, [draftScopeKey, updateComposerDraft]);

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

  return {
    createDisabledReason: resolveCreateDisabledReason(
      roomMembers,
      leadAgentId,
      t("room.goal_no_assignable_agent"),
      t("room.goal_lead_required"),
    ),
    leadAgentId,
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
