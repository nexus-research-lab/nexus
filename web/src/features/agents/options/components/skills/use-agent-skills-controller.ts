import {
  useCallback,
  useDeferredValue,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { installSkillApi, uninstallSkillApi } from "@/lib/api/skill-api";
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
    if (!scopeAgentId || skill.locked) {
      return;
    }
    if (activeCommandRef.current?.agentId === scopeAgentId) {
      return;
    }

    const command = { agentId: scopeAgentId, skillName: skill.name };
    activeCommandRef.current = command;
    setBusyCommand(command);
    setActionError(null);
    try {
      if (skill.installed) {
        await uninstallSkillApi(scopeAgentId, skill.name);
      } else {
        await installSkillApi(scopeAgentId, skill.name);
      }
      if (activeAgentIdRef.current === scopeAgentId) {
        await refreshResource();
      }
    } catch (error) {
      if (activeAgentIdRef.current === scopeAgentId) {
        setActionError(
          error instanceof Error
            ? error.message
            : t("agent_options.skills.toggle_failed"),
        );
      }
    } finally {
      if (activeCommandRef.current === command) {
        activeCommandRef.current = null;
      }
      if (activeAgentIdRef.current === scopeAgentId) {
        setBusyCommand((current) => current === command ? null : current);
      }
    }
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
