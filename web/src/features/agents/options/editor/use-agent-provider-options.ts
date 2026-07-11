import { useEffect, useRef, useState } from "react";

import { getDefaultAgentRuntimeKind } from "@/config/runtime-options";
import { listProviderOptionsApi } from "@/lib/api/settings/provider-api";
import type { AgentProvider } from "@/types/agent/agent";
import type { ProviderOption } from "@/types/capability/provider";

import { normalizeAgentOptionProvider } from "@/lib/agent-options";

interface ProviderOptionsState {
  defaultModel: string;
  defaultProvider: AgentProvider;
  error: string | null;
  items: ProviderOption[];
  loading: boolean;
  runtimeKind: string;
}

export function useAgentProviderOptions(isActive: boolean, fallbackError: string) {
  const runtimeKind = getDefaultAgentRuntimeKind();
  const requestSequenceRef = useRef(0);
  const [storedState, setStoredState] = useState<ProviderOptionsState>(() =>
    createProviderOptionsState(runtimeKind),
  );
  const state = storedState.runtimeKind === runtimeKind
    ? storedState
    : createProviderOptionsState(runtimeKind);

  useEffect(() => {
    if (!isActive) {
      return undefined;
    }
    const requestSequence = requestSequenceRef.current + 1;
    requestSequenceRef.current = requestSequence;
    setStoredState((current) => ({
      ...(current.runtimeKind === runtimeKind
        ? current
        : createProviderOptionsState(runtimeKind)),
      error: null,
      loading: true,
    }));

    void listProviderOptionsApi(runtimeKind)
      .then((payload) => {
        if (requestSequenceRef.current !== requestSequence) {
          return;
        }
        setStoredState({
          defaultModel: payload.default_model?.trim() || "",
          defaultProvider: normalizeAgentOptionProvider(payload.default_provider),
          error: null,
          items: payload.items,
          loading: false,
          runtimeKind,
        });
      })
      .catch((error: unknown) => {
        if (requestSequenceRef.current !== requestSequence) {
          return;
        }
        setStoredState((current) => ({
          ...(current.runtimeKind === runtimeKind
            ? current
            : createProviderOptionsState(runtimeKind)),
          error: error instanceof Error ? error.message : fallbackError,
          loading: false,
        }));
      });
    return () => {
      requestSequenceRef.current += 1;
    };
  }, [fallbackError, isActive, runtimeKind]);

  return isActive ? state : { ...state, loading: false };
}

function createProviderOptionsState(runtimeKind: string): ProviderOptionsState {
  return {
    defaultModel: "",
    defaultProvider: "",
    error: null,
    items: [],
    loading: false,
    runtimeKind,
  };
}
