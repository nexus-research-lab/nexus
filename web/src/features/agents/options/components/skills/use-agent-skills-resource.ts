import { useCallback, useEffect, useRef, useState } from "react";

import { getAgentSkillsApi } from "@/lib/api/capability/skill-api";
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

  const refresh = useCallback(async (
    mode: AgentSkillsRefreshMode = "foreground",
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
    setStoredState((current) => {
      const scoped = current.agentId === scopeAgentId
        ? current
        : createResourceState(scopeAgentId);
      return {
        ...scoped,
        error: null,
        loading: mode === "background" ? scoped.loading : true,
      };
    });

    try {
      const items = await getAgentSkillsApi(scopeAgentId, controller.signal);
      if (requestSequenceRef.current !== requestSequence) {
        return;
      }
      setStoredState({
        agentId: scopeAgentId,
        error: null,
        items,
        loading: false,
      });
    } catch (error) {
      if (controller.signal.aborted || requestSequenceRef.current !== requestSequence) {
        return;
      }
      setStoredState((current) => ({
        ...(current.agentId === scopeAgentId
          ? current
          : createResourceState(scopeAgentId)),
        error: error instanceof Error ? error.message : fallbackErrorMessage,
        loading: false,
      }));
    }
  }, [fallbackErrorMessage, scopeAgentId]);

  useEffect(() => {
    if (!isVisible) {
      return undefined;
    }
    void refresh();

    const refreshIfVisible = (): void => {
      if (!document.hidden) {
        void refresh("background");
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
  }, [isVisible, refresh]);

  return {
    error: state.error,
    items: state.items,
    loading: isVisible ? state.loading : false,
    refresh,
  };
}
