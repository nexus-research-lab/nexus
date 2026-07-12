import {
  type Dispatch,
  type RefObject,
  type SetStateAction,
  useCallback,
  useDeferredValue,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { installSkillApi, uninstallSkillApi } from "@/lib/api/capability/skill-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { AgentSkillEntry } from "@/types/capability/skill";

import { projectAgentSkills } from "./agent-skills-model";
import { useAgentSkillsResource } from "./use-agent-skills-resource";

interface UseAgentSkillsControllerParams {
  agentId?: string;
  isVisible: boolean;
}

interface SkillCommandToken {
  agentId: string;
  skillName: string;
}

interface SkillCommandExecution {
  activeAgentIdRef: RefObject<string | null>;
  activeCommandRef: RefObject<SkillCommandToken | null>;
  command: SkillCommandToken;
  fallbackErrorMessage: string;
  refresh: () => Promise<void>;
  setActionError: Dispatch<SetStateAction<string | null>>;
  setBusyCommand: Dispatch<SetStateAction<SkillCommandToken | null>>;
  skill: AgentSkillEntry;
}

function createSkillCommand(
  agentId: string | null,
  skill: AgentSkillEntry,
  activeCommand: SkillCommandToken | null,
): SkillCommandToken | null {
  if (!agentId || skill.locked || activeCommand?.agentId === agentId) {
    return null;
  }
  return { agentId, skillName: skill.name };
}

async function mutateAgentSkill(
  command: SkillCommandToken,
  skill: AgentSkillEntry,
): Promise<void> {
  const mutate = skill.installed ? uninstallSkillApi : installSkillApi;
  await mutate(command.agentId, command.skillName);
}

// 命令结果只写回发起时的 Agent 作用域，切换 Agent 后旧结果直接失效。
async function executeSkillCommand({
  activeAgentIdRef,
  activeCommandRef,
  command,
  fallbackErrorMessage,
  refresh,
  setActionError,
  setBusyCommand,
  skill,
}: SkillCommandExecution): Promise<void> {
  activeCommandRef.current = command;
  setBusyCommand(command);
  setActionError(null);
  try {
    await mutateAgentSkill(command, skill);
    if (activeAgentIdRef.current === command.agentId) {
      await refresh();
    }
  } catch (error) {
    if (activeAgentIdRef.current === command.agentId) {
      setActionError(
        error instanceof Error ? error.message : fallbackErrorMessage,
      );
    }
  } finally {
    if (activeCommandRef.current === command) {
      activeCommandRef.current = null;
    }
    if (activeAgentIdRef.current === command.agentId) {
      setBusyCommand((current) => current === command ? null : current);
    }
  }
}

export function useAgentSkillsController({
  agentId,
  isVisible,
}: UseAgentSkillsControllerParams) {
  const { t } = useI18n();
  const scopeAgentId = agentId?.trim() || null;
  const activeAgentIdRef = useRef(scopeAgentId);
  activeAgentIdRef.current = scopeAgentId;
  const activeCommandRef = useRef<SkillCommandToken | null>(null);
  const [busyCommand, setBusyCommand] = useState<SkillCommandToken | null>(null);
  const [searchQuery, setSearchQuery] = useResettableState("", scopeAgentId);
  const [pendingRemoveSkill, setPendingRemoveSkill] = useResettableState<
    AgentSkillEntry | null
  >(null, scopeAgentId);
  const [actionError, setActionError] = useResettableState<string | null>(
    null,
    scopeAgentId,
  );
  const deferredSearchQuery = useDeferredValue(searchQuery);
  const {
    error: resourceError,
    items,
    loading,
    refresh: refreshResource,
  } = useAgentSkillsResource({
    agentId: scopeAgentId ?? undefined,
    fallbackErrorMessage: t("agent_options.skills.load_failed"),
    isVisible,
  });
  const projection = useMemo(
    () => projectAgentSkills(items, deferredSearchQuery),
    [deferredSearchQuery, items],
  );

  useEffect(() => () => {
    if (activeAgentIdRef.current === scopeAgentId) {
      activeAgentIdRef.current = null;
    }
  }, [scopeAgentId]);

  const runSkillToggle = useCallback(async (skill: AgentSkillEntry) => {
    const command = createSkillCommand(
      scopeAgentId,
      skill,
      activeCommandRef.current,
    );
    if (!command) {
      return;
    }
    await executeSkillCommand({
      activeAgentIdRef,
      activeCommandRef,
      command,
      fallbackErrorMessage: t("agent_options.skills.toggle_failed"),
      refresh: refreshResource,
      setActionError,
      setBusyCommand,
      skill,
    });
  }, [refreshResource, scopeAgentId, setActionError, t]);

  const requestSkillAction = useCallback((skill: AgentSkillEntry): void => {
    if (skill.installed) {
      setPendingRemoveSkill(skill);
      return;
    }
    void runSkillToggle(skill);
  }, [runSkillToggle, setPendingRemoveSkill]);

  const confirmRemove = useCallback((): void => {
    if (!pendingRemoveSkill) {
      return;
    }
    setPendingRemoveSkill(null);
    void runSkillToggle(pendingRemoveSkill);
  }, [pendingRemoveSkill, runSkillToggle, setPendingRemoveSkill]);

  const refresh = useCallback((): void => {
    setActionError(null);
    void refreshResource();
  }, [refreshResource, setActionError]);

  return {
    agentId: scopeAgentId,
    busySkillName: busyCommand?.agentId === scopeAgentId
      ? busyCommand.skillName
      : null,
    cancelRemove: () => setPendingRemoveSkill(null),
    commandBusy: busyCommand?.agentId === scopeAgentId,
    confirmRemove,
    errorMessage: actionError ?? resourceError,
    loading,
    pendingRemoveSkill,
    projection,
    refresh,
    requestSkillAction,
    searchQuery,
    setSearchQuery,
  };
}
