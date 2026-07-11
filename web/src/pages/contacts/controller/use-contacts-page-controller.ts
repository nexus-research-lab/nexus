import { useCallback, useEffect, useMemo, useState } from "react";

import { isMainAgent } from "@/config/options";
import { useExistingAgentOptionsCommands } from "@/features/agents/options/use-existing-agent-options-commands";
import { useAgentStore } from "@/store/agent";

import { useContactAgentEditor } from "./use-contact-agent-editor";

interface PendingDeleteAgent {
  id: string;
  name: string;
}

export function useContactsPageController() {
  const agents = useAgentStore((state) => state.agents);
  const createAgent = useAgentStore((state) => state.create_agent);
  const updateAgent = useAgentStore((state) => state.update_agent);
  const deleteAgent = useAgentStore((state) => state.delete_agent);
  const loadAgents = useAgentStore((state) => state.load_agents_from_server);
  const loading = useAgentStore((state) => state.loading);
  const contactAgents = useMemo(
    () => agents.filter((agent) => !isMainAgent(agent.agent_id)),
    [agents],
  );
  const agentOptions = useExistingAgentOptionsCommands({updateAgent});
  const editor = useContactAgentEditor({
    agents: contactAgents,
    createAgent,
    saveAgentOptions: agentOptions.saveAgentOptions,
    validateAgentName: agentOptions.validateAgentName,
  });
  const closeEditor = editor.close;
  const [pendingDeleteAgent, setPendingDeleteAgent] = useState<PendingDeleteAgent | null>(null);

  useEffect(() => {
    void loadAgents();
  }, [loadAgents]);

  const requestDeleteAgent = useCallback((agentId: string) => {
    const targetAgent = contactAgents.find((agent) => agent.agent_id === agentId);
    if (!targetAgent) {
      return;
    }
    closeEditor();
    setPendingDeleteAgent({id: agentId, name: targetAgent.name});
  }, [closeEditor, contactAgents]);

  const confirmDeleteAgent = useCallback(async (): Promise<string | null> => {
    if (!pendingDeleteAgent) {
      return null;
    }
    await deleteAgent(pendingDeleteAgent.id);
    setPendingDeleteAgent(null);
    return pendingDeleteAgent.id;
  }, [deleteAgent, pendingDeleteAgent]);
  const cancelDeleteAgent = useCallback(() => setPendingDeleteAgent(null), []);

  return {
    contactAgents,
    loading,
    editor,
    pendingDeleteAgent,
    requestDeleteAgent,
    cancelDeleteAgent,
    confirmDeleteAgent,
    saveAgentOptions: agentOptions.saveAgentOptions,
    validateAgentName: agentOptions.validateAgentName,
  };
}
