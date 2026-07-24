import { useCallback, useMemo, useState } from "react";

import { getInitialAgentOptions } from "@/config/runtime-options";
import { buildAgentMutationParams } from "@/features/agents/options/agent-options-mutation";
import { buildAgentOptionsCreateSource } from "@/features/agents/options/agent-options-editor-model";
import type { AgentOptionsDialogState } from "@/features/agents/options/dialog/agent-options-dialog-model";
import type {
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions,
  CreateAgentParams,
} from "@/types/agent/agent";

type ContactAgentEditorState =
  | {kind: "closed"}
  | {kind: "create"};

interface UseContactAgentEditorOptions {
  createAgent: (params: CreateAgentParams) => Promise<string>;
  validateAgentName: (name: string) => Promise<AgentNameValidationResult>;
}

export function useContactAgentEditor({
  createAgent,
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
    if (state.kind !== "create") {
      return;
    }
    await createAgent(buildAgentMutationParams(title, options, identity));
  }, [createAgent, state]);

  const validateName = useCallback(
    (name: string) => validateAgentName(name),
    [validateAgentName],
  );
  const openCreate = useCallback(() => setState({kind: "create"}), []);
  const close = useCallback(() => setState({kind: "closed"}), []);

  return {
    dialogState,
    openCreate,
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
  }
}
