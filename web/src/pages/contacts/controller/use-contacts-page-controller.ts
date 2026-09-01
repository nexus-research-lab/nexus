// INPUT: Agent 目录、编辑命令与删除响应中的结构化提交证据。
// OUTPUT: 联系人页资源、编辑状态，以及删除、对账和安全重试状态。
// POS: Contacts 页面业务控制器；删除结果未知时必须先刷新权威目录，不得重复提交。
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { isMainAgent } from "@/config/runtime-options";
import { useExistingAgentOptionsCommands } from "@/features/agents/options/use-existing-agent-options-commands";
import { projectMutationFailure } from "@/lib/error-message";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
} from "@/shared/auth/auth-owner-generation";
import { useAgentStore } from "@/store/agent";

import { useContactAgentEditor } from "./use-contact-agent-editor";

interface PendingDeleteAgent {
  id: string;
  name: string;
}

export interface AgentDeletionFailure {
  directoryCheck: "failed" | "not_checked" | "target_present";
  kind:
    | "committed_cleanup_incomplete"
    | "not_applied"
    | "outcome_unknown"
    | "resource_absent";
}
export type AgentDeletionAction = "check" | "delete" | null;

export function useContactsPageController() {
  const agents = useAgentStore((state) => state.agents);
  const createAgent = useAgentStore((state) => state.create_agent);
  const updateAgent = useAgentStore((state) => state.update_agent);
  const deleteAgent = useAgentStore((state) => state.delete_agent);
  const loadAgents = useAgentStore((state) => state.load_agents_from_server);
  const reconcileAgents = useAgentStore((state) => state.reconcile_agents_from_server);
  const loading = useAgentStore((state) => state.loading);
  const contactAgents = useMemo(
    () => agents.filter((agent) => !isMainAgent(agent.agent_id)),
    [agents],
  );
  const agentOptions = useExistingAgentOptionsCommands({updateAgent});
  const editor = useContactAgentEditor({
    createAgent,
    validateAgentName: agentOptions.validateAgentName,
  });
  const closeEditor = editor.close;
  const [pendingDeleteAgent, setPendingDeleteAgent] = useState<PendingDeleteAgent | null>(null);
  const [deleteAction, setDeleteAction] = useState<AgentDeletionAction>(null);
  const [deleteFailure, setDeleteFailure] = useState<AgentDeletionFailure | null>(null);
  const deletionRunningRef = useRef(false);
  const unresolvedDeletionsRef = useRef(new Map<string, {
    failure: AgentDeletionFailure;
    ownerGeneration: number;
  }>());

  useEffect(() => {
    void loadAgents();
  }, [loadAgents]);

  const requestDeleteAgent = useCallback((agentId: string) => {
    if (deletionRunningRef.current) {
      return;
    }
    const targetAgent = contactAgents.find((agent) => agent.agent_id === agentId);
    if (!targetAgent) {
      return;
    }
    closeEditor();
    const unresolved = unresolvedDeletionsRef.current.get(agentId);
    setDeleteFailure(
      unresolved?.ownerGeneration === captureAuthOwnerScopeGeneration()
        ? unresolved.failure
        : null,
    );
    setPendingDeleteAgent({id: agentId, name: targetAgent.name});
  }, [closeEditor, contactAgents]);

  const confirmDeleteAgent = useCallback(async (): Promise<string | null> => {
    if (!pendingDeleteAgent || deletionRunningRef.current) {
      return null;
    }
    deletionRunningRef.current = true;
    const ownerGeneration = captureAuthOwnerScopeGeneration();
    const target = pendingDeleteAgent;
    try {
      if (deleteFailure && deleteFailure.kind !== "not_applied") {
        setDeleteAction("check");
        const refreshed = await reconcileAgents();
        if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
          return null;
        }
        if (!refreshed) {
          const next = { ...deleteFailure, directoryCheck: "failed" } as const;
          unresolvedDeletionsRef.current.set(target.id, {
            failure: next,
            ownerGeneration,
          });
          setDeleteFailure(next);
          return null;
        }
        const targetStillExists = useAgentStore.getState().agents.some(
          (agent) => agent.agent_id === target.id,
        );
        if (targetStillExists) {
          // 一次读取只能证明“此刻仍存在”，不能证明响应丢失的原请求
          // 已经终止；继续保留原结果事实，避免把 unknown 误降为可重试。
          const next = { ...deleteFailure, directoryCheck: "target_present" } as const;
          unresolvedDeletionsRef.current.set(target.id, {
            failure: next,
            ownerGeneration,
          });
          setDeleteFailure(next);
          return null;
        }
        unresolvedDeletionsRef.current.delete(target.id);
        setDeleteFailure(null);
        setPendingDeleteAgent(null);
        return target.id;
      }

      setDeleteAction("delete");
      setDeleteFailure(null);
      try {
        await deleteAgent(target.id);
        if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
          return null;
        }
        unresolvedDeletionsRef.current.delete(target.id);
        setPendingDeleteAgent(null);
        return target.id;
      } catch (error) {
        if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
          return null;
        }
        const failure = projectMutationFailure(error, "成员删除结果无法确认");
        const kind: AgentDeletionFailure["kind"] = failure.code === "agent.not_found"
          ? "resource_absent"
          : failure.effect === "committed"
            ? "committed_cleanup_incomplete"
            : failure.effect === "not_applied"
              ? "not_applied"
              : "outcome_unknown";
        const next = { directoryCheck: "not_checked", kind } as const;
        if (kind === "not_applied") {
          unresolvedDeletionsRef.current.delete(target.id);
        } else {
          unresolvedDeletionsRef.current.set(target.id, {
            failure: next,
            ownerGeneration,
          });
        }
        setDeleteFailure(next);
        return null;
      }
    } finally {
      deletionRunningRef.current = false;
      setDeleteAction(null);
    }
  }, [deleteAgent, deleteFailure, pendingDeleteAgent, reconcileAgents]);
  const cancelDeleteAgent = useCallback(() => {
    if (deletionRunningRef.current) {
      return;
    }
    setDeleteFailure(null);
    setPendingDeleteAgent(null);
  }, []);

  return {
    contactAgents,
    loading,
    editor,
    deleteAction,
    deleteFailure,
    pendingDeleteAgent,
    requestDeleteAgent,
    cancelDeleteAgent,
    confirmDeleteAgent,
    saveAgentOptions: agentOptions.saveAgentOptions,
    validateAgentName: agentOptions.validateAgentName,
  };
}
