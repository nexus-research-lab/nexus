// INPUT: 当前 Agent scope、页面可见性与前台/后台读取意图。
// OUTPUT: 保留旧快照的 Skill 列表资源、结构化读取失败和可核对刷新结果。
// POS: Agent Options Skill 读取边界；不解释或清理任何 mutation unknown。
import { useCallback, useEffect, useRef, useState } from "react";

import { getAgentSkillsApi } from "@/lib/api/capability/skill-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { AgentSkillEntry } from "@/types/capability/skill";

import {
  buildAgentSkillsReadFailure,
  type AgentSkillsReadFailure,
} from "./agent-skills-model";

interface AgentSkillsResourceState {
  agentId: string | null;
  failure: AgentSkillsReadFailure | null;
  items: AgentSkillEntry[];
  loading: boolean;
}

interface UseAgentSkillsResourceParams {
  agentId?: string;
  isVisible: boolean;
}

type AgentSkillsRefreshMode = "background" | "foreground";

export type AgentSkillsRefreshResult =
  | { status: "loaded" }
  | { error: unknown; status: "failed" }
  | { status: "superseded" };

function createResourceState(agentId: string | null): AgentSkillsResourceState {
  return { agentId, failure: null, items: [], loading: false };
}

function getScopedResourceState(
  state: AgentSkillsResourceState,
  agentId: string,
): AgentSkillsResourceState {
  return state.agentId === agentId ? state : createResourceState(agentId);
}

function createLoadingState(
  state: AgentSkillsResourceState,
  agentId: string,
  mode: AgentSkillsRefreshMode,
): AgentSkillsResourceState {
  const scoped = getScopedResourceState(state, agentId);
  return {
    ...scoped,
    loading: mode === "foreground" || scoped.loading,
  };
}

function isStaleRequest(
  activeSequence: number,
  requestSequence: number,
  signal: AbortSignal,
): boolean {
  return signal.aborted || activeSequence !== requestSequence;
}

export function useAgentSkillsResource({
  agentId,
  isVisible,
}: UseAgentSkillsResourceParams) {
  const { t } = useI18n();
  const scopeAgentId = agentId?.trim() || null;
  const requestSequenceRef = useRef(0);
  const requestControllerRef = useRef<AbortController | null>(null);
  const [storedState, setStoredState] = useState<AgentSkillsResourceState>(
    () => createResourceState(scopeAgentId),
  );
  const state = storedState.agentId === scopeAgentId
    ? storedState
    : createResourceState(scopeAgentId);

  const runRefresh = useCallback(async (
    mode: AgentSkillsRefreshMode,
    reportFailure = true,
  ): Promise<AgentSkillsRefreshResult> => {
    requestControllerRef.current?.abort();
    const requestSequence = requestSequenceRef.current + 1;
    requestSequenceRef.current = requestSequence;

    if (!scopeAgentId) {
      setStoredState(createResourceState(null));
      return { status: "loaded" };
    }

    const controller = new AbortController();
    requestControllerRef.current = controller;
    setStoredState((current) => createLoadingState(current, scopeAgentId, mode));

    try {
      const items = await getAgentSkillsApi(scopeAgentId, controller.signal);
      if (isStaleRequest(
        requestSequenceRef.current,
        requestSequence,
        controller.signal,
      )) {
        return { status: "superseded" };
      }
      setStoredState({
        agentId: scopeAgentId,
        failure: null,
        items,
        loading: false,
      });
      return { status: "loaded" };
    } catch (error) {
      if (isStaleRequest(
        requestSequenceRef.current,
        requestSequence,
        controller.signal,
      )) {
        return { status: "superseded" };
      }
      setStoredState((current) => ({
        ...getScopedResourceState(current, scopeAgentId),
        failure: reportFailure
          ? buildAgentSkillsReadFailure(error, t)
          : null,
        loading: false,
      }));
      return { error, status: "failed" };
    }
  }, [scopeAgentId, t]);

  const refresh = useCallback(
    () => runRefresh("foreground"),
    [runRefresh],
  );
  const refreshAfterMutation = useCallback(
    () => runRefresh("background", false),
    [runRefresh],
  );
  const applyCommittedSkill = useCallback((skill: AgentSkillEntry): void => {
    if (!scopeAgentId) {
      return;
    }
    setStoredState((current) => {
      const scoped = getScopedResourceState(current, scopeAgentId);
      const existingIndex = scoped.items.findIndex((item) => item.name === skill.name);
      const items = existingIndex < 0
        ? [...scoped.items, skill]
        : scoped.items.map((item, index) => index === existingIndex ? skill : item);
      return { ...scoped, items };
    });
  }, [scopeAgentId]);

  useEffect(() => {
    if (!isVisible) {
      return undefined;
    }
    void runRefresh("foreground");

    const refreshIfVisible = (): void => {
      if (!document.hidden) {
        void runRefresh("background");
      }
    };
    window.addEventListener("focus", refreshIfVisible);
    document.addEventListener("visibilitychange", refreshIfVisible);
    return () => {
      requestSequenceRef.current += 1;
      requestControllerRef.current?.abort();
      window.removeEventListener("focus", refreshIfVisible);
      document.removeEventListener("visibilitychange", refreshIfVisible);
    };
  }, [isVisible, runRefresh]);

  return {
    applyCommittedSkill,
    failure: state.failure,
    items: state.items,
    loading: isVisible ? state.loading : false,
    refresh,
    refreshAfterMutation,
  };
}
