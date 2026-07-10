import { useCallback, useEffect, useMemo, useState } from "react";

import { createGoalApi } from "@/lib/api/goal-api";
import type { Agent } from "@/types/agent/agent";
import type { LoopCatalogItem } from "@/types/capability/loop";

import {
  buildRoomGoalMetadata,
  buildRoomLoopGoalMetadata,
  buildRoomLoopGoalObjective,
  resolveDefaultRoomGoalLead,
} from "../room-goal-model";

interface UseRoomGoalComposerOptions {
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
  roomHostAgentId,
  roomMembers,
  sessionKey,
}: UseRoomGoalComposerOptions): RoomGoalComposerModel {
  const defaultLeadAgentId = useMemo(
    () => resolveDefaultRoomGoalLead(roomMembers, roomHostAgentId),
    [roomHostAgentId, roomMembers],
  );
  const [leadAgentId, setLeadAgentId] = useState(defaultLeadAgentId);
  const [refreshSequence, setRefreshSequence] = useState(0);

  useEffect(() => {
    setLeadAgentId((current) => {
      const isCurrentMember = roomMembers.some(
        (agent) => agent.agent_id === current,
      );
      return isCurrentMember ? current : defaultLeadAgentId;
    });
  }, [defaultLeadAgentId, roomMembers]);

  const refresh = useCallback(() => {
    setRefreshSequence((value) => value + 1);
  }, []);
  const createGoal = useCallback(
    async (
      objective: string,
      metadata: Record<string, unknown>,
    ) => {
      if (!sessionKey) {
        throw new Error("当前房间会话尚未准备好，暂时无法启动 Goal。");
      }
      await createGoalApi({
        metadata,
        objective,
        session_key: sessionKey,
        token_budget: null,
      });
      refresh();
    },
    [refresh, sessionKey],
  );
  const requireLeadAgentId = useCallback(() => {
    const normalized = leadAgentId.trim();
    if (!normalized) {
      throw new Error("请选择 Room Goal 负责人。");
    }
    return normalized;
  }, [leadAgentId]);
  const onCreateGoal = useCallback(
    async (objective: string) => {
      const leadAgent = requireLeadAgentId();
      await createGoal(
        objective,
        buildRoomGoalMetadata(roomMembers, leadAgent),
      );
    },
    [createGoal, requireLeadAgentId, roomMembers],
  );
  const onCreateLoopGoal = useCallback(
    async (loop: LoopCatalogItem) => {
      const leadAgent = requireLeadAgentId();
      await createGoal(
        buildRoomLoopGoalObjective(loop),
        buildRoomLoopGoalMetadata(roomMembers, leadAgent, loop),
      );
    },
    [createGoal, requireLeadAgentId, roomMembers],
  );

  return {
    createDisabledReason: resolveCreateDisabledReason(
      roomMembers,
      leadAgentId,
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
): string | null {
  if (roomMembers.length === 0) {
    return "房间还没有可指派的 Agent";
  }
  if (leadAgentId.trim() === "") {
    return "请选择 Room Goal 负责人";
  }
  return null;
}
