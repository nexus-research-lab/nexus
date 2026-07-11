import { useCallback, useEffect, useMemo, useRef } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useWorkspaceFilesStore } from "@/store/workspace-files";

interface UseWorkspaceAgentScopeOptions {
  agentId: string;
  isDm: boolean;
  onOpenWorkspaceFile: (path: string | null) => void;
}

function normalizeAgentId(agentId?: string | null): string | null {
  const normalized = agentId?.trim();
  return normalized || null;
}

export function useWorkspaceAgentScope({
  agentId,
  isDm,
  onOpenWorkspaceFile,
}: UseWorkspaceAgentScopeOptions) {
  const [selectedAgentId, setSelectedAgentId] = useResettableState(agentId, agentId);
  const requestedOpenAgentId = useWorkspaceFilesStore((state) => state.requested_open_agent_id);
  const requestOpenAgent = useWorkspaceFilesStore((state) => state.request_open_agent);
  const previousConversationScopeRef = useRef({agentId, isDm});

  const requestedAgentId = useMemo(
    () => normalizeAgentId(requestedOpenAgentId),
    [requestedOpenAgentId],
  );
  const pendingOpenAgentId = isDm ? null : requestedAgentId;
  const viewAgentId = isDm ? agentId : (pendingOpenAgentId ?? selectedAgentId);

  // 外部文件请求已经设置好打开路径，这里只消费 Agent 切换信号。
  useEffect(() => {
    if (!requestedAgentId) {
      return;
    }
    requestOpenAgent(null);
    if (!isDm && requestedAgentId !== selectedAgentId) {
      setSelectedAgentId(requestedAgentId);
    }
  }, [isDm, requestOpenAgent, requestedAgentId, selectedAgentId, setSelectedAgentId]);

  useEffect(() => {
    const previousScope = previousConversationScopeRef.current;
    previousConversationScopeRef.current = {agentId, isDm};
    if (previousScope.agentId !== agentId || previousScope.isDm !== isDm) {
      onOpenWorkspaceFile(null);
    }
  }, [agentId, isDm, onOpenWorkspaceFile]);

  const selectAgent = useCallback((nextAgentId: string) => {
    if (isDm || nextAgentId === selectedAgentId) {
      return;
    }
    setSelectedAgentId(nextAgentId);
    onOpenWorkspaceFile(null);
  }, [isDm, onOpenWorkspaceFile, selectedAgentId, setSelectedAgentId]);

  return {
    selectedAgentId,
    selectAgent,
    viewAgentId,
  };
}
