// INPUT: 当前 Agent、Skill 列表资源与 exact Agent+Skill 开关意图。
// OUTPUT: 分离的读取/修改失败、逐 Skill 锁和显式新意图恢复动作。
// POS: Agent Options Skill 编排器；unknown 只锁同一意图且绝不自动重放。
import {
  useCallback,
  useDeferredValue,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { setAgentSkillEnabledApi } from "@/lib/api/capability/skill-api";
import { getSkillDisplayDescription } from "@/lib/skill-description";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { AgentSkillEntry } from "@/types/capability/skill";

import {
  buildAgentSkillMutationFailure,
  buildAgentSkillRefreshAfterMutationFailure,
  projectAgentSkills,
  type AgentSkillMutationFailure,
} from "./agent-skills-model";
import { useAgentSkillsResource } from "./use-agent-skills-resource";

interface UseAgentSkillsControllerParams {
  agentId?: string;
  isVisible: boolean;
}

interface SkillCommandToken {
  agentId: string;
  desiredEnabled: boolean;
  skillName: string;
}

function normalizeAgentSkillScope(agentId: string | undefined): string | null {
  return agentId?.trim() || null;
}

function createSkillCommand(
  agentId: string | null,
  skill: AgentSkillEntry,
  activeCommand: SkillCommandToken | null,
  failures: Readonly<Record<string, AgentSkillMutationFailure>>,
): SkillCommandToken | null {
  if (
    !agentId
    || skill.locked
    || activeCommand?.agentId === agentId
    || failures[skill.name]?.blocksRepeat
  ) {
    return null;
  }
  return {
    agentId,
    desiredEnabled: !skill.enabled_for_agent,
    skillName: skill.name,
  };
}

async function mutateAgentSkill(
  command: SkillCommandToken,
  skill: AgentSkillEntry,
): Promise<AgentSkillEntry> {
  const targetScope = skill.storage_scope === "agent_workspace"
    || skill.source_type === "workspace"
    ? "agent_workspace"
    : "global_library";
  return setAgentSkillEnabledApi(
    command.agentId,
    command.skillName,
    command.desiredEnabled,
    targetScope,
  );
}

function finishSkillCommand(
  command: SkillCommandToken,
  activeCommandRef: RefObject<SkillCommandToken | null>,
  mountedRef: RefObject<boolean>,
  setBusyCommand: Dispatch<SetStateAction<SkillCommandToken | null>>,
): void {
  if (activeCommandRef.current === command) {
    activeCommandRef.current = null;
  }
  if (mountedRef.current) {
    setBusyCommand((current) => current === command ? null : current);
  }
}

export function useAgentSkillsController({
  agentId,
  isVisible,
}: UseAgentSkillsControllerParams) {
  const { t } = useI18n();
  const scopeAgentId = normalizeAgentSkillScope(agentId);
  const activeAgentIdRef = useRef(scopeAgentId);
  activeAgentIdRef.current = scopeAgentId;
  const activeCommandRef = useRef<SkillCommandToken | null>(null);
  const mountedRef = useRef(true);
  const [busyCommand, setBusyCommand] = useState<SkillCommandToken | null>(null);
  const [searchQuery, setSearchQuery] = useResettableState("", scopeAgentId);
  const [pendingDisableSkill, setPendingDisableSkill] = useResettableState<
    AgentSkillEntry | null
  >(null, scopeAgentId);
  const [actionFailures, setActionFailures] = useResettableState<Record<
    string,
    AgentSkillMutationFailure
  >>(
    {},
    scopeAgentId,
  );
  const deferredSearchQuery = useDeferredValue(searchQuery);
  const {
    applyCommittedSkill,
    failure: readFailure,
    items,
    loading,
    refresh: refreshResource,
    refreshAfterMutation,
  } = useAgentSkillsResource({
    agentId: scopeAgentId ?? undefined,
    isVisible,
  });
  const projection = useMemo(
    () => projectAgentSkills(
      items,
      deferredSearchQuery,
      (skill) => getSkillDisplayDescription(skill, t),
    ),
    [deferredSearchQuery, items, t],
  );
  const scopedCommand = busyCommand?.agentId === scopeAgentId
    ? busyCommand
    : null;
  const refresh = useCallback(async () => {
    const result = await refreshResource();
    if (result.status === "loaded") {
      setActionFailures({});
    }
    return result;
  }, [refreshResource, setActionFailures]);

  useEffect(() => () => {
    if (activeAgentIdRef.current === scopeAgentId) {
      activeAgentIdRef.current = null;
    }
  }, [scopeAgentId]);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const runSkillToggle = useCallback(async (skill: AgentSkillEntry) => {
    const command = createSkillCommand(
      scopeAgentId,
      skill,
      activeCommandRef.current,
      actionFailures,
    );
    if (!command) {
      return;
    }
    activeCommandRef.current = command;
    setBusyCommand(command);
    setActionFailures((current) => {
      const next = { ...current };
      delete next[command.skillName];
      return next;
    });
    let committedSkill: AgentSkillEntry;
    try {
      committedSkill = await mutateAgentSkill(command, skill);
    } catch (error) {
      if (activeAgentIdRef.current === command.agentId) {
        setActionFailures((current) => ({
          ...current,
          [command.skillName]: buildAgentSkillMutationFailure(
            error,
            command,
            t,
          ),
        }));
      }
      finishSkillCommand(command, activeCommandRef, mountedRef, setBusyCommand);
      return;
    }
    if (activeAgentIdRef.current === command.agentId) {
      applyCommittedSkill(committedSkill);
      const refreshResult = await refreshAfterMutation();
      if (refreshResult.status === "failed") {
        setActionFailures((current) => ({
          ...current,
          [command.skillName]: buildAgentSkillRefreshAfterMutationFailure(
            refreshResult.error,
            command,
            t,
          ),
        }));
      }
    }
    finishSkillCommand(command, activeCommandRef, mountedRef, setBusyCommand);
  }, [
    actionFailures,
    applyCommittedSkill,
    refreshAfterMutation,
    scopeAgentId,
    setActionFailures,
    t,
  ]);

  const requestSkillAction = useCallback((skill: AgentSkillEntry): void => {
    if (skill.enabled_for_agent) {
      setPendingDisableSkill(skill);
      return;
    }
    void runSkillToggle(skill);
  }, [runSkillToggle, setPendingDisableSkill]);

  const confirmDisable = useCallback((): void => {
    if (!pendingDisableSkill) {
      return;
    }
    setPendingDisableSkill(null);
    void runSkillToggle(pendingDisableSkill);
  }, [pendingDisableSkill, runSkillToggle, setPendingDisableSkill]);

  const mutationFailures = Object.values(actionFailures).filter(
    (failure) => failure.target.agentId === scopeAgentId,
  );

  return {
    agentId: scopeAgentId,
    blockedSkillNames: new Set(mutationFailures
      .filter((failure) => failure.blocksRepeat)
      .map((failure) => failure.target.skillName)),
    busySkillName: scopedCommand?.skillName ?? null,
    cancelDisable: () => setPendingDisableSkill(null),
    commandBusy: scopedCommand !== null,
    confirmDisable,
    loading,
    mutationFailures,
    pendingDisableSkill,
    projection,
    readFailure,
    refresh,
    requestSkillAction,
    searchQuery,
    setSearchQuery,
  };
}
