import { useCallback, useEffect, useRef, useState } from "react";

import { getAgentSkillsApi } from "@/lib/api/capability/skill-api";
import { getErrorMessage } from "@/lib/error-message";
import type { AgentSkillEntry } from "@/types/capability/skill";

const SKILL_REFRESH_INTERVAL_MS = 5000;

interface AgentSkillsResourceState {
  agentId: string | null;
  error: string | null;
  items: AgentSkillEntry[];
  loading: boolean;
}

interface UseAgentSkillsResourceParams {
  agentId?: string;
  fallbackErrorMessage: string;
  isVisible: boolean;
}

type AgentSkillsRefreshMode = "background" | "foreground";

function createResourceState(agentId: string | null): AgentSkillsResourceState {
  return { agentId, error: null, items: [], loading: false };
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
    error: null,
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
  fallbackErrorMessage,
  isVisible,
}: UseAgentSkillsResourceParams) {
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
  ): Promise<void> => {
    requestControllerRef.current?.abort();
    const requestSequence = requestSequenceRef.current + 1;
    requestSequenceRef.current = requestSequence;

    if (!scopeAgentId) {
      setStoredState(createResourceState(null));
      return;
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
        return;
      }
      setStoredState({
        agentId: scopeAgentId,
        error: null,
        items,
        loading: false,
      });
    } catch (error) {
      if (isStaleRequest(
        requestSequenceRef.current,
        requestSequence,
        controller.signal,
      )) {
        return;
      }
      setStoredState((current) => ({
        ...getScopedResourceState(current, scopeAgentId),
        error: getErrorMessage(error, fallbackErrorMessage),
        loading: false,
      }));
    }
  }, [fallbackErrorMessage, scopeAgentId]);

  const refresh = useCallback(
    () => runRefresh("foreground"),
    [runRefresh],
  );

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
    const intervalId = window.setInterval(
      refreshIfVisible,
      SKILL_REFRESH_INTERVAL_MS,
    );
    window.addEventListener("focus", refreshIfVisible);
    document.addEventListener("visibilitychange", refreshIfVisible);
    return () => {
      requestSequenceRef.current += 1;
      requestControllerRef.current?.abort();
      window.clearInterval(intervalId);
      window.removeEventListener("focus", refreshIfVisible);
      document.removeEventListener("visibilitychange", refreshIfVisible);
    };
  }, [isVisible, runRefresh]);

  return {
    error: state.error,
    items: state.items,
    loading: isVisible ? state.loading : false,
    refresh,
  };
}
