import { useCallback, useMemo, useState } from "react";

import { getInitialAgentOptions } from "@/config/runtime-options";
import { pickAgentEditableOptions } from "@/lib/agent-options";
import { buildAgentMutationParams } from "@/features/agents/options/agent-options-mutation";
import type {
  Agent,
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions,
  CreateAgentParams,
} from "@/types/agent/agent";

type ContactAgentEditorState =
  | {kind: "closed"}
  | {kind: "create"}
  | {kind: "edit"; agentId: string};

interface UseContactAgentEditorOptions {
  agents: Agent[];
  createAgent: (params: CreateAgentParams) => Promise<string>;
  saveAgentOptions: (
    agentId: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => Promise<void>;
  validateAgentName: (
    name: string,
    excludeAgentId?: string,
  ) => Promise<AgentNameValidationResult>;
}

export function useContactAgentEditor({
  agents,
  createAgent,
  saveAgentOptions,
  validateAgentName,
}: UseContactAgentEditorOptions) {
  const [state, setState] = useState<ContactAgentEditorState>({kind: "closed"});
  const editingAgent = state.kind === "edit"
    ? agents.find((agent) => agent.agent_id === state.agentId) ?? null
    : null;
  const mode: "create" | "edit" = state.kind === "edit" ? "edit" : "create";
  const initialOptions = useMemo(
    () => editingAgent
      ? pickAgentEditableOptions(editingAgent.options)
      : getInitialAgentOptions(),
    [editingAgent],
  );

  const save = useCallback(async (
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => {
    if (state.kind === "closed") {
      return;
    }
    if (state.kind === "create") {
      await createAgent(buildAgentMutationParams(title, options, identity));
      return;
    }
    await saveAgentOptions(state.agentId, title, options, identity);
  }, [createAgent, saveAgentOptions, state]);

  const validateName = useCallback((name: string) => (
    validateAgentName(
      name,
      state.kind === "edit" ? state.agentId : undefined,
    )
  ), [state, validateAgentName]);
  const openCreate = useCallback(() => setState({kind: "create"}), []);
  const openEdit = useCallback(
    (agentId: string) => setState({kind: "edit", agentId}),
    [],
  );
  const close = useCallback(() => setState({kind: "closed"}), []);

  return {
    isOpen: state.kind !== "closed",
    mode,
    editingAgent,
    initialOptions,
    openCreate,
    openEdit,
    close,
    save,
    validateName,
  };
}
