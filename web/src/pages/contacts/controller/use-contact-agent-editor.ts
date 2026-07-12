import { useCallback, useMemo, useState } from "react";

import { getInitialAgentOptions } from "@/config/runtime-options";
import { buildAgentMutationParams } from "@/features/agents/options/agent-options-mutation";
import {
  buildAgentOptionsCreateSource,
  buildAgentOptionsEditSource,
} from "@/features/agents/options/agent-options-editor-model";
import type { AgentOptionsDialogState } from "@/features/agents/options/dialog/agent-options-dialog-model";
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
  | {agent: Agent; kind: "edit"};

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
  const dialogState = useMemo(
    () => buildContactAgentDialogState(state),
    [state],
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
    await saveAgentOptions(state.agent.agent_id, title, options, identity);
  }, [createAgent, saveAgentOptions, state]);

  const validateName = useCallback((name: string) => (
    validateAgentName(
      name,
      state.kind === "edit" ? state.agent.agent_id : undefined,
    )
  ), [state, validateAgentName]);
  const openCreate = useCallback(() => setState({kind: "create"}), []);
  const openEdit = useCallback((agentId: string) => {
    const agent = agents.find((candidate) => candidate.agent_id === agentId);
    if (agent) {
      setState({agent, kind: "edit"});
    }
  }, [agents]);
  const close = useCallback(() => setState({kind: "closed"}), []);

  return {
    dialogState,
    openCreate,
    openEdit,
    close,
    save,
    validateName,
  };
}

function buildContactAgentDialogState(
  state: ContactAgentEditorState,
): AgentOptionsDialogState {
  switch (state.kind) {
    case "closed":
      return {kind: "closed"};
    case "create":
      return buildAgentOptionsCreateSource(getInitialAgentOptions());
    case "edit":
      return buildAgentOptionsEditSource(state.agent);
  }
}
